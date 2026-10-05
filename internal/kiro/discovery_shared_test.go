//go:build schemaprobe || responsediscovery

package kiro

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// schemaJSON checks every member, including discarded catalogue records. The
// depth limit is checked before allocating a container at that depth.
func schemaJSON(data []byte, limit int) (map[string]any, error) {
	invalid := errors.New("invalid_response")
	if len(data) > limit || !utf8.Valid(data) {
		return nil, invalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		tok, err := d.Token()
		if err != nil {
			return nil, invalid
		}
		delim, container := tok.(json.Delim)
		if !container {
			return tok, nil
		}
		if depth >= 64 {
			return nil, invalid
		}
		switch delim {
		case '{':
			m := make(map[string]any)
			for d.More() {
				key, err := d.Token()
				name, ok := key.(string)
				if err != nil || !ok {
					return nil, invalid
				}
				if _, exists := m[name]; exists {
					return nil, invalid
				}
				value, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				m[name] = value
			}
			if end, err := d.Token(); err != nil || end != json.Delim('}') {
				return nil, invalid
			}
			return m, nil
		case '[':
			a := []any{}
			for d.More() {
				value, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				a = append(a, value)
			}
			if end, err := d.Token(); err != nil || end != json.Delim(']') {
				return nil, invalid
			}
			return a, nil
		default:
			return nil, invalid
		}
	}
	v, err := read(0)
	if err != nil {
		return nil, invalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, invalid
	}
	root, ok := v.(map[string]any)
	if !ok {
		return nil, invalid
	}
	return root, nil
}

func schemaHex(s string, length int) bool {
	return len(s) == length && strings.Trim(s, "0123456789abcdef") == ""
}

// The pinned native binary exceeds 1 GiB. Keep hashing bounded while allowing
// the complete installed artifact, with cancellation checked on every read.
const schemaSoftwareLimit int64 = 2 << 30

// schemaHashFile never executes the native binary or bundled source. Regular
// file checks also prevent a named pipe from holding preflight indefinitely.
func schemaHashFile(ctx context.Context, path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > schemaSoftwareLimit {
		return "", errPlanInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errPlanInvalid
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return "", errPlanInvalid
	}
	h := sha256.New()
	buf := make([]byte, 32<<10)
	var total int64
	for {
		if ctx.Err() != nil {
			return "", errPlanInvalid
		}
		n, err := f.Read(buf)
		total += int64(n)
		if total > schemaSoftwareLimit {
			return "", errPlanInvalid
		}
		_, _ = h.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errPlanInvalid
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
