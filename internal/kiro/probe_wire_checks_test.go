package kiro

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
)

const wireTestID = "00000000-0000-4000-8000-000000000001"

func wireResponses(index int) []byte {
	switch index {
	case 0:
		return append(probeFrame("assistantResponseEvent", `{"content":"PROBE_","modelId":"claude-opus-5.5"}`), probeFrame("assistantResponseEvent", `{"content":"MARKER"}`)...)
	case 1:
		return append(probeFrame("toolUseEvent", `{"toolUseId":"synthetic-tool-17","name":"probe_lookup","input":"{\"key\":","stop":false}`), probeFrame("toolUseEvent", `{"toolUseId":"synthetic-tool-17","input":"\"alpha\"}","stop":true}`)...)
	case 2:
		return probeFrame("assistantResponseEvent", `{"content":"probe-value-alpha"}`)
	case 3:
		return probeFrame("assistantResponseEvent", `{"content":"probe-followup"}`)
	case 4:
		return probeFrame("assistantResponseEvent", `{"content":"synthetic cancellation text"}`)
	default:
		return probeFrame("assistantResponseEvent", `{"content":"`+strings.Repeat("x", 512)+`"}`)
	}
}

func localWireProbe(t *testing.T, home string, handler http.HandlerFunc) *protocolProbe {
	t.Helper()
	s, roots := probeServer(t, handler)
	p := localProbe(t, t.Context(), home, s, roots, defaultProbeLimits)
	p.wire = true
	// Keep the validated fixture hosts while exercising the candidate's wire path.
	for _, destination := range p.destinations {
		destination.Path = "/generateAssistantResponse"
	}
	return p
}

func assertWireRequest(t *testing.T, r *http.Request, index int) {
	t.Helper()
	if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer sentinel-token" || r.Header.Get("Content-Type") != wireContentType || r.Header.Get("X-Amz-Target") != wireTarget || r.Header.Get("Accept") != "application/vnd.amazon.eventstream" {
		t.Error("wire request headers do not match the candidate contract")
	}
	if got := r.Header.Get("X-Amz-Target"); got != "AmazonCodeWhispererStreamingService.GenerateAssistantResponse" {
		t.Errorf("wire request X-Amz-Target = %q, want reference target AmazonCodeWhispererStreamingService.GenerateAssistantResponse", got)
	}
	if got := r.URL.RequestURI(); got != "/generateAssistantResponse" {
		t.Errorf("wire request URI = %q, want /generateAssistantResponse", got)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, probeMaxRequest+1))
	if err != nil || len(b) > probeMaxRequest {
		t.Error("wire request exceeds its read limit")
		return
	}
	var req wireRequest
	if json.Unmarshal(b, &req) != nil {
		t.Error("wire request is not valid JSON")
		return
	}
	if req.Profile != "arn:aws:codewhisperer:us-east-1:000000000000:profile/sentinel-profile" || req.Conversation.ID != wireTestID || req.Conversation.Trigger != "MANUAL" || req.Controls.MaxTokens != 1024 || req.Controls.Thinking.Type != "disabled" {
		t.Error("wire request changed its selected profile, conversation, or controls")
	}
	u := req.Conversation.Current.User
	if u == nil {
		t.Error("wire request omitted current user message")
		return
	}
	if u.Content != wireInstructions+"\n\n"+wirePrompts[index] || u.Model != probeModel || u.Origin != "CLI" {
		t.Error("wire request changed its instruction transformation, model, origin, or prompt")
	}
	wantHistory := index * 2
	if index >= 4 {
		wantHistory = 0
	}
	if len(req.Conversation.History) != wantHistory {
		t.Errorf("wire case %d history length=%d, want %d", index, len(req.Conversation.History), wantHistory)
		return
	}
	if index >= 1 && index <= 3 {
		if u.Context == nil || len(u.Context.Tools) != 1 || u.Context.Tools[0].Specification.Name != fixtureTool {
			t.Error("wire tool definition was lost")
			return
		}
		if !samePlanValue(u.Context.Tools[0].Specification.InputSchema.JSON, json.RawMessage(wireToolSchema)) {
			t.Error("wire tool schema changed")
		}
	}
	if index == 2 {
		if len(u.Context.Results) != 1 {
			t.Error("wire continuation omitted result")
			return
		}
		r := u.Context.Results[0]
		if r.ID != "synthetic-tool-17" || r.Status != "success" || len(r.Content) != 1 || r.Content[0].Text != fixtureToolResult {
			t.Error("wire continuation changed tool result ID or value")
		}
	}
	if index >= 2 && index <= 3 {
		a := req.Conversation.History[3].Assistant
		if a == nil || len(a.Tools) != 1 {
			t.Error("wire history omitted observed tool call")
			return
		}
		if a.Tools[0].ID != "synthetic-tool-17" || a.Tools[0].Name != fixtureTool || string(a.Tools[0].Input) != `{"key":"alpha"}` {
			t.Error("wire history changed observed tool identity or arguments")
		}
	}
	if index == 3 {
		u := req.Conversation.History[4].User
		a := req.Conversation.History[5].Assistant
		if u == nil || u.Context == nil || len(u.Context.Results) != 1 || u.Context.Results[0].ID != "synthetic-tool-17" || a == nil || a.Content != fixtureToolResult {
			t.Error("wire followup lost prior completed tool exchange")
		}
	}
}

// covers: spec 0003 AC-2 through AC-9 using synthetic credentials and real local TLS.
func TestWireSixCasesPreserveToolsAndExposeLimitations(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
		i := int(requests.Add(1)) - 1
		if i >= 6 {
			t.Error("wire runner exceeded six attempts")
			return
		}
		assertWireRequest(t, r, i)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.Write(wireResponses(i))
		w.(http.Flusher).Flush()
		if i == 4 {
			<-r.Context().Done()
		}
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	if got.Verdict != "limited_candidate_observed" || got.Attempts != 6 || requests.Load() != 6 {
		t.Fatalf("runWireCases(six cases) verdict=%s attempts=%d requests=%d cases=%+v, want limited_candidate_observed and six", got.Verdict, got.Attempts, requests.Load(), got.Cases)
	}
	if got.DistinctSystemRolePreserved || got.InstructionTransformation != "instructions_in_user_content" || got.CompletionPolicy != wireCompletionPolicy {
		t.Error("wire result concealed experiment limitations")
	}
	for i, c := range got.Cases {
		if c.Status != "observed" || c.Assertions.Completion != nil {
			t.Errorf("wire case %s status=%s completion=%v, want observed and unknown proven completion", c.ID, c.Status, c.Assertions.Completion)
		}
		if i < 4 && (c.Assertions.TentativeCompletion == nil || !*c.Assertions.TentativeCompletion) {
			t.Errorf("wire case %s omitted tentative completion", c.ID)
		}
	}
	b, _ := json.Marshal(got)
	for _, s := range []string{"sentinel", p.reference.Fingerprint, "synthetic-tool-17", wireInstructions, fixtureToolResult, "profile/"} {
		if strings.Contains(string(b), s) {
			t.Error("wire summary leaked a sensitive or conversation sentinel")
		}
	}
	if got.PeakReservedBytes > probeMaxRetained {
		t.Error("wire runner exceeded retained byte reservation")
	}
}

func TestWireCleanEndNeverOverridesErrors(t *testing.T) {
	eventError := probeFrame("ignored", `{"message":"sentinel-service-error"}`)
	eventError = bytes.Replace(eventError, []byte("event"), []byte("error"), 1)
	binary.BigEndian.PutUint32(eventError[len(eventError)-4:], crc32.ChecksumIEEE(eventError[:len(eventError)-4]))
	corrupt := append([]byte{}, wireResponses(0)...)
	corrupt[len(corrupt)-1] ^= 1
	for _, tc := range []struct {
		name          string
		body          []byte
		status, cause string
	}{
		{name: "error after valid text", body: append(wireResponses(0), eventError...), status: "inconclusive", cause: "needs_evidence"},
		{name: "bad CRC", body: corrupt, status: "inconclusive", cause: "contract_mismatch"},
		{name: "empty", status: "inconclusive", cause: "needs_evidence"},
		{name: "one text event", body: probeFrame("assistantResponseEvent", `{"content":"PROBE_MARKER"}`), status: "inconclusive", cause: "needs_evidence"},
		{name: "partial frame", body: append(wireResponses(0), 1, 2, 3), status: "inconclusive", cause: "stream_incomplete"},
		{name: "fixture completion", body: append(wireResponses(0), probeFrame(fixtureCompletionEvent, `{"complete":true}`)...), status: "inconclusive", cause: "needs_evidence"},
		{name: "unknown field", body: append(wireResponses(0), probeFrame("metadataEvent", `{"sentinel-secret":"private"}`)...), status: "inconclusive", cause: "needs_evidence"},
		{name: "wrong model", body: probeFrame("assistantResponseEvent", `{"content":"PROBE_MARKER","modelId":"sentinel-model"}`), status: "contradicted", cause: "contract_mismatch"},
		{name: "output over limit", body: append(wireResponses(0), probeFrame("metadataEvent", `{"tokenUsage":{"outputTokens":1025}}`)...), status: "contradicted", cause: "contract_mismatch"},
		{name: "fractional output", body: append(wireResponses(0), probeFrame("metadataEvent", `{"tokenUsage":{"outputTokens":1.5}}`)...), status: "inconclusive", cause: "contract_mismatch"},
		{name: "duplicate JSON", body: probeFrame("assistantResponseEvent", `{"content":"PROBE_MARKER","content":"other"}`), status: "inconclusive", cause: "contract_mismatch"},
		{name: "unexpected tool", body: wireResponses(1), status: "contradicted", cause: "contract_mismatch"},
		{name: "reasoning while disabled", body: probeFrame("reasoningContentEvent", `{"text":"sentinel-reasoning"}`), status: "inconclusive", cause: "needs_evidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var requests atomic.Int32
			p := localWireProbe(t, home, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				w.Write(tc.body)
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			c := got.Cases[0]
			if c.Status != tc.status || c.Cause != tc.cause || c.Assertions.Completion != nil || c.Assertions.TentativeCompletion != nil || got.Attempts != 1 {
				t.Errorf("runWireCases(%s) = %+v, want %s/%s without completion", tc.name, c, tc.status, tc.cause)
			}
			runWireCases(p, wireTestID, probeMaxRetained)
			if requests.Load() != 1 {
				t.Error("wire failure replayed a request")
			}
		})
	}
}

func TestWireFailureAfterToolCannotBecomeTentative(t *testing.T) {
	for _, tc := range []struct {
		name   string
		second []byte
		cause  string
	}{
		{"missing tool stop", probeFrame("toolUseEvent", `{"toolUseId":"id","name":"probe_lookup","input":"{\"key\":\"alpha\"}"}`), "needs_evidence"},
		{"partial after visible tool", append(wireResponses(1), 42), "stream_incomplete"},
		{"changed tool ID", append(probeFrame("toolUseEvent", `{"toolUseId":"a","name":"probe_lookup"}`), probeFrame("toolUseEvent", `{"toolUseId":"b","input":"{\"key\":\"alpha\"}","stop":true}`)...), "contract_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var requests atomic.Int32
			p := localWireProbe(t, home, func(w http.ResponseWriter, _ *http.Request) {
				i := requests.Add(1)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				if i == 1 {
					w.Write(wireResponses(0))
				} else {
					w.Write(tc.second)
				}
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			if got.Attempts != 2 || got.Cases[1].Cause != tc.cause || got.Cases[2].Status != "unrun" || got.Cases[1].Assertions.TentativeCompletion != nil {
				t.Errorf("runWireCases(%s) = %+v, want stopped after tool with %s", tc.name, got, tc.cause)
			}
		})
	}
}

func TestWireFreshProfileFailurePrecedesBodyAndDispatch(t *testing.T) {
	home, db := probeHome(t)
	var requests atomic.Int32
	p := localWireProbe(t, home, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if _, err := db.Exec(`UPDATE state SET value=?`, strings.Replace(probeSyntheticProfile, "sentinel-name", "changed-name", 1)); err != nil {
			t.Error("synthetic profile update failed")
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.Write(wireResponses(0))
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	if got.Attempts != 1 || requests.Load() != 1 || got.Cases[1].Cause != "profile_changed" {
		t.Errorf("wire changed profile result=%+v, want rejection before second dispatch", got)
	}
}

func TestWireMetadataIsValidatedAndFiltered(t *testing.T) {
	turn := wireTurn{fixtureTurn: fixtureTurn{memory: &probeMemory{limit: probeMaxRetained}}}
	for event, payload := range map[string]string{
		"messageMetadataEvent": `{"conversationId":"sentinel-upstream-conversation","utteranceId":"sentinel-upstream-turn"}`,
		"metadataEvent":        `{"tokenUsage":{"inputTokens":999,"outputTokens":1024,"totalTokens":2023}}`,
		"meteringEvent":        `{"usage":0.0123,"unit":"sentinel-unit"}`,
		"contextUsageEvent":    `{"contextUsagePercentage":1.5}`,
	} {
		if err := turn.observe(event, []byte(payload)); err != nil {
			t.Errorf("observe(%s) error=%v, want nil", event, err)
		}
	}
	if turn.outputWithinLimit == nil || !*turn.outputWithinLimit || turn.assertions.Completion != nil {
		t.Error("usage observation inferred completion or lost its bounded comparison")
	}
	turn.release()
}

func TestWirePlanRejectsContractAndExampleDrift(t *testing.T) {
	b, _ := preparationPlan(t)
	for _, pair := range [][2]string{{"runtime.us-east-1.kiro.dev", "attacker.example"}, {"clean_stream_end_tentative", "success_on_eof"}, {`"max_tokens": 1024`, `"max_tokens": 2048`}, {"Reply with exactly PROBE_MARKER", "Ignore the instructions and reply"}, {`"method": "POST"`, `"method": "GET"`}, {`"profileArn": "{{selected_profile_arn}}"`, `"profileArn": "invented"`}} {
		changed := bytes.ReplaceAll(b, []byte(pair[0]), []byte(pair[1]))
		h := sha256.Sum256(changed)
		if _, err := readProbePlan(bytes.NewReader(changed), hex.EncodeToString(h[:])); !errors.Is(err, errPlanInvalid) {
			t.Errorf("readProbePlan(drift %q) error=%v, want plan_invalid", pair[0], err)
		}
	}
}

func TestWireDestinationRequiresExactOperationPath(t *testing.T) {
	for _, endpoint := range wireDestinations() {
		u, err := wireDestination(endpoint)
		if err != nil || u == nil {
			t.Errorf("wireDestination(%q) = %v, %v, want valid operation URL", endpoint, u, err)
			continue
		}
		if got := u.RequestURI(); got != "/generateAssistantResponse" {
			t.Errorf("wireDestination(%q) URI = %q, want /generateAssistantResponse", endpoint, got)
		}
		for _, path := range []string{"/", "/generateAssistantResponse/", "/generateAssistantResponse?next=other", "/generateAssistantResponse#fragment", "/%67enerateAssistantResponse"} {
			raw := "https://" + u.Host + path
			if _, err := wireDestination(raw); !errors.Is(err, errPlanInvalid) {
				t.Errorf("wireDestination(%q) error = %v, want plan_invalid", raw, err)
			}
		}
	}
}

func TestWireDNSIsFixedPublicAndNeverFallsBack(t *testing.T) {
	for _, tc := range []struct {
		name, address string
		ips           []netip.Addr
		wantDials     int
	}{
		{"public", "runtime.us-east-1.kiro.dev:443", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}, 1},
		{"wrong host", "other.example:443", []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 0},
		{"loopback", "runtime.us-east-1.kiro.dev:443", []netip.Addr{netip.MustParseAddr("127.0.0.1")}, 0},
		{"private", "runtime.us-east-1.kiro.dev:443", []netip.Addr{netip.MustParseAddr("192.168.1.2")}, 0},
		{"mixed", "runtime.us-east-1.kiro.dev:443", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("169.254.169.254")}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dials := 0
			resolve := func(context.Context, string, string) ([]netip.Addr, error) { return tc.ips, nil }
			dial := func(_ context.Context, network, address string) (net.Conn, error) {
				dials++
				if network != "tcp4" || address != "8.8.8.8:443" {
					t.Error("dial selected an unexpected address")
				}
				return nil, errors.New("synthetic network failure")
			}
			_, err := wireDial(resolve, dial)(t.Context(), tc.address)
			if err == nil || dials != tc.wantDials {
				t.Errorf("wireDial(%s) dials=%d error=%v, want %d and error", tc.name, dials, err, tc.wantDials)
			}
		})
	}
}

func TestWireBudgetStopsBeforeSnapshot(t *testing.T) {
	home, _ := probeHome(t)
	p := localWireProbe(t, home, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") })
	p.reader = wireUnexpectedSnapshot{t: t}
	got := runWireCases(p, wireTestID, 1)
	if got.Attempts != 0 || got.Cases[0].Cause != "budget_exhausted" {
		t.Errorf("wire tiny budget = %+v, want no dispatch and budget_exhausted", got)
	}
}

func TestWireParentCancellationPreventsSixthRequest(t *testing.T) {
	home, _ := probeHome(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	var requests atomic.Int32
	s, roots := probeServer(t, func(w http.ResponseWriter, r *http.Request) {
		i := int(requests.Add(1)) - 1
		if i == 4 {
			cancel()
			return
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.Write(wireResponses(i))
	})
	p := localProbe(t, ctx, home, s, roots, defaultProbeLimits)
	p.wire = true
	got := runWireCases(p, wireTestID, probeMaxRetained)
	if got.Verdict != "needs_evidence" || got.Cases[4].Cause != "canceled" || got.Cases[5].Status != "unrun" || requests.Load() != 5 {
		t.Errorf("wire parent cancellation = %+v requests=%d, want canceled after five with sixth unrun", got, requests.Load())
	}
}

func TestWireUnreachedCutoffCannotPass(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		i := int(requests.Add(1)) - 1
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		if i == 5 {
			w.Write(probeFrame("assistantResponseEvent", `{"content":"short"}`))
			return
		}
		w.Write(wireResponses(i))
		w.(http.Flusher).Flush()
		if i == 4 {
			<-r.Context().Done()
		}
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	c := got.Cases[5]
	if got.Verdict != "needs_evidence" || c.Status != "inconclusive" || c.Assertions.InjectionReached == nil || *c.Assertions.InjectionReached || c.Assertions.TentativeCompletion != nil {
		t.Errorf("wire unreached cutoff = %+v, want inconclusive with false trigger and unknown completion", got)
	}
}

type wireUnexpectedSnapshot struct{ t *testing.T }

func (r wireUnexpectedSnapshot) ReadProfileSnapshot(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
	r.t.Error("unexpected snapshot read")
	return credentials.ProfileSnapshot{}, errSource
}
