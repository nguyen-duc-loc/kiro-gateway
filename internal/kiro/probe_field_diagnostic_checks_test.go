package kiro

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWireUnknownFieldDiagnosticsNeverExposeValues(t *testing.T) {
	for _, tc := range []struct {
		name, event, payload, location, hint, kind string
	}{
		{"known string", "assistantResponseEvent", `{"content":"synthetic-text","messageId":"sentinel-private-id"}`, "event", "messageId", "string"},
		{"known boolean", "assistantResponseEvent", `{"content":"synthetic-text","followupPrompt":true}`, "event", "followupPrompt", "boolean"},
		{"unknown object", "metadataEvent", `{"sentinel_private_name":{"secret":"sentinel-private-value"}}`, "event", "unlisted", "object"},
		{"unknown array", "metadataEvent", `{"sentinel_private_name":["sentinel-private-value"]}`, "event", "unlisted", "array"},
		{"unknown null", "metadataEvent", `{"sentinel_private_name": null}`, "event", "unlisted", "null"},
		{"nested number", "metadataEvent", `{"tokenUsage":{"maxOutputTokens":2048}}`, "token_usage", "maxOutputTokens", "number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turn := wireTurn{fixtureTurn: fixtureTurn{memory: &probeMemory{limit: probeMaxRetained}}}
			if err := turn.observe(tc.event, []byte(tc.payload)); !errors.Is(err, errNeedsEvidence) {
				t.Fatalf("observe(%s) error=%v, want needs_evidence", tc.name, err)
			}
			if len(turn.unknownDetails) != 1 || turn.unknownFields != 1 {
				t.Fatalf("observe(%s) details=%d count=%d, want one each", tc.name, len(turn.unknownDetails), turn.unknownFields)
			}
			want := wireUnknownField{tc.event, tc.location, tc.hint, tc.kind}
			if got := turn.unknownDetails[0]; got != want {
				t.Errorf("observe(%s) detail=%+v, want %+v", tc.name, got, want)
			}
			b, err := json.Marshal(turn.unknownDetails)
			if err != nil || strings.Contains(string(b), "sentinel") || strings.Contains(string(b), "synthetic-text") || strings.Contains(string(b), "2048") {
				t.Errorf("observe(%s) diagnostic leaked input or failed encoding", tc.name)
			}
			turn.release()
		})
	}
}

func TestWireUnknownFieldDetailsAreBounded(t *testing.T) {
	turn := wireTurn{fixtureTurn: fixtureTurn{memory: &probeMemory{limit: probeMaxRetained}}}
	err := turn.observe("metadataEvent", []byte(`{"a_private":1,"b_private":2,"c_private":3,"d_private":4,"e_private":5}`))
	if !errors.Is(err, errNeedsEvidence) || turn.unknownFields != 5 || len(turn.unknownDetails) != 4 {
		t.Errorf("observe(five unknowns) error=%v count=%d details=%d, want needs_evidence, five, four", err, turn.unknownFields, len(turn.unknownDetails))
	}
	turn.release()
}

func TestWireUnknownFieldSummaryStillStopsSequence(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	p := localWireProbe(t, home, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.Write(append(wireResponses(0), probeFrame("assistantResponseEvent", `{"content":"sentinel-private-text","messageId":"sentinel-private-id"}`)...))
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	if got.Attempts != 1 || requests.Load() != 1 || got.Cases[0].Cause != "needs_evidence" || len(got.Cases[0].UnknownFieldDetails) != 1 || got.Cases[1].Status != "unrun" {
		t.Errorf("runWireCases(unknown field) attempts=%d requests=%d cause=%s, want one stopped attempt with diagnostic", got.Attempts, requests.Load(), got.Cases[0].Cause)
	}
	b, err := json.Marshal(got)
	if err != nil || strings.Contains(string(b), "sentinel-private") {
		t.Error("unknown field summary exposed raw response data or failed encoding")
	}
}
