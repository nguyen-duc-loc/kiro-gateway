// Package credentials captures a reference to one saved Kiro credential snapshot.
package credentials

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/jsonobject"
	"kiro-gateway/internal/safepath"
)

const selectedKey = "kirocli:odic:token"

// Capture errors are fixed categories and contain no underlying database text.
var (
	ErrSource   = errors.New("saved IAM Identity Center source is unavailable or unsupported; check Kiro CLI sign in and local file access")
	ErrRecord   = errors.New("saved IAM Identity Center record is invalid or unsupported; sign in through Kiro CLI, then link again")
	ErrExpired  = errors.New("saved IAM Identity Center credential has expired; sign in through Kiro CLI, then link again")
	ErrBusy     = errors.New("saved IAM Identity Center source is busy; wait for Kiro CLI, then retry")
	ErrTimeout  = errors.New("session capture timed out; retry when the local source is available")
	ErrCanceled = errors.New("session capture canceled; retry when ready")
)

// Reader reads only the fixed source in Home. Now is an injectable wall clock.
// Each capture owns its connection and transaction, with no shared stream state.
type Reader struct {
	Home string
	Now  func() time.Time
	// Test hook observes the boundary between the size query and value query.
	afterMetadata func()
}

// Capture returns only a reference. Tokens and record fields remain local.
func (r Reader) Capture(parent context.Context) (reference config.Session, result error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	defer func() {
		if ctx.Err() != nil {
			result = sanitize(ctx, ctx.Err())
			reference = config.Session{}
		}
	}()
	if ctx.Err() != nil {
		return config.Session{}, sanitize(ctx, ctx.Err())
	}
	path, err := sourcePath(r.Home)
	if err != nil {
		return config.Session{}, ErrSource
	}
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"mode": {"ro"}, "_query_only": {"1"}, "_busy_timeout": {"1000"}}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return config.Session{}, sanitize(ctx, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return config.Session{}, sanitize(ctx, err)
	}
	defer tx.Rollback()
	if err := checkSchema(ctx, tx); err != nil {
		return config.Session{}, sanitize(ctx, err)
	}
	value, err := readValue(ctx, tx, r.afterMetadata)
	if err != nil {
		return config.Session{}, sanitize(ctx, err)
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	if err := validateRecord(value, now()); err != nil {
		return config.Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return config.Session{}, sanitize(ctx, err)
	}
	h := sha256.New()
	h.Write([]byte("kiro-gateway/" + config.Source + "\x00"))
	h.Write(value)
	return config.Session{Source: config.Source, Fingerprint: hex.EncodeToString(h.Sum(nil))}, nil
}

func sourcePath(home string) (string, error) {
	r, err := safepath.Home(home)
	if err != nil {
		return "", err
	}
	for _, component := range []string{"Library", "Application Support", "kiro-cli"} {
		next, err := safepath.Child(r, component, false, false)
		r.Close()
		if err != nil {
			return "", err
		}
		r = next
	}
	defer r.Close()
	f, err := safepath.File(r, "data.sqlite3", os.O_RDONLY, false)
	if err != nil {
		return "", err
	}
	f.Close()
	return filepath.Join(r.Name(), "data.sqlite3"), nil
}

func checkSchema(ctx context.Context, tx *sql.Tx) error {
	// A direct PRAGMA cannot be shadowed by a source table named pragma_table_list.
	var schema, name, kind string
	var columns, withoutRowID, strict int
	if err := tx.QueryRowContext(ctx, `PRAGMA main.table_list('auth_kv')`).Scan(&schema, &name, &kind, &columns, &withoutRowID, &strict); err != nil {
		return err
	}
	if schema != "main" || name != "auth_kv" || kind != "table" {
		return ErrSource
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA main.table_xinfo('auth_kv')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	key, value, primary := false, false, 0
	for rows.Next() {
		var cid, notnull, pk, hidden int
		var name, declared string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &declared, &notnull, &defaultValue, &pk, &hidden); err != nil {
			return err
		}
		if pk > 0 {
			primary++
		}
		if name == "key" || name == "value" {
			if hidden != 0 || strings.ToUpper(strings.TrimSpace(declared)) != "TEXT" {
				return ErrSource
			}
			if name == "key" {
				key = pk == 1
			} else {
				value = pk == 0
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !key || !value || primary != 1 {
		return ErrSource
	}
	return nil
}

func readValue(ctx context.Context, tx *sql.Tx, afterMetadata func()) ([]byte, error) {
	// Size and type are checked before scanning the credential value into Go.
	// BINARY prevents a source column collation from selecting a different key.
	rows, err := tx.QueryContext(ctx, `SELECT typeof(value), length(CAST(value AS BLOB)) FROM main.auth_kv WHERE key COLLATE BINARY = ? LIMIT 2`, selectedKey)
	if err != nil {
		return nil, err
	}
	var kind string
	var size sql.NullInt64
	count := 0
	for rows.Next() {
		count++
		if err := rows.Scan(&kind, &size); err != nil {
			rows.Close()
			return nil, err
		}
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if count != 1 || kind != "text" || !size.Valid || size.Int64 < 1 || size.Int64 > config.MaxBytes {
		return nil, ErrRecord
	}
	if afterMetadata != nil {
		afterMetadata()
	}
	var data []byte
	if err := tx.QueryRowContext(ctx, `SELECT value FROM main.auth_kv WHERE key COLLATE BINARY = ? LIMIT 1`, selectedKey).Scan(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func validateRecord(data []byte, now time.Time) error {
	o, err := jsonobject.Parse(data, config.MaxBytes)
	if err != nil {
		return ErrRecord
	}
	fields := make(map[string]string, 4)
	for _, name := range []string{"access_token", "expires_at", "region", "start_url"} {
		var value string
		if json.Unmarshal(o[name], &value) != nil || value == "" {
			return ErrRecord
		}
		fields[name] = value
	}
	expiry, err := time.Parse(time.RFC3339, fields["expires_at"])
	if err != nil {
		return ErrRecord
	}
	if !config.VisibleASCII(fields["region"], 128) || len(fields["start_url"]) > 2048 {
		return ErrRecord
	}
	u, err := url.Parse(fields["start_url"])
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return ErrRecord
	}
	if !expiry.After(now) {
		return ErrExpired
	}
	return nil
}

func sanitize(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, ErrRecord) {
		return ErrRecord
	}
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && (sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked) {
		return ErrBusy
	}
	return ErrSource
}
