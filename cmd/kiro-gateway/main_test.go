package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const executableToken = "test-only-gateway-credential-12345"

// The foundation covers AC-1 through AC-3. AC-4 needs the later bridge and a
// live Claude Code tool loop, so this suite makes no compatibility assertion.
func TestExecutable(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "kiro-gateway")
	buildCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-race", "-ldflags", "-X main.version=test-release", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	t.Run("saved settings and process locking", func(t *testing.T) { testSavedSettings(t, binary) })

	// covers: AC-1, AC-3. Test the real entry point and its OS exit status.
	for _, tc := range []struct {
		name           string
		args           []string
		code           int
		stdout, stderr string
	}{
		{"release version", []string{"version"}, 0, "test-release\n", ""},
		{"no command", nil, 1, "", "choose serve or version"},
		{"unknown command", []string{"private-argument"}, 1, "", "unknown command"},
		{"missing credential", []string{"serve", "--listen", "127.0.0.1:0"}, 1, "", "KIRO_GATEWAY_TOKEN is required"},
		{"external listener", []string{"serve", "--listen", "0.0.0.0:0"}, 1, "", "IPv4 loopback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			cmd := exec.CommandContext(ctx, binary, tc.args...)
			cmd.Env = isolatedEnvironment(t, "")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tc.code || stdout.String() != tc.stdout {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
			}
			if tc.stderr == "" {
				if stderr.Len() != 0 {
					t.Fatalf("stderr = %q", stderr.String())
				}
			} else if !strings.Contains(stderr.String(), "Error: ") || !strings.Contains(stderr.String(), tc.stderr) {
				t.Fatalf("stderr = %q", stderr.String())
			}
			if strings.Contains(stderr.String(), "private-argument") {
				t.Fatal("raw argument leaked")
			}
		})
	}

	for _, tc := range []struct {
		name    string
		signal  os.Signal
		pending bool
	}{
		{"interrupt stops serving", os.Interrupt, false},
		{"termination closes pending connection", syscall.SIGTERM, true},
	} {
		// covers: AC-1, AC-2, AC-3. Exercise real transport and process signals.
		t.Run(tc.name, func(t *testing.T) {
			ctx, stop := context.WithTimeout(context.Background(), 20*time.Second)
			defer stop()
			cmd := exec.CommandContext(ctx, binary, "serve", "--listen", "127.0.0.1:0")
			cmd.Env = isolatedEnvironment(t, executableToken)
			logs := &startupLog{ready: make(chan string, 1)}
			var stdout bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, logs
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			var waitErr error
			go func() { waitErr = cmd.Wait(); close(done) }()
			t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })
			var address string
			select {
			case address = <-logs.ready:
			case <-done:
				t.Fatalf("server exited before startup: %v\n%s", waitErr, logs.String())
			case <-ctx.Done():
				t.Fatal("server never reported startup")
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil || host != "127.0.0.1" || port == "0" {
				t.Fatalf("invalid startup address %q", address)
			}
			transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
			for _, request := range []struct {
				name, auth       string
				headerSize, want int
			}{
				{"health", "Bearer " + executableToken, 0, 200},
				{"missing credential", "", 0, 401},
				{"invalid credential", "Bearer invalid", 0, 401},
				{"oversized headers", "Bearer " + executableToken, 32 << 10, 431},
			} {
				t.Run(request.name, func(t *testing.T) {
					req, err := http.NewRequestWithContext(ctx, "GET", "http://"+address+"/healthz", nil)
					if err != nil {
						t.Fatal(err)
					}
					if request.auth != "" {
						req.Header.Set("Authorization", request.auth)
					}
					if request.headerSize != 0 {
						req.Header.Set("X-Large", strings.Repeat("x", request.headerSize))
					}
					res, err := client.Do(req)
					if err != nil {
						t.Fatal(err)
					}
					defer res.Body.Close()
					body, err := io.ReadAll(res.Body)
					if err != nil {
						t.Fatal(err)
					}
					if res.StatusCode != request.want {
						t.Fatalf("status = %d, want %d", res.StatusCode, request.want)
					}
					if request.want == 200 {
						var health map[string]string
						if err := json.Unmarshal(body, &health); err != nil {
							t.Fatal(err)
						}
						if len(health) != 2 || health["status"] != "running" || health["version"] != "test-release" {
							t.Fatalf("health = %v", health)
						}
					}
				})
			}
			var pending net.Conn
			if tc.pending {
				pending, err = net.DialTimeout("tcp4", address, time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer pending.Close()
				if err := pending.SetDeadline(time.Now().Add(8 * time.Second)); err != nil {
					t.Fatal(err)
				}
				if _, err := fmt.Fprint(pending, "GET /healthz HTTP/1.1\r\nHost: localhost\r\nX-Pending: "); err != nil {
					t.Fatal(err)
				}
			}
			if err := cmd.Process.Signal(tc.signal); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
				if waitErr != nil {
					t.Fatalf("shutdown: %v\n%s", waitErr, logs.String())
				}
			case <-time.After(7 * time.Second): // Five second grace plus scheduling and race runtime exit.
				t.Fatal("shutdown exceeded its bounded grace period")
			}
			if stdout.Len() != 0 {
				t.Fatalf("server wrote diagnostics to stdout: %q", stdout.String())
			}
			if strings.Contains(logs.String(), executableToken) {
				t.Fatal("credential leaked into diagnostics")
			}
			if !strings.Contains(logs.String(), "Gateway stopped") {
				t.Fatalf("missing shutdown diagnostic: %s", logs.String())
			}
			if pending != nil {
				_, err := io.ReadAll(pending)
				if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
					t.Fatal("pending connection remained open")
				}
			}
			conn, err := net.DialTimeout("tcp4", address, time.Second)
			if err == nil {
				conn.Close()
				t.Fatal("listener still accepts connections after exit")
			}
		})
	}
}

// No personal home, account settings, or inherited credential reaches a child.
func isolatedEnvironment(t *testing.T, token string) []string {
	t.Helper()
	return []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH"), "KIRO_GATEWAY_TOKEN=" + token, "GORACE=atexit_sleep_ms=0"}
}

// Observe the documented startup message instead of reserving a port or sleeping.
type startupLog struct {
	mu     sync.Mutex
	output bytes.Buffer
	ready  chan string
	once   sync.Once
}

var listenMessage = regexp.MustCompile(`Gateway listening on (127\.0\.0\.1:[0-9]+)`)

func (w *startupLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.output.Write(p)
	if match := listenMessage.FindStringSubmatch(w.output.String()); len(match) == 2 {
		w.once.Do(func() { w.ready <- match[1] })
	}
	return n, err
}

func (w *startupLog) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.output.String()
}
