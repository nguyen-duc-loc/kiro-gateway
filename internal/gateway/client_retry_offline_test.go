//go:build clientbridge

package gateway

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/bridge"
)

// covers: AC-9. Compare zero with an absent setting using only local failures.
func TestInstalledClientRetriesOffline(t *testing.T) {
	if !*clientBridge {
		t.Skip("requires explicit -client-bridge flag")
	}
	binary, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal("client unavailable")
	}
	version, err := exec.Command(binary, "--version").Output()
	if err != nil || !strings.HasPrefix(string(version), "2.1.287 ") {
		t.Fatal("client version mismatch")
	}
	for _, fault := range []string{"429", "502", "interrupted"} {
		for _, setting := range []string{"zero", "absent"} {
			t.Run(fault+"/"+setting, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				t.Cleanup(cancel)
				var attempts atomic.Int64
				gen := generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
					n := attempts.Add(1)
					// Observing the second attempt establishes retry behavior. Stop the
					// isolated client instead of waiting through all default backoffs.
					if setting == "absent" && n >= 2 {
						cancel()
					}
					if fault == "interrupted" {
						if err := emit(bridge.Event{Text: "Synthetic partial response."}); err != nil {
							return bridge.End{}, err
						}
						return bridge.End{}, bridge.ProtocolFailure()
					}
					status := http.StatusBadGateway
					typ := "api_error"
					if fault == "429" {
						status = http.StatusTooManyRequests
						typ = "rate_limit_error"
					}
					return bridge.End{}, &bridge.Failure{Status: status, Type: typ, Message: "Synthetic failure.", Category: "synthetic_fault"}
				})
				token := strings.Repeat("dummy", 8)
				handler := NewExperimentalHandler(token, "offline", slog.New(slog.NewTextHandler(io.Discard, nil)), gen)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
				t.Cleanup(srv.Close)
				cmd := exec.CommandContext(ctx, binary, "-p", "--no-session-persistence", "--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", "Return a synthetic response.")
				cmd.Dir = t.TempDir()
				cmd.Env = offlineClientEnv(srv.URL, token, t.TempDir())
				if setting == "absent" {
					filtered := cmd.Env[:0]
					for _, value := range cmd.Env {
						if !strings.HasPrefix(value, "CLAUDE_CODE_MAX_RETRIES=") {
							filtered = append(filtered, value)
						}
					}
					cmd.Env = filtered
				}
				cmd.Stdout = io.Discard
				cmd.Stderr = io.Discard
				err := cmd.Run()
				t.Logf("fault=%s attempts=%d retry_setting=%s client_success=%t deadline_reached=%t stopped_after_retry=%t kiro_dispatches=0", fault, attempts.Load(), setting, err == nil, ctx.Err() == context.DeadlineExceeded, setting == "absent" && attempts.Load() >= 2)
				if err == nil || ctx.Err() == context.DeadlineExceeded || attempts.Load() < 1 || setting == "zero" && (attempts.Load() != 1 || ctx.Err() != nil) {
					t.Errorf("client(fault=%s, retries=%s) attempts=%d error=%v context=%v, want failed client before deadline and exactly one attempt when retries=0", fault, setting, attempts.Load(), err, ctx.Err())
				}
			})
		}
	}
}
