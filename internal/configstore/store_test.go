package configstore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kiro-gateway/internal/config"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	home := t.TempDir()
	s, err := Open(home, true)
	if err != nil {
		t.Fatalf("Open(temporary home) = %v, want nil", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, filepath.Join(home, ".config", "kiro-gateway", "config.json")
}

// covers: spec 0002 AC-2, AC-5, AC-6. Rejected documents leave the linked state intact.
func TestSaveInvalidDocumentPreservesLinkedSettings(t *testing.T) {
	s, path := newStore(t)
	d := config.Default()
	d.Session = &config.Session{Source: config.Source, Fingerprint: strings.Repeat("a", 64)}
	d.Models["Opus"] = "exact-model"
	if err := s.Save(d, true); err != nil {
		t.Fatalf("Save(linked fixture) error = %v, want nil", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(saved fixture) error = %v, want nil", err)
	}
	d.Session = nil
	if err := s.Save(d, false); !errors.Is(err, config.ErrInvalid) {
		t.Errorf("Save(mappings without session) error = %v, want ErrInvalid", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(after rejected save) error = %v, want nil", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("Save(invalid document) saved bytes = %q, want %q", after, before)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir(after rejected save) error = %v, want nil", err)
	}
	for _, entry := range entries {
		if entry.Name() != "config.json" && entry.Name() != ".lock" {
			t.Errorf("Save(invalid document) left file %q, want only config.json and .lock", entry.Name())
		}
	}
}

// covers: spec 0002 AC-6. Replacement and cleanup must preserve the lock inode.
func TestSaveAndCleanupKeepStableLock(t *testing.T) {
	s, path := newStore(t)
	lockPath := filepath.Join(filepath.Dir(path), ".lock")
	before, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("Stat(initial lock) error = %v, want nil", err)
	}
	d := config.Default()
	if err := s.Save(d, true); err != nil {
		t.Fatalf("Save(initial document) error = %v, want nil", err)
	}
	d.Listen = "127.0.0.1:0"
	if err := s.Save(d, false); err != nil {
		t.Fatalf("Save(replacement) error = %v, want nil", err)
	}
	if err := s.Cleanup(); err != nil {
		t.Fatalf("Cleanup(after replacement) error = %v, want nil", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close(after replacement) error = %v, want nil", err)
	}
	after, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("Stat(released lock) error = %v, want nil", err)
	}
	if !os.SameFile(before, after) || after.Size() != 0 {
		t.Errorf("Save/Cleanup/Close lock = same file %t, size %d, want true, 0", os.SameFile(before, after), after.Size())
	}
}

// covers: spec 0002 AC-2, AC-6.
func TestAbsentReadonlyAndAtomicInitialization(t *testing.T) {
	home := t.TempDir()
	s, err := Open(home, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, exists, err := s.Load(); err != nil || exists {
		t.Errorf("Load(absent) = exists %t, error %v, want false, nil", exists, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open(readonly absent) created directory, want none: %v", err)
	}

	w, path := newStore(t)
	if err := w.Save(config.Default(), true); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d := config.Default()
	d.Listen = "127.0.0.1:0"
	if err := w.Save(d, true); !errors.Is(err, ErrExists) {
		t.Errorf("Save(repeated init) = %v, want ErrExists", err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, want) {
		t.Error("Save(repeated init) replaced configuration")
	}
	for _, name := range []string{"config.json", ".lock"} {
		info, err := os.Stat(filepath.Join(filepath.Dir(path), name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("mode(%s) = %o, want 0600", name, info.Mode().Perm())
		}
	}
}

// covers: spec 0002 AC-6, AC-8.
func TestFailureBeforeAndAfterInstallation(t *testing.T) {
	for _, initialize := range []bool{true, false} {
		for _, stage := range []string{"write", "file_sync", "install", "installed", "unlink", "directory_sync"} {
			if !initialize && stage == "unlink" {
				continue
			}
			t.Run(stage+map[bool]string{true: " init", false: " replace"}[initialize], func(t *testing.T) {
				s, path := newStore(t)
				var before []byte
				if !initialize {
					if err := s.Save(config.Default(), true); err != nil {
						t.Fatal(err)
					}
					before, _ = os.ReadFile(path)
				}
				s.fault = func(point string) error {
					if point == stage {
						return errors.New("synthetic I/O failure")
					}
					return nil
				}
				d := config.Default()
				d.Listen = "127.0.0.1:0"
				err := s.Save(d, initialize)
				installed := stage == "installed" || stage == "unlink" || stage == "directory_sync"
				want := ErrSave
				if installed {
					want = ErrUncertain
				}
				if !errors.Is(err, want) {
					t.Errorf("Save(fault %s) = %v, want %v", stage, err, want)
				}
				after, _ := os.ReadFile(path)
				if !installed && !bytes.Equal(after, before) {
					t.Error("Save(before installation failure) changed destination")
				}
				if installed {
					if got, err := config.Parse(after); err != nil || got.Listen != d.Listen {
						t.Errorf("Parse(installed) = %+v, %v, want new complete document", got, err)
					}
				}
			})
		}
	}
}

// covers: spec 0002 AC-6.
func TestInitCannotOverwriteCompetingCreation(t *testing.T) {
	s, path := newStore(t)
	const competing = "editor-owned sentinel"
	s.fault = func(stage string) error {
		if stage == "install" {
			return os.WriteFile(path, []byte(competing), 0600)
		}
		return nil
	}
	if err := s.Save(config.Default(), true); !errors.Is(err, ErrExists) {
		t.Errorf("Save(competing creation) = %v, want ErrExists", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != competing {
		t.Errorf("ReadFile(competing) = %q, want original", got)
	}
}

// covers: spec 0002 AC-6.
func TestCleanupOfInterruptedWrite(t *testing.T) {
	s, path := newStore(t)
	dir := filepath.Dir(path)
	if err := os.WriteFile(filepath.Join(dir, tempPrefix+"12345"), []byte(`{"schema_version":`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"editor.backup", tempPrefix + "not-a-temp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("user backup"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Save(config.Default(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, tempPrefix+"12345")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Save(recovery) retained interrupted temp: %v", err)
	}
	for _, name := range []string{"editor.backup", tempPrefix + "not-a-temp"} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != "user backup" {
			t.Errorf("Save(recovery) changed user file %s: %v", name, err)
		}
	}
	if err := os.Symlink("config.json", filepath.Join(dir, tempPrefix+"6789")); err != nil {
		t.Fatal(err)
	}
	if err := s.Cleanup(); !errors.Is(err, ErrUnsafe) {
		t.Errorf("Cleanup(symlink temp) = %v, want ErrUnsafe", err)
	}
}

// covers: spec 0002 AC-6.
func TestProcessLockExcludesAndReleases(t *testing.T) {
	s, path := newStore(t)
	home := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	if other, err := Open(home, true); !errors.Is(err, ErrLocked) {
		if other != nil {
			other.Close()
		}
		t.Errorf("Open(held lock) = %v, want ErrLocked", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	other, err := Open(home, true)
	if err != nil {
		t.Fatalf("Open(released lock) = %v, want nil", err)
	}
	other.Close()
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".lock")); err != nil {
		t.Errorf("Stat(stable lock) = %v, want nil", err)
	}
}

// covers: spec 0002 AC-6.
func TestRejectUnsafeFilesAndParents(t *testing.T) {
	for _, target := range []string{"config.json", ".lock", "kiro-gateway", ".config"} {
		for _, kind := range []string{"permissions", "symlink", "relative symlink"} {
			t.Run(target+" "+kind, func(t *testing.T) {
				home := t.TempDir()
				s, err := Open(home, true)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Save(config.Default(), true); err != nil {
					t.Fatal(err)
				}
				s.Close()
				path := filepath.Join(home, ".config", "kiro-gateway", target)
				if target == "kiro-gateway" {
					path = filepath.Dir(path)
				}
				if target == ".config" {
					path = filepath.Join(home, ".config")
				}
				if kind == "permissions" {
					if err := os.Chmod(path, 0777); err != nil {
						t.Fatal(err)
					}
				} else {
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
				}
				s, err = Open(home, true)
				if err == nil {
					defer s.Close()
					_, _, err = s.Load()
				}
				if !errors.Is(err, ErrUnsafe) {
					t.Errorf("Open/Load(%s %s) = %v, want ErrUnsafe", target, kind, err)
				}
			})
		}
	}
}
