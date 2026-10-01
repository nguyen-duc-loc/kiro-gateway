package config_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"kiro-gateway/internal/config"
)

func TestParseRejectsAmbiguousOrInvalidDocuments(t *testing.T) {
	cases := map[string]string{
		"missing version":    `{}`,
		"null version":       `{"schema_version":null}`,
		"fractional version": `{"schema_version":1.0}`,
		"case variant":       `{"schema_version":1,"Listen":"127.0.0.1:0"}`,
		"unknown secret":     `{"schema_version":1,"token":"sentinel"}`,
		"duplicate version":  `{"schema_version":1,"schema_version":1}`,
		"escaped duplicate":  `{"schema_version":1,"\u0073chema_version":1}`,
		"null listen":        `{"schema_version":1,"listen":null}`,
		"empty listen":       `{"schema_version":1,"listen":""}`,
		"external listen":    `{"schema_version":1,"listen":"0.0.0.0:80"}`,
		"hostname":           `{"schema_version":1,"listen":"localhost:80"}`,
		"IPv6":               `{"schema_version":1,"listen":"[::1]:80"}`,
		"null models":        `{"schema_version":1,"models":null}`,
		"unowned models":     `{"schema_version":1,"models":{"opus":"upstream"}}`,
		"duplicate model":    `{"schema_version":1,"models":{"x":"a","x":"b"}}`,
		"incomplete session": `{"schema_version":1,"session":{}}`,
		"trailing JSON":      `{"schema_version":1} {}`,
		"array":              `[]`,
		"null":               `null`,
		"invalid UTF8":       "{\"schema_version\":1,\"x\":\"\xff\"}",
		"size":               strings.Repeat(" ", config.MaxBytes) + `{}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := config.Parse([]byte(input))
			if !errors.Is(err, config.ErrInvalid) {
				t.Errorf("Parse(%s) error = %v, want ErrInvalid", name, err)
			}
			if strings.Contains(err.Error(), "sentinel") {
				t.Error("Parse(secret) exposed input, want fixed category")
			}
		})
	}
}

func TestDefaultsVersionsAndModelValidation(t *testing.T) {
	for _, input := range []string{`{"schema_version":1}`, `{"schema_version":1,"session":null,"models":{}}`} {
		d, err := config.Parse([]byte(input))
		if err != nil {
			t.Fatalf("Parse(%q) error = %v, want nil", input, err)
		}
		if d.Listen != config.DefaultListen || d.Session != nil || len(d.Models) != 0 {
			t.Errorf("Parse(%q) = %+v, want defaults", input, d)
		}
	}
	for _, input := range []string{`{"schema_version":0}`, `{"schema_version":2}`} {
		if _, err := config.Parse([]byte(input)); !errors.Is(err, config.ErrVersion) {
			t.Errorf("Parse(%q) error = %v, want ErrVersion", input, err)
		}
	}
	d := config.Default()
	d.Session = &config.Session{Source: config.Source, Fingerprint: strings.Repeat("a", 64)}
	d.Models = map[string]string{"Opus": "exact-ID", "opus": "exact-ID"}
	d.Listen = "127.42.0.1:0"
	if _, err := config.Encode(d); err != nil {
		t.Fatalf("Encode(valid exact model names) = %v, want nil", err)
	}
	for _, invalid := range []string{"", "space name", "\t", "é", strings.Repeat("x", 257)} {
		d.Models = map[string]string{invalid: "upstream"}
		if err := d.Validate(); err == nil {
			t.Errorf("Validate(model key %q) = nil, want error", invalid)
		}
		d.Models = map[string]string{"client": invalid}
		if err := d.Validate(); err == nil {
			t.Errorf("Validate(model value %q) = nil, want error", invalid)
		}
	}
	d.Models = make(map[string]string)
	for i := range 32 {
		d.Models[fmt.Sprintf("client%d", i)] = strings.Repeat("x", 256)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("Validate(32 maximum length mappings) = %v, want nil", err)
	}
	d.Models["extra"] = "upstream"
	if err := d.Validate(); err == nil {
		t.Error("Validate(33 mappings) = nil, want error")
	}
}

func TestSessionTransitions(t *testing.T) {
	d := config.Default()
	s := config.Session{Source: config.Source, Fingerprint: strings.Repeat("a", 64)}
	if !d.Link(s) {
		t.Error("Link(first) = false, want true")
	}
	d.Models["opus"] = "model"
	if d.Link(s) || d.Models["opus"] != "model" {
		t.Error("Link(same) changed mappings, want preservation")
	}
	s.Fingerprint = strings.Repeat("b", 64)
	if !d.Link(s) || len(d.Models) != 0 {
		t.Error("Link(changed) did not clear mappings")
	}
	d.Models["sonnet"] = "model"
	if !d.Forget() || d.Session != nil || len(d.Models) != 0 || d.Forget() {
		t.Error("Forget(repeated) violates idempotent removal")
	}
}
