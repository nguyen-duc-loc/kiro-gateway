package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"kiro-gateway/internal/cli"
)

// covers: AC-1. Informational commands need no credential or running server.
func TestRun_InformationalCommands(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		version string
		want    string
	}{
		{"default version", []string{"version"}, "", "dev\n"},
		{"release version", []string{"version"}, "1.2.3", "1.2.3\n"},
		{"help command", []string{"help"}, "", "Usage: kiro-gateway"},
		{"long help", []string{"--help"}, "", "Usage: kiro-gateway"},
		{"short help", []string{"-h"}, "", "Usage: kiro-gateway"},
		{"serve help", []string{"serve", "--help"}, "", "Usage: kiro-gateway"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			getenv := func(key string) string { t.Fatalf("unexpected environment read: %s", key); return "" }
			err := cli.Run(context.Background(), tc.args, getenv, &stdout, &stderr, tc.version)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(tc.name, "version") {
				if stdout.String() != tc.want {
					t.Fatalf("stdout = %q, want %q", stdout.String(), tc.want)
				}
			} else if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("missing usage in %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %q", stderr.String())
			}
		})
	}
}

// covers: AC-2, AC-3. Invalid commands fail without echoing sensitive input.
func TestRun_InvalidArguments(t *testing.T) {
	const secret = "private-argument-marker"
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "choose serve or version"},
		{"unknown command", []string{secret}, "unknown command"},
		{"version argument", []string{"version", secret}, "version does not accept arguments"},
		{"unknown flag", []string{"serve", "--" + secret}, "invalid serve options"},
		{"missing flag value", []string{"serve", "--listen"}, "invalid serve options"},
		{"positional argument", []string{"serve", secret}, "serve does not accept positional arguments"},
		{"credential flag", []string{"serve", "--token", secret}, "invalid serve options"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cli.Run(context.Background(), tc.args, func(string) string { t.Fatal("read environment before validating command"); return "" }, &stdout, &stderr, "dev")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if strings.Contains(err.Error()+stderr.String()+stdout.String(), secret) {
				t.Fatal("argument leaked into output")
			}
			if stdout.Len() != 0 {
				t.Fatalf("unexpected stdout: %q", stdout.String())
			}
		})
	}
}

// covers: AC-2, AC-3. Startup requires only the separate gateway credential.
func TestRun_MissingGatewayCredential(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var keys []string
	err := cli.Run(context.Background(), []string{"serve"}, func(key string) string { keys = append(keys, key); return "" }, &stdout, &stderr, "dev")
	if err == nil || !strings.Contains(err.Error(), "KIRO_GATEWAY_TOKEN is required") {
		t.Fatalf("error = %v", err)
	}
	if len(keys) != 1 || keys[0] != "KIRO_GATEWAY_TOKEN" {
		t.Fatalf("environment reads = %v", keys)
	}
	if stdout.Len()+stderr.Len() != 0 {
		t.Fatal("failed startup should not report a running server")
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// covers: AC-3. A failed output stream is visible to the process entry point.
func TestRun_OutputFailure(t *testing.T) {
	want := errors.New("output unavailable")
	for _, args := range [][]string{{"version"}, {"help"}, {"serve", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := cli.Run(context.Background(), args, func(string) string { return "" }, failingWriter{want}, io.Discard, "dev")
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}
