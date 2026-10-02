package kiro

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"
)

func TestProbeInitialIdleBudgetCoversResolutionAndDial(t *testing.T) {
	for _, phase := range []string{"resolution", "dial"} {
		t.Run(phase, func(t *testing.T) {
			home, _ := probeHome(t)
			limits := defaultProbeLimits
			limits.idle, limits.request = 75*time.Millisecond, 800*time.Millisecond
			p, err := openOfflineProbe(t.Context(), home, "https://127.0.0.1:1/probe", x509.NewCertPool(), limits)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.close)
			done := make(chan struct{})
			var resolverDeadline time.Time
			stall := func(ctx context.Context) error {
				defer close(done)
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > limits.idle+25*time.Millisecond {
					t.Errorf("%s context deadline=%v present=%t, want initial idle bound", phase, deadline, ok)
				}
				if phase == "dial" && !deadline.Equal(resolverDeadline) {
					t.Error("dial restarted the idle budget after resolution")
				}
				<-ctx.Done()
				return ctx.Err()
			}
			resolve := func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
				resolverDeadline, _ = ctx.Deadline()
				if phase == "resolution" {
					return nil, stall(ctx)
				}
				return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
			}
			dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
				return nil, stall(ctx)
			}
			// Exercise the actual resolver/dial adapter with synthetic callbacks;
			// neither callback opens a socket or performs external DNS.
			p.dial = func(ctx context.Context, _ string) (net.Conn, error) {
				return wireDial(resolve, dial)(ctx, "runtime.us-east-1.kiro.dev:443")
			}
			start := time.Now()
			got := p.dispatch(probeSyntheticBody)
			if elapsed := time.Since(start); elapsed > 400*time.Millisecond || got.Cause != "timed_out" || got.Attempts != 1 {
				t.Errorf("dispatch(stalled %s) elapsed=%s cause=%s attempts=%d, want idle timeout and one attempt", phase, elapsed, got.Cause, got.Attempts)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("stalled resolver/dial did not finish after dispatch cleanup")
			}
		})
	}
}

func TestProbeDialKeepsShorterRequestDeadline(t *testing.T) {
	home, _ := probeHome(t)
	limits := defaultProbeLimits
	limits.idle, limits.request = time.Second, 50*time.Millisecond
	p, err := openOfflineProbe(t.Context(), home, "https://127.0.0.1:1/probe", x509.NewCertPool(), limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.close)
	done := make(chan struct{})
	p.dial = func(ctx context.Context, _ string) (net.Conn, error) {
		defer close(done)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > limits.request+25*time.Millisecond {
			t.Error("dial lost the earlier request deadline")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	got := p.dispatch(probeSyntheticBody)
	if got.Cause != "timed_out" || got.Attempts != 1 {
		t.Errorf("dispatch(short request deadline) cause=%s attempts=%d, want timed_out and one", got.Cause, got.Attempts)
	}
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Error("dial outlived the shorter request deadline")
	}
}

func TestProbePreflightBoundsDescendantHeldStdout(t *testing.T) {
	for _, delay := range []string{"0.6", "0.08"} {
		t.Run(delay, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
			defer cancel()
			start := time.Now()
			// The child exits by itself after a bounded interval, including on
			// the unfixed implementation. No file or network access is needed.
			output, err := realProbeCommand(ctx, "sh", "-c", "sleep "+delay+" & printf synthetic-version; exit 0")
			if elapsed := time.Since(start); elapsed > 350*time.Millisecond || !errors.Is(err, errCodeChanged) || output != "" {
				t.Errorf("realProbeCommand(descendant delay %s) elapsed=%s error=%v output=%q, want bounded code_changed and empty output", delay, elapsed, err, output)
			}
		})
	}
}

func TestProbeStreamingProgressRenewsIdleBudget(t *testing.T) {
	home, _ := probeHome(t)
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		for i := range 4 {
			if i != 0 {
				select {
				case <-r.Context().Done():
					return
				case <-time.After(80 * time.Millisecond):
				}
			}
			w.Write(probeFrame("assistantResponseEvent", `{"content":"synthetic-progress"}`))
			w.(http.Flusher).Flush()
		}
	})
	limits := defaultProbeLimits
	limits.idle, limits.request = 150*time.Millisecond, time.Second
	p := localProbe(t, t.Context(), home, s, roots, limits)
	start := time.Now()
	got := p.dispatch(probeSyntheticBody)
	if elapsed := time.Since(start); elapsed <= limits.idle || got.Cause != "stream_incomplete" || got.TextEvents != 4 {
		t.Errorf("dispatch(progressing stream) elapsed=%s cause=%s text_events=%d, want four events beyond the initial idle deadline", elapsed, got.Cause, got.TextEvents)
	}
}

func TestProbePreflightShortCommandStillSucceeds(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	output, err := realProbeCommand(ctx, "sh", "-c", "printf synthetic-version")
	if err != nil || output != "synthetic-version" {
		t.Errorf("realProbeCommand(short command) output=%q error=%v, want synthetic-version and nil", output, err)
	}
}

func TestProbeBaselineRejectsContextExpiryAfterVersionOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	env := probeEnvironment{runtime: "go1.27.1", goMod: "module synthetic\ngo 1.27.1\n", toolchain: "local", workspace: "off", cgo: "1"}
	run := func(_ context.Context, name string, _ ...string) (string, error) {
		if name == "claude" {
			return "2.1.286 (Claude Code)", nil
		}
		cancel()
		return "kiro-cli 2.8.0", nil
	}
	if err := checkProbeBaseline(ctx, env, run); !errors.Is(err, errBaselineChanged) {
		t.Errorf("checkProbeBaseline(expired after matching output) error=%v, want baseline_changed", err)
	}
}
