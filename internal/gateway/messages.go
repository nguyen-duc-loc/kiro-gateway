package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"kiro-gateway/internal/bridge"
)

// CompatibilityNotice is printed only when the experimental server starts.
const CompatibilityNotice = "Experimental bridge: top level and history instructions become user context; system priority and instruction replacement are not guaranteed; completion is inferred; reasoning is discarded; usage is estimated; max_tokens is advisory; effort is ignored; beta features are unsupported; serving model identity is unverified. GA compatibility is not established."

// NewExperimentalHandler wires the authenticated local API to a synchronous
// generator. One slot covers source access, generation, writes, and cleanup.
func NewExperimentalHandler(token, version string, logger *slog.Logger, generator bridge.Generator) http.Handler {
	return NewObservedExperimentalHandler(token, version, logger, generator, nil)
}

// NewObservedExperimentalHandler adds the synchronous development run boundary.
// The caller supplies its run context as the HTTP server's base context.
func NewObservedExperimentalHandler(token, version string, logger *slog.Logger, generator bridge.Generator, observer bridge.Observer) http.Handler {
	h := &messagesHandler{generator: generator, logger: logger, slot: make(chan struct{}, 1)}
	health := newHealthHandler(token, version, logger)
	expected := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" && r.URL.Path != "/v1/messages/count_tokens" {
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
			health.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		id, err := randomID("req_")
		if err != nil {
			writeAPIError(w, "", &bridge.Failure{Status: 500, Type: "api_error", Message: "Local randomness is unavailable.", Category: "randomness"})
			return
		}
		w.Header().Set("request-id", id)
		provided := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if len(r.Header.Values("Authorization")) != 1 || subtle.ConstantTimeCompare(expected[:], provided[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeAPIError(w, id, &bridge.Failure{Status: 401, Type: "authentication_error", Message: "Invalid gateway credential.", Category: "authentication"})
			return
		}
		for k, v := range map[string]string{"Compatibility": "experimental", "Usage": "estimated", "Completion": "inferred", "Controls": "advisory", "Effort": "ignored", "Betas": "unsupported", "Instructions": "user-context"} {
			w.Header().Set("X-Kiro-Gateway-"+k, v)
		}
		r = r.WithContext(bridge.WithObserver(r.Context(), id, observer))
		h.serve(w, r, id)
	})
}

type messagesHandler struct {
	generator bridge.Generator
	logger    *slog.Logger
	slot      chan struct{}
}

func (h *messagesHandler) serve(w http.ResponseWriter, r *http.Request, id string) {
	started := time.Now()
	category := "invalid_request"
	reasoningEvents := 0
	completionBasis := ""
	defer func() {
		attrs := []any{"event", "request", "request_id", id, "error_category", category, "elapsed", time.Since(started), "compatibility", "experimental", "usage_source", "estimated", "discarded_reasoning_events", reasoningEvents}
		if completionBasis != "" {
			attrs = append(attrs, "completion_basis", completionBasis)
		}
		h.logger.Info("Request completed", attrs...)
	}()
	fail := func(err error) {
		f := bridge.SafeFailure(err)
		category = f.Category
		bridge.Observe(r.Context(), "failure", category, true)
		writeAPIError(w, id, f)
	}
	if err := bridge.Before(r.Context(), "local"); err != nil {
		fail(err)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		fail(&bridge.Failure{Status: 405, Type: "invalid_request_error", Message: "Method not allowed.", Category: "method"})
		return
	}
	if err := validateHeaders(r); err != nil {
		fail(err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, bridge.MaxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			fail(&bridge.Failure{Status: 413, Type: "request_too_large", Message: "Request body is too large.", Category: "request_too_large"})
		} else {
			fail(bridge.Invalid())
		}
		return
	}
	count := r.URL.Path == "/v1/messages/count_tokens"
	request, err := bridge.Parse(body, count)
	if err != nil {
		fail(err)
		return
	}
	if count {
		if err := bridge.Before(r.Context(), "count"); err != nil {
			fail(err)
			return
		}
		category = "success"
		if writeJSON(w, 200, map[string]int{"input_tokens": bridge.InputTokens(request)}) != nil {
			category = "response_write"
			bridge.Observe(r.Context(), "failure", category, true)
		}
		return
	}
	if err := bridge.Before(r.Context(), "admission"); err != nil {
		fail(err)
		return
	}
	select {
	case h.slot <- struct{}{}:
		defer func() {
			<-h.slot
			bridge.Observe(r.Context(), "cleanup", category, category != "cleanup_failed")
		}()
	default:
		fail(&bridge.Failure{Status: 529, Type: "overloaded_error", Message: "An inference request is already active.", Category: "busy"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	messageID, err := randomID("msg_")
	if err != nil {
		fail(&bridge.Failure{Status: 500, Type: "api_error", Message: "Local randomness is unavailable.", Category: "randomness"})
		return
	}
	state := bridge.NewResponse(request)
	sse := streamWriter{w: w, state: state, id: messageID}
	end, err := h.generator.Generate(ctx, request, func(e bridge.Event) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := state.Add(e); err != nil {
			return err
		}
		if request.Stream {
			if err := sse.emit(e); err != nil {
				category = "response_write"
				bridge.Observe(ctx, "failure", category, false)
				cancel()
				return err
			}
		}
		return nil
	})
	if category == "response_write" {
		bridge.Observe(ctx, "cleanup", category, true)
		panic(http.ErrAbortHandler)
	}
	if ctx.Err() != nil {
		err = context.Cause(ctx)
	}
	if err == nil {
		err = state.Complete(end)
		if err == nil {
			reasoningEvents = end.ReasoningEvents
			completionBasis = end.Basis
		}
	}
	if err != nil {
		if r.Context().Err() != nil && errors.Is(context.Cause(r.Context()), context.Canceled) {
			category = "canceled"
			bridge.Observe(ctx, "failure", category, false)
			return
		}
		f := bridge.SafeFailure(err)
		category = f.Category
		bridge.Observe(ctx, "failure", category, false)
		if sse.started {
			if sse.send("error", errorBody(id, f)) != nil {
				category = "response_write"
				bridge.Observe(ctx, "failure", category, true)
				panic(http.ErrAbortHandler)
			}
			return
		}
		fail(err)
		return
	}
	category = "success"
	if request.Stream {
		err = sse.finish()
	} else {
		err = writeJSON(w, 200, state.Message(messageID, true))
	}
	if err != nil {
		category = "response_write"
		bridge.Observe(ctx, "failure", category, true)
		panic(http.ErrAbortHandler)
	}
	bridge.Observe(ctx, "terminal", "success", true)
}

func validateHeaders(r *http.Request) error {
	if len(r.Header.Values("X-Api-Key")) > 0 || len(r.Header.Values("Content-Encoding")) > 0 || len(r.Header.Values("Anthropic-Version")) != 1 || r.Header.Get("Anthropic-Version") != "2023-06-01" || len(r.Header.Values("Content-Type")) != 1 {
		return bridge.Invalid()
	}
	t, p, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || t != "application/json" || len(p) > 1 {
		return bridge.Invalid()
	}
	for k, v := range p {
		if k != "charset" || !strings.EqualFold(v, "utf-8") {
			return bridge.Invalid()
		}
	}
	if r.URL.RawQuery != "" && r.URL.RawQuery != "beta=true" {
		return bridge.Invalid()
	}
	return validateBetas(r.Header.Values("Anthropic-Beta"))
}

func validateBetas(values []string) error {
	fail := &bridge.Failure{Status: 400, Type: "invalid_request_error", Message: "Unsupported or invalid beta header.", Category: "beta_header"}
	n := max(0, len(values)-1)
	for _, v := range values {
		n += len(v)
	}
	if n > 512 {
		return fail
	}
	if len(values) == 0 || len(values) == 1 && strings.Trim(values[0], " \t") == "" {
		return nil
	}
	allowed := map[string]bool{"claude-code-20250219": true, "interleaved-thinking-2025-05-14": true, "mid-conversation-system-2026-04-07": true, "effort-2025-11-24": true}
	seen := map[string]bool{}
	for _, v := range values {
		for _, s := range strings.Split(v, ",") {
			s = strings.Trim(s, " \t")
			if !allowed[s] || seen[s] || len(seen) >= 4 {
				return fail
			}
			seen[s] = true
		}
	}
	return nil
}

func randomID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b[:]), nil
}

func errorBody(id string, f *bridge.Failure) map[string]any {
	b := map[string]any{"type": "error", "error": map[string]string{"type": f.Type, "message": f.Message}}
	if id != "" {
		b["request_id"] = id
	}
	return b
}

func writeAPIError(w http.ResponseWriter, id string, f *bridge.Failure) {
	if writeJSON(w, f.Status, errorBody(id, f)) != nil {
		panic(http.ErrAbortHandler)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(append(bridge.Canonical(v), '\n'))
	return err
}

type streamWriter struct {
	w                 http.ResponseWriter
	state             *bridge.Response
	id                string
	started, textOpen bool
	index             int
}

func (s *streamWriter) send(kind string, v any) error {
	c := http.NewResponseController(s.w)
	_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := s.w.Write(append(append([]byte("event: "+kind+"\ndata: "), bridge.Canonical(v)...), '\n', '\n')); err != nil {
		return err
	}
	return c.Flush()
}

func (s *streamWriter) emit(e bridge.Event) error {
	if !s.started {
		s.w.Header().Set("Content-Type", "text/event-stream")
		s.started = true
		if err := s.send("message_start", map[string]any{"type": "message_start", "message": s.state.Message(s.id, false)}); err != nil {
			return err
		}
	}
	if e.Tool == nil {
		if !s.textOpen {
			if err := s.send("content_block_start", map[string]any{"type": "content_block_start", "index": s.index, "content_block": map[string]string{"type": "text", "text": ""}}); err != nil {
				return err
			}
			s.textOpen = true
		}
		return s.send("content_block_delta", map[string]any{"type": "content_block_delta", "index": s.index, "delta": map[string]string{"type": "text_delta", "text": e.Text}})
	}
	if err := s.closeText(); err != nil {
		return err
	}
	b := e.Tool
	if err := s.send("content_block_start", map[string]any{"type": "content_block_start", "index": s.index, "content_block": map[string]any{"type": "tool_use", "id": b.ID, "name": b.Name, "input": map[string]any{}}}); err != nil {
		return err
	}
	if err := s.send("content_block_delta", map[string]any{"type": "content_block_delta", "index": s.index, "delta": map[string]string{"type": "input_json_delta", "partial_json": string(bridge.Canonical(b.Input))}}); err != nil {
		return err
	}
	if err := s.closeBlock(); err != nil {
		return err
	}
	s.index++
	return nil
}

func (s *streamWriter) closeBlock() error {
	return s.send("content_block_stop", map[string]any{"type": "content_block_stop", "index": s.index})
}
func (s *streamWriter) closeText() error {
	if !s.textOpen {
		return nil
	}
	if err := s.closeBlock(); err != nil {
		return err
	}
	s.textOpen = false
	s.index++
	return nil
}
func (s *streamWriter) finish() error {
	if err := s.closeText(); err != nil {
		return err
	}
	if err := s.send("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": s.state.StopReason(), "stop_sequence": nil}, "usage": map[string]int{"output_tokens": bridge.OutputTokens(s.state.Content)}}); err != nil {
		return err
	}
	return s.send("message_stop", map[string]string{"type": "message_stop"})
}
