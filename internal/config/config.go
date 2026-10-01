// Package config owns the saved settings model and pure state transitions.
package config

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strings"

	"kiro-gateway/internal/jsonobject"
)

// DefaultListen is the listener used when no setting is present.
const DefaultListen = "127.0.0.1:8787"

// Source identifies the single supported saved Kiro record.
const Source = "kiro_cli_idc_sqlite_v1"

// MaxBytes bounds a saved document.
const MaxBytes = 64 << 10

// ErrInvalid describes malformed settings without exposing their contents.
var ErrInvalid = errors.New("invalid configuration; edit the saved configuration and run config check")

// ErrVersion rejects unsupported versions without attempting an upgrade.
var ErrVersion = errors.New("unsupported configuration version; use a compatible gateway version")

// Session identifies a credential snapshot, not a verified user identity.
type Session struct {
	Source      string `json:"source"`
	Fingerprint string `json:"fingerprint"`
}

// Document holds ordinary settings and a reference, never bearer credentials.
type Document struct {
	SchemaVersion int               `json:"schema_version"`
	Listen        string            `json:"listen"`
	Session       *Session          `json:"session"`
	Models        map[string]string `json:"models"`
}

// Default returns an independent initial document.
func Default() Document {
	return Document{SchemaVersion: 1, Listen: DefaultListen, Models: map[string]string{}}
}

// ValidListen accepts only numeric IPv4 loopback addresses with a port.
func ValidListen(listen string) bool {
	a, err := netip.ParseAddrPort(listen)
	return err == nil && a.Addr().Is4() && a.Addr().IsLoopback()
}

// VisibleASCII reports whether a value meets the exact identifier limits.
func VisibleASCII(s string, max int) bool {
	if len(s) == 0 || len(s) > max {
		return false
	}
	for i := range len(s) {
		if s[i] < '!' || s[i] > '~' {
			return false
		}
	}
	return true
}

// Validate checks all invariants of a complete document.
func (d Document) Validate() error {
	if d.SchemaVersion != 1 {
		return ErrVersion
	}
	if !ValidListen(d.Listen) || d.Models == nil || len(d.Models) > 32 {
		return ErrInvalid
	}
	if d.Session == nil && len(d.Models) != 0 {
		return ErrInvalid
	}
	if d.Session != nil {
		if d.Session.Source != Source || len(d.Session.Fingerprint) != 64 {
			return ErrInvalid
		}
		for _, c := range d.Session.Fingerprint {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return ErrInvalid
			}
		}
	}
	for k, v := range d.Models {
		if !VisibleASCII(k, 256) || !VisibleASCII(v, 256) {
			return ErrInvalid
		}
	}
	return nil
}

// Parse applies defaults only to absent optional fields and rejects ambiguity.
func Parse(data []byte) (Document, error) {
	d := Default()
	o, err := jsonobject.Parse(data, MaxBytes)
	if err != nil {
		return Document{}, ErrInvalid
	}
	for k := range o {
		switch k {
		case "schema_version", "listen", "session", "models":
		default:
			return Document{}, ErrInvalid
		}
	}
	v, ok := o["schema_version"]
	if !ok || string(v) == "null" || json.Unmarshal(v, &d.SchemaVersion) != nil {
		return Document{}, ErrInvalid
	}
	if d.SchemaVersion != 1 {
		return Document{}, ErrVersion
	}
	if v, ok := o["listen"]; ok {
		if string(v) == "null" || json.Unmarshal(v, &d.Listen) != nil {
			return Document{}, ErrInvalid
		}
	}
	if v, ok := o["session"]; ok && string(v) != "null" {
		var s map[string]json.RawMessage
		if json.Unmarshal(v, &s) != nil || len(s) != 2 || s["source"] == nil || s["fingerprint"] == nil {
			return Document{}, ErrInvalid
		}
		if json.Unmarshal(v, &d.Session) != nil {
			return Document{}, ErrInvalid
		}
	}
	if v, ok := o["models"]; ok {
		if string(v) == "null" || json.Unmarshal(v, &d.Models) != nil {
			return Document{}, ErrInvalid
		}
	}
	if err := d.Validate(); err != nil {
		return Document{}, err
	}
	return d, nil
}

// Encode serializes a complete validated document with a final newline.
func Encode(d Document) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, ErrInvalid
	}
	b = append(b, '\n')
	if len(b) > MaxBytes {
		return nil, ErrInvalid
	}
	return b, nil
}

// Link adopts a snapshot and clears mappings only when the reference changes.
func (d *Document) Link(s Session) bool {
	if d.Session != nil && *d.Session == s {
		return false
	}
	d.Session = &s
	d.Models = map[string]string{}
	return true
}

// Forget removes the reference and its mappings. Repeating it is a no op.
func (d *Document) Forget() bool {
	if d.Session == nil && len(d.Models) == 0 {
		return false
	}
	d.Session = nil
	d.Models = map[string]string{}
	return true
}
