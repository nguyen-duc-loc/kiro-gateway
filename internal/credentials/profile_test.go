package credentials

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"kiro-gateway/internal/config"
)

const profileRecord = `{"arn":"arn:aws:codewhisperer:us-east-1:000000000000:profile/sentinel-profile","profileName":"sentinel-name"}`

func profileFixture(t *testing.T, mode string) (Reader, *sql.DB, config.Session) {
	t.Helper()
	r, db, _ := fixture(t, `CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`, mode)
	if _, err := db.Exec(`CREATE TABLE state(key TEXT PRIMARY KEY,value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO state VALUES(?,?)`, selectedProfileKey, profileRecord); err != nil {
		t.Fatal(err)
	}
	return r, db, config.Session{Source: config.Source, Fingerprint: expectedFingerprint(record)}
}

// covers: spec 0003 AC-2, AC-6, AC-9. Exact bytes and distinct region sources.
func TestReadProfileSnapshot(t *testing.T) {
	for _, region := range []string{"us-east-1", "eu-central-1"} {
		t.Run(region, func(t *testing.T) {
			r, db, ref := profileFixture(t, "delete")
			data := strings.Replace(profileRecord, "us-east-1", region, 1)
			if _, err := db.Exec(`UPDATE state SET value=?`, data); err != nil {
				t.Fatal(err)
			}
			got, err := r.ReadProfileSnapshot(t.Context(), ref)
			if err != nil {
				t.Fatalf("ReadProfileSnapshot(%s) error = %v, want nil", region, err)
			}
			wantDigest := sha256.Sum256(append([]byte("kiro-gateway/probe-profile-v1\x00"), []byte(data)...))
			if got.ProfileRegion() != region || got.Credential().Region() != "synthetic-region" || got.Credential().AccessToken() != "synthetic-access-secret" || got.Credential().ExpiresAt().Year() != 2030 || got.ProfileDigest() != wantDigest {
				t.Error("ReadProfileSnapshot(valid pair) returned wrong fields, want exact token and profile sources")
			}
			if got.ProfileARN() != "arn:aws:codewhisperer:"+region+":000000000000:profile/sentinel-profile" {
				t.Error("ReadProfileSnapshot(valid pair) ARN differs from source")
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			for _, out := range []string{string(b), fmt.Sprint(got), fmt.Sprintf("%+v", got), fmt.Sprintf("%#v", got)} {
				for _, secret := range []string{"sentinel", "000000000000", region, "synthetic-access-secret", hex.EncodeToString(wantDigest[:])} {
					if strings.Contains(out, secret) {
						t.Error("format(ProfileSnapshot) exposed account data, want omitted")
					}
				}
			}
			captured, err := r.Capture(t.Context())
			if err != nil || captured != ref {
				t.Errorf("Capture(with profile) error=%v, want original token reference", err)
			}
		})
	}
}

func TestProfileValidation(t *testing.T) {
	validARN := "arn:aws:codewhisperer:us-east-1:000000000000:profile/sentinel-profile"
	cases := []struct {
		name, data string
		want       error
	}{
		{"valid", profileRecord, nil},
		{"snake name", strings.Replace(profileRecord, "profileName", "profile_name", 1), nil},
		{"empty name", strings.Replace(profileRecord, "sentinel-name", "", 1), nil},
		{"extra members", strings.TrimSuffix(profileRecord, "}") + `,"extra":{"unused":true}}`, nil},
		{"missing name", `{"arn":"` + validARN + `"}`, ErrProfileInvalid},
		{"both names", strings.TrimSuffix(profileRecord, "}") + `,"profile_name":"sentinel-name"}`, ErrProfileInvalid},
		{"null name", strings.Replace(profileRecord, `"sentinel-name"`, `null`, 1), ErrProfileInvalid},
		{"numeric name", strings.Replace(profileRecord, `"sentinel-name"`, `123`, 1), ErrProfileInvalid},
		{"case name", strings.Replace(profileRecord, "profileName", "ProfileName", 1), ErrProfileInvalid},
		{"case ARN", strings.Replace(profileRecord, "arn\"", "ARN\"", 1), ErrProfileInvalid},
		{"null ARN", `{"arn":null,"profileName":""}`, ErrProfileInvalid},
		{"empty ARN", `{"arn":"","profileName":""}`, ErrProfileInvalid},
		{"nonASCII ARN", strings.Replace(profileRecord, "sentinel-profile", "é", 1), ErrProfileInvalid},
		{"control ARN", strings.Replace(profileRecord, "sentinel-profile", `\n`, 1), ErrProfileInvalid},
		{"long ARN", strings.Replace(profileRecord, validARN, strings.Repeat("a", 2049), 1), ErrProfileInvalid},
		{"invalid UTF8", profileRecord + string([]byte{255}), ErrProfileInvalid},
		{"duplicate nested", strings.TrimSuffix(profileRecord, "}") + `,"extra":{"x":1,"x":2}}`, ErrProfileInvalid},
		{"duplicate ARN", strings.TrimSuffix(profileRecord, "}") + `,"arn":"` + validARN + `"}`, ErrProfileInvalid},
		{"trailing JSON", profileRecord + ` {}`, ErrProfileInvalid},
		{"array", `[]`, ErrProfileInvalid},
		{"partition", strings.Replace(profileRecord, ":aws:", ":aws-cn:", 1), ErrProfileUnsupported},
		{"service", strings.Replace(profileRecord, ":codewhisperer:", ":other:", 1), ErrProfileUnsupported},
		{"region", strings.Replace(profileRecord, "us-east-1", "ap-south-1", 1), ErrProfileUnsupported},
		{"account letters", strings.Replace(profileRecord, "000000000000", "accountvalue", 1), ErrProfileUnsupported},
		{"short account", strings.Replace(profileRecord, "000000000000", "000", 1), ErrProfileUnsupported},
		{"wrong prefix", strings.Replace(profileRecord, "arn:aws", "ARN:aws", 1), ErrProfileUnsupported},
		{"resource path", strings.Replace(profileRecord, "profile/sentinel-profile", "profile/other/child", 1), ErrProfileUnsupported},
		{"resource punctuation", strings.Replace(profileRecord, "sentinel-profile", "has.dot", 1), ErrProfileUnsupported},
		{"empty resource", strings.Replace(profileRecord, "sentinel-profile", "", 1), ErrProfileUnsupported},
		{"long resource", strings.Replace(profileRecord, "sentinel-profile", strings.Repeat("a", 129), 1), ErrProfileUnsupported},
		{"max resource", strings.Replace(profileRecord, "sentinel-profile", strings.Repeat("a", 128), 1), nil},
		{"leading space", strings.Replace(profileRecord, "arn:aws", " arn:aws", 1), ErrProfileInvalid},
		{"trailing space", strings.Replace(profileRecord, "sentinel-profile", "sentinel-profile ", 1), ErrProfileInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseProfile([]byte(tc.data))
			if !errors.Is(err, tc.want) {
				t.Errorf("parseProfile(%s) error=%v, want %v", tc.name, err, tc.want)
			}
			if err != nil && got != (profileSnapshot{}) {
				t.Errorf("parseProfile(%s) returned material on failure, want zero", tc.name)
			}
		})
	}
}

func TestProfileRecordSelectionAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		want  error
	}{
		{"max bytes", profileRecord + strings.Repeat(" ", config.MaxBytes-len(profileRecord)), nil},
		{"too large", profileRecord + strings.Repeat(" ", config.MaxBytes+1-len(profileRecord)), ErrProfileInvalid},
		{"blob", []byte(profileRecord), ErrProfileInvalid},
		{"null", nil, ErrProfileInvalid},
		{"empty", "", ErrProfileInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, ref := profileFixture(t, "delete")
			if _, err := db.Exec(`UPDATE state SET value=?`, tc.value); err != nil {
				t.Fatal(err)
			}
			_, err := r.ReadProfileSnapshot(t.Context(), ref)
			if !errors.Is(err, tc.want) {
				t.Errorf("ReadProfileSnapshot(%s) error=%v, want %v", tc.name, err, tc.want)
			}
		})
	}
	t.Run("exact key under NOCASE", func(t *testing.T) {
		r, db, ref := profileFixture(t, "delete")
		if _, err := db.Exec(`DROP TABLE state; CREATE TABLE state(key TEXT COLLATE NOCASE PRIMARY KEY,value TEXT); INSERT INTO state VALUES('API.CODEWHISPERER.PROFILE',?)`, profileRecord); err != nil {
			t.Fatal(err)
		}
		got, err := r.ReadProfileSnapshot(t.Context(), ref)
		if !errors.Is(err, ErrProfileInvalid) || got != (ProfileSnapshot{}) {
			t.Errorf("ReadProfileSnapshot(case variant key) error=%v, want profile_invalid", err)
		}
	})
}

func TestProfileSchema(t *testing.T) {
	for _, tc := range []struct {
		name, schema string
		want         error
	}{
		{"missing", "", ErrSource},
		{"view", `CREATE VIEW state AS SELECT key,value FROM auth_kv`, ErrSource},
		{"no primary", `CREATE TABLE state(key TEXT,value TEXT)`, ErrSource},
		{"composite primary", `CREATE TABLE state(key TEXT,value TEXT,PRIMARY KEY(key,value))`, ErrSource},
		{"blob declaration with text storage", `CREATE TABLE state(key TEXT PRIMARY KEY,value BLOB)`, nil},
		{"wrong type", `CREATE TABLE state(key TEXT PRIMARY KEY,value INTEGER)`, ErrSource},
		{"blob key", `CREATE TABLE state(key BLOB PRIMARY KEY,value BLOB)`, ErrSource},
		{"generated blob value", `CREATE TABLE state(key TEXT PRIMARY KEY,value BLOB GENERATED ALWAYS AS ('x'))`, ErrSource},
		{"generated value", `CREATE TABLE state(key TEXT PRIMARY KEY,value TEXT GENERATED ALWAYS AS ('x'))`, ErrSource},
		{"generated key", `CREATE TABLE state(id TEXT PRIMARY KEY,key TEXT GENERATED ALWAYS AS ('x'),value TEXT)`, ErrSource},
		{"additive", `CREATE TABLE state(key TEXT PRIMARY KEY,value TEXT,unused INTEGER)`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, ref := profileFixture(t, "delete")
			if _, err := db.Exec(`DROP TABLE state`); err != nil {
				t.Fatal(err)
			}
			if tc.schema != "" {
				if _, err := db.Exec(tc.schema); err != nil {
					t.Fatal(err)
				}
			}
			if tc.want == nil {
				if _, err := db.Exec(`INSERT INTO state(key,value) VALUES(?,?)`, selectedProfileKey, profileRecord); err != nil {
					t.Fatal(err)
				}
			}
			got, err := r.ReadProfileSnapshot(t.Context(), ref)
			if !errors.Is(err, tc.want) {
				t.Errorf("ReadProfileSnapshot(%s) error=%v, want %v", tc.name, err, tc.want)
			}
			if err != nil && got != (ProfileSnapshot{}) {
				t.Error("ReadProfileSnapshot(bad schema) returned account material")
			}
			if _, err := r.ReadSnapshot(t.Context(), ref); err != nil {
				t.Errorf("ReadSnapshot(%s profile table) error=%v, want nil", tc.name, err)
			}
		})
	}
}

// Kiro declares state.value as BLOB while binding the profile JSON as TEXT.
// The declaration must not bypass validation of the actual selected value.
func TestProfileBlobDeclarationPreservesValueChecks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		want  error
	}{
		{name: "text JSON", value: profileRecord},
		{name: "binary JSON", value: []byte(profileRecord), want: ErrProfileInvalid},
		{name: "oversized text", value: profileRecord + strings.Repeat(" ", config.MaxBytes), want: ErrProfileInvalid},
		{name: "null", value: nil, want: ErrProfileInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, ref := profileFixture(t, "delete")
			if _, err := db.Exec(`DROP TABLE state; CREATE TABLE state(key TEXT PRIMARY KEY,value BLOB)`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO state VALUES(?,?)`, selectedProfileKey, tc.value); err != nil {
				t.Fatal(err)
			}
			valueRead := false
			r.afterProfileMetadata = func() { valueRead = true }
			got, err := r.ReadProfileSnapshot(t.Context(), ref)
			if !errors.Is(err, tc.want) {
				t.Errorf("ReadProfileSnapshot(BLOB declaration, %s) error=%v, want %v", tc.name, err, tc.want)
			}
			if tc.want != nil && (valueRead || got != (ProfileSnapshot{})) {
				t.Error("invalid profile reached value read or returned material")
			}
			if tc.want == nil {
				wantDigest := sha256.Sum256(append([]byte("kiro-gateway/probe-profile-v1\x00"), []byte(profileRecord)...))
				if !valueRead || got.ProfileDigest() != wantDigest || got.Credential().AccessToken() != "synthetic-access-secret" {
					t.Error("text profile in BLOB declaration lost exact snapshot fields")
				}
			}
		})
	}
}

// A writer commits both rows after token metadata or profile metadata is read.
// WAL readers must retain the old pair through the whole transaction.
func TestProfileTransactionWAL(t *testing.T) {
	for _, phase := range []string{"token metadata", "profile metadata"} {
		t.Run(phase, func(t *testing.T) {
			r, db, ref := profileFixture(t, "wal")
			changedToken := strings.Replace(record, "synthetic-access-secret", "replacement-secret", 1)
			changedProfile := strings.Replace(profileRecord, "us-east-1", "eu-central-1", 1)
			change := func() {
				tx, err := db.Begin()
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				if _, err = tx.Exec(`UPDATE auth_kv SET value=?`, changedToken); err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(`UPDATE state SET value=?`, changedProfile); err != nil {
					t.Fatal(err)
				}
				if err = tx.Commit(); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "token metadata" {
				r.afterMetadata = change
			} else {
				r.afterProfileMetadata = change
			}
			got, err := r.ReadProfileSnapshot(t.Context(), ref)
			if err != nil || got.Credential().AccessToken() != "synthetic-access-secret" || got.ProfileRegion() != "us-east-1" {
				t.Errorf("ReadProfileSnapshot(%s replacement) error=%v, want original pair", phase, err)
			}
			r.afterMetadata, r.afterProfileMetadata = nil, nil
			next, err := r.ReadProfileSnapshot(t.Context(), ref)
			if !errors.Is(err, ErrChanged) || next != (ProfileSnapshot{}) {
				t.Errorf("ReadProfileSnapshot(next pair) error=%v, want session changed", err)
			}
		})
	}
}

func TestProfileTransactionRollbackJournal(t *testing.T) {
	r, db, ref := profileFixture(t, "delete")
	// A reserved writer can change its own snapshot, but cannot commit while the
	// reader holds a shared lock. Rollback gives the blocked writer a bounded end.
	r.afterProfileMetadata = func() {
		if _, err := db.Exec(`PRAGMA busy_timeout=50`); err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`UPDATE state SET value=?`, strings.Replace(profileRecord, "us-east-1", "eu-central-1", 1)); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err == nil {
			t.Error("writer.Commit(during read) succeeded, want busy")
		}
	}
	got, err := r.ReadProfileSnapshot(t.Context(), ref)
	if err != nil || got.ProfileRegion() != "us-east-1" {
		t.Errorf("ReadProfileSnapshot(rollback writer) error=%v, want original pair", err)
	}
}

func TestProfileReadCancellationAndPrecedence(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			r, _, ref := profileFixture(t, "delete")
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			want := ErrCanceled
			if deadline {
				ctx, cancel = context.WithTimeout(t.Context(), 20*time.Millisecond)
				t.Cleanup(cancel)
				want = ErrTimeout
			}
			r.afterProfileMetadata = func() {
				if deadline {
					<-ctx.Done()
				} else {
					cancel()
				}
			}
			got, err := r.ReadProfileSnapshot(ctx, ref)
			if !errors.Is(err, want) || got != (ProfileSnapshot{}) {
				t.Errorf("ReadProfileSnapshot(canceled second read) error=%v, want %v and zero value", err, want)
			}
		})
	}
	r, db, ref := profileFixture(t, "delete")
	if _, err := db.Exec(`UPDATE auth_kv SET value=?; DROP TABLE state`, strings.Replace(record, "synthetic-access-secret", "changed", 1)); err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadProfileSnapshot(t.Context(), ref)
	if !errors.Is(err, ErrChanged) || got != (ProfileSnapshot{}) {
		t.Errorf("ReadProfileSnapshot(changed token and missing profile table) error=%v, want token rejection first", err)
	}
	ref.Source = "unsupported"
	r.afterMetadata = func() { t.Error("ReadProfileSnapshot(unsupported reference) read source") }
	if _, err = r.ReadProfileSnapshot(t.Context(), ref); !errors.Is(err, ErrChanged) {
		t.Errorf("ReadProfileSnapshot(unsupported reference) error=%v, want ErrChanged", err)
	}
}

// covers: spec 0003 AC-1, AC-2, AC-8, AC-9.
// An unusable selected profile cannot broaden ordinary token capture or reads.
func TestProfileFailureLeavesTokenOnlyOperationsAvailable(t *testing.T) {
	for _, tc := range []struct {
		name, profile string
		want          error
	}{
		{name: "missing", want: ErrProfileInvalid},
		{name: "malformed", profile: `{"arn":"sentinel-invalid"}`, want: ErrProfileInvalid},
		{name: "unsupported", profile: strings.Replace(profileRecord, "us-east-1", "ap-south-1", 1), want: ErrProfileUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, ref := profileFixture(t, "delete")
			if _, err := db.Exec(`DELETE FROM state`); err != nil {
				t.Fatalf("delete synthetic profile error=%v, want nil", err)
			}
			if tc.profile != "" {
				if _, err := db.Exec(`INSERT INTO state VALUES(?,?)`, selectedProfileKey, tc.profile); err != nil {
					t.Fatalf("insert %s profile error=%v, want nil", tc.name, err)
				}
			}
			// A plausible alternate row must never substitute for the fixed key.
			if _, err := db.Exec(`INSERT INTO state VALUES(?,?)`, "other.selected.profile", profileRecord); err != nil {
				t.Fatalf("insert alternate synthetic profile error=%v, want nil", err)
			}
			got, err := r.ReadProfileSnapshot(t.Context(), ref)
			if !errors.Is(err, tc.want) || got != (ProfileSnapshot{}) {
				t.Errorf("ReadProfileSnapshot(%s selected profile) error=%v snapshot=%v, want %v and zero snapshot without fallback", tc.name, err, got, tc.want)
			}
			r.afterProfileMetadata = func() {
				t.Errorf("token only operation(%s profile) read profile metadata, want token source only", tc.name)
			}
			captured, err := r.Capture(t.Context())
			if err != nil || captured != ref {
				t.Errorf("Capture(%s profile) error=%v reference matches=%t, want nil and unchanged reference", tc.name, err, captured == ref)
			}
			token, err := r.ReadSnapshot(t.Context(), ref)
			if err != nil || token.AccessToken() != "synthetic-access-secret" {
				t.Errorf("ReadSnapshot(%s profile) error=%v token matches=%t, want nil and original token", tc.name, err, token.AccessToken() == "synthetic-access-secret")
			}
		})
	}
}
