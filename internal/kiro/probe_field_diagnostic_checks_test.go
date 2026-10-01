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
			var wantError error
			if tc.event != "metadataEvent" || tc.location != "event" {
				wantError = errNeedsEvidence
			}
			if err := turn.observe(tc.event, []byte(tc.payload)); !errors.Is(err, wantError) {
				t.Fatalf("observe(%s) error=%v, want %v", tc.name, err, wantError)
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
	if err != nil || turn.unknownFields != 5 || len(turn.unknownDetails) != 4 {
		t.Errorf("observe(five metadata extensions) error=%v count=%d details=%d, want nil, five, four", err, turn.unknownFields, len(turn.unknownDetails))
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

func TestWireMetadataExtensionsDoNotInterruptToolsOrClaimCompletion(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
		i := int(requests.Add(1)) - 1
		if i >= 6 {
			t.Error("metadata extension sequence exceeded six attempts")
			return
		}
		assertWireRequest(t, r, i)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.Write(probeFrame("metadataEvent", `{"sentinel_private_name":"sentinel-private-value","tokenUsage":{"outputTokens":3}}`))
		w.Write(wireResponses(i))
		w.(http.Flusher).Flush()
		if i == 4 {
			<-r.Context().Done()
		}
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	if got.Verdict != "limited_candidate_observed" || requests.Load() != 6 {
		t.Fatalf("runWireCases(metadata extensions) verdict=%s attempts=%d, want six observed cases", got.Verdict, requests.Load())
	}
	for _, c := range got.Cases {
		if c.UnknownFields != 1 || len(c.UnknownFieldDetails) != 1 || c.Assertions.Completion != nil {
			t.Errorf("case %s lost metadata diagnostics or inferred completion", c.ID)
		}
	}
	b, err := json.Marshal(got)
	if err != nil || strings.Contains(string(b), "sentinel_private_name") || strings.Contains(string(b), "sentinel-private-value") {
		t.Error("accepted metadata extensions leaked names or values into summary")
	}
}

func TestWireMetadataExtensionsCannotHideInvalidUsageOrMakeText(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		want          error
	}{
		{name: "extension only", payload: `{"unlisted":"synthetic"}`},
		{name: "malformed usage", payload: `{"unlisted":"synthetic","tokenUsage":"invalid"}`, want: errContract},
		{name: "negative usage", payload: `{"unlisted":"synthetic","tokenUsage":{"outputTokens":-1}}`, want: errContract},
		{name: "unknown usage member", payload: `{"unlisted":"synthetic","tokenUsage":{"unexpected":1}}`, want: errNeedsEvidence},
		{name: "duplicate", payload: `{"unlisted":"a","unlisted":"b"}`, want: errContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turn := wireTurn{fixtureTurn: fixtureTurn{memory: &probeMemory{limit: probeMaxRetained}}}
			if err := turn.observe("metadataEvent", []byte(tc.payload)); !errors.Is(err, tc.want) {
				t.Errorf("observe(%s) error=%v, want %v", tc.name, err, tc.want)
			}
			if err := turn.validate(0); !errors.Is(err, errNeedsEvidence) {
				t.Errorf("validate(metadata only, %s) error=%v, want needs_evidence", tc.name, err)
			}
			turn.release()
		})
	}
}

func TestWireUnsupportedCancellationEventHasOnlyFiniteHint(t *testing.T) {
	for _, tc := range []struct{ name, event, want string }{
		{name: "known schema", event: "reasoningContentEvent", want: "reasoningContentEvent"},
		{name: "unlisted schema", event: "sentinel-private-event", want: "unlisted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var requests atomic.Int32
			p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
				i := int(requests.Add(1)) - 1
				if i > 4 {
					t.Error("unsupported event must stop before interruption case")
					return
				}
				assertWireRequest(t, r, i)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				if i == 4 {
					w.Write(probeFrame(tc.event, `{"text":"sentinel-private-value"}`))
				} else {
					w.Write(wireResponses(i))
				}
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			c := got.Cases[4]
			if requests.Load() != 5 || c.UnsupportedEventHint != tc.want || c.UnknownEvents != 1 || c.Cause != "needs_evidence" || c.Assertions.InjectionReached == nil || *c.Assertions.InjectionReached || got.Cases[5].Status != "unrun" {
				t.Errorf("runWireCases(%s) requests=%d hint=%q cause=%s, want five, %q, needs_evidence before trigger", tc.name, requests.Load(), c.UnsupportedEventHint, c.Cause, tc.want)
			}
			b, err := json.Marshal(got)
			if err != nil || strings.Contains(string(b), "sentinel-private") {
				t.Error("unsupported event diagnostic leaked a name or value")
			}
		})
	}
}
