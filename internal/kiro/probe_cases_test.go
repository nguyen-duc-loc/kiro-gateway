package kiro

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"kiro-gateway/internal/jsonobject"
)

// These values are invented local fixture semantics, never proposed Kiro fields.
const (
	fixtureCompletionEvent = "probeFixtureComplete"
	fixtureInstructions    = "Keep the marker PROBE_MARKER in the synthetic conversation."
	fixtureMarker          = "PROBE_MARKER"
	fixtureTool            = "probe_lookup"
	fixtureToolResult      = "probe-value-alpha"
	fixtureFollowup        = "probe-followup"
	fixtureCutoff          = 256
)

var (
	errInjectedCancel = errors.New("canceled")
	errInjectedCutoff = errors.New("stream_incomplete")
	errContradicted   = errors.New("contract_mismatch")
)

var fixtureCaseIDs = [6]string{"text", "tool", "result", "followup", "cancel", "interrupt"}
var fixturePrompts = [6]string{
	"Return PROBE_MARKER in two text events.",
	"Request probe_lookup for key alpha.",
	"Continue using the fixed tool result.",
	"Return probe-followup using the prior exchange.",
	"Stream synthetic text until cancellation.",
	"Stream synthetic text for the fixed byte cutoff.",
}

type probeCaseAssertions struct {
	Incremental          *bool `json:"incremental"`
	InstructionPlacement *bool `json:"instruction_placement"`
	MarkerMatch          *bool `json:"marker_match"`
	ValidArguments       *bool `json:"valid_arguments"`
	ToolNameMatch        *bool `json:"matching_tool_name"`
	ToolIDMatch          *bool `json:"matching_tool_id"`
	ModelMatch           *bool `json:"matching_model_identity"`
	UsagePresent         *bool `json:"usage_present"`
	Completion           *bool `json:"observed_completion"`
	InjectionReached     *bool `json:"reached_injection_trigger"`
	CleanupCompleted     *bool `json:"completed_cleanup"`
}

type probeCaseResult struct {
	ID            string              `json:"case"`
	Status        string              `json:"status"`
	Cause         string              `json:"cause,omitempty"`
	Attempt       int                 `json:"attempt"`
	ReceivedBytes int64               `json:"received_bytes"`
	UnknownFields int                 `json:"unknown_field_count"`
	Assertions    probeCaseAssertions `json:"assertions"`
}

// This value is returned only by the fixture runner, not emitted as live evidence.
// The verdict function is shared logic whose all observed path can be exercised
// without assuming a real upstream completion or instruction contract.
type probeFixtureResult struct {
	Cases             [6]probeCaseResult `json:"cases"`
	Verdict           string             `json:"verdict"`
	Attempts          int                `json:"attempts"`
	PeakReservedBytes int64              `json:"peak_reserved_bytes"`
}

func probeVerdict(cases [6]probeCaseResult) string {
	all := true
	for _, c := range cases {
		if c.Status == "contradicted" {
			return "candidate_rejected"
		}
		if c.Status != "observed" {
			all = false
		}
	}
	if all {
		return "candidate_supported"
	}
	return "needs_evidence"
}

func probeBool(value bool) *bool { return &value }

type fixtureTurn struct {
	contradicted                      bool
	text, toolID, toolName, arguments string
	toolStopped, complete             bool
	unknownFields                     int
	assertions                        probeCaseAssertions
	memory                            *probeMemory
}

// Strings transferred from decoder scratch into turn state retain their own
// reservations. Raw payloads and decoded maps never survive the frame callback.
func (s *fixtureTurn) release() {
	s.memory.release(int64(len(s.text) + len(s.toolID) + len(s.toolName) + len(s.arguments)))
	s.text, s.toolID, s.toolName, s.arguments = "", "", "", ""
}

func (s *fixtureTurn) observe(event string, payload []byte) error {
	if s.complete {
		return errContradicted
	}
	o, err := jsonobject.Parse(payload, probeMaxEvent)
	if err != nil {
		return errContract
	}
	allowed := func(name string) bool {
		switch event {
		case "assistantResponseEvent":
			return name == "content"
		case "toolUseEvent":
			return name == "toolUseId" || name == "name" || name == "input" || name == "stop"
		case fixtureCompletionEvent:
			return name == "complete" || name == "model" || name == "usage"
		}
		return false
	}
	for name := range o {
		if !allowed(name) {
			s.unknownFields++
		}
	}
	if s.unknownFields != 0 {
		return errNeedsEvidence
	}
	readString := func(name string) (string, error) {
		v, exists := o[name]
		if !exists {
			return "", nil
		}
		var decoded *string
		if json.Unmarshal(v, &decoded) != nil || decoded == nil {
			return "", errContradicted
		}
		return *decoded, nil
	}
	switch event {
	case "assistantResponseEvent":
		content, err := readString("content")
		if err != nil {
			return err
		}
		if _, exists := o["content"]; !exists {
			return errNeedsEvidence
		}
		return s.memory.appendString(&s.text, content)
	case "toolUseEvent":
		if s.toolStopped {
			return errContradicted
		}
		id, err := readString("toolUseId")
		if err != nil {
			return err
		}
		name, err := readString("name")
		if err != nil {
			return err
		}
		input, err := readString("input")
		if err != nil {
			return err
		}
		if s.toolID == "" {
			if id == "" || name == "" {
				return errNeedsEvidence
			}
			s.assertions.ToolNameMatch = probeBool(name == fixtureTool)
			if name != fixtureTool {
				return errContradicted
			}
			if err := s.memory.appendString(&s.toolID, id); err != nil {
				return err
			}
			if err := s.memory.appendString(&s.toolName, name); err != nil {
				return err
			}
		} else if id != "" && id != s.toolID {
			s.assertions.ToolIDMatch = probeBool(false)
			return errContradicted
		} else if name != "" && name != s.toolName {
			s.assertions.ToolNameMatch = probeBool(false)
			return errContradicted
		}
		s.assertions.ToolIDMatch = probeBool(true)
		if err := s.memory.appendString(&s.arguments, input); err != nil {
			return err
		}
		if v, exists := o["stop"]; exists {
			var stopped *bool
			if json.Unmarshal(v, &stopped) != nil || stopped == nil {
				return errContradicted
			}
			s.toolStopped = *stopped
		}
		if s.toolStopped {
			result, err := fixtureLookup(s.arguments, s.memory)
			if err == nil {
				s.assertions.ValidArguments = probeBool(result == fixtureToolResult)
			} else if errors.Is(err, errContradicted) {
				s.assertions.ValidArguments = probeBool(false)
			}
			return err
		}
	case fixtureCompletionEvent:
		var complete *bool
		if _, exists := o["complete"]; !exists {
			return errNeedsEvidence
		}
		if json.Unmarshal(o["complete"], &complete) != nil || complete == nil || !*complete {
			return errContradicted
		}
		if s.toolID != "" && !s.toolStopped {
			return errNeedsEvidence
		}
		if v, exists := o["model"]; exists {
			var model *string
			if json.Unmarshal(v, &model) != nil || model == nil {
				return errContradicted
			}
			s.assertions.ModelMatch = probeBool(*model == probeModel)
			if *model != probeModel {
				return errContradicted
			}
		}
		if _, exists := o["usage"]; exists {
			s.assertions.UsagePresent = probeBool(true)
		}
		s.complete = true
		s.assertions.Completion = probeBool(true)
	}
	return nil
}

// This is a fixed lookup, never execution of a model supplied command.
func fixtureLookup(arguments string, memory *probeMemory) (string, error) {
	if len(arguments) > probeMaxEvent {
		return "", errBudget
	}
	scratch := int64(len(arguments))*64 + 4096
	if err := memory.reserve(scratch); err != nil {
		return "", err
	}
	defer memory.release(scratch)
	o, err := jsonobject.Parse([]byte(arguments), probeMaxEvent)
	if err != nil || len(o) != 1 {
		return "", errContradicted
	}
	var key string
	if json.Unmarshal(o["key"], &key) != nil || key != "alpha" {
		return "", errContradicted
	}
	return fixtureToolResult, nil
}

// Only the local fixture envelope uses these roles and field names. They do not
// assert that the Kiro request has an instructions field or accepts these roles.
type fixtureMessage struct {
	Role      string `json:"role"`
	Text      string `json:"text,omitempty"`
	ToolID    string `json:"tool_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Result    string `json:"result,omitempty"`
	IsError   bool   `json:"is_error"`
}

func (m fixtureMessage) bytes() int64 {
	return int64(len(m.Role) + len(m.Text) + len(m.ToolID) + len(m.ToolName) + len(m.Arguments) + len(m.Result))
}

type fixtureRequest struct {
	Case           string                  `json:"case"`
	Model          string                  `json:"model"`
	ConversationID string                  `json:"conversation_id"`
	Instructions   string                  `json:"instructions"`
	Tools          []fixtureToolDefinition `json:"tools,omitempty"`
	History        []fixtureMessage        `json:"history"`
	Current        fixtureMessage          `json:"current"`
}

type fixtureToolDefinition struct {
	Name        string          `json:"name"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func fixtureBody(index int, history []fixtureMessage, toolID string) (string, fixtureMessage, error) {
	current := fixtureMessage{Role: "user", Text: fixturePrompts[index]}
	if index == 2 {
		if toolID == "" {
			return "", current, errNeedsEvidence
		}
		current.ToolID, current.Result = toolID, fixtureToolResult
	}
	req := fixtureRequest{Case: fixtureCaseIDs[index], Model: probeModel, ConversationID: "fixture-conversation", Instructions: fixtureInstructions, History: history, Current: current}
	if index >= 1 && index <= 3 {
		req.Tools = []fixtureToolDefinition{{Name: fixtureTool, InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string","enum":["alpha"]}},"required":["key"],"additionalProperties":false}`)}}
	}
	// JSON escaping expands a byte by at most six. Reject before marshaling when
	// its conservative bound exceeds the fixed request allocation reservation.
	bound := int64(4096) + current.bytes()*6
	for _, m := range history {
		bound += 256 + m.bytes()*6
	}
	if bound > probeMaxRequest {
		return "", current, errBudget
	}
	b, err := json.Marshal(req)
	if err != nil || len(b) > probeMaxRequest {
		return "", current, errBudget
	}
	return string(b), current, nil
}

// A cutoff is an injection only if its fixed byte boundary was actually reached.
// A naturally short stream cannot pass this case merely by being incomplete.
type fixtureCutoffReader struct {
	reader    io.Reader
	remaining int64
	reached   bool
}

func (r *fixtureCutoffReader) Read(b []byte) (int, error) {
	if r.remaining == 0 {
		r.reached = true
		return 0, io.ErrUnexpectedEOF
	}
	if int64(len(b)) > r.remaining {
		b = b[:r.remaining]
	}
	n, err := r.reader.Read(b)
	r.remaining -= int64(n)
	return n, err
}

func runFixtureCases(p *offlineProbe, memoryLimit int64) (out probeFixtureResult) {
	memory := &probeMemory{limit: memoryLimit}
	for i, id := range fixtureCaseIDs {
		out.Cases[i] = probeCaseResult{ID: id, Status: "unrun"}
	}
	defer func() {
		out.Attempts = p.attempts
		out.PeakReservedBytes = memory.peak
		out.Verdict = probeVerdict(out.Cases)
		if out.Verdict == "candidate_supported" && p.ctx.Err() != nil {
			out.Cases[5].Status = "inconclusive"
			out.Cases[5].Cause = probeContextError(p.ctx, p.ctx.Err()).Error()
			out.Verdict = "needs_evidence"
		}
		// One invocation owns one sequence, including after success. No restart.
		p.stopped = true
	}()
	if memoryLimit <= 0 || memoryLimit > probeMaxRetained {
		out.Cases[0].Status, out.Cases[0].Cause = "inconclusive", "plan_invalid"
		return out
	}
	// Fixed allowance for result records, turn objects, history descriptors,
	// fixture literals, and both bounded account records, including source byte
	// copies and decoded strings. Payload work reserves separately. The source
	// allowance is conservative and retained throughout all six attempts.
	if err := memory.reserve((256 << 10) + probeSourceAllowance); err != nil {
		out.Cases[0].Status, out.Cases[0].Cause = "inconclusive", err.Error()
		return out
	}
	history := make([]fixtureMessage, 0, 8)
	toolID := ""
	for i := range fixtureCaseIDs {
		c := &out.Cases[i]
		c.Status = "inconclusive"
		if p.ctx.Err() != nil {
			c.Cause = probeContextError(p.ctx, p.ctx.Err()).Error()
			return out
		}
		// Covers the encoded body, its []byte to string copy, and encoder scratch.
		if err := memory.reserve(4 * probeMaxRequest); err != nil {
			c.Cause = err.Error()
			return out
		}
		body, current, err := fixtureBody(i, history, toolID)
		if err != nil {
			memory.release(4 * probeMaxRequest)
			c.Cause = err.Error()
			return out
		}
		turn := &fixtureTurn{memory: memory}
		reached := false
		beforeAttempt := p.attempts
		result, err := p.exchange(body, func(ctx context.Context, cancel context.CancelFunc, input io.Reader, observation *probeObservation) error {
			var cutoff *fixtureCutoffReader
			if i == 5 {
				cutoff = &fixtureCutoffReader{reader: input, remaining: fixtureCutoff}
				input = cutoff
			}
			decodeErr := walkProbeFrames(input, observation, memory, func(event string, payload []byte) error {
				if err := turn.observe(event, payload); err != nil {
					turn.contradicted = errors.Is(err, errContradicted)
					return err
				}
				if i == 4 && observation.TextEvents == 1 {
					reached = true
					cancel()
					return errInjectedCancel
				}
				return nil
			})
			if cutoff != nil {
				reached = cutoff.reached
				if reached && errors.Is(decodeErr, errIncomplete) && !turn.complete {
					return errInjectedCutoff
				}
			}
			if errors.Is(decodeErr, io.EOF) && turn.complete {
				return nil
			}
			return probeReadError(decodeErr)
		})
		memory.release(4 * probeMaxRequest)
		body = ""
		c.ReceivedBytes, c.UnknownFields = result.ReceivedBytes, turn.unknownFields
		if p.attempts > beforeAttempt {
			c.Attempt = p.attempts
		}
		c.Assertions = turn.assertions
		c.Assertions.CleanupCompleted = result.CleanupCompleted
		if i >= 4 {
			c.Assertions.InjectionReached = probeBool(reached)
		}
		if err == nil && (result.CleanupCompleted == nil || !*result.CleanupCompleted) {
			err = errTimedOut
		}
		if i == 4 && errors.Is(err, errInjectedCancel) && reached && p.ctx.Err() == nil {
			err = nil
		}
		if i == 5 && errors.Is(err, errInjectedCutoff) && reached {
			err = nil
		}
		if i >= 4 && !reached && err == nil {
			err = errNeedsEvidence
		}
		if (err == nil || turn.complete) && i < 4 {
			c.Assertions.InstructionPlacement = probeBool(true) // Named field in this fixture only.
			switch i {
			case 0:
				c.Assertions.Incremental = probeBool(result.TextEvents >= 2)
				c.Assertions.MarkerMatch = probeBool(turn.text == fixtureMarker)
				if result.TextEvents < 2 {
					err = errNeedsEvidence
				} else if turn.text != fixtureMarker {
					err = errContradicted
				}
			case 1:
				if turn.toolID == "" || !turn.toolStopped {
					err = errNeedsEvidence
				} else {
					toolID = turn.toolID
				}
			case 2, 3:
				marker := fixtureToolResult
				if i == 3 {
					marker = fixtureFollowup
				}
				c.Assertions.MarkerMatch = probeBool(turn.text == marker)
				if turn.text == "" {
					err = errNeedsEvidence
				} else if turn.text != marker {
					err = errContradicted
				}
			}
		}
		// A decoded contradiction survives operator cancellation during cleanup.
		if turn.contradicted || isFalse(turn.assertions.ToolNameMatch) || isFalse(turn.assertions.ToolIDMatch) || isFalse(turn.assertions.ValidArguments) || isFalse(turn.assertions.ModelMatch) {
			err = errContradicted
		}
		if err != nil {
			c.Cause = err.Error()
			if errors.Is(err, errContradicted) {
				c.Status = "contradicted"
			}
			turn.release()
			return out
		}
		if p.ctx.Err() != nil {
			c.Cause = probeContextError(p.ctx, p.ctx.Err()).Error()
			turn.release()
			return out
		}
		if i < 4 {
			assistant := fixtureMessage{Role: "assistant", Text: turn.text, ToolID: turn.toolID, ToolName: turn.toolName, Arguments: turn.arguments}
			if err := memory.reserve(current.bytes() + assistant.bytes()); err != nil {
				c.Cause = err.Error()
				turn.release()
				return out
			}
			history = append(history, current, assistant)
		}
		turn.release()
		c.Status = "observed"
	}
	return out
}

func isFalse(value *bool) bool { return value != nil && !*value }
