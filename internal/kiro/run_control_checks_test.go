package kiro

import (
	"context"
	"kiro-gateway/internal/bridge"
	"sync"
	"testing"
	"time"
)

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
