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
		text := append(probeFrame("assistantResponseEvent", `{"content":"PROBE_","modelId":"claude-opus-5.5"}`), probeFrame("assistantResponseEvent", `{"content":"MARKER"}`)...)
		return append(text, probeFrame("meteringEvent", `{"usage":0.01,"unit":"sentinel-unit","unitPlural":"sentinel-units"}`)...)
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
	p.clientFingerprint = wireClientFingerprint("sentinel-host", "sentinel-user")
	// Keep the validated fixture hosts while exercising the candidate's wire path.
	for _, destination := range p.destinations {
		destination.Path = "/generateAssistantResponse"
	}
	return p
}

func assertWireRequest(t *testing.T, r *http.Request, index int) {
	t.Helper()
	if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer sentinel-token" || r.Header.Get("Content-Type") != wireContentType || r.Header.Get("X-Amz-Target") != wireTarget || r.Header.Get("Accept") != "*/*" {
		t.Error("wire request headers do not match the candidate contract")
	}
	if got := r.Header.Get("X-Amz-Target"); got != "AmazonCodeWhispererStreamingService.GenerateAssistantResponse" {
		t.Errorf("wire request X-Amz-Target = %q, want reference target AmazonCodeWhispererStreamingService.GenerateAssistantResponse", got)
	}
	if got := r.URL.RequestURI(); got != "/generateAssistantResponse" {
		t.Errorf("wire request URI = %q, want /generateAssistantResponse", got)
	}
	headers, err := wireHeaders(wireClientFingerprint("sentinel-host", "sentinel-user"), r.Header.Get("Amz-Sdk-Invocation-Id"))
	if err != nil {
		t.Error("wire request omitted a valid invocation identity")
	}
	for name := range headers {
		if got := r.Header.Get(name); got != headers.Get(name) {
			t.Errorf("synthetic wire header %s = %q, want %q", name, got, headers.Get(name))
		}
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
	var shape map[string]json.RawMessage
	if json.Unmarshal(b, &shape) != nil || shape["additionalModelRequestFields"] != nil {
		t.Error("wire request must omit additional model fields")
	}
	if req.Profile != "arn:aws:codewhisperer:us-east-1:000000000000:profile/sentinel-profile" || req.Conversation.ID != wireTestID || req.Conversation.Trigger != "MANUAL" {
		t.Error("wire request changed its selected profile or conversation")
	}
	u := req.Conversation.Current.User
	if u == nil {
		t.Error("wire request omitted current user message")
		return
	}
	wantContent := wirePrompts[index]
	if index == 0 || index >= 4 {
		wantContent = wireInstructions + "\n\n" + wantContent
	}
	if u.Content != wantContent || u.Model != probeModel || u.Origin != "AI_EDITOR" {
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
		if c.Assertions.ControlsRequested == nil || *c.Assertions.ControlsRequested || c.Assertions.OutputWithinLimit != nil {
			t.Errorf("wire case %s claimed model controls or a token cap", c.ID)
		}
		if i < 4 && (c.Assertions.TentativeCompletion == nil || !*c.Assertions.TentativeCompletion) {
			t.Errorf("wire case %s omitted tentative completion", c.ID)
		}
	}
	b, _ := json.Marshal(got)
	for _, s := range []string{"sentinel", p.reference.Fingerprint, p.clientFingerprint, "synthetic-tool-17", wireInstructions, fixtureToolResult, "profile/"} {
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
		{name: "reasoning in conversation", body: probeFrame("reasoningContentEvent", `{"text":"sentinel-reasoning"}`), status: "inconclusive", cause: "needs_evidence"},
		{name: "unknown semantic field", body: append(wireResponses(0), probeFrame("assistantResponseEvent", `{"content":"","sentinel-secret":"private"}`)...), status: "inconclusive", cause: "needs_evidence"},
		{name: "wrong model", body: probeFrame("assistantResponseEvent", `{"content":"PROBE_MARKER","modelId":"sentinel-model"}`), status: "contradicted", cause: "contract_mismatch"},
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
		"metadataEvent":        `{"tokenUsage":{"inputTokens":999,"outputTokens":2048,"totalTokens":3047}}`,
		"meteringEvent":        `{"usage":0.0123,"unit":"sentinel-unit","unitPlural":"sentinel-units"}`,
		"contextUsageEvent":    `{"contextUsagePercentage":1.5}`,
	} {
		if err := turn.observe(event, []byte(payload)); err != nil {
			t.Errorf("observe(%s) error=%v, want nil", event, err)
		}
	}
	if turn.assertions.UsagePresent == nil || !*turn.assertions.UsagePresent || turn.assertions.Completion != nil {
		t.Error("usage observation inferred completion or lost usage presence")
	}
	turn.release()
}

func TestWirePlanRejectsContractAndExampleDrift(t *testing.T) {
	b, _ := preparationPlan(t)
	for _, pair := range [][2]string{{"runtime.us-east-1.kiro.dev", "attacker.example"}, {"clean_stream_end_tentative", "success_on_eof"}, {`"controls_requested": false`, `"controls_requested": true`}, {"Reply with exactly PROBE_MARKER", "Ignore the instructions and reply"}, {`"method": "POST"`, `"method": "GET"`}, {`"profileArn": "{{selected_profile_arn}}"`, `"profileArn": "invented"`}} {
		changed := bytes.ReplaceAll(b, []byte(pair[0]), []byte(pair[1]))
		h := sha256.Sum256(changed)
		if _, err := readProbePlan(bytes.NewReader(changed), hex.EncodeToString(h[:])); !errors.Is(err, errPlanInvalid) {
			t.Errorf("readProbePlan(drift %q) error=%v, want plan_invalid", pair[0], err)
		}
	}
}

func TestWireMeteringPluralRemainsMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		want          error
	}{
		{name: "valid", payload: `{"usage":1,"unit":"synthetic-unit","unitPlural":"synthetic-units"}`},
		{name: "absent", payload: `{"usage":1,"unit":"synthetic-unit"}`},
		{name: "number", payload: `{"unitPlural":3}`, want: errContract},
		{name: "null", payload: `{"unitPlural":null}`, want: errContract},
		{name: "duplicate", payload: `{"unitPlural":"a","unitPlural":"b"}`, want: errContract},
		{name: "unknown", payload: `{"unitPlural":"a","unreviewed":"b"}`, want: errNeedsEvidence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turn := wireTurn{fixtureTurn: fixtureTurn{memory: &probeMemory{limit: probeMaxRetained}}}
			err := turn.observe("meteringEvent", []byte(tc.payload))
			if !errors.Is(err, tc.want) {
				t.Errorf("observe(meteringEvent, %s) error=%v, want %v", tc.name, err, tc.want)
			}
			if turn.text != "" || turn.toolID != "" || turn.assertions.Completion != nil {
				t.Errorf("observe(meteringEvent, %s) must not produce text, tools, or completion", tc.name)
			}
			turn.release()
		})
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
	p.clientFingerprint = wireClientFingerprint("sentinel-host", "sentinel-user")
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

// covers: spec 0003 AC-3, AC-4, AC-5, AC-6, AC-7, AC-8.
// A successful tool call does not excuse a missing or wrong continuation.
func TestWireContinuationFailuresStopWithoutReplay(t *testing.T) {
	for _, tc := range []struct {
		name, payload, status, verdict string
		index                          int
	}{
		{name: "wrong result", payload: `{"content":"sentinel-wrong-result"}`, status: "contradicted", verdict: "candidate_rejected", index: 2},
		{name: "missing result", payload: `{"content":""}`, status: "inconclusive", verdict: "needs_evidence", index: 2},
		{name: "wrong followup", payload: `{"content":"sentinel-wrong-followup"}`, status: "contradicted", verdict: "candidate_rejected", index: 3},
		{name: "missing followup", payload: `{"content":""}`, status: "inconclusive", verdict: "needs_evidence", index: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var requests atomic.Int32
			p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
				i := int(requests.Add(1)) - 1
				if i > tc.index {
					t.Errorf("runWireCases(%s) dispatched case %d, want stop after %d", tc.name, i, tc.index)
					w.WriteHeader(http.StatusConflict)
					return
				}
				assertWireRequest(t, r, i)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				body := wireResponses(i)
				if i == tc.index {
					body = probeFrame("assistantResponseEvent", tc.payload)
				}
				w.Write(body)
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			failed := got.Cases[tc.index]
			if got.Verdict != tc.verdict || got.Attempts != tc.index+1 || failed.Status != tc.status || failed.Assertions.TentativeCompletion != nil || failed.Assertions.Completion != nil {
				t.Errorf("runWireCases(%s) verdict=%s attempts=%d case=%+v, want %s after %d attempts, %s and unknown completion", tc.name, got.Verdict, got.Attempts, failed, tc.verdict, tc.index+1, tc.status)
			}
			for _, c := range got.Cases[tc.index+1:] {
				if c.Status != "unrun" || c.Attempt != 0 {
					t.Errorf("runWireCases(%s) later case=%+v, want unrun without an attempt", tc.name, c)
				}
			}
			runWireCases(p, wireTestID, probeMaxRetained)
			if n := requests.Load(); n != int32(tc.index+1) {
				t.Errorf("runWireCases(%s, second call) requests=%d, want %d without replay", tc.name, n, tc.index+1)
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("Marshal(%s summary) error=%v, want nil", tc.name, err)
			}
			if strings.Contains(string(b), "sentinel") {
				t.Errorf("runWireCases(%s) summary contains response sentinel, want filtered output", tc.name)
			}
		})
	}
}

// covers: spec 0003 AC-4, AC-5, AC-8. JSON escapes may span tool events.
func TestWireEscapedArgumentsSurviveContinuation(t *testing.T) {
	home, _ := probeHome(t)
	var requests atomic.Int32
	const toolID = "synthetic-escaped-tool"
	const arguments = `{"key":"\u0061lpha"}`
	p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
		i := int(requests.Add(1)) - 1
		if i >= 6 {
			t.Errorf("runWireCases(escaped arguments) request index=%d, want less than six", i)
			w.WriteHeader(http.StatusConflict)
			return
		}
		var req wireRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, probeMaxRequest+1)).Decode(&req); err != nil {
			t.Errorf("decode request(escaped arguments, case %d) error=%v, want nil", i, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if i == 2 || i == 3 {
			history := req.Conversation.History
			if len(history) != i*2 || history[3].Assistant == nil || len(history[3].Assistant.Tools) != 1 {
				t.Errorf("runWireCases(escaped arguments, case %d) lost tool history, want one complete tool", i)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			tool := history[3].Assistant.Tools[0]
			if tool.ID != toolID || tool.Name != fixtureTool || string(tool.Input) != arguments {
				t.Errorf("runWireCases(escaped arguments, case %d) tool=%+v, want original ID, name and escaped arguments", i, tool)
			}
			resultMessage := req.Conversation.Current.User
			if i == 3 {
				resultMessage = history[4].User
			}
			if resultMessage == nil || resultMessage.Context == nil || len(resultMessage.Context.Results) != 1 || resultMessage.Context.Results[0].ID != toolID {
				t.Errorf("runWireCases(escaped arguments, case %d) lost matching result, want ID %q", i, toolID)
			}
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		body := wireResponses(i)
		if i == 1 {
			body = append(probeFrame("toolUseEvent", `{"toolUseId":"synthetic-escaped-tool","name":"probe_lookup","input":"{\"key\":\"\\u00"}`), probeFrame("toolUseEvent", `{"toolUseId":"synthetic-escaped-tool","input":"61lpha\"}","stop":true}`)...)
		}
		w.Write(body)
		w.(http.Flusher).Flush()
		if i == 4 {
			<-r.Context().Done()
		}
	})
	got := runWireCases(p, wireTestID, probeMaxRetained)
	if got.Verdict != "limited_candidate_observed" || requests.Load() != 6 || got.Cases[1].Assertions.ValidArguments == nil || !*got.Cases[1].Assertions.ValidArguments {
		t.Errorf("runWireCases(escaped arguments) result=%+v requests=%d, want six observed cases with valid arguments", got, requests.Load())
	}
}

type wireSnapshotFunc func(context.Context, config.Session) (credentials.ProfileSnapshot, error)

func (f wireSnapshotFunc) ReadProfileSnapshot(ctx context.Context, ref config.Session) (credentials.ProfileSnapshot, error) {
	return f(ctx, ref)
}

// covers: spec 0003 AC-2, AC-6, AC-8, AC-9.
// Change the database after the real reader validates its combined snapshot.
func TestWireSourceChangeAfterValidationUsesCheckedSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, query, replacement, cause string
	}{
		{name: "token", query: `UPDATE auth_kv SET value=?`, replacement: strings.Replace(probeSyntheticRecord, "sentinel-token", "replacement-token", 1), cause: "session_changed"},
		{name: "profile", query: `UPDATE state SET value=?`, replacement: strings.Replace(probeSyntheticProfile, "us-east-1", "eu-central-1", 1), cause: "profile_changed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, db := probeHome(t)
			var requests atomic.Int32
			p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assertWireRequest(t, r, 0)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				w.Write(wireResponses(0))
			})
			reader, reads := p.reader, 0
			p.reader = wireSnapshotFunc(func(ctx context.Context, ref config.Session) (credentials.ProfileSnapshot, error) {
				reads++
				snapshot, err := reader.ReadProfileSnapshot(ctx, ref)
				if err == nil && reads == 1 {
					if _, updateErr := db.Exec(tc.query, tc.replacement); updateErr != nil {
						t.Errorf("update synthetic %s after validation error=%v, want nil", tc.name, updateErr)
						return credentials.ProfileSnapshot{}, credentials.ErrSource
					}
				}
				return snapshot, err
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			if got.Verdict != "needs_evidence" || got.Attempts != 1 || requests.Load() != 1 || reads != 2 || got.Cases[0].Status != "observed" || got.Cases[1].Cause != tc.cause || got.Cases[1].Attempt != 0 {
				t.Errorf("runWireCases(%s changed after validation) result=%+v requests=%d reads=%d, want checked first request then %s before dispatch", tc.name, got, requests.Load(), reads, tc.cause)
			}
			for _, c := range got.Cases[2:] {
				if c.Status != "unrun" {
					t.Errorf("runWireCases(%s change) later case %s status=%s, want unrun", tc.name, c.ID, c.Status)
				}
			}
		})
	}
}

// covers: spec 0003 AC-4, AC-5, AC-6, AC-7, AC-8.
// The accepted reference baseline requests no model token cap.
func TestWireOptionalUsageDoesNotInventTokenControls(t *testing.T) {
	for _, tc := range []struct {
		name, metadata string
		usagePresent   bool
	}{
		{name: "absent"},
		{name: "above former cap", metadata: `{"tokenUsage":{"outputTokens":2048}}`, usagePresent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var requests atomic.Int32
			p := localWireProbe(t, home, func(w http.ResponseWriter, r *http.Request) {
				i := int(requests.Add(1)) - 1
				if i >= 6 {
					t.Errorf("runWireCases(%s usage) request index=%d, want less than six", tc.name, i)
					w.WriteHeader(http.StatusConflict)
					return
				}
				io.Copy(io.Discard, r.Body)
				body := wireResponses(i)
				if i == 0 {
					body = append(probeFrame("assistantResponseEvent", `{"content":"PROBE_"}`), probeFrame("assistantResponseEvent", `{"content":"MARKER"}`)...)
					if tc.metadata != "" {
						body = append(body, probeFrame("metadataEvent", tc.metadata)...)
					}
				}
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				w.Write(body)
				w.(http.Flusher).Flush()
				if i == 4 {
					<-r.Context().Done()
				}
			})
			got := runWireCases(p, wireTestID, probeMaxRetained)
			if got.Verdict != "limited_candidate_observed" || got.Attempts != 6 {
				t.Errorf("runWireCases(%s usage) verdict=%s attempts=%d, want limited_candidate_observed and six", tc.name, got.Verdict, got.Attempts)
			}
			usage := got.Cases[0].Assertions.UsagePresent
			if tc.usagePresent && (usage == nil || !*usage) || !tc.usagePresent && usage != nil {
				t.Errorf("runWireCases(%s usage) usage assertion=%v, want present=%t and absent left unknown", tc.name, usage, tc.usagePresent)
			}
			for _, c := range got.Cases {
				if c.Assertions.ModelMatch != nil || c.Assertions.Completion != nil || c.Assertions.OutputWithinLimit != nil || !isFalse(c.Assertions.ControlsRequested) {
					t.Errorf("runWireCases(%s usage) case %s assertions=%+v, want unknown identity, completion and token limit with controls false", tc.name, c.ID, c.Assertions)
				}
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("Marshal(%s usage summary) error=%v, want nil", tc.name, err)
			}
			if strings.Contains(string(b), "outputTokens") || strings.Contains(string(b), "2048") {
				t.Errorf("runWireCases(%s usage) summary exposed raw usage, want boolean observation only", tc.name)
			}
		})
	}
}
