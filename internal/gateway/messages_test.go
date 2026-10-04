package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"kiro-gateway/internal/bridge"
)

type blockedDeltaWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *blockedDeltaWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}
func (w *blockedDeltaWriter) Write(b []byte) (int, error) {
	if bytes.Contains(b, []byte("event: content_block_delta\n")) {
		time.Sleep(time.Until(w.deadline))
		return 0, context.DeadlineExceeded
	}
	return w.ResponseRecorder.Write(b)
}

// covers: AC-6. A canceled slow write holds admission until the generator exits.
func TestMessagesSlowWriterKeepsAdmissionUntilCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		calls := 0
		h := messagesHandlerForTest(t, generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
			calls++
			if err := emit(bridge.Event{Text: "visible"}); err != nil {
				return bridge.End{}, err
			}
			return bridge.End{Basis: bridge.InferredCleanEOF}, nil
		}))
		body := strings.Replace(messageBody, `"max_tokens":1`, `"max_tokens":1,"stream":true`, 1)
		r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Authorization", dummyBearer)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Anthropic-Version", "2023-06-01")
		w := &blockedDeltaWriter{ResponseRecorder: httptest.NewRecorder()}
		done := make(chan struct{})
		go func() { defer close(done); h.ServeHTTP(w, r) }()
		synctest.Wait()
		time.Sleep(time.Second)
		cancel()
		synctest.Wait()
		if got := messagesRequest(t, h, "/v1/messages", messageBody, nil).Code; got != bridge.StatusOverloaded {
			t.Errorf("Messages(during slow cleanup).status = %d, want 529", got)
		}
		if got := messagesRequest(t, h, "/v1/messages/count_tokens", messageBody, nil).Code; got != http.StatusOK {
			t.Errorf("CountTokens(during slow cleanup).status = %d, want 200", got)
		}
		if calls != 1 {
			t.Errorf("Generate(during slow cleanup).calls = %d, want 1", calls)
		}
		time.Sleep(4 * time.Second)
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Error("Messages(canceled slow writer) active at write deadline, want joined handler")
		}
		if strings.Contains(w.Body.String(), "event: message_stop") {
			t.Error("Messages(canceled slow writer) emitted message_stop, want no terminal")
		}
		if got := messagesRequest(t, h, "/v1/messages", messageBody, nil).Code; got != http.StatusOK || calls != 2 {
			t.Errorf("Messages(after slow cleanup) status=%d calls=%d, want 200 and 2", got, calls)
		}
	})
}

// covers: AC-3, AC-7. Reconstruct the client stream independently of Response.
func TestMessagesJSONAndSSEPreserveToolsAndEstimates(t *testing.T) {
	body := `{"model":"claude-opus-5.5","max_tokens":1,"system":"top","messages":[{"role":"user","content":"question"},{"role":"system","content":"suffix"}],"tools":[{"name":"Read","input_schema":{"type":"object"}},{"name":"Edit","input_schema":{"type":"object"}}]}`
	h := messagesHandlerForTest(t, generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
		for _, e := range []bridge.Event{
			{Text: "hello "}, {Text: "世界"},
			{Tool: &bridge.Block{Type: "tool_use", ID: "read_id", Name: "Read", Input: map[string]any{"n": json.Number("9007199254740993")}}},
			{Tool: &bridge.Block{Type: "tool_use", ID: "edit_id", Name: "Edit", Input: map[string]any{"text": "\"quoted\"\n"}}},
		} {
			if err := emit(e); err != nil {
				return bridge.End{}, err
			}
		}
		return bridge.End{Basis: bridge.InferredCleanEOF}, nil
	}))
	decode := func(raw []byte) map[string]any {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var value map[string]any
		if err := d.Decode(&value); err != nil {
			t.Fatalf("Decode(response) = %v, want JSON object", err)
		}
		return value
	}
	ordinary := messagesRequest(t, h, "/v1/messages", body, nil)
	if ordinary.Code != http.StatusOK {
		t.Fatalf("Messages(JSON).status = %d, want 200", ordinary.Code)
	}
	want := decode(ordinary.Body.Bytes())
	stream := messagesRequest(t, h, "/v1/messages", strings.Replace(body, `"max_tokens":1`, `"max_tokens":1,"stream":true`, 1), nil)
	var content []any
	var initial, final map[string]any
	var stop any
	terminal := 0
	for _, frame := range strings.Split(stream.Body.String(), "\n\n") {
		lines := strings.Split(frame, "\n")
		if len(lines) < 2 {
			continue
		}
		value := decode([]byte(strings.TrimPrefix(lines[1], "data: ")))
		switch value["type"] {
		case "message_start":
			initial = value["message"].(map[string]any)["usage"].(map[string]any)
		case "content_block_start":
			if value["index"] != json.Number(fmt.Sprint(len(content))) {
				t.Errorf("SSE block index = %v, want %d", value["index"], len(content))
			}
			content = append(content, value["content_block"])
		case "content_block_delta":
			delta := value["delta"].(map[string]any)
			block := content[len(content)-1].(map[string]any)
			if delta["type"] == "text_delta" {
				block["text"] = block["text"].(string) + delta["text"].(string)
			} else {
				block["input"] = decode([]byte(delta["partial_json"].(string)))
			}
		case "message_delta":
			final = value["usage"].(map[string]any)
			stop = value["delta"].(map[string]any)["stop_reason"]
		case "message_stop":
			terminal++
		case "error":
			t.Errorf("SSE(valid tools) = error, want completed message")
		}
	}
	count := decode(messagesRequest(t, h, "/v1/messages/count_tokens", body, nil).Body.Bytes())
	usage := want["usage"].(map[string]any)
	if !reflect.DeepEqual(content, want["content"]) {
		t.Errorf("Messages(SSE).content = %v, want JSON content %v", content, want["content"])
	}
	if initial["input_tokens"] != count["input_tokens"] || usage["input_tokens"] != count["input_tokens"] || initial["output_tokens"] != json.Number("0") || final["output_tokens"] != usage["output_tokens"] {
		t.Errorf("Messages(usage) initial=%v final=%v JSON=%v count=%v, want equal input and final output estimates", initial, final, usage, count)
	}
	if stop != "tool_use" || want["stop_reason"] != stop || terminal != 1 {
		t.Errorf("Messages(SSE) stop=%v terminal=%d, want tool_use and one terminal", stop, terminal)
	}
}

// covers: AC-2, AC-3, AC-7. Every subset is legal on both authenticated routes.
func TestMessagesBetaSubsetsAndEffortDoNotChangeRequests(t *testing.T) {
	tokens := []string{"claude-code-20250219", "interleaved-thinking-2025-05-14", "mid-conversation-system-2026-04-07", "effort-2025-11-24"}
	var requests []bridge.Request
	h := messagesHandlerForTest(t, generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
		requests = append(requests, r)
		return bridge.End{Basis: bridge.InferredCleanEOF}, emit(bridge.Event{Text: "ok"})
	}))
	base, err := bridge.Parse([]byte(messageBody), false)
	if err != nil {
		t.Fatal(err)
	}
	for mask := 0; mask < 16; mask++ {
		for _, effort := range []bool{false, true} {
			body := messageBody
			if effort {
				body = strings.Replace(body, `"max_tokens":1`, `"max_tokens":1,"output_config":{"effort":"high"}`, 1)
			}
			for _, path := range []string{"/v1/messages", "/v1/messages/count_tokens"} {
				w := messagesRequest(t, h, path, body, func(r *http.Request) {
					for i := len(tokens) - 1; i >= 0; i-- {
						if mask&(1<<i) != 0 {
							r.Header.Add("Anthropic-Beta", " \t"+tokens[i]+"\t ")
						}
					}
				})
				if w.Code != http.StatusOK {
					t.Errorf("Messages(%s, subset=%d, effort=%t).status = %d, want 200", path, mask, effort, w.Code)
				}
				for k, v := range map[string]string{"Compatibility": "experimental", "Usage": "estimated", "Completion": "inferred", "Controls": "advisory", "Effort": "ignored", "Betas": "unsupported", "Instructions": "user-context"} {
					if got := w.Header().Get("X-Kiro-Gateway-" + k); got != v {
						t.Errorf("Messages(%s).header[%s] = %q, want %q", path, k, got, v)
					}
				}
			}
		}
	}
	if len(requests) != 32 {
		t.Errorf("Generate(subsets).calls = %d, want 32", len(requests))
	}
	for _, r := range requests {
		if !reflect.DeepEqual(r, base) {
			t.Errorf("Generate(subsets).request = %+v, want unchanged normalized request %+v", r, base)
		}
	}
}

// covers: AC-2, AC-6. A canceled source differs from a disconnected client.
func TestMessagesCancellationCausesRemainDistinct(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cause    error
		status   int
		category string
	}{
		{"disconnect", context.Canceled, 0, "canceled"},
		{"deadline", context.DeadlineExceeded, http.StatusGatewayTimeout, "timed_out"},
		{"shutdown", &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "Gateway is stopping.", Category: "stopping"}, http.StatusServiceUnavailable, "stopping"},
		{"run stop", &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "The live run has stopped.", Category: "run_stopped"}, http.StatusServiceUnavailable, "run_stopped"},
		{"run deadline", &bridge.Failure{Status: http.StatusGatewayTimeout, Type: "api_error", Message: "Live run budget exhausted.", Category: "budget_exhausted"}, http.StatusGatewayTimeout, "budget_exhausted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(t.Context())
			t.Cleanup(func() { cancel(context.Canceled) })
			var logs bytes.Buffer
			h := NewExperimentalHandler(strings.TrimPrefix(dummyBearer, "Bearer "), "test", slog.New(slog.NewTextHandler(&logs, nil)), generateFunc(func(context.Context, bridge.Request, func(bridge.Event) error) (bridge.End, error) {
				cancel(tc.cause)
				return bridge.End{}, context.Canceled
			}))
			w := messagesRequest(t, h, "/v1/messages", messageBody, func(r *http.Request) { *r = *r.WithContext(ctx) })
			if tc.status == 0 {
				if w.Body.Len() != 0 {
					t.Errorf("Messages(disconnect).bytes = %d, want 0", w.Body.Len())
				}
			} else if w.Code != tc.status {
				t.Errorf("Messages(%s).status = %d, want %d", tc.name, w.Code, tc.status)
			}
			if !strings.Contains(logs.String(), "error_category="+tc.category) {
				t.Errorf("Messages(%s).category log = %q, want %s", tc.name, logs.String(), tc.category)
			}
		})
	}
}

type generateFunc func(context.Context, bridge.Request, func(bridge.Event) error) (bridge.End, error)

func (f generateFunc) Generate(c context.Context, r bridge.Request, e func(bridge.Event) error) (bridge.End, error) {
	return f(c, r, e)
}

const messageBody = `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":"synthetic"}]}`
const dummyBearer = "Bearer synthetic-gateway-token-for-tests-0001"

type cleanupObservations struct{ values []bridge.Observation }

func (o *cleanupObservations) Before(bridge.Observation) error  { return nil }
func (o *cleanupObservations) Observe(value bridge.Observation) { o.values = append(o.values, value) }

type failedCleanupWriter struct{ *httptest.ResponseRecorder }

func (w *failedCleanupWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type failedErrorEventWriter struct {
	*httptest.ResponseRecorder
	errorWrites int
}

func (w *failedErrorEventWriter) Write(b []byte) (int, error) {
	if bytes.HasPrefix(b, []byte("event: error\n")) {
		w.errorWrites++
		return 0, errors.New("synthetic-client-write-secret")
	}
	return w.ResponseRecorder.Write(b)
}

// covers: AC-5, AC-6, AC-8, AC-10. A broken error write cannot turn failed
// cleanup into a successful cleanup observation or a terminal client event.
func TestMessagesCleanupErrorWriteAbortsWithoutCompletion(t *testing.T) {
	observer := &cleanupObservations{}
	var logs bytes.Buffer
	h := NewObservedExperimentalHandler(strings.TrimPrefix(dummyBearer, "Bearer "), "test", slog.New(slog.NewTextHandler(&logs, nil)), generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
		if err := emit(bridge.Event{Text: "synthetic-partial-text"}); err != nil {
			return bridge.End{}, err
		}
		return bridge.End{}, &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "Inference cleanup failed. Restart the gateway.", Category: "cleanup_failed"}
	}), observer)
	body := strings.Replace(messageBody, `"max_tokens":1`, `"max_tokens":1,"stream":true`, 1)
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	r.Header.Set("Authorization", dummyBearer)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Anthropic-Version", "2023-06-01")
	w := &failedErrorEventWriter{ResponseRecorder: httptest.NewRecorder()}
	var aborted any
	func() {
		defer func() { aborted = recover() }()
		h.ServeHTTP(w, r)
	}()
	if aborted != http.ErrAbortHandler || w.errorWrites != 1 {
		t.Errorf("Messages(broken cleanup error write) panic=%v error_writes=%d, want ErrAbortHandler and one attempt", aborted, w.errorWrites)
	}
	if !strings.Contains(w.Body.String(), "synthetic-partial-text") || strings.Contains(w.Body.String(), "event: message_stop") || strings.Contains(w.Body.String(), "event: message_delta") {
		t.Errorf("Messages(broken cleanup error write).body = %q, want partial text without completion", w.Body.String())
	}
	cleanup := 0
	for _, observation := range observer.values {
		if observation.Phase == "terminal" || observation.Cleanup {
			t.Errorf("Messages(broken cleanup error write).observation = %+v, want no terminal or completed cleanup", observation)
		}
		if observation.Phase == "cleanup" && observation.Category == "cleanup_failed" {
			cleanup++
		}
	}
	if cleanup != 1 {
		t.Errorf("Messages(broken cleanup error write).cleanup_failed observations = %d, want 1", cleanup)
	}
	if !strings.Contains(logs.String(), "error_category=cleanup_failed") || strings.Contains(logs.String(), "synthetic-client-write-secret") || strings.Contains(logs.String(), "synthetic-partial-text") {
		t.Errorf("Messages(broken cleanup error write).logs = %q, want fixed cleanup category without content", logs.String())
	}
}

// covers: AC-5, AC-6. Ordinary JSON never exposes buffered text as a successful
// message when cleanup fails after generation.
func TestMessagesJSONCleanupFailureDiscardsBufferedText(t *testing.T) {
	var logs bytes.Buffer
	h := NewExperimentalHandler(strings.TrimPrefix(dummyBearer, "Bearer "), "test", slog.New(slog.NewTextHandler(&logs, nil)), generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
		if err := emit(bridge.Event{Text: "synthetic-buffered-text"}); err != nil {
			return bridge.End{}, err
		}
		return bridge.End{}, &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "Inference cleanup failed. Restart the gateway.", Category: "cleanup_failed"}
	}))
	w := messagesRequest(t, h, "/v1/messages", messageBody, nil)
	var got struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("Messages(JSON cleanup failure) decode = %v, want JSON error", err)
	}
	if w.Code != http.StatusServiceUnavailable || got.Type != "error" || got.Error.Type != "api_error" || got.Error.Message != "Inference cleanup failed. Restart the gateway." {
		t.Errorf("Messages(JSON cleanup failure) status=%d body=%+v, want fixed 503 api_error", w.Code, got)
	}
	if strings.Contains(w.Body.String()+logs.String(), "synthetic-buffered-text") || strings.Contains(logs.String(), "completion_basis=") {
		t.Errorf("Messages(JSON cleanup failure) body=%q logs=%q, want no buffered text or completion claim", w.Body.String(), logs.String())
	}
}

// covers: AC-6, AC-10. Cancellation must not relabel failed cleanup as successful.
func TestMessagesCleanupFailureSurvivesCancellation(t *testing.T) {
	for _, mode := range []string{"disconnect", "deadline", "client write", "stream", "before output"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(t.Context())
			t.Cleanup(func() { cancel(context.Canceled) })
			var logs bytes.Buffer
			observer := &cleanupObservations{}
			generator := generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
				switch mode {
				case "disconnect":
					cancel(context.Canceled)
				case "deadline":
					cancel(context.DeadlineExceeded)
				case "client write":
					_ = emit(bridge.Event{Text: "partial"})
				case "stream":
					if err := emit(bridge.Event{Text: "partial"}); err != nil {
						t.Errorf("emit(before cleanup failure) = %v, want nil", err)
					}
				}
				return bridge.End{}, &bridge.Failure{Status: http.StatusServiceUnavailable, Type: "api_error", Message: "Inference cleanup failed. Restart the gateway.", Category: "cleanup_failed"}
			})
			h := NewObservedExperimentalHandler(strings.TrimPrefix(dummyBearer, "Bearer "), "test", slog.New(slog.NewTextHandler(&logs, nil)), generator, observer)
			body := strings.Replace(messageBody, `"max_tokens":1`, `"max_tokens":1,"stream":true`, 1)
			r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body)).WithContext(ctx)
			r.Header.Set("Authorization", dummyBearer)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Anthropic-Version", "2023-06-01")
			w := httptest.NewRecorder()
			func() {
				defer func() {
					if p := recover(); p != nil && (mode != "client write" || p != http.ErrAbortHandler) {
						panic(p)
					}
				}()
				if mode == "client write" {
					h.ServeHTTP(&failedCleanupWriter{w}, r)
				} else {
					h.ServeHTTP(w, r)
				}
			}()
			found := false
			for _, o := range observer.values {
				if o.Phase == "cleanup" {
					found = true
					if o.Cleanup || o.Category != "cleanup_failed" {
						t.Errorf("Messages(%s).cleanup = %+v, want cleanup_failed with Cleanup=false", mode, o)
					}
				}
			}
			if !found || !strings.Contains(logs.String(), "error_category=cleanup_failed") {
				t.Errorf("Messages(%s) cleanup_observed=%t logs=%q, want recorded cleanup_failed", mode, found, logs.String())
			}
			if mode == "disconnect" && w.Body.Len() != 0 {
				t.Errorf("Messages(disconnected cleanup failure).bytes = %d, want 0", w.Body.Len())
			}
			if mode == "stream" && (w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "event: error\n") || strings.Contains(w.Body.String(), "event: message_stop")) {
				t.Errorf("Messages(stream cleanup failure) status=%d error=%t terminal=%t, want 200, an error and no terminal", w.Code, strings.Contains(w.Body.String(), "event: error\n"), strings.Contains(w.Body.String(), "event: message_stop"))
			}
			if mode == "before output" && (w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Inference cleanup failed. Restart the gateway.")) {
				t.Errorf("Messages(cleanup failure before output) status=%d, want 503 with the fixed recovery message", w.Code)
			}
		})
	}
}

func messagesRequest(t *testing.T, h http.Handler, path, body string, edit func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.Header.Set("Authorization", dummyBearer)
	r.Header.Set("Anthropic-Version", "2023-06-01")
	r.Header.Set("Content-Type", "application/json")
	if edit != nil {
		edit(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func messagesHandlerForTest(t *testing.T, g bridge.Generator) http.Handler {
	t.Helper()
	return NewExperimentalHandler(strings.TrimPrefix(dummyBearer, "Bearer "), "test", slog.New(slog.NewTextHandler(io.Discard, nil)), g)
}

func TestMessagesValidateBeforeGeneration(t *testing.T) {
	var calls atomic.Int64
	h := messagesHandlerForTest(t, generateFunc(func(context.Context, bridge.Request, func(bridge.Event) error) (bridge.End, error) {
		calls.Add(1)
		return bridge.End{}, errors.New("secret")
	}))
	for _, tc := range []struct {
		name, body string
		status     int
		edit       func(*http.Request)
	}{
		{"auth first", `invalid`, http.StatusUnauthorized, func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer wrong")
			r.Header.Set("Anthropic-Beta", "unknown")
		}},
		{"ambiguous key", messageBody, http.StatusBadRequest, func(r *http.Request) { r.Header.Set("X-Api-Key", "synthetic") }},
		{"version", messageBody, http.StatusBadRequest, func(r *http.Request) { r.Header.Del("Anthropic-Version") }},
		{"second beta header", messageBody, http.StatusBadRequest, func(r *http.Request) {
			r.Header.Add("Anthropic-Beta", "claude-code-20250219")
			r.Header.Add("Anthropic-Beta", "unknown")
		}},
		{"body", `{}`, http.StatusBadRequest, nil},
		{"size", strings.Repeat(" ", bridge.MaxBody+1), http.StatusRequestEntityTooLarge, nil},
		{"leading system role", `{"model":"claude-opus-5.5","max_tokens":4096,"messages":[{"role":"system","content":"invented context"},{"role":"user","content":"invented"}]}`, http.StatusBadRequest, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := messagesRequest(t, h, "/v1/messages", tc.body, tc.edit)
			if w.Code != tc.status {
				t.Errorf("%s status=%d, want %d", tc.name, w.Code, tc.status)
			}
			if tc.status != http.StatusUnauthorized && w.Header().Get("X-Kiro-Gateway-Effort") != "ignored" {
				t.Error("authenticated error omitted policy headers")
			}
		})
	}
	if calls.Load() != 0 {
		t.Errorf("invalid requests generated %d times, want 0", calls.Load())
	}
}

func TestBetaContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		valid  bool
	}{
		{"absent", nil, true}, {"empty", []string{" \t"}, true},
		{"subset", []string{"claude-code-20250219, effort-2025-11-24", "interleaved-thinking-2025-05-14, mid-conversation-system-2026-04-07"}, true},
		{"duplicate", []string{"effort-2025-11-24", "effort-2025-11-24"}, false},
		{"empty element", []string{"effort-2025-11-24,"}, false},
		{"double empty", []string{"", ""}, false},
		{"case", []string{"Effort-2025-11-24"}, false},
		{"parameter", []string{"effort-2025-11-24;x=1"}, false},
		{"byte limit", []string{strings.Repeat(" ", 513)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateBetas(tc.values); (err == nil) != tc.valid {
				t.Errorf("validateBetas(%s) = %v, want valid=%t", tc.name, err, tc.valid)
			}
		})
	}
}

func TestMessagesResponseModesAndFailure(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			h := messagesHandlerForTest(t, generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
				for _, text := range []string{"hello", " world"} {
					if err := emit(bridge.Event{Text: text}); err != nil {
						return bridge.End{}, err
					}
				}
				if fail {
					return bridge.End{}, errors.New("SECRET_SERVICE_CONTENT")
				}
				return bridge.End{Basis: bridge.InferredCleanEOF}, nil
			}))
			body := messageBody
			if stream {
				body = strings.Replace(body, `"max_tokens":1`, `"max_tokens":1,"stream":true`, 1)
			}
			w := messagesRequest(t, h, "/v1/messages", body, nil)
			s := w.Body.String()
			if strings.Contains(s, "SECRET_SERVICE_CONTENT") {
				t.Error("raw error leaked")
			}
			if stream {
				if strings.Contains(s, "event: message_stop") == fail || strings.Contains(s, "event: error") != fail {
					t.Errorf("SSE stream=%t fail=%t has wrong terminal events", stream, fail)
				}
			} else {
				want := http.StatusOK
				if fail {
					want = http.StatusBadGateway
				}
				if w.Code != want {
					t.Errorf("JSON fail=%t status=%d, want %d", fail, w.Code, want)
				}
				if !fail {
					var m struct {
						Content []struct{ Text string }
						Usage   map[string]int
					}
					if json.Unmarshal(w.Body.Bytes(), &m) != nil || len(m.Content) != 1 || m.Content[0].Text != "hello world" || m.Usage["output_tokens"] == 0 {
						t.Error("JSON response lost content or estimates")
					}
				}
			}
		}
	}
}

func TestBusyCountsAndCancellation(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan struct{})
	var calls atomic.Int64
	h := messagesHandlerForTest(t, generateFunc(func(ctx context.Context, r bridge.Request, e func(bridge.Event) error) (bridge.End, error) {
		calls.Add(1)
		close(started)
		<-ctx.Done()
		return bridge.End{}, ctx.Err()
	}))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		defer close(finished)
		messagesRequest(t, h, "/v1/messages", messageBody, func(r *http.Request) { *r = *r.WithContext(ctx) })
	}()
	<-started
	if w := messagesRequest(t, h, "/v1/messages", messageBody, nil); w.Code != bridge.StatusOverloaded {
		t.Errorf("overlap status=%d, want 529", w.Code)
	}
	if w := messagesRequest(t, h, "/v1/messages/count_tokens", messageBody, nil); w.Code != http.StatusOK {
		t.Errorf("counts while busy status=%d, want 200", w.Code)
	}
	if calls.Load() != 1 {
		t.Errorf("Generate calls=%d, want 1", calls.Load())
	}
	cancel()
	<-finished
}

func TestTerminalWritesRespectInferenceDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	s := streamWriter{w: w, ctx: ctx}
	if err := s.send("message_stop", map[string]string{"type": "message_stop"}); !errors.Is(err, context.Canceled) || w.Body.Len() != 0 {
		t.Errorf("send(terminal after cancel) err=%v bytes=%d, want canceled with no terminal", err, w.Body.Len())
	}
	if err := writeJSONWithin(ctx, w, http.StatusOK, map[string]string{"type": "message"}); !errors.Is(err, context.Canceled) || w.Body.Len() != 0 {
		t.Errorf("writeJSONWithin(canceled) err=%v bytes=%d, want canceled without success", err, w.Body.Len())
	}
	deadline := time.Now().Add(time.Second)
	short, stop := context.WithDeadline(context.Background(), deadline)
	defer stop()
	if got := responseDeadline(short); !got.Equal(deadline) {
		t.Errorf("responseDeadline(short)=%v, want request deadline=%v", got, deadline)
	}
}
