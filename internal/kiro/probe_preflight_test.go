package kiro

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var errBaselineChanged = errors.New("baseline_changed")

type probeCommand func(context.Context, string, ...string) (string, error)

func checkProbeCode(ctx context.Context, commit string, inputs []string, run probeCommand) error {
	if len(commit) != 40 && len(commit) != 64 || strings.Trim(commit, "0123456789abcdef") != "" || len(inputs) == 0 {
		return errCodeChanged
	}
	head, err := run(ctx, "git", "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(head) != commit {
		return errCodeChanged
	}
	status, err := run(ctx, "git", "status", "--porcelain=v1", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil || strings.TrimSpace(status) != "" {
		return errCodeChanged
	}
	tracked, err := run(ctx, "git", "ls-files", "-z", "--", "internal/kiro")
	if err != nil {
		return errCodeChanged
	}
	known := make(map[string]bool)
	for _, name := range strings.Split(tracked, "\x00") {
		known[name] = true
	}
	for _, name := range inputs {
		if !known[name] {
			return errCodeChanged
		}
	}
	return nil
}

// Enumerating actual inputs also detects ignored files that git status omits.
// Symlinks cannot silently substitute a probe source or plan outside the commit.
func probeInputFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(root, "internal", "kiro"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&os.ModeSymlink != 0 {
			return errCodeChanged
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() || len(files) >= 1024 {
			return errCodeChanged
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return errCodeChanged
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil || len(files) == 0 {
		return nil, errCodeChanged
	}
	return files, nil
}

type probeEnvironment struct {
	runtime, goMod, toolchain, workspace, cgo, flags string
}

func realProbeEnvironment() probeEnvironment {
	f, err := os.Open("../../go.mod")
	if err != nil {
		return probeEnvironment{}
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return probeEnvironment{}
	}
	return probeEnvironment{runtime: runtime.Version(), goMod: string(b), toolchain: os.Getenv("GOTOOLCHAIN"), workspace: os.Getenv("GOWORK"), cgo: os.Getenv("CGO_ENABLED"), flags: os.Getenv("GOFLAGS")}
}

func checkProbeBaseline(ctx context.Context, env probeEnvironment, run probeCommand) error {
	version := ""
	for _, line := range strings.Split(env.goMod, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			if version != "" {
				return errBaselineChanged
			}
			version = "go" + fields[1]
		}
	}
	if version == "" || env.runtime != version || env.toolchain != "local" || env.workspace != "off" || env.cgo != "1" || env.flags != "" {
		return errBaselineChanged
	}
	claude, err := run(ctx, "claude", "--version")
	if err != nil || strings.TrimSpace(claude) != "2.1.285 (Claude Code)" {
		return errBaselineChanged
	}
	kiro, err := run(ctx, "kiro-cli", "--version")
	if err != nil || strings.TrimSpace(kiro) != "kiro-cli 2.8.0" {
		return errBaselineChanged
	}
	return nil
}

// Every command and argument is fixed by its caller. No shell, raw command error
// output, credential commands, or environment endpoint overrides are used.
func realProbeCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = "../.."
	if name == "git" {
		// Git environment overrides must not point the check at another tree.
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "GIT_") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")
	}
	var output probeCommandOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", errCodeChanged
	}
	return output.String(), nil
}

type probeCommandOutput struct{ buffer bytes.Buffer }

func (b *probeCommandOutput) String() string { return b.buffer.String() }

func (b *probeCommandOutput) Write(p []byte) (int, error) {
	if len(p) > (64<<10)-b.buffer.Len() {
		return 0, errCodeChanged
	}
	return b.buffer.Write(p)
}

func TestProbeCodeChecksActualInputsAndCleanCommit(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, head, status, tracked string
		want                        error
	}{
		{"clean", commit, "", "internal/kiro/probe_live_test.go\x00", nil},
		{"different commit", strings.Repeat("b", 40), "", "", errCodeChanged},
		{"staged", commit, "M  internal/kiro/probe_live_test.go", "", errCodeChanged},
		{"unstaged", commit, " M internal/kiro/probe_live_test.go", "", errCodeChanged},
		{"untracked", commit, "?? unrelated-file", "", errCodeChanged},
		{"ignored probe input", commit, "", "", errCodeChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, name string, args ...string) (string, error) {
				if name != "git" {
					t.Error("code check invoked a non Git command")
				}
				switch args[0] {
				case "rev-parse":
					return tc.head, nil
				case "status":
					return tc.status, nil
				case "ls-files":
					return tc.tracked, nil
				}
				return "", errors.New("sentinel-command-error")
			}
			err := checkProbeCode(t.Context(), commit, []string{"internal/kiro/probe_live_test.go"}, run)
			if !errors.Is(err, tc.want) {
				t.Errorf("checkProbeCode(%s) error=%v, want %v", tc.name, err, tc.want)
			}
		})
	}
}

func TestProbeInputInventoryRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "kiro")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := probeInputFiles(root)
	if err != nil || len(files) != 1 || files[0] != "internal/kiro/probe_test.go" {
		t.Errorf("probeInputFiles(regular) = %v,%v, want one tracked candidate", files, err)
	}
	if err := os.Symlink("probe_test.go", filepath.Join(dir, "ignored_test.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := probeInputFiles(root); !errors.Is(err, errCodeChanged) {
		t.Errorf("probeInputFiles(symlink) error=%v, want code_changed", err)
	}
}

func TestProbeBaselineDriftIsSanitized(t *testing.T) {
	base := probeEnvironment{runtime: "go1.27.1", goMod: "module fixture\n\ngo 1.27.1\n", toolchain: "local", workspace: "off", cgo: "1"}
	for _, tc := range []struct {
		name         string
		change       func(*probeEnvironment)
		claude, kiro string
		want         error
	}{
		{"exact", func(*probeEnvironment) {}, "2.1.285 (Claude Code)", "kiro-cli 2.8.0", nil},
		{"Go", func(e *probeEnvironment) { e.runtime = "go1.26.0" }, "", "", errBaselineChanged},
		{"workspace", func(e *probeEnvironment) { e.workspace = "" }, "", "", errBaselineChanged},
		{"download", func(e *probeEnvironment) { e.toolchain = "auto" }, "", "", errBaselineChanged},
		{"cgo", func(e *probeEnvironment) { e.cgo = "0" }, "", "", errBaselineChanged},
		{"flags", func(e *probeEnvironment) { e.flags = "-mod=mod" }, "", "", errBaselineChanged},
		{"Claude", func(*probeEnvironment) {}, "sentinel-version", "kiro-cli 2.8.0", errBaselineChanged},
		{"Kiro", func(*probeEnvironment) {}, "2.1.285 (Claude Code)", "sentinel-version", errBaselineChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := base
			tc.change(&env)
			run := func(_ context.Context, name string, args ...string) (string, error) {
				if len(args) != 1 || args[0] != "--version" {
					t.Error("baseline check used a non version command")
				}
				if name == "claude" {
					return tc.claude, nil
				}
				if name == "kiro-cli" {
					return tc.kiro, nil
				}
				return "", errors.New("sentinel-error")
			}
			if err := checkProbeBaseline(t.Context(), env, run); !errors.Is(err, tc.want) {
				t.Errorf("checkProbeBaseline(%s) error=%v, want %v", tc.name, err, tc.want)
			}
		})
	}
}

func TestProbeCommandOutputCannotBypassLimit(t *testing.T) {
	var output probeCommandOutput
	// LimitReader prevents the source from supplying WriteTo, exercising the
	// io.Copy path that would bypass Write through an embedded Buffer.ReadFrom.
	r := io.LimitReader(strings.NewReader(strings.Repeat("x", 65<<10)), 65<<10)
	_, err := io.Copy(&output, r)
	if !errors.Is(err, errCodeChanged) || output.buffer.Len() > 64<<10 {
		t.Errorf("command output limit error=%v bytes=%d, want code_changed within 64 KiB", err, output.buffer.Len())
	}
}
