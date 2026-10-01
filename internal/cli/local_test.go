package cli

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
)

func command(t *testing.T, home string, args ...string) (string, string, error) {
	t.Helper()
	var out, logs bytes.Buffer
	err := run(t.Context(), args, func(string) string { t.Error("local command read token environment"); return "" }, &out, &logs, "test", func() (string, error) { return home, nil })
	return out.String(), logs.String(), err
}

// covers: spec 0002 AC-3, AC-4, AC-5, AC-6, AC-8.
func TestFailedRelinkPreservesReferenceAndMappings(t *testing.T) {
	home := t.TempDir()
	if _, _, err := command(t, home, "config", "init"); err != nil {
		t.Fatalf("config init error = %v, want nil", err)
	}
	d := config.Default()
	d.Listen = "127.9.8.7:0"
	d.Session = &config.Session{Source: config.Source, Fingerprint: strings.Repeat("a", 64)}
	d.Models = map[string]string{"Opus": "exact-model", "opus": "exact-model"}
	before, err := config.Encode(d)
	if err != nil {
		t.Fatalf("Encode(linked fixture) error = %v, want nil", err)
	}
	before = append(before, ' ', '\n')
	path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatalf("WriteFile(linked fixture) error = %v, want nil", err)
	}
	// An absent source must not prevent checking or forgetting a saved reference.
	out, logs, err := command(t, home, "account", "link")
	if !errors.Is(err, credentials.ErrSource) {
		t.Errorf("account link(missing source) error = %v, want ErrSource", err)
	}
	if out != "" || !strings.Contains(logs, "error_category=source_unavailable") {
		t.Errorf("account link(missing source) output = %q, logs = %q, want no success output and source_unavailable category", out, logs)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(after failed relink) error = %v, want nil", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("account link(missing source) settings = %q, want %q", after, before)
	}
	if _, _, err := command(t, home, "config", "check"); err != nil {
		t.Errorf("config check(linked with missing source) error = %v, want nil", err)
	}
	forgetOut, forgetLogs, err := command(t, home, "account", "forget")
	if err != nil {
		t.Fatalf("account forget(missing source) error = %v, want nil", err)
	}
	for _, sentinel := range []string{d.Session.Fingerprint, "exact-model", home} {
		if strings.Contains(out+logs+forgetOut+forgetLogs, sentinel) {
			t.Errorf("link/forget output contains %q, want sanitized diagnostics", sentinel)
		}
	}
	after, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(after forget) error = %v, want nil", err)
	}
	got, err := config.Parse(after)
	if err != nil {
		t.Fatalf("Parse(after forget) error = %v, want nil", err)
	}
	if got.Session != nil || len(got.Models) != 0 || got.Listen != d.Listen {
		t.Errorf("account forget(missing source) document = %+v, want no reference or mappings and listen %q", got, d.Listen)
	}
}

// covers: spec 0002 AC-2, AC-7. Unsupported versions fail without a rewrite.
func TestUpgradeRefusesMissingAndUnknownVersions(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		home := t.TempDir()
		out, _, err := command(t, home, "config", "upgrade")
		if !errors.Is(err, configstore.ErrMissing) || out != "" {
			t.Errorf("config upgrade(missing) = %q, %v, want empty output, ErrMissing", out, err)
		}
		path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Stat(after refused upgrade) error = %v, want ErrNotExist", err)
		}
	})
	for _, input := range []string{`{"schema_version":0}`, `{"schema_version":2}`} {
		t.Run(input, func(t *testing.T) {
			home := t.TempDir()
			if _, _, err := command(t, home, "config", "init"); err != nil {
				t.Fatalf("config init error = %v, want nil", err)
			}
			path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatalf("WriteFile(unknown version) error = %v, want nil", err)
			}
			out, _, err := command(t, home, "config", "upgrade")
			if !errors.Is(err, config.ErrVersion) || out != "" {
				t.Errorf("config upgrade(%s) = %q, %v, want empty output, ErrVersion", input, out, err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != input {
				t.Errorf("ReadFile(after refused upgrade) = %q, %v, want %q, nil", got, err, input)
			}
		})
	}
}

// covers: spec 0002 AC-2, AC-3, AC-4, AC-5, AC-8.
func TestLocalCommandsAndRemoval(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
	for _, args := range [][]string{{"config", "check"}, {"account", "forget"}} {
		if _, _, err := command(t, home, args...); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("command(%v) created config, want absent", args)
		}
	}
	if _, _, err := command(t, home, "account", "link"); !errors.Is(err, configstore.ErrMissing) {
		t.Errorf("account link(absent) = %v, want ErrMissing", err)
	}
	if _, _, err := command(t, home, "config", "init"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"config", "upgrade"}, {"config", "check"}, {"account", "forget"}} {
		if _, _, err := command(t, home, args...); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Errorf("command(%v) rewrote unchanged document", args)
		}
	}
	databasePath := filepath.Join(home, "Library", "Application Support", "kiro-cli", "data.sqlite3")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0700); err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Path: databasePath}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE auth_kv(key TEXT PRIMARY KEY,value TEXT)`); err != nil {
		t.Fatal(err)
	}
	const record = `{"access_token":"synthetic-secret-sentinel","expires_at":"2099-01-01T00:00:00Z","region":"region-sentinel","start_url":"https://account-sentinel.example/","conversation":"conversation-sentinel"}`
	if _, err := db.Exec(`INSERT INTO auth_kv VALUES(?,?)`, "kirocli:odic:token", record); err != nil {
		t.Fatal(err)
	}
	var allOutput string
	out, logs, err := command(t, home, "account", "link")
	if err != nil {
		t.Fatal(err)
	}
	allOutput += out + logs
	if out != "Saved IAM Identity Center session linked; model mappings are empty.\n" {
		t.Errorf("account link stdout = %q, want documented result", out)
	}
	raw, _ := os.ReadFile(path)
	d, err := config.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := d.Session.Fingerprint
	d.Models["opus"] = "upstream-id"
	raw, err = config.Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberate formatting proves no op commands preserve the exact bytes.
	raw = append(raw, ' ', '\n')
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, logs, err = command(t, home, "account", "link")
	if err != nil {
		t.Fatal(err)
	}
	allOutput += out + logs
	if out != "Session is already linked; no changes made.\n" {
		t.Errorf("same link stdout = %q", out)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Error("same link changed saved bytes")
	}
	if _, err := db.Exec(`UPDATE auth_kv SET value=?`, " "+record); err != nil {
		t.Fatal(err)
	}
	out, logs, err = command(t, home, "account", "link")
	if err != nil {
		t.Fatal(err)
	}
	allOutput += out + logs
	after, _ = os.ReadFile(path)
	d, err = config.Parse(after)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Models) != 0 || d.Session.Fingerprint == fingerprint {
		t.Error("changed link did not replace reference and clear mappings")
	}
	for range 2 {
		out, logs, err = command(t, home, "account", "forget")
		if err != nil {
			t.Fatal(err)
		}
		allOutput += out + logs
	}
	after, _ = os.ReadFile(path)
	d, err = config.Parse(after)
	if err != nil {
		t.Fatal(err)
	}
	if d.Session != nil || len(d.Models) != 0 {
		t.Error("forget retained session or mappings")
	}
	var source string
	if err := db.QueryRow(`SELECT value FROM auth_kv`).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != " "+record {
		t.Error("link or forget changed Kiro fixture")
	}
	for _, sentinel := range []string{"synthetic-secret-sentinel", "region-sentinel", "account-sentinel", "conversation-sentinel", fingerprint} {
		if strings.Contains(allOutput, sentinel) {
			t.Errorf("routine output contains %q, want sanitized output", sentinel)
		}
	}
	err = filepath.WalkDir(filepath.Dir(path), func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, sentinel := range []string{"synthetic-secret-sentinel", "region-sentinel", "account-sentinel", "conversation-sentinel", fingerprint} {
			if bytes.Contains(b, []byte(sentinel)) {
				t.Errorf("managed file %s retains %q after forget", e.Name(), sentinel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// covers: spec 0002 AC-3, AC-6, AC-7, AC-8.
func TestFailedCaptureAndUnsupportedVersionPreserveSettings(t *testing.T) {
	home := t.TempDir()
	if _, _, err := command(t, home, "config", "init"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
	before, _ := os.ReadFile(path)
	if _, _, err := command(t, home, "account", "link"); err == nil {
		t.Error("link(missing source) = nil, want error")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("failed link changed settings")
	}
	unknown := []byte(`{"schema_version":999,"secret":"must-not-echo"}`)
	if err := os.WriteFile(path, unknown, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"config", "upgrade"}, {"config", "check"}, {"account", "forget"}, {"account", "link"}, {"serve", "--listen", "127.0.0.1:0"}} {
		out, logs, err := command(t, home, args...)
		if err == nil {
			t.Errorf("command(%v invalid config) = nil, want error", args)
		}
		if strings.Contains(out+logs, "must-not-echo") {
			t.Error("invalid config input leaked")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(unknown, after) {
			t.Errorf("command(%v) changed unsupported settings", args)
		}
	}
}

// covers: spec 0002 AC-2, AC-8.
func TestLocalHelpAndArgumentsDoNotOpenSettings(t *testing.T) {
	for _, args := range [][]string{{"config", "--help"}, {"config", "init", "--help"}, {"account", "link", "--help"}, {"version"}, {"help"}} {
		var out bytes.Buffer
		err := run(context.Background(), args, func(string) string { t.Fatal("unexpected environment access"); return "" }, &out, &out, "dev", func() (string, error) { t.Fatal("unexpected home access"); return "", nil })
		if err != nil {
			t.Errorf("run(%v) = %v, want nil", args, err)
		}
	}
	for _, args := range [][]string{{"config"}, {"config", "unknown-sentinel"}, {"config", "init", "--token", "secret-sentinel"}, {"account", "forget", "secret-sentinel"}} {
		out, logs, err := command(t, "nonexistent-home", args...)
		if err == nil {
			t.Errorf("command(%v) = nil, want error", args)
			continue
		}
		if strings.Contains(out+logs+err.Error(), "sentinel") {
			t.Error("unknown arguments leaked")
		}
	}
}
