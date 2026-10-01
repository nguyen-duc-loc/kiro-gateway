package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testSavedSettings uses separate real processes and a single isolated home.
// covers: spec 0002 AC-1, AC-2, AC-6, AC-7, AC-9.
func testSavedSettings(t *testing.T, binary string) {
	t.Helper()
	home := t.TempDir()
	env := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "KIRO_GATEWAY_TOKEN=" + executableToken, "GORACE=atexit_sleep_ms=0"}
	invoke := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	if output, err := invoke("config", "check"); err != nil || !strings.Contains(output, "defaults are valid") {
		t.Fatalf("config check(absent) = %q, %v", output, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatalf("config check(absent) created directory: %v", err)
	}
	if output, err := invoke("config", "init"); err != nil {
		t.Fatalf("config init = %q, %v", output, err)
	}
	path := filepath.Join(home, ".config", "kiro-gateway", "config.json")
	saved := []byte("{\n  \"schema_version\": 1,\n  \"listen\": \"127.0.0.1:0\"\n}\n")
	if err := os.WriteFile(path, saved, 0600); err != nil {
		t.Fatal(err)
	}
	start := func(args ...string) (*exec.Cmd, <-chan error, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = env
		logs := &startupLog{ready: make(chan string, 1)}
		cmd.Stderr = logs
		if err := cmd.Start(); err != nil {
			cancel()
			t.Fatal(err)
		}
		done := make(chan error, 1)
		// Wait owns process cleanup and always exits after cancellation or Kill.
		go func() { done <- cmd.Wait(); close(done) }()
		t.Cleanup(func() { _ = cmd.Process.Kill(); cancel(); <-done })
		select {
		case address := <-logs.ready:
			return cmd, done, address
		case err := <-done:
			t.Fatalf("serve startup = %v, logs %s", err, logs.String())
		case <-ctx.Done():
			t.Fatalf("serve startup timed out, logs %s", logs.String())
		}
		return nil, nil, ""
	}
	checkHealth := func(address string) {
		t.Helper()
		transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		req, err := http.NewRequestWithContext(t.Context(), "GET", "http://"+address+"/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+executableToken)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("health(saved listener) = %d, want 200", res.StatusCode)
		}
		_, _ = io.Copy(io.Discard, res.Body)
	}
	cmd, done, address := start("serve")
	checkHealth(address)
	for _, args := range [][]string{{"config", "init"}, {"config", "upgrade"}, {"account", "forget"}, {"account", "link"}, {"serve", "--listen", "127.0.0.1:0"}} {
		if output, err := invoke(args...); err == nil || !strings.Contains(output, "Stop the running gateway") {
			t.Errorf("command(%v while serving) = %q, %v, want lock conflict", args, output, err)
		}
	}
	if output, err := invoke("config", "check"); err != nil {
		t.Errorf("readonly check while serving = %q, %v", output, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("killed server did not exit")
	}
	if output, err := invoke("config", "upgrade"); err != nil {
		t.Fatalf("upgrade after process termination = %q, %v", output, err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, saved) {
		t.Error("serve/upgrade changed saved bytes")
	}
	cmd, done, address = start("serve")
	checkHealth(address)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("restart failed to shut down")
	}
	// Occupy the saved port so successful startup proves explicit flag precedence.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	saved = []byte(`{"schema_version":1,"listen":"` + listener.Addr().String() + `"}`)
	if err := os.WriteFile(path, saved, 0600); err != nil {
		t.Fatal(err)
	}
	cmd, done, address = start("serve", "--listen", "127.0.0.1:0")
	checkHealth(address)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("override server failed to shut down")
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := invoke("serve", "--listen", "127.0.0.1:0"); err == nil || !strings.Contains(output, "unsupported configuration version") {
		t.Errorf("serve(invalid file with override) = %q, %v, want version error", output, err)
	}
}
