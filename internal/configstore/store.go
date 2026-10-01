// Package configstore persists settings atomically under a process lock.
package configstore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/safepath"
)

// Fixed errors keep filesystem paths and saved metadata out of diagnostics.
var (
	ErrUnsafe    = errors.New("unsafe configuration path or permissions; use owned directories (gateway mode 0700) and regular files (mode 0600), without symlinks")
	ErrLocked    = errors.New("Stop the running gateway or wait for the other configuration command, then retry.")
	ErrMissing   = errors.New("no saved configuration; run config init first")
	ErrExists    = errors.New("configuration already exists; use config check")
	ErrRead      = errors.New("could not read configuration; check local file access")
	ErrSave      = errors.New("could not save configuration; previous configuration is unchanged; check local file access and available space")
	ErrUncertain = errors.New("configuration save outcome is uncertain; run config check before retrying")
)

const tempPrefix = ".kiro-gateway-tmp-"

// Store owns a validated directory and, for mutations or serving, its stable lock.
// Calls on one Store are sequential; separate instances coordinate via flock.
type Store struct {
	root *os.Root
	lock *os.File
	// Per instance fault injection keeps recovery tests isolated.
	fault func(string) error
}

// Open creates private paths only when exclusive is true and acquires the lock
// before any configuration read. A readonly absent directory remains absent.
func Open(home string, exclusive bool) (*Store, error) {
	return open(home, exclusive, exclusive)
}

// OpenExisting locks an existing store without creating paths or a lock file.
// It is intended for operations that must not initialize missing configuration.
func OpenExisting(home string) (*Store, error) {
	return open(home, true, false)
}

func open(home string, exclusive, create bool) (*Store, error) {
	r, err := safepath.Home(home)
	if err != nil {
		return nil, ErrUnsafe
	}
	for _, part := range []string{".config", "kiro-gateway"} {
		next, err := safepath.Child(r, part, create, part == "kiro-gateway")
		r.Close()
		if err != nil {
			if exclusive && !create && errors.Is(err, os.ErrNotExist) {
				return nil, ErrMissing
			}
			if !exclusive && errors.Is(err, os.ErrNotExist) {
				return &Store{}, nil
			}
			return nil, ErrUnsafe
		}
		r = next
	}
	s := &Store{root: r}
	if exclusive {
		flags := os.O_RDWR
		if create {
			flags |= os.O_CREATE
		}
		s.lock, err = safepath.File(r, ".lock", flags, true)
		if err != nil {
			s.Close()
			return nil, ErrUnsafe
		}
		if err := syscall.Flock(int(s.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			s.Close()
			if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
				return nil, ErrLocked
			}
			return nil, ErrUnsafe
		}
	}
	return s, nil
}

// Close releases the process lock without deleting the stable lock file.
func (s *Store) Close() error {
	var err error
	if s.lock != nil {
		err = s.lock.Close()
		s.lock = nil
	}
	if s.root != nil {
		err = errors.Join(err, s.root.Close())
		s.root = nil
	}
	if err != nil {
		return ErrUnsafe
	}
	return nil
}

// Load validates one complete snapshot, returning defaults if the file is absent.
func (s *Store) Load() (config.Document, bool, error) {
	if s.root == nil {
		return config.Default(), false, nil
	}
	f, err := safepath.File(s.root, "config.json", os.O_RDONLY, true)
	if errors.Is(err, os.ErrNotExist) {
		return config.Default(), false, nil
	}
	if err != nil {
		return config.Document{}, false, ErrUnsafe
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, config.MaxBytes+1))
	if err != nil {
		return config.Document{}, false, ErrRead
	}
	d, err := config.Parse(b)
	return d, true, err
}

func (s *Store) point(stage string) error {
	if s.fault != nil {
		return s.fault(stage)
	}
	return nil
}

// Save installs a complete document. initialize uses a hard link so it cannot
// replace a destination created concurrently, even by an uncooperative editor.
func (s *Store) Save(d config.Document, initialize bool) error {
	if s.root == nil || s.lock == nil {
		return ErrSave
	}
	b, err := config.Encode(d)
	if err != nil {
		return err
	}
	// CreateTemp supplies unpredictable names. Check its open descriptor against
	// the anchored directory before writing; same user path replacement is outside
	// the local authority boundary. All install and cleanup operations use Root.
	f, err := os.CreateTemp(s.root.Name(), tempPrefix)
	if err != nil {
		return ErrSave
	}
	name := filepath.Base(f.Name())
	defer f.Close()
	defer s.root.Remove(name)
	info, err := f.Stat()
	if err != nil || safepath.Check(info, false, true) != nil {
		return ErrSave
	}
	anchored, err := s.root.Lstat(name)
	if err != nil || !os.SameFile(info, anchored) {
		return ErrSave
	}
	if err := s.point("write"); err != nil {
		return ErrSave
	}
	if n, err := f.Write(b); err != nil || n != len(b) {
		return ErrSave
	}
	if err := s.point("file_sync"); err != nil {
		return ErrSave
	}
	if err := f.Sync(); err != nil {
		return ErrSave
	}
	if err := f.Close(); err != nil {
		return ErrSave
	}
	if err := s.point("install"); err != nil {
		return ErrSave
	}
	if initialize {
		err = s.root.Link(name, "config.json")
	} else {
		// Refuse an unsafe target rather than automatically replacing it.
		target, openErr := safepath.File(s.root, "config.json", os.O_RDONLY, true)
		if openErr != nil {
			return ErrUnsafe
		}
		target.Close()
		err = s.root.Rename(name, "config.json")
	}
	if err != nil {
		if initialize && errors.Is(err, os.ErrExist) {
			return ErrExists
		}
		return ErrSave
	}
	if err := s.point("installed"); err != nil {
		return ErrUncertain
	}
	if initialize {
		if err := s.point("unlink"); err != nil {
			return ErrUncertain
		}
		if err := s.root.Remove(name); err != nil {
			return ErrUncertain
		}
	}
	if err := s.Cleanup(); err != nil {
		return ErrUncertain
	}
	return nil
}

// Cleanup removes only recognizable, private, regular temporary files. It is
// called on successful mutators, including commands whose document is unchanged.
func (s *Store) Cleanup() error {
	if s.root == nil || s.lock == nil {
		return ErrSave
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return ErrSave
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return ErrSave
	}
	for _, e := range entries {
		if !temporaryName(e.Name()) {
			continue
		}
		f, err := safepath.File(s.root, e.Name(), os.O_RDONLY, true)
		if err != nil {
			return ErrUnsafe
		}
		f.Close()
		if err := s.root.Remove(e.Name()); err != nil {
			return ErrSave
		}
	}
	if err := s.point("directory_sync"); err != nil {
		return ErrSave
	}
	if err := dir.Sync(); err != nil {
		return ErrSave
	}
	return nil
}

func temporaryName(name string) bool {
	suffix, ok := strings.CutPrefix(name, tempPrefix)
	if !ok || len(suffix) == 0 || len(suffix) > 10 {
		return false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
