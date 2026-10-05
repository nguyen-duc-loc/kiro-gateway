//go:build schemaprobe

package kiro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type schemaPreflightFixture struct {
	repo, planPath, commit string
	plan                   schemaPlan
}

func schemaPreflightWrite(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile(%q) = %v, want fixture bytes written", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func schemaPreflightGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "rtk", append([]string{"proxy", "git", "-C", repo}, args...)...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git(%v in fixture) = %v, output=%s, want success", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// The real preflight sees a clean temporary repository and inert software
// files. Serial tests restore the process directory, environment, and flags.
// The real launch flag stays disabled throughout.
func newSchemaPreflightFixture(t *testing.T) *schemaPreflightFixture {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	for _, dir := range []string{filepath.Join(repo, "internal", "kiro"), filepath.Join(root, "home")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("MkdirAll(%q) = %v, want fixture directory", dir, err)
		}
	}
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "home", ".config"))
	for key, value := range map[string]string{"GOTOOLCHAIN": "local", "GOWORK": "off", "CGO_ENABLED": "1", "GOFLAGS": ""} {
		t.Setenv(key, value)
	}
	schemaPreflightWrite(t, filepath.Join(repo, "internal", "kiro", "fixture_test.go"), []byte("package kiro\n"))
	schemaPreflightGit(t, repo, "init", "--template=", "--initial-branch=fixture")
	schemaPreflightGit(t, repo, "config", "core.hooksPath", "/dev/null")
	schemaPreflightGit(t, repo, "add", ".")
	schemaPreflightGit(t, repo, "-c", "user.name=Schema Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--no-gpg-sign", "-m", "fixture")
	f := &schemaPreflightFixture{repo: repo, planPath: filepath.Join(root, "plan.json"), commit: schemaPreflightGit(t, repo, "rev-parse", "HEAD")}
	f.plan = schemaPlan{
		Version: 1, CodeCommit: f.commit, Contract: schemaContract(),
		SyntheticChecks:   []string{"scripts/check:pass", "schema_probe_race:pass"},
		IndependentReview: schemaPtr("synthetic review only"),
		LiveApproval:      schemaPtr("approved_for_one_catalogue_run"),
		Command:           []string{"rtk", "proxy", "env", "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED=1", "GOFLAGS=", "go", "test", "-mod=readonly", "-tags=schemaprobe", "./internal/kiro", "-run=^TestSchemaProbe$", "-count=1", "-v", "-args", "-schema-probe-launch", "-schema-probe-plan=" + f.planPath, "-schema-probe-plan-sha256=<PLAN_SHA256>", "-schema-probe-code-commit=" + f.commit},
	}
	for _, role := range []string{"native_binary", "bundled_source"} {
		path := filepath.Join(root, role)
		digest := schemaPreflightWrite(t, path, []byte("inert synthetic "+role))
		f.plan.Software = append(f.plan.Software, schemaSoftware{Role: role, Path: path, SHA256: digest})
	}
	oldPath, oldDigest, oldCommit := *schemaPlanPath, *schemaPlanDigest, *schemaCodeCommit
	t.Cleanup(func() { *schemaPlanPath, *schemaPlanDigest, *schemaCodeCommit = oldPath, oldDigest, oldCommit })
	*schemaPlanPath, *schemaCodeCommit = f.planPath, f.commit
	f.save(t)
	t.Chdir(filepath.Join(repo, "internal", "kiro"))
	return f
}

func (f *schemaPreflightFixture) save(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(f.plan)
	if err != nil {
		t.Fatalf("Marshal(synthetic plan) = %v, want valid JSON", err)
	}
	*schemaPlanDigest = schemaPreflightWrite(t, f.planPath, data)
}

func schemaPreflightRejectsBeforeAccess(t *testing.T, ctx context.Context) {
	t.Helper()
	var homes, opens, readers, dials int
	r := runSchemaProbe(ctx, true, schemaDependencies{
		preflight: schemaLivePreflight,
		home: func() (string, error) {
			homes++
			return "", errPlanInvalid
		},
		open: func(string) (schemaStore, error) {
			opens++
			return nil, errPlanInvalid
		},
		reader: func(string) ProfileReader {
			readers++
			return nil
		},
		wire: transport{dial: func(context.Context, string) (net.Conn, error) {
			dials++
			return nil, errPlanInvalid
		}},
	})
	if r.FailureCategory == nil || *r.FailureCategory != "preflight" || r.RunID != nil || r.CodeCommit != nil || r.PlanDigest != nil || r.DispatchCount != 0 || r.Outcome != "needs_evidence" || r.CleanupOutcome != "complete" {
		t.Errorf("runSchemaProbe(rejected artifact) = %+v, want preflight without provenance or dispatch", r)
	}
	if homes != 0 || opens != 0 || readers != 0 || dials != 0 {
		t.Errorf("runSchemaProbe(rejected artifact) homes=%d opens=%d readers=%d dials=%d, want all zero", homes, opens, readers, dials)
	}
}

// covers: AC-16. A matching artifact passes the actual preflight and reaches
// the complete synthetic settings, snapshot, TLS, and structural report path.
func TestSchemaLivePreflightMatchingArtifact(t *testing.T) {
	f := newSchemaPreflightFixture(t)
	home, _ := probeHome(t)
	d := schemaFixtureDependencies(t, home)
	d.preflight = schemaLivePreflight
	d.wire = schemaTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, schemaFixture)
	})
	r := runSchemaProbe(t.Context(), true, d)
	if r.FailureCategory != nil || r.Outcome != "schema_observed" || r.CleanupOutcome != "complete" || r.DispatchCount != 1 || r.CodeCommit == nil || *r.CodeCommit != f.commit || r.PlanDigest == nil || *r.PlanDigest != *schemaPlanDigest {
		t.Errorf("runSchemaProbe(matching artifact) = %+v, want one observed request with matching provenance", r)
	}
}

// covers: AC-16. Mutations keep the other artifact inputs valid, so removing
// the command or software digest comparison makes its rejection test fail.
func TestSchemaLivePreflightChangedCommand(t *testing.T) {
	f := newSchemaPreflightFixture(t)
	f.plan.Command = append(f.plan.Command, "-schema-probe-launch=false")
	f.save(t)
	schemaPreflightRejectsBeforeAccess(t, t.Context())
}

func TestSchemaLivePreflightChangedSoftware(t *testing.T) {
	for _, index := range []int{0, 1} {
		t.Run([]string{"native_binary", "bundled_source"}[index], func(t *testing.T) {
			f := newSchemaPreflightFixture(t)
			schemaPreflightWrite(t, f.plan.Software[index].Path, []byte("changed software bytes"))
			schemaPreflightRejectsBeforeAccess(t, t.Context())
		})
	}
}

func TestSchemaLivePreflightStalePlanDigest(t *testing.T) {
	f := newSchemaPreflightFixture(t)
	data, err := os.ReadFile(f.planPath)
	if err != nil {
		t.Fatalf("ReadFile(synthetic plan) = %v, want bytes", err)
	}
	schemaPreflightWrite(t, f.planPath, append(data, '\n'))
	schemaPreflightRejectsBeforeAccess(t, t.Context())
}

func TestSchemaLivePreflightChangedCommit(t *testing.T) {
	f := newSchemaPreflightFixture(t)
	schemaPreflightGit(t, f.repo, "-c", "user.name=Schema Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--no-gpg-sign", "--allow-empty", "-m", "changed commit")
	schemaPreflightRejectsBeforeAccess(t, t.Context())
}

func TestSchemaLivePreflightDirtyCode(t *testing.T) {
	f := newSchemaPreflightFixture(t)
	schemaPreflightWrite(t, filepath.Join(f.repo, "internal", "kiro", "fixture_test.go"), []byte("package changed\n"))
	schemaPreflightRejectsBeforeAccess(t, t.Context())
}

func TestSchemaLivePreflightCanceled(t *testing.T) {
	newSchemaPreflightFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	schemaPreflightRejectsBeforeAccess(t, ctx)
}

func TestSchemaLivePreflightEnvironment(t *testing.T) {
	for key, value := range map[string]string{"GOTOOLCHAIN": "auto", "GOWORK": "", "CGO_ENABLED": "0", "GOFLAGS": "-mod=mod"} {
		t.Run(key, func(t *testing.T) {
			newSchemaPreflightFixture(t)
			t.Setenv(key, value)
			schemaPreflightRejectsBeforeAccess(t, t.Context())
		})
	}
}
