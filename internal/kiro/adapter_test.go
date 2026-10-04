package kiro

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
	"kiro-gateway/internal/gateway"
)

// covers: AC-6. Failed cleanup permanently closes inference before source access.
func TestAdapterCleanupFailureDisablesInference(t *testing.T) {
	home, _ := probeHome(t)
	a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) {
		t.Error("Generate(cleanup fixture) reached network, want injected transport")
	})
	snapshot, err := a.reader.ReadProfileSnapshot(t.Context(), a.reference)
	if err != nil {
		t.Fatal(err)
	}
	reads, dials := 0, 0
	a.reader = wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
		reads++
		return snapshot, nil
	})
	a.transport.dial = func(context.Context, string) (net.Conn, error) {
		dials++
		return &delayedCloseConn{delay: 6 * time.Second}, nil
	}
	r := adapterRequest(t)
	synctest.Test(t, func(t *testing.T) {
		for attempt := 0; attempt < 2; attempt++ {
			_, err := a.Generate(t.Context(), r, func(bridge.Event) error { t.Error("Generate(cleanup failure) emitted output, want none"); return nil })
			if err == nil || bridge.SafeFailure(err).Category != "cleanup_failed" {
				t.Errorf("Generate(cleanup failure, attempt=%d) = %v, want cleanup_failed", attempt, err)
			}
		}
		if reads != 1 || dials != 1 {
			t.Errorf("Generate(after cleanup failure) reads=%d dials=%d, want one initial read and dial only", reads, dials)
		}
		// The second request was rejected while the first connection still had
		// an owner. Let that finite fixture finish before leaving the bubble.
		time.Sleep(time.Second)
		synctest.Wait()
		end, err := a.Generate(t.Context(), r, func(bridge.Event) error {
			t.Error("Generate(after late cleanup) emitted output, want none")
			return nil
		})
		if err == nil || bridge.SafeFailure(err).Category != "cleanup_failed" || end.Basis != "" || reads != 1 || dials != 1 {
			t.Errorf("Generate(after late cleanup) = %v, %v reads=%d dials=%d, want cleanup_failed without completion or new source access", end, err, reads, dials)
		}
	})
}

// covers: AC-5, AC-6. Tools become visible only after successful socket cleanup.
func TestAdapterEmitsCompletedToolAfterCleanup(t *testing.T) {
	home, _ := probeHome(t)
	a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) {
		t.Error("Generate(cleanup ordering) reached real socket, want in memory TLS")
	})
	snapshot, err := a.reader.ReadProfileSnapshot(t.Context(), a.reference)
	if err != nil {
		t.Fatal(err)
	}
	a.reader = wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) { return snapshot, nil })
	r := adapterRequest(t)
	synctest.Test(t, func(t *testing.T) {
		wire, conn := pipeTransport(t, time.Second, eventStreamResponse(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}","stop":true}`)))
		a.transport = wire
		start := time.Now()
		emitted := 0
		end, err := a.Generate(t.Context(), r, func(event bridge.Event) error {
			emitted++
			if time.Since(start) != time.Second || conn.calls.Load() != 1 {
				t.Errorf("Generate(cleanup ordering).emit elapsed=%v closes=%d, want 1s and one close", time.Since(start), conn.calls.Load())
			}
			if event.Tool == nil || event.Tool.ID != "call" || event.Tool.Name != "Read" || event.Tool.Input == nil || len(event.Tool.Input) != 0 {
				t.Errorf("Generate(cleanup ordering).event = %+v, want complete Read call with empty input", event)
			}
			return nil
		})
		if err != nil || end.Basis != bridge.InferredCleanEOF || emitted != 1 {
			t.Errorf("Generate(cleanup ordering) = %v, %v emitted=%d, want inferred completion and one tool", end, err, emitted)
		}
	})
}

// covers: AC-5, AC-6. A complete tool stays buffered when upstream cleanup fails.
func TestAdapterCleanupTimeoutWithholdsCompletedTools(t *testing.T) {
	home, _ := probeHome(t)
	a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) {
		t.Error("Generate(cleanup fixture) reached real socket, want in memory TLS")
	})
	snapshot, err := a.reader.ReadProfileSnapshot(t.Context(), a.reference)
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	a.reader = wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
		reads++
		return snapshot, nil
	})
	r := adapterRequest(t)
	synctest.Test(t, func(t *testing.T) {
		wire, conn := pipeTransport(t, 6*time.Second, eventStreamResponse(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}","stop":true}`)))
		a.transport = wire
		start := time.Now()
		for attempt := 0; attempt < 2; attempt++ {
			end, err := a.Generate(t.Context(), r, func(bridge.Event) error { t.Error("Generate(cleanup timeout) emitted tool, want none"); return nil })
			if err == nil || bridge.SafeFailure(err).Category != "cleanup_failed" || end.Basis != "" {
				t.Errorf("Generate(cleanup timeout, attempt=%d) = %v, %v, want cleanup_failed without completion", attempt, end, err)
			}
		}
		if time.Since(start) != 5*time.Second || reads != 1 {
			t.Errorf("Generate(cleanup timeout) elapsed=%v reads=%d, want 5s and one read", time.Since(start), reads)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if conn.calls.Load() != 1 {
			t.Errorf("Generate(cleanup timeout).Close calls = %d, want 1", conn.calls.Load())
		}
	})
}

// covers: AC-3. Each optional instruction contributes at its own position only.
func TestEncodeIndependentInstructionPlacementAndSchemas(t *testing.T) {
	for _, top := range []string{"", "same", "<system>literal</system>"} {
		for _, first := range []string{"", "same", "  spaced\n"} {
			for _, last := range []string{"", "same"} {
				body := bridge.Canonical(map[string]any{"model": bridge.Model, "max_tokens": 1, "system": top, "output_config": map[string]any{"effort": "high"}, "messages": []any{
					map[string]any{"role": "user", "content": "U1"}, map[string]any{"role": "system", "content": first}, map[string]any{"role": "assistant", "content": "A1"}, map[string]any{"role": "user", "content": "U2"}, map[string]any{"role": "system", "content": last},
				}, "tools": []any{map[string]any{"name": "Read", "input_schema": map[string]any{"type": "object", "additionalProperties": false, "$defs": map[string]any{"v": map[string]any{"enum": []any{json.Number("9007199254740993")}}}}}}})
				r, err := bridge.Parse(body, false)
				if err != nil {
					t.Fatal(err)
				}
				before := bridge.Canonical(r)
				wire, err := encodeRequest(r, "profile", "conversation")
				if err != nil {
					t.Fatal(err)
				}
				var root map[string]any
				decoder := json.NewDecoder(bytes.NewReader(wire))
				decoder.UseNumber()
				if err := decoder.Decode(&root); err != nil {
					t.Fatal(err)
				}
				state := root["conversationState"].(map[string]any)
				history := state["history"].([]any)
				past := history[0].(map[string]any)["userInputMessage"].(map[string]any)
				current := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
				wantPast, wantCurrent := "U1", "U2"
				if first != "" {
					wantPast += "\n\n" + first
				}
				if top != "" {
					wantPast = top + "\n\n" + wantPast
				}
				if last != "" {
					wantCurrent += "\n\n" + last
				}
				if len(history) != 2 || past["content"] != wantPast || current["content"] != wantCurrent || history[1].(map[string]any)["assistantResponseMessage"].(map[string]any)["content"] != "A1" {
					t.Errorf("encodeRequest(top=%q, first=%q, last=%q) content=%v/%v, want %q/%q with unchanged assistant", top, first, last, past["content"], current["content"], wantPast, wantCurrent)
				}
				tools := current["userInputMessageContext"].(map[string]any)["tools"].([]any)
				schema := tools[0].(map[string]any)["toolSpecification"].(map[string]any)["inputSchema"].(map[string]any)["json"]
				if !reflect.DeepEqual(schema, r.Tools[0].InputSchema) {
					t.Errorf("encodeRequest(schema) = %v, want full schema %v", schema, r.Tools[0].InputSchema)
				}
				again, err := encodeRequest(r, "profile", "conversation")
				if err != nil || !bytes.Equal(wire, again) || !bytes.Equal(before, bridge.Canonical(r)) || bytes.Contains(wire, []byte("output_config")) {
					t.Errorf("encodeRequest(repeated) error=%v, want unchanged source and identical wire without effort", err)
				}
			}
		}
	}
}

// covers: AC-2, AC-6. Reject invalid and overlapping requests before a snapshot.
func TestAdapterHTTPAdmissionPrecedesSource(t *testing.T) {
	home, _ := probeHome(t)
	a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) { t.Error("Generate(canceled source) dispatched, want none") })
	entered := make(chan struct{})
	var reads atomic.Int64
	a.reader = wireSnapshotFunc(func(ctx context.Context, _ config.Session) (credentials.ProfileSnapshot, error) {
		if reads.Add(1) == 1 {
			close(entered)
		}
		<-ctx.Done()
		return credentials.ProfileSnapshot{}, credentials.ErrCanceled
	})
	h := gateway.NewExperimentalHandler("synthetic-bearer", "test", slog.New(slog.NewTextHandler(io.Discard, nil)), a)
	valid := `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":"fixture"}]}`
	for _, tc := range []struct {
		name, body, auth string
		status           int
	}{
		{"authorization", "invalid", "wrong", http.StatusUnauthorized},
		{"validation", "{}", "synthetic-bearer", http.StatusBadRequest},
		{"model", strings.Replace(valid, bridge.Model, "unknown", 1), "synthetic-bearer", http.StatusBadRequest},
	} {
		r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+tc.auth)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Anthropic-Version", "2023-06-01")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status || reads.Load() != 0 {
			t.Errorf("Messages(%s) status=%d reads=%d, want %d and zero reads", tc.name, w.Code, reads.Load(), tc.status)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(valid)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer synthetic-bearer")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Anthropic-Version", "2023-06-01")
	done := make(chan struct{})
	go func() { defer close(done); h.ServeHTTP(httptest.NewRecorder(), r) }()
	t.Cleanup(func() { cancel(); <-done })
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Messages(valid) did not reach source, want admitted read")
	}
	if w := adapterHTTP(t, h, valid); w.Code != bridge.StatusOverloaded {
		t.Errorf("Messages(overlap).status = %d, want 529", w.Code)
	}
	for _, path := range []string{"/healthz", "/v1/messages/count_tokens"} {
		method := "POST"
		if path == "/healthz" {
			method = "GET"
		}
		req := httptest.NewRequest(method, path, strings.NewReader(valid))
		req.Header = r.Header.Clone()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("HTTP(%s while busy).status = %d, want 200", path, w.Code)
		}
	}
	if reads.Load() != 1 {
		t.Errorf("Messages(overlap and counts).reads = %d, want 1", reads.Load())
	}
	cancel()
	<-done
}

// covers: AC-3, AC-7, AC-8. Distinct sentinels expose cross boundary leaks.
func TestAdapterLogsExcludeContentAndPreserveSourceFiles(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("upstream_error=%t", fail), func(t *testing.T) {
			home, _ := probeHome(t)
			a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if fail {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(w, "PRIVATE_UPSTREAM_SENTINEL")
					return
				}
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				_, _ = w.Write(probeFrame("reasoningContentEvent", `{"text":"PRIVATE_REASONING_SENTINEL"}`))
				_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"PUBLIC_RESPONSE_SENTINEL"}`))
			})
			a.metadata = func() (string, error) { return strings.Repeat("ab", 32), nil }
			paths := []string{filepath.Join(home, "Library", "Application Support", "kiro-cli", "data.sqlite3"), filepath.Join(home, ".config", "kiro-gateway", "config.json")}
			before := make([][]byte, len(paths))
			for i, p := range paths {
				var err error
				before[i], err = os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
			}
			var logs bytes.Buffer
			h := gateway.NewExperimentalHandler("PRIVATE_GATEWAY_SENTINEL", "test", slog.New(slog.NewTextHandler(&logs, nil)), a)
			body := `{"model":"claude-opus-5.5","max_tokens":1,"system":"PRIVATE_SYSTEM_SENTINEL","metadata":{"user_id":"PRIVATE_METADATA_SENTINEL"},"messages":[{"role":"user","content":"PRIVATE_PROMPT_SENTINEL"},{"role":"assistant","content":[{"type":"tool_use","id":"old_id","name":"Read","input":{"value":"PRIVATE_ARGUMENT_SENTINEL"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"old_id","content":"PRIVATE_RESULT_SENTINEL","is_error":true}]},{"role":"system","content":"PRIVATE_HISTORY_SENTINEL"}],"tools":[{"name":"Read","description":"PRIVATE_DESCRIPTION_SENTINEL","input_schema":{"type":"object","properties":{"x":{"description":"PRIVATE_SCHEMA_SENTINEL"}}}}]}`
			r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer PRIVATE_GATEWAY_SENTINEL")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Anthropic-Version", "2023-06-01")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := http.StatusOK
			if fail {
				want = http.StatusBadGateway
			}
			if w.Code != want {
				t.Fatalf("Messages(sentinel fixture).status = %d, want %d", w.Code, want)
			}
			for _, sentinel := range []string{"sentinel-token", "sentinel-account", "sentinel-profile", "sentinel-name", strings.Repeat("ab", 32), "PRIVATE_GATEWAY_SENTINEL", "PRIVATE_SYSTEM_SENTINEL", "PRIVATE_METADATA_SENTINEL", "PRIVATE_PROMPT_SENTINEL", "PRIVATE_ARGUMENT_SENTINEL", "PRIVATE_RESULT_SENTINEL", "PRIVATE_HISTORY_SENTINEL", "PRIVATE_DESCRIPTION_SENTINEL", "PRIVATE_SCHEMA_SENTINEL", "PRIVATE_REASONING_SENTINEL", "PRIVATE_UPSTREAM_SENTINEL", "PUBLIC_RESPONSE_SENTINEL"} {
				if strings.Contains(logs.String(), sentinel) {
					t.Errorf("Messages(sentinel fixture).logs contain %q, want no content or account values", sentinel)
				}
				if sentinel != "PUBLIC_RESPONSE_SENTINEL" && strings.Contains(w.Body.String(), sentinel) {
					t.Errorf("Messages(sentinel fixture).response contains %q, want no private values", sentinel)
				}
			}
			for i, p := range paths {
				after, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before[i], after) {
					t.Errorf("Generate(sentinel fixture) changed %s, want original bytes", filepath.Base(p))
				}
			}
		})
	}
}

// covers: AC-5, AC-7. Exercise the production decoder through the HTTP handler.
func TestAdapterMalformedStreamsNeverComplete(t *testing.T) {
	stopped := probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}","stop":true}`)
	badCRC := probeFrame("assistantResponseEvent", `{"content":"hidden"}`)
	badCRC[len(badCRC)-1] ^= 1
	prelude := func(total, headers uint32) []byte {
		b := make([]byte, 12)
		binary.BigEndian.PutUint32(b, total)
		binary.BigEndian.PutUint32(b[4:], headers)
		binary.BigEndian.PutUint32(b[8:], crc32.ChecksumIEEE(b[:8]))
		return b
	}
	for _, tc := range []struct {
		name   string
		suffix []byte
	}{
		{"unknown event", probeFrame("inventedEvent", `{}`)},
		{"reasoning array", probeFrame("reasoningContentEvent", `[]`)},
		{"reasoning duplicate", probeFrame("reasoningContentEvent", `{"x":1,"x":2}`)},
		{"metadata wrong type", probeFrame("metadataEvent", `{"tokenUsage":[]}`)},
		{"metadata negative", probeFrame("metadataEvent", `{"tokenUsage":{"inputTokens":-1}}`)},
		{"metadata fractional output", probeFrame("metadataEvent", `{"tokenUsage":{"outputTokens":1.5}}`)},
		{"metadata unknown usage", probeFrame("metadataEvent", `{"tokenUsage":{"unknown":1}}`)},
		{"message metadata type", probeFrame("messageMetadataEvent", `{"conversationId":42}`)},
		{"model mismatch", probeFrame("assistantResponseEvent", `{"content":"hidden","modelId":"other"}`)},
		{"invalid arguments", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{","stop":true}`)},
		{"array arguments", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"[]","stop":true}`)},
		{"missing tool stop", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}"}`)},
		{"duplicate tool stop", append(append([]byte{}, stopped...), stopped...)},
		{"unoffered tool", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Unknown","input":"{}","stop":true}`)},
		{"exception", probeFrame("exception", `{"message":"PRIVATE_UPSTREAM_ERROR"}`)},
		{"post tool text", append(append([]byte{}, stopped...), probeFrame("assistantResponseEvent", `{"content":"late"}`)...)},
		{"partial prelude", []byte{1}},
		{"partial stopped tool", append(append([]byte{}, stopped...), 1)},
		{"frame CRC", badCRC},
		{"announced frame too large", prelude((1<<20)+1, 0)},
		{"announced frame too small", prelude(15, 0)},
		{"announced headers too large", prelude(1<<20, (16<<10)+1)},
		{"announced headers exceed frame", prelude(20, 5)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var dispatches atomic.Int64
			a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				dispatches.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"visible"}`))
				w.(http.Flusher).Flush()
				_, _ = w.Write(tc.suffix)
			})
			h := gateway.NewExperimentalHandler("synthetic-bearer", "test", slog.New(slog.NewTextHandler(io.Discard, nil)), a)
			w := adapterHTTP(t, h, `{"model":"claude-opus-5.5","max_tokens":1,"stream":true,"messages":[{"role":"user","content":"fixture"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
			body := w.Body.String()
			if w.Code != http.StatusOK || !strings.Contains(body, `"text":"visible"`) || strings.Count(body, "event: error\n") != 1 || strings.Contains(body, "event: message_stop") || strings.Contains(body, "event: message_delta") || strings.Contains(body, `"type":"tool_use"`) {
				t.Errorf("Messages(%s) status=%d visible=%t errors=%d terminal=%t tools=%t, want partial text, one error and no terminal or tools", tc.name, w.Code, strings.Contains(body, `"text":"visible"`), strings.Count(body, "event: error\n"), strings.Contains(body, "event: message_stop"), strings.Contains(body, `"type":"tool_use"`))
			}
			if dispatches.Load() != 1 {
				t.Errorf("Generate(%s).dispatches = %d, want 1", tc.name, dispatches.Load())
			}
		})
	}
}

func adapterHTTP(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer synthetic-bearer")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Anthropic-Version", "2023-06-01")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// covers: AC-5. Metadata and discarded reasoning cannot fabricate a completion.
func TestAdapterMetadataOnlyEOFRejectsCompletion(t *testing.T) {
	home, _ := probeHome(t)
	a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		_, _ = w.Write(probeFrame("metadataEvent", `{"tokenUsage":{"outputTokens":10}}`))
		_, _ = w.Write(probeFrame("reasoningContentEvent", `{"text":"discarded"}`))
	})
	end, err := a.Generate(t.Context(), adapterRequest(t), func(bridge.Event) error { t.Error("Generate(metadata only) emitted an event, want none"); return nil })
	if err == nil || end.Basis != "" {
		t.Errorf("Generate(metadata only) = %v, %v, want error without completion", end, err)
	}
}

// covers: AC-2, AC-8. Assert the full fixed HTTP table, including unknown errors.
func TestAdapterSourceErrorsHaveFixedHTTPMessages(t *testing.T) {
	for _, tc := range []struct {
		name              string
		source            error
		status            int
		category, message string
	}{
		{"missing", credentials.ErrSource, http.StatusServiceUnavailable, "source_unavailable", "Saved Kiro source is unavailable or unsupported. Check Kiro CLI sign in and local file access."},
		{"unknown", errors.New("PRIVATE_SOURCE_ERROR"), http.StatusServiceUnavailable, "source_unavailable", "Saved Kiro source is unavailable or unsupported. Check Kiro CLI sign in and local file access."},
		{"invalid", credentials.ErrRecord, http.StatusServiceUnavailable, "credential_invalid", "Saved Kiro token record is invalid or unsupported. Stop the gateway, sign in through Kiro CLI, link again, restore the model mapping, and restart."},
		{"expired", credentials.ErrExpired, http.StatusServiceUnavailable, "credential_expired", "Saved Kiro credential has expired. Stop the gateway, sign in through Kiro CLI, link again, restore the model mapping, and restart."},
		{"busy", credentials.ErrBusy, http.StatusServiceUnavailable, "source_busy", "Saved Kiro source is busy. Wait for Kiro CLI, then retry explicitly."},
		{"timeout", credentials.ErrTimeout, http.StatusGatewayTimeout, "source_timeout", "Reading the saved Kiro source timed out. Check local source availability before retrying."},
		{"changed", credentials.ErrChanged, http.StatusConflict, "session_changed", "Saved Kiro session has changed. Stop the gateway, link again, restore the model mapping, and restart."},
		{"profile invalid", credentials.ErrProfileInvalid, http.StatusServiceUnavailable, "profile_invalid", "Saved Kiro profile is invalid. Check the selected profile through Kiro CLI, then restart the gateway."},
		{"profile unsupported", credentials.ErrProfileUnsupported, http.StatusServiceUnavailable, "profile_unsupported", "Saved Kiro profile is unsupported by this gateway. Check the supported profile and region before restarting."},
		{"source canceled", credentials.ErrCanceled, http.StatusServiceUnavailable, "source_canceled", "Reading the saved Kiro source was canceled. Retry only when the gateway is ready."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) {
				t.Errorf("Generate(%s) dispatched, want zero calls", tc.name)
			})
			a.reader = wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
				return credentials.ProfileSnapshot{}, fmt.Errorf("PRIVATE_SOURCE_ERROR: %w", tc.source)
			})
			var logs bytes.Buffer
			h := gateway.NewExperimentalHandler("synthetic-bearer", "test", slog.New(slog.NewTextHandler(&logs, nil)), a)
			w := adapterHTTP(t, h, `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":"fixture"}]}`)
			var body struct {
				Type      string
				Error     struct{ Type, Message string }
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("Messages(%s).decode = %v, want valid JSON", tc.name, err)
			}
			if w.Code != tc.status || body.Type != "error" || body.Error.Type != "api_error" || body.Error.Message != tc.message || body.RequestID == "" || body.RequestID != w.Header().Get("request-id") {
				t.Errorf("Messages(%s) status=%d type=%s message=%q id_matches=%t, want %d api_error %q and matching request ID", tc.name, w.Code, body.Error.Type, body.Error.Message, body.RequestID == w.Header().Get("request-id"), tc.status, tc.message)
			}
			if !strings.Contains(logs.String(), "error_category="+tc.category) || strings.Contains(logs.String()+w.Body.String(), "PRIVATE_SOURCE_ERROR") {
				t.Errorf("Messages(%s) category or redaction differs, want %s and no private source error", tc.name, tc.category)
			}
		})
	}
}

// covers: AC-2. The real combined reader observes changed bytes before dispatch.
func TestAdapterChangedTokenNeverDispatches(t *testing.T) {
	home, db := probeHome(t)
	a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) {
		t.Error("Generate(changed token) dispatched, want zero calls")
	})
	if _, err := db.Exec(`UPDATE auth_kv SET value=?`, strings.Replace(probeSyntheticRecord, "sentinel-token", "different-token", 1)); err != nil {
		t.Fatal(err)
	}
	_, err := a.Generate(t.Context(), adapterRequest(t), func(bridge.Event) error { t.Error("Generate(changed token) emitted, want none"); return nil })
	if err == nil || bridge.SafeFailure(err).Category != "session_changed" {
		t.Errorf("Generate(changed token) = %v, want session_changed", err)
	}
}

// covers: AC-2. Exercise expiry and SQLite locking through the real snapshot reader.
func TestAdapterExpiredAndBusySQLiteNeverDispatch(t *testing.T) {
	for _, mode := range []string{"expired", "busy"} {
		t.Run(mode, func(t *testing.T) {
			home, db := probeHome(t)
			a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) { t.Errorf("Generate(%s source) dispatched, want none", mode) })
			want := "credential_expired"
			if mode == "expired" {
				a.reader = credentials.Reader{Home: home, Now: func() time.Time { return time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC) }}
			} else {
				want = "source_busy"
				db.SetMaxOpenConns(1)
				if _, err := db.Exec("BEGIN EXCLUSIVE"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := db.Exec("ROLLBACK"); err != nil {
						t.Errorf("ROLLBACK(fixture) = %v, want nil", err)
					}
				})
			}
			_, err := a.Generate(t.Context(), adapterRequest(t), func(bridge.Event) error { t.Errorf("Generate(%s source) emitted, want none", mode); return nil })
			if err == nil || bridge.SafeFailure(err).Category != want {
				t.Errorf("Generate(%s source) = %v, want %s", mode, err, want)
			}
		})
	}
}

func adapterFixture(t *testing.T, home string, h http.HandlerFunc) *Adapter {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"runtime.us-east-1.kiro.dev", "runtime.eu-central-1.kiro.dev"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(h)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	reader := credentials.Reader{Home: home}
	ref, err := reader.Capture(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	doc := config.Default()
	doc.Link(ref)
	doc.Models[bridge.Model] = bridge.Model
	a, err := New(reader, doc)
	if err != nil {
		t.Fatal(err)
	}
	a.metadata = func() (string, error) { return strings.Repeat("0", 64), nil }
	a.transport = transport{roots: roots, dial: func(ctx context.Context, address string) (net.Conn, error) {
		if address != "runtime.us-east-1.kiro.dev:443" && address != "runtime.eu-central-1.kiro.dev:443" {
			return nil, errors.New("bad fixture destination")
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp4", server.Listener.Addr().String())
	}}
	return a
}
func adapterRequest(t *testing.T) bridge.Request {
	t.Helper()
	r, err := bridge.Parse([]byte(`{"model":"claude-opus-5.5","max_tokens":1,"system":"T","messages":[{"role":"user","content":"U"},{"role":"system","content":"S"}],"tools":[{"name":"Read","input_schema":{"type":"object","additionalProperties":false}}]}`), false)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAdapterSyntheticTextToolsAndProfilePin(t *testing.T) {
	home, db := probeHome(t)
	var dispatches atomic.Int64
	a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
		dispatches.Add(1)
		if r.Header.Get("Authorization") != "Bearer sentinel-token" || r.URL.Path != "/generateAssistantResponse" || r.ProtoMajor != 1 || !r.Close {
			t.Error("wire auth, path, or lifecycle differs from contract")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("wire body invalid")
		}
		state := body["conversationState"].(map[string]any)
		u := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
		if u["content"] != "T\n\nU\n\nS" {
			t.Error("wire instruction placement changed")
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"one"}`))
		w.(http.Flusher).Flush()
		_, _ = w.Write(probeFrame("reasoningContentEvent", `{"invented":"discard"}`))
		_, _ = w.Write(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{\"n\":9007199254740993}","stop":true}`))
	})
	r := adapterRequest(t)
	var events []bridge.Event
	end, err := a.Generate(t.Context(), r, func(e bridge.Event) error { events = append(events, e); return nil })
	if err != nil || end.Basis != bridge.InferredCleanEOF || end.ReasoningEvents != 1 || len(events) != 2 {
		t.Fatalf("Generate(synthetic) end=%v err=%v events=%d, want clean text/tool", end, err, len(events))
	}
	if events[1].Tool.Input["n"] != json.Number("9007199254740993") {
		t.Error("tool integer lost precision")
	}
	if _, err := db.Exec(`UPDATE state SET value=?`, probeSyntheticProfile+" "); err != nil {
		t.Fatal(err)
	}
	_, err = a.Generate(t.Context(), r, func(bridge.Event) error { t.Error("changed source emitted output"); return nil })
	if err == nil || bridge.SafeFailure(err).Category != "profile_changed" || dispatches.Load() != 1 {
		t.Errorf("Generate(changed profile)=%v dispatches=%d, want profile_changed and 1", err, dispatches.Load())
	}
}

func TestAdapterWithholdsToolsOnInvalidEnd(t *testing.T) {
	for _, suffix := range []struct {
		name string
		data []byte
	}{
		{"unfinished", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{"}`)},
		{"truncated after stopped tool", append(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}","stop":true}`), 1)},
		{"text after tool", append(probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{}","stop":true}`), probeFrame("assistantResponseEvent", `{"content":"late"}`)...)},
		{"duplicate argument", probeFrame("toolUseEvent", `{"toolUseId":"call","name":"Read","input":"{\"a\":1,\"a\":2}","stop":true}`)},
		{"unknown semantic field", probeFrame("assistantResponseEvent", `{"content":"late","unknown":1}`)},
	} {
		t.Run(suffix.name, func(t *testing.T) {
			home, _ := probeHome(t)
			a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"partial"}`))
				_, _ = w.Write(suffix.data)
			})
			tools, texts := 0, 0
			end, err := a.Generate(t.Context(), adapterRequest(t), func(e bridge.Event) error {
				if e.Tool != nil {
					tools++
				} else {
					texts++
				}
				return nil
			})
			if err == nil || end.Basis != "" || tools != 0 || texts != 1 {
				t.Errorf("Generate(%s) end=%v err=%v tools=%d texts=%d, want partial text only", suffix.name, end, err, tools, texts)
			}
		})
	}
}

func TestAdapterCancellationAndNoReplay(t *testing.T) {
	home, _ := probeHome(t)
	var dispatches atomic.Int64
	a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
		dispatches.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"partial"}`))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := time.Now()
	_, err := a.Generate(ctx, adapterRequest(t), func(bridge.Event) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) || time.Since(started) > 5*time.Second || dispatches.Load() != 1 {
		t.Errorf("Generate(cancel)=%v dispatches=%d, want canceled once", err, dispatches.Load())
	}
}

func TestEncodeHistoryPairingPreservesInputsAndResults(t *testing.T) {
	body := `{"model":"claude-opus-5.5","max_tokens":1,"system":"T","messages":[{"role":"user","content":"U1"},{"role":"system","content":"S1"},{"role":"assistant","content":[{"type":"tool_use","id":"c","name":"Old","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"c","content":[],"is_error":true}]},{"role":"system","content":"S2"}]}`
	r, err := bridge.Parse([]byte(body), false)
	if err != nil {
		t.Fatal(err)
	}
	before := bridge.Canonical(r)
	encoded, err := encodeRequest(r, "invented-profile", "invented-conversation")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if json.Unmarshal(encoded, &got) != nil {
		t.Fatal("decode failed")
	}
	state := got["conversationState"].(map[string]any)
	history := state["history"].([]any)
	first := history[0].(map[string]any)["userInputMessage"].(map[string]any)
	current := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	results := current["userInputMessageContext"].(map[string]any)["toolResults"]
	want := []any{map[string]any{"toolUseId": "c", "status": "error", "content": []any{map[string]any{"text": ""}}}}
	if len(history) != 2 || first["content"] != "T\n\nU1\n\nS1" || current["content"] != "\n\nS2" || !reflect.DeepEqual(results, want) || !bytes.Equal(before, bridge.Canonical(r)) {
		t.Error("encodeRequest(history) changed pairing, results, or normalized input")
	}
	again, err := encodeRequest(r, "invented-profile", "invented-conversation")
	if err != nil || !bytes.Equal(encoded, again) {
		t.Error("repeated encoding accumulated instructions")
	}
}

func TestAdapterSourceFailuresNeverDispatch(t *testing.T) {
	for _, tc := range []struct {
		source   error
		category string
		status   int
	}{
		{credentials.ErrSource, "source_unavailable", http.StatusServiceUnavailable}, {credentials.ErrRecord, "credential_invalid", http.StatusServiceUnavailable}, {credentials.ErrExpired, "credential_expired", http.StatusServiceUnavailable}, {credentials.ErrBusy, "source_busy", http.StatusServiceUnavailable}, {credentials.ErrTimeout, "source_timeout", http.StatusGatewayTimeout}, {credentials.ErrChanged, "session_changed", http.StatusConflict}, {credentials.ErrProfileInvalid, "profile_invalid", http.StatusServiceUnavailable}, {credentials.ErrProfileUnsupported, "profile_unsupported", http.StatusServiceUnavailable}, {credentials.ErrCanceled, "source_canceled", http.StatusServiceUnavailable},
	} {
		t.Run(tc.category, func(t *testing.T) {
			home, _ := probeHome(t)
			a := adapterFixture(t, home, func(http.ResponseWriter, *http.Request) { t.Error("source failure dispatched") })
			a.reader = wireSnapshotFunc(func(context.Context, config.Session) (credentials.ProfileSnapshot, error) {
				return credentials.ProfileSnapshot{}, fmt.Errorf("synthetic private diagnostic: %w", tc.source)
			})
			_, err := a.Generate(t.Context(), adapterRequest(t), func(bridge.Event) error { t.Error("source failure emitted"); return nil })
			if err == nil {
				t.Fatal("Generate(source error) succeeded")
			}
			f := bridge.SafeFailure(err)
			if f.Category != tc.category || f.Status != tc.status || strings.Contains(f.Message, "synthetic") {
				t.Errorf("Generate(source) category=%s status=%d, want %s %d", f.Category, f.Status, tc.category, tc.status)
			}
		})
	}
}
func TestAdapterRedirectThrottleAndIdle(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		category string
	}{{"redirect", http.StatusTemporaryRedirect, "upstream_status"}, {"throttle", http.StatusTooManyRequests, "upstream_throttle"}, {"forbidden", http.StatusForbidden, "upstream_status"}, {"idle", 0, "timed_out"}} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := probeHome(t)
			var attempts atomic.Int64
			a := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if tc.status == 0 {
					<-r.Context().Done()
					return
				}
				w.Header().Set("Location", "http://127.0.0.1:1/forbidden")
				w.WriteHeader(tc.status)
			})
			a.transport.idle = 100 * time.Millisecond
			_, err := a.Generate(t.Context(), adapterRequest(t), func(bridge.Event) error { return nil })
			if err == nil || bridge.SafeFailure(err).Category != tc.category || attempts.Load() != 1 {
				t.Errorf("Generate(%s)=%v attempts=%d, want category=%s once", tc.name, err, attempts.Load(), tc.category)
			}
		})
	}
}
func TestFrameChecksumsAndLimits(t *testing.T) {
	valid := probeFrame("assistantResponseEvent", `{"content":"ok"}`)
	for _, offset := range []int{8, len(valid) - 1} {
		bad := append([]byte{}, valid...)
		bad[offset] ^= 1
		called := false
		err := readFrames(bytes.NewReader(bad), func(string, []byte) error { called = true; return nil })
		if err == nil || called {
			t.Errorf("readFrames(corrupt byte %d) err=%v called=%t, want reject before consume", offset, err, called)
		}
	}
}
