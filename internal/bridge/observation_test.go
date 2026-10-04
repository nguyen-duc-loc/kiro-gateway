package bridge

import "testing"

type terminalObservations struct{ values []Observation }

func (*terminalObservations) Before(Observation) error { return nil }
func (o *terminalObservations) Observe(value Observation) {
	o.values = append(o.values, value)
}

// covers: AC-12. Terminal evidence contains a request ID and fixed labels only.
func TestObserveTerminalAcceptsOnlyFixedStopReasons(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
		valid  bool
	}{
		{name: "completed turn", reason: "end_turn", valid: true},
		{name: "tool handoff", reason: "tool_use", valid: true},
		{name: "missing reason"},
		{name: "unsupported reason", reason: "max_tokens"},
		{name: "untrusted content", reason: "synthetic secret or conversation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observer := &terminalObservations{}
			ctx := WithObserver(t.Context(), "req_synthetic", observer)
			ObserveTerminal(ctx, tc.reason)
			wantCount := 0
			if tc.valid {
				wantCount = 1
			}
			if len(observer.values) != wantCount {
				t.Fatalf("ObserveTerminal(%q) observations=%d, want %d", tc.reason, len(observer.values), wantCount)
			}
			if tc.valid {
				want := Observation{RequestID: "req_synthetic", Phase: "terminal", Category: "success", Cleanup: true, StopReason: tc.reason}
				if got := observer.values[0]; got != want {
					t.Errorf("ObserveTerminal(%q) = %+v, want %+v", tc.reason, got, want)
				}
			}
		})
	}
}

// covers: AC-12. Ordinary serving has no development observer installed.
func TestObserveTerminalWithoutObserver(t *testing.T) {
	ObserveTerminal(t.Context(), "end_turn")
	ObserveTerminal(WithObserver(t.Context(), "req_synthetic", nil), "tool_use")
}
