package kiro

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
)

type fixtureResponseEvent struct {
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

type fixtureResponses struct {
	Purpose string `json:"purpose"`
	Cutoff  int64  `json:"cutoff_bytes"`
	Cases   []struct {
		ID     string                 `json:"id"`
		Events []fixtureResponseEvent `json:"events"`
	} `json:"cases"`
}

func loadFixtureResponses(t *testing.T) fixtureResponses {
	t.Helper()
	b, err := os.ReadFile("testdata/probe-offline-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtureResponses
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) != 6 || f.Cutoff != fixtureCutoff {
		t.Fatal("offline fixtures do not match the six case sequence and cutoff")
	}
	return f
}

// The hook changes one synthetic response or surrounding local state for a
// failure case. Request assertions always run before it, including continuation.
type fixtureResponseHook func(int, fixtureRequest, []fixtureResponseEvent) []fixtureResponseEvent

func sequenceServer(t *testing.T, hook fixtureResponseHook) (*httptest.Server, *x509.CertPool, *atomic.Int32) {
	t.Helper()
	f := loadFixtureResponses(t)
	requests := &atomic.Int32{}
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		index := int(requests.Add(1)) - 1
		if index >= len(f.Cases) {
			t.Error("sequence sent a seventh request")
			w.WriteHeader(409)
			return
		}
		var req fixtureRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, probeMaxRequest+1)).Decode(&req); err != nil {
			t.Error("fixture request was not bounded JSON")
			w.WriteHeader(400)
			return
		}
		io.Copy(io.Discard, r.Body)
		if req.Case != f.Cases[index].ID || req.Model != probeModel || req.ConversationID != "fixture-conversation" || req.Instructions != fixtureInstructions || req.Current.Text != fixturePrompts[index] || req.Current.Role != "user" {
			t.Errorf("fixture request %d lost its declared case, model, instructions, conversation, or current input", index+1)
		}
		wantHistory := index * 2
		if wantHistory > 8 {
			wantHistory = 8
		}
		if len(req.History) != wantHistory {
			t.Errorf("fixture request %d history length=%d, want %d", index+1, len(req.History), wantHistory)
		}
		if index >= 1 && len(req.History) >= 2 && req.History[1].Text != fixtureMarker {
			t.Error("continuation lost the original assistant marker")
		}
		if index >= 1 && index <= 3 && (len(req.Tools) != 1 || req.Tools[0].Name != fixtureTool) {
			t.Error("continuation lost the fixed tool definition")
		}
		if index >= 2 && len(req.History) >= 4 {
			tool := req.History[3]
			if tool.Role != "assistant" || tool.ToolID != "fixture-tool-17" || tool.ToolName != fixtureTool || tool.Arguments != `{"key":"alpha"}` {
				t.Error("continuation changed the observed tool identity or arguments")
			}
		}
		if index == 2 && (req.Current.ToolID != "fixture-tool-17" || req.Current.Result != fixtureToolResult || req.Current.IsError) {
			t.Error("result request did not return the fixture result to the observed ID")
		}
		if index >= 3 && len(req.History) >= 6 && (req.History[4].ToolID != "fixture-tool-17" || req.History[4].Result != fixtureToolResult || req.History[5].Text != fixtureToolResult) {
			t.Error("follow up lost the completed tool exchange")
		}
		events := f.Cases[index].Events
		if hook != nil {
			events = hook(index, req, events)
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		for _, event := range events {
			w.Write(probeFrame(event.Name, string(event.Payload)))
			w.(http.Flusher).Flush()
		}
		if index == 4 && len(events) > 0 {
			<-r.Context().Done()
		}
	})
	return s, roots, requests
}

func TestFixtureSixCaseSequence(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, requests := sequenceServer(t, nil)
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	if got.Verdict != "candidate_supported" || got.Attempts != 6 || requests.Load() != 6 {
		t.Errorf("runFixtureCases(complete synthetic sequence) = %s, attempts=%d requests=%d, want supported with six", got.Verdict, got.Attempts, requests.Load())
	}
	for _, c := range got.Cases {
		if c.Status != "observed" {
			t.Errorf("fixture case %s status=%s cause=%s, want observed", c.ID, c.Status, c.Cause)
		}
		if c.Assertions.ModelMatch != nil || c.Assertions.UsagePresent != nil {
			t.Errorf("fixture case %s invented optional model or usage observations", c.ID)
		}
	}
	if p.ctx.Err() != nil {
		t.Errorf("parent after deliberate child cancellation = %v, want active", p.ctx.Err())
	}
	if got.PeakReservedBytes <= 0 || got.PeakReservedBytes > probeMaxRetained {
		t.Errorf("peak retained bytes=%d, want within fixed budget", got.PeakReservedBytes)
	}
	next := p.dispatch(probeSyntheticBody)
	if next.Attempts != 6 || next.Cause != "budget_exhausted" || requests.Load() != 6 {
		t.Errorf("seventh dispatch = %+v, requests=%d, want budget refusal without replay", next, requests.Load())
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{"sentinel-token", "fixture-tool-17", fixtureToolResult, fixtureInstructions, p.reference.Fingerprint} {
		if bytes.Contains(b, []byte(sentinel)) {
			t.Error("fixture result exposed a credential or conversation value")
		}
	}
}

func TestFixtureWrongToolStopsDependentCases(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, requests := sequenceServer(t, func(i int, _ fixtureRequest, events []fixtureResponseEvent) []fixtureResponseEvent {
		if i == 1 {
			events[0].Payload = bytes.ReplaceAll(events[0].Payload, []byte(fixtureTool), []byte("sentinel-wrong-tool"))
		}
		return events
	})
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	if got.Verdict != "candidate_rejected" || got.Cases[1].Status != "contradicted" || got.Attempts != 2 || requests.Load() != 2 {
		t.Errorf("wrong tool verdict=%s attempts=%d, want rejected after two", got.Verdict, got.Attempts)
	}
	for _, c := range got.Cases[2:] {
		if c.Status != "unrun" {
			t.Errorf("dependent case %s status=%s, want unrun", c.ID, c.Status)
		}
	}
}

func TestFixtureMissingCompletionStopsWithoutReplay(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, requests := sequenceServer(t, func(i int, _ fixtureRequest, events []fixtureResponseEvent) []fixtureResponseEvent {
		if i == 0 {
			return events[:2]
		}
		return events
	})
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	if got.Verdict != "needs_evidence" || got.Cases[0].Cause != "stream_incomplete" || got.Cases[1].Status != "unrun" || requests.Load() != 1 {
		t.Errorf("missing completion verdict=%s cause=%s requests=%d, want incomplete without replay", got.Verdict, got.Cases[0].Cause, requests.Load())
	}
}

func TestFixtureUnreachedCutoffIsInconclusive(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, _ := sequenceServer(t, func(i int, _ fixtureRequest, events []fixtureResponseEvent) []fixtureResponseEvent {
		if i == 5 {
			return []fixtureResponseEvent{{Name: fixtureCompletionEvent, Payload: json.RawMessage(`{"complete":true}`)}}
		}
		return events
	})
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	last := got.Cases[5]
	if got.Verdict != "needs_evidence" || last.Status != "inconclusive" || !isFalse(last.Assertions.InjectionReached) {
		t.Errorf("unreached cutoff verdict=%s last=%+v, want inconclusive and trigger false", got.Verdict, last)
	}
}

func TestFixtureParentCancellationPreventsSixthRequest(t *testing.T) {
	home, _ := probeHome(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	s, roots, requests := sequenceServer(t, func(i int, _ fixtureRequest, events []fixtureResponseEvent) []fixtureResponseEvent {
		if i == 4 {
			cancel()
			return nil
		}
		return events
	})
	p := localProbe(t, ctx, home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	if got.Verdict != "needs_evidence" || got.Cases[4].Cause != "canceled" || got.Cases[5].Status != "unrun" || requests.Load() != 5 {
		t.Errorf("parent cancel verdict=%s cause=%s requests=%d, want canceled after five", got.Verdict, got.Cases[4].Cause, requests.Load())
	}
}

func TestFixtureFrozenSettingsAndFreshCredentials(t *testing.T) {
	home, db := probeHome(t)
	s, roots, requests := sequenceServer(t, func(i int, _ fixtureRequest, events []fixtureResponseEvent) []fixtureResponseEvent {
		if i == 0 {
			d := config.Default() // A manual settings edit cannot erase the run snapshot.
			b, err := config.Encode(d)
			if err != nil {
				t.Error("could not encode synthetic settings")
				return events
			}
			if err := os.WriteFile(filepath.Join(home, ".config", "kiro-gateway", "config.json"), b, 0600); err != nil {
				t.Error("could not replace synthetic settings")
			}
		}
		if i == 1 {
			if _, err := db.Exec(`UPDATE auth_kv SET value=?`, strings.Replace(probeSyntheticRecord, "sentinel-token", "replacement-token", 1)); err != nil {
				t.Error("could not replace synthetic credential")
			}
		}
		return events
	})
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	// The server has completed before exchange returns; no hook is still active.
	s.Close()
	if got.Attempts != 2 || requests.Load() != 2 || got.Cases[2].Cause != "session_changed" || got.Cases[2].Attempt != 0 || got.Cases[3].Status != "unrun" {
		t.Errorf("fresh source selection verdict=%s case3=%+v attempts=%d, want source rejection before third dispatch", got.Verdict, got.Cases[2], got.Attempts)
	}
}

type countingSnapshots struct {
	reader snapshotReader
	reads  int
}

func (s *countingSnapshots) ReadSnapshot(ctx context.Context, ref config.Session) (credentials.Snapshot, error) {
	s.reads++
	return s.reader.ReadSnapshot(ctx, ref)
}

func TestFixtureReadsOneSnapshotPerAttempt(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, _ := sequenceServer(t, nil)
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	reader := &countingSnapshots{reader: p.reader}
	p.reader = reader
	got := runFixtureCases(p, probeMaxRetained)
	if got.Attempts != 6 || reader.reads != 6 {
		t.Errorf("snapshot reads=%d attempts=%d, want exactly six fresh reads", reader.reads, got.Attempts)
	}
}

func TestFixtureRetainedBudgetStopsBeforeAllocation(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, requests := sequenceServer(t, nil)
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, 300<<10)
	if got.Verdict != "needs_evidence" || got.Cases[0].Cause != "budget_exhausted" || got.Attempts != 0 || requests.Load() != 0 || got.PeakReservedBytes > 300<<10 {
		t.Errorf("retained budget verdict=%s case1=%+v peak=%d, want refusal before dispatch", got.Verdict, got.Cases[0], got.PeakReservedBytes)
	}
}

func TestFixtureDecoderFragmentIdentityAndSchema(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []string
		want   error
	}{
		{"escaped valid arguments", []string{`{"toolUseId":"id","name":"probe_lookup","input":"{\"key\":\"\\u0061"}`, `{"toolUseId":"id","input":"lpha\"}","stop":true}`}, nil},
		{"mismatched ID", []string{`{"toolUseId":"id","name":"probe_lookup"}`, `{"toolUseId":"other","input":"{}","stop":true}`}, errContradicted},
		{"missing ID", []string{`{"name":"probe_lookup","input":"{}","stop":true}`}, errNeedsEvidence},
		{"malformed arguments", []string{`{"toolUseId":"id","name":"probe_lookup","input":"{","stop":true}`}, errContradicted},
		{"extra property", []string{`{"toolUseId":"id","name":"probe_lookup","input":"{\"key\":\"alpha\",\"command\":\"never execute\"}","stop":true}`}, errContradicted},
		{"duplicate property", []string{`{"toolUseId":"id","name":"probe_lookup","input":"{\"key\":\"alpha\",\"key\":\"alpha\"}","stop":true}`}, errContradicted},
		{"unknown field", []string{`{"toolUseId":"id","name":"probe_lookup","sentinel-secret":true}`}, errNeedsEvidence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			memory := &probeMemory{limit: probeMaxRetained}
			turn := &fixtureTurn{memory: memory}
			var err error
			for _, event := range tc.events {
				err = turn.observe("toolUseEvent", []byte(event))
				if err != nil {
					break
				}
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("fixture decoder(%s) error=%v, want %v", tc.name, err, tc.want)
			}
			turn.release()
			if memory.used != 0 {
				t.Errorf("fixture decoder(%s) retained=%d, want zero after release", tc.name, memory.used)
			}
		})
	}
}

func TestProbeVerdictPriority(t *testing.T) {
	var cases [6]probeCaseResult
	for i := range cases {
		cases[i].Status = "observed"
	}
	if got := probeVerdict(cases); got != "candidate_supported" {
		t.Errorf("verdict(all observed)=%s, want supported", got)
	}
	cases[5].Status = "inconclusive"
	if got := probeVerdict(cases); got != "needs_evidence" {
		t.Errorf("verdict(last inconclusive)=%s, want needs_evidence", got)
	}
	cases[1].Status = "contradicted"
	cases[2].Status = "unrun"
	if got := probeVerdict(cases); got != "candidate_rejected" {
		t.Errorf("verdict(contradiction and cancellation)=%s, want rejected", got)
	}
}

func TestFixtureRunDeadlinePreventsFurtherDispatch(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	})
	limits := defaultProbeLimits
	limits.run = 150 * time.Millisecond
	p := localProbe(t, t.Context(), home, s, roots, limits)
	got := runFixtureCases(p, probeMaxRetained)
	if got.Verdict != "needs_evidence" || got.Cases[0].Cause != "timed_out" || requests.Load() != 1 || got.Cases[1].Status != "unrun" {
		t.Errorf("run deadline verdict=%s cause=%s requests=%d, want timed_out after one request", got.Verdict, got.Cases[0].Cause, requests.Load())
	}
}

func TestFixtureOptionalMetadataIsFiltered(t *testing.T) {
	home, _ := probeHome(t)
	s, roots, _ := sequenceServer(t, func(i int, _ fixtureRequest, events []fixtureResponseEvent) []fixtureResponseEvent {
		if i == 0 {
			events[len(events)-1].Payload = json.RawMessage(`{"complete":true,"model":"claude-sonnet-5","usage":{"sentinel-unknown-usage":999999999999}}`)
		}
		return events
	})
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	got := runFixtureCases(p, probeMaxRetained)
	first := got.Cases[0].Assertions
	if got.Verdict != "candidate_supported" || first.ModelMatch == nil || !*first.ModelMatch || first.UsagePresent == nil || !*first.UsagePresent {
		t.Errorf("optional metadata verdict=%s assertions=%+v, want observed booleans", got.Verdict, first)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("sentinel")) || bytes.Contains(b, []byte("999999")) {
		t.Error("optional metadata escaped the observation filter")
	}
}

func TestFixtureRejectsTrailingEventAfterCompletion(t *testing.T) {
	memory := &probeMemory{limit: probeMaxRetained}
	turn := &fixtureTurn{memory: memory}
	stream := append(probeFrame(fixtureCompletionEvent, `{"complete":true}`), probeFrame("assistantResponseEvent", `{"content":"late"}`)...)
	var out probeObservation
	err := walkProbeFrames(bytes.NewReader(stream), &out, memory, turn.observe)
	if !errors.Is(err, errContradicted) {
		t.Errorf("walkProbeFrames(event after terminal) error=%v, want contract contradiction", err)
	}
	turn.release()
}
