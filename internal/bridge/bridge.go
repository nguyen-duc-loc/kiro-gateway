// Package bridge owns the experimental client protocol independently of transport.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// Model is the only model admitted by this experiment.
const Model = "claude-opus-5.5"

// StatusOverloaded is the client protocol's nonstandard HTTP status for busy inference.
const StatusOverloaded = 529

// MaxBody is the maximum client JSON size in bytes.
const MaxBody = 4 << 20

// Request is a validated, self contained conversation. Discarded compatibility
// inputs never enter this representation. Callers must not mutate it during use.
type Request struct {
	Model           string
	MaxTokens       int
	Stream          bool
	System          string
	Messages        []Message
	Tools           []Tool
	DisableParallel bool
}

// Message holds ordered normalized content.
type Message struct {
	Role    string  `json:"role"`
	Content []Block `json:"content"`
}

// Block represents text, a tool call, or a result. Use ContentValue for the
// canonical client representation, including required empty values.
type Block struct {
	Type      string
	Text      string
	ID        string
	Name      string
	Input     map[string]any
	ToolUseID string
	IsError   bool
	Content   []string
}

// Tool is a current client tool definition, with its full schema preserved.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Event carries a text delta or one complete tool call. Calls are emitted only
// after the adapter has validated the entire response and finished cleanup.
type Event struct {
	Text string
	Tool *Block
}

// End records the explicitly inferred completion basis and discarded reasoning.
type End struct {
	Basis           string
	ReasoningEvents int
}

// InferredCleanEOF names this experiment's limited completion guarantee.
const InferredCleanEOF = "inferred_clean_eof"

// Generator is synchronous. It returns after owned work stops, or reports
// cleanup_failed and disables further generation if cleanup exceeds its bound.
// Unfinished cleanup retains ownership of its resources and must never emit.
// emit is called serially and its errors must stop generation immediately.
type Generator interface {
	Generate(context.Context, Request, func(Event) error) (End, error)
}

// Failure carries only fixed, safe protocol and diagnostic values.
type Failure struct {
	Status                  int
	Type, Message, Category string
}

func (f *Failure) Error() string { return f.Message }

// Invalid is the fixed error for unsupported or malformed client input.
func Invalid() error {
	return &Failure{http.StatusBadRequest, "invalid_request_error", "Unsupported or invalid request.", "invalid_request"}
}

// ProtocolFailure is the fixed error for invalid or incomplete upstream output.
func ProtocolFailure() error {
	return &Failure{http.StatusBadGateway, "api_error", "Upstream response was invalid or incomplete.", "incomplete_stream"}
}

// SafeFailure maps unknown errors without exposing their contents.
func SafeFailure(err error) *Failure {
	var f *Failure
	if errors.As(err, &f) {
		return f
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Failure{http.StatusGatewayTimeout, "api_error", "Inference timed out.", "timed_out"}
	}
	return ProtocolFailure().(*Failure)
}

// Canonical encodes normalized content with stable keys and no HTML escaping.
func Canonical(v any) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if e.Encode(v) != nil {
		return nil
	}
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'})
}

// MarshalJSON preserves the required fields for each block variant.
func (b Block) MarshalJSON() ([]byte, error) { return Canonical(b.ContentValue()), nil }

// ContentValue returns the client shape of a normalized block.
func (b Block) ContentValue() map[string]any {
	switch b.Type {
	case "tool_use":
		return map[string]any{"type": b.Type, "id": b.ID, "name": b.Name, "input": b.Input}
	case "tool_result":
		texts := make([]map[string]any, 0, len(b.Content))
		for _, s := range b.Content {
			texts = append(texts, map[string]any{"type": "text", "text": s})
		}
		return map[string]any{"type": b.Type, "tool_use_id": b.ToolUseID, "is_error": b.IsError, "content": texts}
	default:
		return map[string]any{"type": "text", "text": b.Text}
	}
}

// InputTokens estimates normalized content only, excluding all controls and IDs.
func InputTokens(r Request) int {
	return estimate(Canonical(map[string]any{"system": r.System, "messages": r.Messages, "tools": r.Tools}))
}

// OutputTokens estimates the cumulative delivered client content.
func OutputTokens(content []Block) int {
	if len(content) == 0 {
		return 0
	}
	return estimate(Canonical(content))
}

func estimate(b []byte) int { return max(1, (len(b)+3)/4) }
