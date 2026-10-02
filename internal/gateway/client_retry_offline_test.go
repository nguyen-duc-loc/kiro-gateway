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
		t.Run(fault, func(t *testing.T) {
			var attempts atomic.Int64
			gen := generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
				attempts.Add(1)
				if fault == "interrupted" {
					if err := emit(bridge.Event{Text: "Synthetic partial response."}); err != nil {
						return bridge.End{}, err
					}
					return bridge.End{}, bridge.ProtocolFailure()
				}
				status := 502
				typ := "api_error"
				if fault == "429" {
					status = 429
					typ = "rate_limit_error"
				}
				return bridge.End{}, &bridge.Failure{Status: status, Type: typ, Message: "Synthetic failure.", Category: "synthetic_fault"}
			})
			token := strings.Repeat("dummy", 8)
			handler := NewExperimentalHandler(token, "offline", slog.New(slog.NewTextHandler(io.Discard, nil)), gen)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
			t.Cleanup(srv.Close)
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-p", "--no-session-persistence", "--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", "Return a synthetic response.")
			cmd.Dir = t.TempDir()
			cmd.Env = offlineClientEnv(srv.URL, token, t.TempDir())
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			err := cmd.Run()
			t.Logf("fault=%s attempts=%d max_retries_candidate=0 client_success=%t deadline_reached=%t kiro_dispatches=0", fault, attempts.Load(), err == nil, ctx.Err() != nil)
			if attempts.Load() != 1 || err == nil || ctx.Err() != nil {
				t.Errorf("client fault=%s attempts=%d, want one failed attempt within deadline", fault, attempts.Load())
			}
		})
	}
}
