package kiro

import (
	"context"
	"net/http"
	"sync"
	"time"

	"kiro-gateway/internal/bridge"
)

// runControl belongs to development tooling, never the product executable.
// Admission, source reads, and dispatch checks serialize with stopping.
type runControl struct {
	mu                          sync.Mutex
	ctx                         context.Context
	cancel                      context.CancelCauseFunc
	cutoff                      time.Time
	attempts                    int
	stopped, closed             bool
	cause                       string
	caseID, expected, requestID string
	fault, cleaned, textSeen    bool
	terminals                   int
}

func newRunControl(parent context.Context, cutoff time.Time) *runControl {
	ctx, cancel := context.WithCancelCause(parent)
	return &runControl{ctx: ctx, cancel: cancel, cutoff: cutoff}
}
func stoppedFailure() error {
	return &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "The live run has stopped.", Category: "run_stopped"}
}
func (c *runControl) stop(category string) {
	if !c.stopped {
		c.stopped = true
		c.closed = true
		c.cause = category
		c.cancel(stoppedFailure())
	}
}
func (c *runControl) Before(o bridge.Observation) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !time.Now().Before(c.cutoff) || c.ctx.Err() != nil {
		c.stop("budget_exhausted")
	}
	if c.stopped || c.closed {
		return stoppedFailure()
	}
	if o.Phase == "admission" && c.expected != "" {
		if c.requestID != "" {
			c.stop("unexpected_overlap")
			return stoppedFailure()
		}
		c.requestID = o.RequestID
	}
	if o.Phase == "dispatch" {
		if c.attempts >= 20 {
			c.stop("budget_exhausted")
			return &bridge.Failure{Status: http.StatusGatewayTimeout, Type: "api_error", Message: "Live run budget exhausted.", Category: "budget_exhausted"}
		}
		c.attempts++
	}
	return nil
}
func (c *runControl) Observe(o bridge.Observation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if o.Phase == "text" {
		c.textSeen = true
		return
	}
	if o.Phase == "terminal" {
		c.terminals++
	}
	if o.Phase == "failure" {
		if c.stopped {
			return
		}
		if c.expected != "" && o.RequestID == c.requestID && o.Category == c.expected && c.textSeen {
			c.closed = true
			c.fault = true
			c.cleaned = o.Cleanup
			return
		}
		// A rejected retry while the expected fault is being checked stays local.
		if c.closed && o.Category == "run_stopped" {
			return
		}
		c.stop(o.Category)
	}
	if o.Phase == "cleanup" && o.RequestID == c.requestID {
		c.cleaned = o.Cleanup
		if !o.Cleanup {
			c.stop("cleanup_failed")
		}
	}
}
func (c *runControl) arm(caseID, category string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.closed {
		return false
	}
	c.caseID, c.expected, c.requestID = caseID, category, ""
	c.fault, c.cleaned, c.textSeen = false, false, false
	return true
}
func (c *runControl) advance() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || !c.closed || !c.fault || !c.cleaned || !time.Now().Before(c.cutoff) {
		c.stop("assertion_failed")
		return false
	}
	c.closed = false
	c.caseID, c.expected, c.requestID = "", "", ""
	return true
}
func (c *runControl) fail(category string) { c.mu.Lock(); defer c.mu.Unlock(); c.stop(category) }
