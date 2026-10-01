package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"kiro-gateway/internal/config"
)

// covers: spec 0003 AC-2. A source replacement cannot mix token and fingerprint.
func TestReadSnapshotUsesCheckedBytes(t *testing.T) {
	r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "wal")
	expected := config.Session{Source: config.Source, Fingerprint: expectedFingerprint(record)}
	changed := strings.Replace(record, "synthetic-access-secret", "replacement-secret", 1)
	r.afterMetadata = func() {
		if _, err := db.Exec(`UPDATE auth_kv SET value=? WHERE key=?`, changed, selectedKey); err != nil {
			t.Fatalf("UPDATE(synthetic replacement) error = %v, want nil", err)
		}
	}
	got, err := r.ReadSnapshot(t.Context(), expected)
	if err != nil {
		t.Fatalf("ReadSnapshot(replaced during transaction) error = %v, want nil", err)
	}
	if got.AccessToken() != "synthetic-access-secret" || got.Region() != "synthetic-region" || !got.ExpiresAt().Equal(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("ReadSnapshot(replaced during transaction) returned mixed fields, want original snapshot")
	}
	r.afterMetadata = nil
	next, err := r.ReadSnapshot(t.Context(), expected)
	if !errors.Is(err, ErrChanged) || next != (Snapshot{}) {
		t.Errorf("ReadSnapshot(next attempt) = %v, %v, want empty snapshot, ErrChanged", next, err)
	}
}

func TestReadSnapshotFailureReturnsNoMaterial(t *testing.T) {
	r, _, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "delete")
	expected := config.Session{Source: config.Source, Fingerprint: expectedFingerprint(record)}
	r.Now = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	got, err := r.ReadSnapshot(t.Context(), expected)
	if !errors.Is(err, ErrExpired) || got != (Snapshot{}) {
		t.Errorf("ReadSnapshot(expired) = %v, %v, want empty snapshot, ErrExpired", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err = r.ReadSnapshot(ctx, expected)
	if !errors.Is(err, ErrCanceled) || got != (Snapshot{}) {
		t.Errorf("ReadSnapshot(canceled) = %v, %v, want empty snapshot, ErrCanceled", got, err)
	}
	r.afterMetadata = func() { t.Error("ReadSnapshot(unsupported reference) accessed source, want no read") }
	expected.Source = "unsupported"
	got, err = r.ReadSnapshot(t.Context(), expected)
	if !errors.Is(err, ErrChanged) || got != (Snapshot{}) {
		t.Errorf("ReadSnapshot(unsupported source) = %v, %v, want empty snapshot, ErrChanged", got, err)
	}
}

func TestSnapshotFormattingOmitsSecrets(t *testing.T) {
	s := Snapshot{accessToken: "sentinel-token", region: "sentinel-region", expiresAt: time.Now()}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal(snapshot) error = %v, want nil", err)
	}
	for _, output := range []string{string(b), fmt.Sprint(s), fmt.Sprintf("%+v", s), fmt.Sprintf("%#v", s)} {
		if strings.Contains(output, "sentinel") {
			t.Error("format(snapshot) exposed a sentinel, want credential fields omitted")
		}
	}
}

func TestReadSnapshotUsesExactMemberNames(t *testing.T) {
	r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, "delete")
	data := strings.TrimSuffix(record, "}") + `,"ACCESS_TOKEN":"decoy-token","REGION":"decoy-region","EXPIRES_AT":"2000-01-01T00:00:00Z"}`
	if _, err := db.Exec(`UPDATE auth_kv SET value=?`, data); err != nil {
		t.Fatal(err)
	}
	expected := config.Session{Source: config.Source, Fingerprint: expectedFingerprint(data)}
	got, err := r.ReadSnapshot(t.Context(), expected)
	if err != nil || got.AccessToken() != "synthetic-access-secret" || got.Region() != "synthetic-region" || got.ExpiresAt().Year() != 2030 {
		t.Errorf("ReadSnapshot(case variant decoys) error = %v, want nil and original exact members", err)
	}
}
