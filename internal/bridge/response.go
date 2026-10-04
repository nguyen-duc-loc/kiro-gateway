package bridge

import "strings"

// Response accumulates bounded normalized output for both response modes.
type Response struct {
	Content []Block
	request Request
	text    strings.Builder
	tools   int
	seen    map[string]bool
}

// NewResponse starts an empty response using the offered tools and historical IDs.
func NewResponse(r Request) *Response {
	s := &Response{request: r, Content: []Block{}, seen: map[string]bool{}}
	for _, m := range r.Messages {
		for _, b := range m.Content {
			if b.Type == "tool_use" {
				s.seen[b.ID] = true
			}
		}
	}
	return s
}

// Add validates a normalized event before it can reach the client.
func (s *Response) Add(e Event) error {
	if e.Tool == nil {
		if s.tools > 0 || e.Text == "" || len(e.Text) > (2<<20)-s.text.Len() {
			return ProtocolFailure()
		}
		if len(s.Content) == 0 {
			s.Content = append(s.Content, Block{Type: "text"})
		}
		s.text.WriteString(e.Text)
		return nil
	}
	b := *e.Tool
	if e.Text != "" || b.Type != "tool_use" || !ValidID(b.ID) || !ValidName(b.Name) || b.Input == nil || s.seen[b.ID] || s.tools >= 16 || (s.request.DisableParallel && s.tools > 0) || len(Canonical(b.Input)) > 256<<10 {
		return ProtocolFailure()
	}
	found := false
	for _, t := range s.request.Tools {
		if t.Name == b.Name {
			found = true
		}
	}
	if !found {
		return ProtocolFailure()
	}
	s.seen[b.ID] = true
	s.tools++
	s.Content = append(s.Content, b)
	return nil
}

// Complete accepts only the named inferred terminal state with semantic content.
func (s *Response) Complete(end End) error {
	if end.Basis != InferredCleanEOF || len(s.Content) == 0 {
		return ProtocolFailure()
	}
	if s.text.Len() > 0 {
		s.Content[0].Text = s.text.String()
	}
	return nil
}

// StopReason reports the inferred client handoff after Complete succeeds.
func (s *Response) StopReason() string {
	if s.tools > 0 {
		return "tool_use"
	}
	return "end_turn"
}

// Message returns the client envelope; final selects terminal fields and content.
func (s *Response) Message(id string, final bool) map[string]any {
	var stop any
	content := []Block{}
	output := 0
	if final {
		stop = s.StopReason()
		content = s.Content
		output = OutputTokens(content)
	}
	return map[string]any{"id": id, "type": "message", "role": "assistant", "content": content, "model": s.request.Model, "stop_reason": stop, "stop_sequence": nil, "usage": map[string]int{"input_tokens": InputTokens(s.request), "output_tokens": output}}
}
