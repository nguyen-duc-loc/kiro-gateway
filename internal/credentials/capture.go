// Package credentials captures a reference to one saved Kiro credential snapshot.
package credentials

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
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
	ErrChanged  = errors.New("saved IAM Identity Center snapshot has changed; link again before inference")
)

// Reader reads only the fixed source in Home. Now is an injectable wall clock.
// Each capture owns its connection and transaction, with no shared stream state.
type Reader struct {
	Home string
	Now  func() time.Time
	// Test hook observes the boundary between the size query and value query.
	afterMetadata        func()
	afterProfileMetadata func()
}

// Capture returns only a reference. Tokens and record fields remain local.
func (r Reader) Capture(parent context.Context) (config.Session, error) {
	captured, err := r.readSnapshot(parent)
	return captured.reference, err
}

// Snapshot holds validated credential material in memory. It must not be logged
// or persisted. Its fields are private to prevent accidental JSON serialization.
type Snapshot struct {
	accessToken string
	region      string
	expiresAt   time.Time
}

// AccessToken returns the token from the validated snapshot for request signing.
func (s Snapshot) AccessToken() string { return s.accessToken }

// Region returns the region from the same validated record as AccessToken.
func (s Snapshot) Region() string { return s.region }

// ExpiresAt returns the expiry from the validated record.
func (s Snapshot) ExpiresAt() time.Time { return s.expiresAt }

// String prevents ordinary formatting from revealing credential material.
func (s Snapshot) String() string { return "[credential snapshot]" }

// GoString also protects Go syntax formatting of a snapshot.
func (s Snapshot) GoString() string { return s.String() }

type capturedSnapshot struct {
	reference config.Session
	snapshot  Snapshot
	profile   profileSnapshot
}

// ReadSnapshot reads once and returns credential material only when those exact
// bytes match expected. It uses Capture's source validation and transaction.
// Callers own the returned value and must discard it after the attempt.
func (r Reader) ReadSnapshot(ctx context.Context, expected config.Session) (Snapshot, error) {
	d := config.Default()
	d.Session = &expected
	if d.Validate() != nil {
		return Snapshot{}, ErrChanged
	}
	captured, err := r.readSnapshot(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if subtle.ConstantTimeCompare([]byte(captured.reference.Fingerprint), []byte(expected.Fingerprint)) != 1 {
		return Snapshot{}, ErrChanged
	}
	return captured.snapshot, nil
}

func (r Reader) readSnapshot(parent context.Context) (captured capturedSnapshot, result error) {
	return r.readSelected(parent, nil)
}

// A nonnil expected reference selects the combined feasibility read. Ordinary
// capture and ReadSnapshot never inspect the profile table.
func (r Reader) readSelected(parent context.Context, expected *config.Session) (captured capturedSnapshot, result error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	defer func() {
		if ctx.Err() != nil {
			result = sanitize(ctx, ctx.Err())
			captured = capturedSnapshot{}
		}
	}()
	if ctx.Err() != nil {
		return capturedSnapshot{}, sanitize(ctx, ctx.Err())
	}
	path, err := sourcePath(r.Home)
	if err != nil {
		return capturedSnapshot{}, ErrSource
	}
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"mode": {"ro"}, "_query_only": {"1"}, "_busy_timeout": {"1000"}}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return capturedSnapshot{}, sanitize(ctx, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return capturedSnapshot{}, sanitize(ctx, err)
	}
	defer tx.Rollback()
	if err := checkSchema(ctx, tx, false); err != nil {
		return capturedSnapshot{}, sanitize(ctx, err)
	}
	value, err := readValue(ctx, tx, false, r.afterMetadata)
	if err != nil {
		return capturedSnapshot{}, sanitize(ctx, err)
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	if err := validateRecord(value, now()); err != nil {
		return capturedSnapshot{}, err
	}
	h := sha256.New()
	h.Write([]byte("kiro-gateway/" + config.Source + "\x00"))
	h.Write(value)
	// Decode only the already validated bytes, never reread the source for a token.
	var fields map[string]json.RawMessage
	if json.Unmarshal(value, &fields) != nil {
		return capturedSnapshot{}, ErrRecord
	}
	// Use exact member names just as validateRecord does. Struct decoding would
	// accept case variants among otherwise ignored fields and could overwrite a
	// validated token or region with a different member from the same record.
	var accessToken, region, expiry string
	if json.Unmarshal(fields["access_token"], &accessToken) != nil || json.Unmarshal(fields["region"], &region) != nil || json.Unmarshal(fields["expires_at"], &expiry) != nil {
		return capturedSnapshot{}, ErrRecord
	}
	expiresAt, err := time.Parse(time.RFC3339, expiry)
	if err != nil {
		return capturedSnapshot{}, ErrRecord
	}
	captured = capturedSnapshot{
		reference: config.Session{Source: config.Source, Fingerprint: hex.EncodeToString(h.Sum(nil))},
		snapshot:  Snapshot{accessToken: accessToken, region: region, expiresAt: expiresAt},
	}
	if expected != nil {
		if subtle.ConstantTimeCompare([]byte(captured.reference.Fingerprint), []byte(expected.Fingerprint)) != 1 {
			return capturedSnapshot{}, ErrChanged
		}
		if err := checkSchema(ctx, tx, true); err != nil {
			return capturedSnapshot{}, sanitize(ctx, err)
		}
		profile, err := readValue(ctx, tx, true, r.afterProfileMetadata)
		if err != nil {
			return capturedSnapshot{}, sanitize(ctx, err)
		}
		captured.profile, err = parseProfile(profile)
		if err != nil {
			return capturedSnapshot{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return capturedSnapshot{}, sanitize(ctx, err)
	}
	return captured, nil
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

func checkSchema(ctx context.Context, tx *sql.Tx, profile bool) error {
	table, list, info := "auth_kv", `PRAGMA main.table_list('auth_kv')`, `PRAGMA main.table_xinfo('auth_kv')`
	if profile {
		table, list, info = "state", `PRAGMA main.table_list('state')`, `PRAGMA main.table_xinfo('state')`
	}
	// A direct PRAGMA cannot be shadowed by a source table named pragma_table_list.
	var schema, name, kind string
	var columns, withoutRowID, strict int
	if err := tx.QueryRowContext(ctx, list).Scan(&schema, &name, &kind, &columns, &withoutRowID, &strict); err != nil {
		return err
	}
	if schema != "main" || name != table || kind != "table" {
		return ErrSource
	}
	rows, err := tx.QueryContext(ctx, info)
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
			typ := strings.ToUpper(strings.TrimSpace(declared))
			// Kiro's profile table declares value as BLOB but stores JSON as TEXT.
			// Only that declaration is additional; readValue still rejects BLOB
			// storage before reading bytes. The token table remains TEXT only.
			allowedType := typ == "TEXT" || profile && name == "value" && typ == "BLOB"
			if hidden != 0 || !allowedType {
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

func readValue(ctx context.Context, tx *sql.Tx, profile bool, afterMetadata func()) ([]byte, error) {
	key, invalid := selectedKey, ErrRecord
	metadataQuery := `SELECT typeof(value), length(CAST(value AS BLOB)) FROM main.auth_kv WHERE key COLLATE BINARY = ? LIMIT 2`
	valueQuery := `SELECT value FROM main.auth_kv WHERE key COLLATE BINARY = ? LIMIT 1`
	if profile {
		key, invalid = selectedProfileKey, ErrProfileInvalid
		metadataQuery = `SELECT typeof(value), length(CAST(value AS BLOB)) FROM main.state WHERE key COLLATE BINARY = ? LIMIT 2`
		valueQuery = `SELECT value FROM main.state WHERE key COLLATE BINARY = ? LIMIT 1`
	}
	// Size and type are checked before scanning the credential value into Go.
	// BINARY prevents a source column collation from selecting a different key.
	rows, err := tx.QueryContext(ctx, metadataQuery, key)
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
		return nil, invalid
	}
	if afterMetadata != nil {
		afterMetadata()
	}
	var data []byte
	if err := tx.QueryRowContext(ctx, valueQuery, key).Scan(&data); err != nil {
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
	if errors.Is(err, ErrProfileInvalid) {
		return ErrProfileInvalid
	}
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && (sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked) {
		return ErrBusy
	}
	return ErrSource
}
