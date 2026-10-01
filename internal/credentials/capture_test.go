package credentials

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kiro-gateway/internal/config"
)

const record = `{"access_token":"synthetic-access-secret","expires_at":"2030-01-01T00:00:00Z","region":"synthetic-region","start_url":"https://synthetic-account.example/start","extra":{"unused":true}}`

func fixture(t *testing.T, schema, mode string) (Reader, *sql.DB, string) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home ?#%")
	parent := filepath.Join(home, "Library", "Application Support", "kiro-cli")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "data.sqlite3")
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	// mode is one of the fixed journal modes supplied by tests, never external data.
	if _, err := db.Exec("PRAGMA journal_mode=" + mode); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO auth_kv(key,value) VALUES(?,?)`, selectedKey, record); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	return Reader{Home: home, Now: func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }}, db, path
}

// covers: spec 0002 AC-3, AC-9. Expiry uses the injected clock on every capture.
func TestCaptureExpiryBoundary(t *testing.T) {
	r, _, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "delete")
	expiry := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
		t.Run(offset.String(), func(t *testing.T) {
			r.Now = func() time.Time { return expiry.Add(offset) }
			got, err := r.Capture(t.Context())
			if offset < 0 {
				want := config.Session{Source: config.Source, Fingerprint: expectedFingerprint(record)}
				if err != nil || got != want {
					t.Errorf("Capture(before expiry) = %+v, %v, want %+v, nil", got, err, want)
				}
			} else if !errors.Is(err, ErrExpired) || got != (config.Session{}) {
				t.Errorf("Capture(expiry offset %v) = %+v, %v, want empty reference, ErrExpired", offset, got, err)
			}
		})
	}
}

// covers: spec 0002 AC-3, AC-9. SQLite byte length, not rune count, bounds capture.
func TestCaptureRecordSizeBoundary(t *testing.T) {
	for _, size := range []int{config.MaxBytes, config.MaxBytes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "delete")
			base := strings.Replace(record, `"unused":true`, `"unused":"é"`, 1)
			data := base + strings.Repeat(" ", size-len(base))
			if _, err := db.Exec(`UPDATE auth_kv SET value=? WHERE key=?`, data, selectedKey); err != nil {
				t.Fatalf("UPDATE(size %d) error = %v, want nil", size, err)
			}
			got, err := r.Capture(t.Context())
			if size == config.MaxBytes {
				want := config.Session{Source: config.Source, Fingerprint: expectedFingerprint(data)}
				if err != nil || got != want {
					t.Errorf("Capture(%d bytes) = %+v, %v, want %+v, nil", size, got, err, want)
				}
			} else if !errors.Is(err, ErrRecord) || got != (config.Session{}) {
				t.Errorf("Capture(%d bytes) = %+v, %v, want empty reference, ErrRecord", size, got, err)
			}
		})
	}
}

// covers: spec 0002 AC-3, AC-8. Other rows cannot supply missing selected metadata.
func TestCaptureDoesNotCombineRecords(t *testing.T) {
	r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "wal")
	if _, err := db.Exec(`INSERT INTO auth_kv(key,value) VALUES(?,?)`, "other-login", record); err != nil {
		t.Fatalf("INSERT(decoy) error = %v, want nil", err)
	}
	for _, field := range []string{"access_token", "expires_at", "region", "start_url"} {
		t.Run(field, func(t *testing.T) {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(record), &fields); err != nil {
				t.Fatalf("Unmarshal(fixture) error = %v, want nil", err)
			}
			delete(fields, field)
			data, err := json.Marshal(fields)
			if err != nil {
				t.Fatalf("Marshal(fixture without %s) error = %v, want nil", field, err)
			}
			if _, err := db.Exec(`UPDATE auth_kv SET value=? WHERE key=?`, string(data), selectedKey); err != nil {
				t.Fatalf("UPDATE(selected row) error = %v, want nil", err)
			}
			got, err := r.Capture(t.Context())
			if !errors.Is(err, ErrRecord) || got != (config.Session{}) {
				t.Errorf("Capture(missing %s with valid decoy) = %+v, %v, want empty reference, ErrRecord", field, got, err)
			}
		})
	}
}

// covers: spec 0002 AC-3. Cancellation during capture never returns a reference.
func TestCaptureCancellationAfterMetadata(t *testing.T) {
	r, _, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "wal")
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	r.afterMetadata = cancel
	got, err := r.Capture(ctx)
	if !errors.Is(err, ErrCanceled) || got != (config.Session{}) {
		t.Errorf("Capture(canceled after metadata) = %+v, %v, want empty reference, ErrCanceled", got, err)
	}
	r.afterMetadata = nil
	got, err = r.Capture(t.Context())
	if err != nil || got.Fingerprint != expectedFingerprint(record) {
		t.Errorf("Capture(after canceled capture) = %+v, %v, want original reference, nil", got, err)
	}
}

func expectedFingerprint(data string) string {
	h := sha256.Sum256(append([]byte("kiro-gateway/kiro_cli_idc_sqlite_v1\x00"), []byte(data)...))
	return hex.EncodeToString(h[:])
}

// covers: spec 0002 AC-3, AC-9.
func TestCaptureExactSnapshotInRollbackAndWAL(t *testing.T) {
	for _, mode := range []string{"delete", "wal"} {
		t.Run(mode, func(t *testing.T) {
			r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY, value TEXT, extra INTEGER)`, mode)
			variations := []string{record, " \n" + record, strings.Replace(record, "2030-", "2031-", 1), strings.Replace(record, "synthetic-region", "different-region", 1), strings.Replace(record, "synthetic-access-secret", "another-synthetic-secret", 1)}
			for _, data := range variations {
				if _, err := db.Exec(`UPDATE auth_kv SET value=? WHERE key=?`, data, selectedKey); err != nil {
					t.Fatal(err)
				}
				got, err := r.Capture(t.Context())
				if err != nil {
					t.Fatalf("Capture(%s fixture) = %v, want nil", mode, err)
				}
				if got.Source != config.Source || got.Fingerprint != expectedFingerprint(data) {
					t.Errorf("Capture(%s) returned wrong exact snapshot digest", mode)
				}
				var stored string
				if err := db.QueryRow(`SELECT value FROM auth_kv WHERE key=?`, selectedKey).Scan(&stored); err != nil {
					t.Fatal(err)
				}
				if stored != data {
					t.Error("Capture changed source record")
				}
			}
			var journal, schema string
			if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT sql FROM sqlite_schema WHERE name='auth_kv'`).Scan(&schema); err != nil {
				t.Fatal(err)
			}
			if journal != mode || schema != `CREATE TABLE auth_kv(key TEXT PRIMARY KEY, value TEXT, extra INTEGER)` {
				t.Errorf("Capture changed source schema or journal: %q, %q", schema, journal)
			}
		})
	}
}

// covers: spec 0002 AC-3, AC-9.
func TestCaptureUsesOneTransaction(t *testing.T) {
	r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY, value TEXT)`, "wal")
	changed := strings.Replace(record, "synthetic-access-secret", "replacement-secret", 1)
	r.afterMetadata = func() {
		if _, err := db.Exec(`UPDATE auth_kv SET value=? WHERE key=?`, changed, selectedKey); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != expectedFingerprint(record) {
		t.Error("Capture(changed after metadata) read a second snapshot")
	}
	r.afterMetadata = nil
	got, err = r.Capture(t.Context())
	if err != nil || got.Fingerprint != expectedFingerprint(changed) {
		t.Errorf("Capture(next transaction) = %v, want changed snapshot", err)
	}
}

// covers: spec 0002 AC-3.
func TestCaptureRejectsRecordBeforeValueRead(t *testing.T) {
	for name, data := range map[string]any{"oversize": strings.Repeat("x", 65537), "blob": []byte(record), "null": nil, "empty": ""} {
		t.Run(name, func(t *testing.T) {
			r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY, value TEXT)`, "delete")
			if _, err := db.Exec(`UPDATE auth_kv SET value=?`, data); err != nil {
				t.Fatal(err)
			}
			r.afterMetadata = func() { t.Error("Capture(invalid size/type) reached value read, want rejection first") }
			if _, err := r.Capture(t.Context()); !errors.Is(err, ErrRecord) {
				t.Errorf("Capture(%s) = %v, want ErrRecord", name, err)
			}
		})
	}
}

// covers: spec 0002 AC-3, AC-9.
func TestCaptureSchemaContract(t *testing.T) {
	cases := map[string]string{
		"unique instead of primary": `CREATE TABLE auth_kv(key TEXT UNIQUE, value TEXT)`,
		"composite primary":         `CREATE TABLE auth_kv(key TEXT, value TEXT, PRIMARY KEY(key,value))`,
		"wrong type":                `CREATE TABLE auth_kv(key VARCHAR PRIMARY KEY, value TEXT)`,
		"wrong value type":          `CREATE TABLE auth_kv(key TEXT PRIMARY KEY, value BLOB)`,
		"case changed key":          `CREATE TABLE auth_kv(KEY TEXT PRIMARY KEY, value TEXT)`,
		"view":                      `CREATE TABLE backing(key TEXT PRIMARY KEY,value TEXT); CREATE VIEW auth_kv AS SELECT * FROM backing; CREATE TRIGGER insert_view INSTEAD OF INSERT ON auth_kv BEGIN INSERT INTO backing VALUES(new.key,new.value); END`,
	}
	for name, schema := range cases {
		t.Run(name, func(t *testing.T) {
			r, _, _ := fixture(t, schema, "delete")
			if _, err := r.Capture(t.Context()); !errors.Is(err, ErrSource) {
				t.Errorf("Capture(%s schema) = %v, want ErrSource", name, err)
			}
		})
	}
	t.Run("generated value", func(t *testing.T) {
		r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "delete")
		if _, err := db.Exec(`DROP TABLE auth_kv; CREATE TABLE auth_kv(key TEXT PRIMARY KEY,raw TEXT,value TEXT GENERATED ALWAYS AS (raw) STORED)`); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Capture(t.Context()); !errors.Is(err, ErrSource) {
			t.Errorf("Capture(generated value) = %v, want ErrSource", err)
		}
	})
	t.Run("shadowed metadata table", func(t *testing.T) {
		r, db, _ := fixture(t, `CREATE TABLE backing(key TEXT PRIMARY KEY,value TEXT); CREATE VIEW auth_kv AS SELECT * FROM backing; CREATE TRIGGER insert_view INSTEAD OF INSERT ON auth_kv BEGIN INSERT INTO backing VALUES(new.key,new.value); END`, "delete")
		if _, err := db.Exec(`CREATE TABLE pragma_table_list(schema TEXT,name TEXT,type TEXT); INSERT INTO pragma_table_list VALUES('main','auth_kv','table')`); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Capture(t.Context()); !errors.Is(err, ErrSource) {
			t.Errorf("Capture(shadowed metadata table) = %v, want ErrSource", err)
		}
	})
}

// covers: spec 0002 AC-3.
func TestCaptureValidatesJSONFieldsAndExpiry(t *testing.T) {
	cases := map[string]string{
		"duplicate":        strings.Replace(record, `"extra":`, `"region":"duplicate","extra":`, 1),
		"nested duplicate": strings.Replace(record, `"unused":true`, `"unused":true,"unused":false`, 1),
		"invalid UTF8":     strings.Replace(record, "synthetic-region", "\xff", 1),
		"empty token":      strings.Replace(record, "synthetic-access-secret", "", 1),
		"null token":       strings.Replace(record, `"synthetic-access-secret"`, `null`, 1),
		"no timezone":      strings.Replace(record, "2030-01-01T00:00:00Z", "2030-01-01T00:00:00", 1),
		"region spaces":    strings.Replace(record, "synthetic-region", "bad region", 1),
		"region oversize":  strings.Replace(record, "synthetic-region", strings.Repeat("r", 129), 1),
		"http":             strings.Replace(record, "https://", "http://", 1),
		"url user":         strings.Replace(record, "https://", "https://user@", 1),
		"url missing host": strings.Replace(record, "https://synthetic-account.example/start", "https:///path", 1),
		"trailing":         record + ` {}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validateRecord([]byte(data), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrRecord) {
				t.Errorf("validateRecord(%s) = %v, want ErrRecord", name, err)
			}
		})
	}
	for _, now := range []time.Time{time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)} {
		if err := validateRecord([]byte(record), now); !errors.Is(err, ErrExpired) {
			t.Errorf("validateRecord(expired at %v) = %v, want ErrExpired", now, err)
		}
	}
}

// covers: spec 0002 AC-3.
func TestCaptureMissingRecordAndExactKey(t *testing.T) {
	r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT COLLATE NOCASE PRIMARY KEY,value TEXT)`, "delete")
	if _, err := db.Exec(`UPDATE auth_kv SET key=?`, strings.ToUpper(selectedKey)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Capture(t.Context()); !errors.Is(err, ErrRecord) {
		t.Errorf("Capture(case variant key) = %v, want ErrRecord", err)
	}
}

// covers: spec 0002 AC-3, AC-8.
func TestCaptureBusyAndCancellation(t *testing.T) {
	r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "delete")
	if _, err := db.Exec(`BEGIN EXCLUSIVE`); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`ROLLBACK`)
	started := time.Now()
	if _, err := r.Capture(t.Context()); !errors.Is(err, ErrBusy) {
		t.Errorf("Capture(locked) = %v, want ErrBusy", err)
	}
	if elapsed := time.Since(started); elapsed < 800*time.Millisecond || elapsed > 2500*time.Millisecond {
		t.Errorf("Capture(locked) elapsed = %v, want near one second", elapsed)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.Capture(ctx); !errors.Is(err, ErrCanceled) {
		t.Errorf("Capture(canceled) = %v, want ErrCanceled", err)
	}
	ctx, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	started = time.Now()
	if _, err := r.Capture(ctx); !errors.Is(err, ErrTimeout) {
		t.Errorf("Capture(deadline) = %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(started); elapsed > 1500*time.Millisecond {
		t.Errorf("Capture(deadline during busy wait) elapsed = %v, want <= busy budget plus scheduling", elapsed)
	}
}

// covers: spec 0002 AC-3, AC-6.
func TestCaptureRejectsUnsafeSource(t *testing.T) {
	for _, target := range []string{"Library", "Application Support", "kiro-cli", "data.sqlite3"} {
		for _, kind := range []string{"symlink", "relative symlink", "permissions"} {
			t.Run(target+" "+kind, func(t *testing.T) {
				r, db, path := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "delete")
				db.Close()
				switch target {
				case "Library":
					path = filepath.Join(r.Home, "Library")
				case "Application Support":
					path = filepath.Join(r.Home, "Library", "Application Support")
				case "kiro-cli":
					path = filepath.Dir(path)
				}
				if kind != "permissions" {
					if err := os.Rename(path, path+".original"); err != nil {
						t.Fatal(err)
					}
					destination := path + ".original"
					if kind == "relative symlink" {
						destination = filepath.Base(destination)
					}
					if err := os.Symlink(destination, path); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Chmod(path, 0777); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := r.Capture(t.Context()); !errors.Is(err, ErrSource) {
					t.Errorf("Capture(%s %s) = %v, want ErrSource", target, kind, err)
				}
			})
		}
	}
}
