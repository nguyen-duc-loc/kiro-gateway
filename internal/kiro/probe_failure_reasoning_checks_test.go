package kiro

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWireFailureChecksDiscardReasoningBeforeTheirTriggers(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
		i := int(requests.Add(1)) - 1
		if i >= 6 {
			t.Error("reasoning sequence exceeded six attempts")
			return
		}
		assertWireRequest(t, r, i)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		if i >= 4 {
			w.Write(probeFrame("reasoningContentEvent", `{"text":"sentinel-private-reasoning"}`))
		}
		if i == 5 {
			w.Write(probeFrame("reasoningContentEvent", `{"text":"`+strings.Repeat("x", 512)+`"}`))
		} else {
			w.Write(wireResponses(i))
		}
		w.(http.Flusher).Flush()
		if i == 4 {
			<-r.Context().Done()
		}
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	if got.Verdict != "limited_candidate_observed" || got.Attempts != 6 {
		t.Fatalf("runWireCases(reasoning before triggers) verdict=%s attempts=%d, want six observed cases", got.Verdict, got.Attempts)
	}
	for i, c := range got.Cases {
		if c.Assertions.Completion != nil {
			t.Errorf("case %s inferred authoritative completion", c.ID)
		}
		if i < 4 && c.DiscardedReasoningEvents != 0 {
			t.Errorf("case %s discarded conversation reasoning", c.ID)
		}
		if i >= 4 && (c.DiscardedReasoningEvents != 1 || c.Assertions.InjectionReached == nil || !*c.Assertions.InjectionReached) {
			t.Errorf("case %s discarded=%d trigger=%v, want one complete discarded frame and reached trigger", c.ID, c.DiscardedReasoningEvents, c.Assertions.InjectionReached)
		}
	}
	if got.Cases[4].TextEvents != 1 || got.Cases[5].TextEvents != 0 {
		t.Error("reasoning changed cancellation text counting or the byte cutoff")
	}
	b, err := json.Marshal(got)
	if err != nil || strings.Contains(string(b), "sentinel-private") {
		t.Error("reasoning payload leaked into the summary")
	}
}

func TestWireFailureReasoningCannotBypassValidationOrTriggerCancellation(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		validObject   bool
	}{
		{name: "reasoning only", payload: `{"text":"sentinel-private-reasoning"}`, validObject: true},
		{name: "duplicate", payload: `{"text":"a","text":"b"}`},
		{name: "array", payload: `[]`},
		{name: "null", payload: `null`},
		{name: "malformed", payload: `{"text":`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var requests atomic.Int32
			p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
				i := int(requests.Add(1)) - 1
				if i > 4 {
					t.Error("unreached cancellation must stop before interruption")
					return
				}
				assertWireRequest(t, r, i)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				if i == 4 {
					w.Write(probeFrame("reasoningContentEvent", tc.payload))
				} else {
					w.Write(wireResponses(i))
				}
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			c := got.Cases[4]
			if requests.Load() != 5 || c.Status != "inconclusive" || c.Assertions.InjectionReached == nil || *c.Assertions.InjectionReached || c.TextEvents != 0 || got.Cases[5].Status != "unrun" {
				t.Errorf("runWireCases(%s) accepted missing cancellation trigger", tc.name)
			}
			if tc.validObject && c.DiscardedReasoningEvents != 1 || !tc.validObject && (c.DiscardedReasoningEvents != 0 || c.Cause != "contract_mismatch") {
				t.Errorf("case %s discarded=%d cause=%s, want only valid objects discarded and malformed objects rejected", tc.name, c.DiscardedReasoningEvents, c.Cause)
			}
		})
	}
}
