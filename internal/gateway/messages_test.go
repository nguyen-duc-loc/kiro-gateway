package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"kiro-gateway/internal/bridge"
)

type generateFunc func(context.Context, bridge.Request, func(bridge.Event) error) (bridge.End, error)

func (f generateFunc) Generate(c context.Context, r bridge.Request, e func(bridge.Event) error) (bridge.End, error) {
	return f(c, r, e)
}

const messageBody = `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":"synthetic"}]}`
const dummyBearer = "Bearer synthetic-gateway-token-for-tests-0001"

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
		{"auth first", `invalid`, 401, func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer wrong")
			r.Header.Set("Anthropic-Beta", "unknown")
		}},
		{"ambiguous key", messageBody, 400, func(r *http.Request) { r.Header.Set("X-Api-Key", "synthetic") }},
		{"version", messageBody, 400, func(r *http.Request) { r.Header.Del("Anthropic-Version") }},
		{"second beta header", messageBody, 400, func(r *http.Request) {
			r.Header.Add("Anthropic-Beta", "claude-code-20250219")
			r.Header.Add("Anthropic-Beta", "unknown")
		}},
		{"body", `{}`, 400, nil},
		{"size", strings.Repeat(" ", bridge.MaxBody+1), 413, nil},
		{"leading system role", `{"model":"claude-opus-5.5","max_tokens":4096,"messages":[{"role":"system","content":"invented context"},{"role":"user","content":"invented"}]}`, 400, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := messagesRequest(t, h, "/v1/messages", tc.body, tc.edit)
			if w.Code != tc.status {
				t.Errorf("%s status=%d, want %d", tc.name, w.Code, tc.status)
			}
			if tc.status != 401 && w.Header().Get("X-Kiro-Gateway-Effort") != "ignored" {
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
				want := 200
				if fail {
					want = 502
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
	if w := messagesRequest(t, h, "/v1/messages", messageBody, nil); w.Code != 529 {
		t.Errorf("overlap status=%d, want 529", w.Code)
	}
	if w := messagesRequest(t, h, "/v1/messages/count_tokens", messageBody, nil); w.Code != 200 {
		t.Errorf("counts while busy status=%d, want 200", w.Code)
	}
	if calls.Load() != 1 {
		t.Errorf("Generate calls=%d, want 1", calls.Load())
	}
	cancel()
	<-finished
}
