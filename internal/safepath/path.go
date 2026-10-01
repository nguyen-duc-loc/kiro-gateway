// Package safepath opens user owned directories without following appended symlinks.
package safepath

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// ErrUnsafe never reveals a path or file contents.
var ErrUnsafe = errors.New("unsafe local path, ownership, or permissions")

// Check verifies ownership, type, and access on descriptor metadata.
func Check(info os.FileInfo, directory, private bool) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return ErrUnsafe
	}
	if directory {
		if !info.IsDir() {
			return ErrUnsafe
		}
	} else if !info.Mode().IsRegular() {
		return ErrUnsafe
	}
	mask := os.FileMode(0022)
	if private {
		mask = 0077
		if !directory {
			mask |= 0100
		}
	}
	if info.Mode().Perm()&mask != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return ErrUnsafe
	}
	return nil
}

// Home resolves the home itself to an absolute canonical directory.
func Home(home string) (*os.Root, error) {
	absolute, err := filepath.Abs(home)
	if err != nil {
		return nil, ErrUnsafe
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, ErrUnsafe
	}
	r, err := os.OpenRoot(canonical)
	if err != nil {
		return nil, ErrUnsafe
	}
	f, err := r.Open(".")
	if err != nil {
		r.Close()
		return nil, ErrUnsafe
	}
	info, err := f.Stat()
	f.Close()
	if err != nil || Check(info, true, false) != nil {
		r.Close()
		return nil, ErrUnsafe
	}
	return r, nil
}

// Child opens a direct child anchored to its validated parent. Missing paths can
// be created explicitly; existing access permissions are never repaired.
func Child(parent *os.Root, name string, create, private bool) (*os.Root, error) {
	if create {
		if err := parent.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, ErrUnsafe
		}
	}
	before, err := parent.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, ErrUnsafe
	}
	if Check(before, true, private) != nil {
		return nil, ErrUnsafe
	}
	f, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, ErrUnsafe
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || Check(info, true, private) != nil || !os.SameFile(before, info) {
		return nil, ErrUnsafe
	}
	r, err := parent.OpenRoot(name)
	if err != nil {
		return nil, ErrUnsafe
	}
	opened, err := r.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		r.Close()
		return nil, ErrUnsafe
	}
	return r, nil
}

// File opens a regular file with no symlink following and validates its descriptor.
func File(root *os.Root, name string, flags int, private bool) (*os.File, error) {
	// Root may resolve a relative symlink itself even with O_NOFOLLOW. Reject it
	// explicitly and compare the final path with the opened descriptor.
	before, err := root.Lstat(name)
	if err == nil {
		if Check(before, false, private) != nil {
			return nil, ErrUnsafe
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, ErrUnsafe
	}
	f, err := root.OpenFile(name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, ErrUnsafe
	}
	info, err := f.Stat()
	if err != nil || Check(info, false, private) != nil {
		f.Close()
		return nil, ErrUnsafe
	}
	after, err := root.Lstat(name)
	if err != nil || Check(after, false, private) != nil || !os.SameFile(info, after) || before != nil && !os.SameFile(before, info) {
		f.Close()
		return nil, ErrUnsafe
	}
	return f, nil
}
