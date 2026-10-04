package kiro

import (
	"bytes"
	"context"
	"errors"
	"io"
	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
	"kiro-gateway/internal/gateway"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// covers: AC-10. Actual handler and adapter failures latch before later source access.
func TestRunControlHTTPFailuresStopLaterRequests(t *testing.T) {
	for _, tc := range []struct{ name, category string }{
		{"validation", "invalid_request"}, {"source", "source_unavailable"}, {"dispatch", "upstream_status"}, {"stream", "incomplete_stream"}, {"terminal write", "response_write"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var reads, dispatches atomic.Int64
			a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				dispatches.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if tc.name == "dispatch" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"visible"}`))
				if tc.name == "stream" {
					_, _ = w.Write([]byte{1})
				}
			})
			reader := a.reader
			a.reader = wireSnapshotFunc(func(ctx context.Context, ref config.Session) (credentials.ProfileSnapshot, error) {
				reads.Add(1)
				if tc.name == "source" {
					return credentials.ProfileSnapshot{}, errors.New("private source failure")
				}
				return reader.ReadProfileSnapshot(ctx, ref)
			})
			control := newRunControl(t.Context(), time.Now().Add(time.Minute))
			t.Cleanup(func() { control.cancel(context.Canceled) })
			h := gateway.NewObservedExperimentalHandler("synthetic-bearer", "test", slog.New(slog.NewTextHandler(io.Discard, nil)), a, control)
			body := `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":"fixture"}]}`
			bad := body
			if tc.name == "validation" {
				bad = "{}"
			}
			request := func(raw string) *http.Request {
				r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(raw)).WithContext(control.ctx)
				r.Header.Set("Authorization", "Bearer synthetic-bearer")
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Anthropic-Version", "2023-06-01")
				return r
			}
			w := httptest.NewRecorder()
			func() {
				defer func() {
					if p := recover(); p != nil && (tc.name != "terminal write" || p != http.ErrAbortHandler) {
						panic(p)
					}
				}()
				if tc.name == "terminal write" {
					h.ServeHTTP(&failedTerminalWriter{ResponseRecorder: w}, request(bad))
				} else {
					h.ServeHTTP(w, request(bad))
				}
			}()
			beforeReads, beforeDispatches := reads.Load(), dispatches.Load()
			next := httptest.NewRecorder()
			h.ServeHTTP(next, request(body))
			if next.Code != http.StatusServiceUnavailable || reads.Load() != beforeReads || dispatches.Load() != beforeDispatches || control.cause != tc.category || control.ctx.Err() == nil {
				t.Errorf("Messages(after %s) status=%d reads=%d/%d dispatches=%d/%d cause=%s canceled=%t, want 503, unchanged counters, %s and canceled", tc.name, next.Code, reads.Load(), beforeReads, dispatches.Load(), beforeDispatches, control.cause, control.ctx.Err() != nil, tc.category)
			}
		})
	}
}

type failedTerminalWriter struct{ *httptest.ResponseRecorder }

func (w *failedTerminalWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

// covers: AC-10. Virtual time makes the 19m55s work cutoff exact and cheap.
func TestRunControlDeadlineCancelsActiveWorkAndRejectsAdmissions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		absolute := started.Add(20 * time.Minute)
		cutoff := absolute.Add(-5 * time.Second)
		work, stop := context.WithDeadlineCause(t.Context(), cutoff, &bridge.Failure{Status: http.StatusGatewayTimeout, Type: "api_error", Message: "Live run budget exhausted.", Category: "budget_exhausted"})
		t.Cleanup(stop)
		c := newRunControl(work, cutoff)
		t.Cleanup(func() { c.cancel(context.Canceled) })
		finished := make(chan struct{})
		go func() { defer close(finished); <-c.ctx.Done() }()
		time.Sleep(19*time.Minute + 54*time.Second)
		if err := c.Before(bridge.Observation{RequestID: "active", Phase: "dispatch"}); err != nil {
			t.Fatalf("Before(one second before cutoff) = %v, want admission", err)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		select {
		case <-finished:
		default:
			t.Error("runControl(at cutoff) worker active, want canceled")
		}
		if cause := bridge.SafeFailure(context.Cause(c.ctx)); cause.Status != http.StatusGatewayTimeout || cause.Category != "budget_exhausted" {
			t.Errorf("runControl(at cutoff).cause = %+v, want budget_exhausted 504", cause)
		}
		for _, phase := range []string{"local", "count", "admission", "source", "dispatch"} {
			if c.Before(bridge.Observation{Phase: phase}) == nil {
				t.Errorf("Before(%s after cutoff) succeeded, want rejection", phase)
			}
		}
		if remaining := absolute.Sub(time.Now()); remaining != 5*time.Second {
			t.Errorf("runControl(cutoff).cleanup remaining = %v, want 5s", remaining)
		}
		if c.attempts != 1 {
			t.Errorf("runControl(cutoff).attempts = %d, want 1", c.attempts)
		}
	})
}

// covers: AC-10. Faults on another request, failed cleanup and no trigger cannot advance.
func TestRunControlExpectedFaultNeedsExactRequestAndCleanup(t *testing.T) {
	for _, mode := range []string{"other request", "wrong fault", "cleanup failed", "trigger absent", "first cause"} {
		t.Run(mode, func(t *testing.T) {
			c := newRunControl(t.Context(), time.Now().Add(time.Minute))
			t.Cleanup(func() { c.cancel(context.Canceled) })
			if !c.arm("cancel", "canceled") {
				t.Fatal("arm(cancel) = false, want true")
			}
			if err := c.Before(bridge.Observation{RequestID: "admitted", Phase: "admission"}); err != nil {
				t.Fatal(err)
			}
			if mode != "trigger absent" {
				c.Observe(bridge.Observation{RequestID: "admitted", Phase: "text", Category: "success"})
			}
			id, category, cleaned := "admitted", "canceled", true
			if mode == "other request" {
				id = "other"
			}
			if mode == "wrong fault" {
				category = "incomplete_stream"
			}
			if mode == "cleanup failed" {
				cleaned = false
			}
			if mode == "first cause" {
				c.fail("assertion_failed")
			}
			c.Observe(bridge.Observation{RequestID: id, Phase: "failure", Category: category, Cleanup: cleaned})
			if c.advance() {
				t.Errorf("advance(%s) = true, want stopped", mode)
			}
			before := c.cause
			c.Observe(bridge.Observation{RequestID: "later", Phase: "failure", Category: "canceled"})
			if c.cause != before || c.Before(bridge.Observation{Phase: "dispatch"}) == nil {
				t.Errorf("runControl(%s) cause=%s want %s and closed dispatch", mode, c.cause, before)
			}
		})
	}
}

// covers: AC-10. A failure is reported before an error write can block.
func TestRunControlValidationStopsBeforeResponseWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newRunControl(t.Context(), time.Now().Add(time.Minute))
		t.Cleanup(func() { c.cancel(context.Canceled) })
		h := gateway.NewObservedExperimentalHandler("synthetic-bearer", "test", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, c)
		w := &heldErrorWriter{ResponseRecorder: httptest.NewRecorder(), release: make(chan struct{})}
		r := httptest.NewRequest("POST", "/v1/messages", bytes.NewBufferString("{}"))
		r.Header.Set("Authorization", "Bearer synthetic-bearer")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Anthropic-Version", "2023-06-01")
		done := make(chan struct{})
		go func() { defer close(done); h.ServeHTTP(w, r) }()
		synctest.Wait()
		if c.Before(bridge.Observation{Phase: "source"}) == nil {
			t.Error("Before(source during failed response write) succeeded, want stopped")
		}
		close(w.release)
		<-done
	})
}

type heldErrorWriter struct {
	*httptest.ResponseRecorder
	release chan struct{}
}

func (w *heldErrorWriter) Write(b []byte) (int, error) {
	<-w.release
	return w.ResponseRecorder.Write(b)
}

func TestRunControlStopsEveryBoundary(t *testing.T) {
	for _, phase := range []string{"local", "admission", "source", "dispatch", "stream", "response_write"} {
		t.Run(phase, func(t *testing.T) {
			c := newRunControl(t.Context(), time.Now().Add(time.Minute))
			defer c.cancel(context.Canceled)
			c.Observe(bridge.Observation{RequestID: "req_fixture", Phase: "failure", Category: phase, Cleanup: true})
			for _, boundary := range []string{"local", "count", "admission", "source", "dispatch"} {
				if c.Before(bridge.Observation{RequestID: "req_next", Phase: boundary}) == nil {
					t.Errorf("Before(%s) succeeded after %s failure", boundary, phase)
				}
			}
			if c.cause != phase || c.attempts != 0 || c.ctx.Err() == nil {
				t.Errorf("failure=%s cause=%s attempts=%d, want first failure without dispatch", phase, c.cause, c.attempts)
			}
		})
	}
}
func TestRunControlExpectedFaultCannotReopenWithoutCleanup(t *testing.T) {
	c := newRunControl(t.Context(), time.Now().Add(time.Minute))
	defer c.cancel(context.Canceled)
	if !c.arm("cancel", "canceled") {
		t.Fatal("arm failed")
	}
	if err := c.Before(bridge.Observation{RequestID: "req_one", Phase: "admission"}); err != nil {
		t.Fatal(err)
	}
	c.Observe(bridge.Observation{RequestID: "req_one", Phase: "text", Category: "success"})
	c.Observe(bridge.Observation{RequestID: "req_one", Phase: "failure", Category: "canceled", Cleanup: true})
	if c.Before(bridge.Observation{RequestID: "req_retry", Phase: "source"}) == nil {
		t.Error("retry accessed source while assertions pending")
	}
	if !c.advance() {
		t.Error("expected cleaned cancellation did not advance")
	}
	if err := c.Before(bridge.Observation{RequestID: "req_next", Phase: "dispatch"}); err != nil {
		t.Error(err)
	}
}
func TestRunControlConcurrentDispatchBudget(t *testing.T) {
	c := newRunControl(t.Context(), time.Now().Add(time.Minute))
	defer c.cancel(context.Canceled)
	var wg sync.WaitGroup
	for range 40 {
		wg.Go(func() { _ = c.Before(bridge.Observation{Phase: "dispatch"}) })
	}
	wg.Wait()
	if c.attempts != 20 || !c.stopped || c.cause != "budget_exhausted" {
		t.Errorf("dispatch budget attempts=%d stopped=%t cause=%s, want 20 and stopped", c.attempts, c.stopped, c.cause)
	}
}
