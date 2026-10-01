package kiro

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"

	"kiro-gateway/internal/credentials"
	"kiro-gateway/internal/jsonobject"
)

const (
	wireContentType       = "application/x-amz-json-1.0"
	wireTarget            = "KiroRuntimeService.GenerateAssistantResponse"
	wireInstructionPolicy = "translate_into_user_context"
	wireCompletionPolicy  = "clean_stream_end_tentative"
	wireInstructions      = "Synthetic experiment instructions: follow the current request exactly. Use only the offered probe_lookup tool when requested."
	wireToolDescription   = "Return the fixed value for key alpha."
	wireToolSchema        = `{"type":"object","properties":{"key":{"type":"string","enum":["alpha"]}},"required":["key"],"additionalProperties":false}`
	wireMaxTokens         = 1024
)

var wirePrompts = [6]string{
	"Reply with exactly PROBE_MARKER and no other text.",
	"Call probe_lookup with key alpha. Do not answer instead of calling the tool.",
	"Reply with exactly the value returned by probe_lookup and no other text.",
	"Using the prior conversation and tool exchange, reply with exactly probe-followup and no other text.",
	"Write a long sequence of numbered synthetic words, one word per line.",
	"Write a long sequence of numbered synthetic words, one word per line.",
}

// These types describe the bounded candidate wire path, not a production API.
// None implements String or appears in the allowed observation result.
type wireMessage struct {
	User      *wireUser      `json:"userInputMessage,omitempty"`
	Assistant *wireAssistant `json:"assistantResponseMessage,omitempty"`
}
type wireUser struct {
	Content string           `json:"content"`
	Model   string           `json:"modelId"`
	Origin  string           `json:"origin"`
	Context *wireUserContext `json:"userInputMessageContext,omitempty"`
}
type wireUserContext struct {
	Tools   []wireTool       `json:"tools,omitempty"`
	Results []wireToolResult `json:"toolResults,omitempty"`
}
type wireTool struct {
	Specification wireToolSpecification `json:"toolSpecification"`
}
type wireToolSpecification struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema struct {
		JSON json.RawMessage `json:"json"`
	} `json:"inputSchema"`
}
type wireToolResult struct {
	ID      string     `json:"toolUseId"`
	Status  string     `json:"status"`
	Content []wireText `json:"content"`
}
type wireText struct {
	Text string `json:"text"`
}
type wireAssistant struct {
	Content string        `json:"content"`
	Tools   []wireToolUse `json:"toolUses,omitempty"`
}
type wireToolUse struct {
	ID    string          `json:"toolUseId"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}
type wireRequest struct {
	Conversation struct {
		ID      string        `json:"conversationId"`
		Trigger string        `json:"chatTriggerType"`
		History []wireMessage `json:"history"`
		Current wireMessage   `json:"currentMessage"`
	} `json:"conversationState"`
	Profile  string `json:"profileArn"`
	Controls struct {
		MaxTokens int `json:"max_tokens"`
		Thinking  struct {
			Type string `json:"type"`
		} `json:"thinking"`
	} `json:"additionalModelRequestFields"`
}

func wireCurrent(index int, toolID string) wireMessage {
	u := &wireUser{Content: wireInstructions + "\n\n" + wirePrompts[index], Model: probeModel, Origin: "CLI"}
	if index >= 1 && index <= 3 {
		spec := wireToolSpecification{Name: fixtureTool, Description: wireToolDescription}
		spec.InputSchema.JSON = json.RawMessage(wireToolSchema)
		u.Context = &wireUserContext{Tools: []wireTool{{Specification: spec}}}
		if index == 2 {
			u.Context.Results = []wireToolResult{{ID: toolID, Status: "success", Content: []wireText{{Text: fixtureToolResult}}}}
		}
	}
	return wireMessage{User: u}
}

func wireMessageBytes(m wireMessage) int64 {
	// Fixed reservation covers struct and slice descriptors, tool definitions,
	// and literals. Dynamic strings are counted even when sharing an allocation.
	n := int64(512)
	if m.User != nil {
		n += int64(len(m.User.Content))
		if m.User.Context != nil {
			for _, r := range m.User.Context.Results {
				n += int64(len(r.ID))
			}
		}
	}
	if m.Assistant != nil {
		n += int64(len(m.Assistant.Content))
		for _, t := range m.Assistant.Tools {
			n += int64(len(t.ID) + len(t.Name) + len(t.Input))
		}
	}
	return n
}

func wireBody(index int, history []wireMessage, toolID, conversationID, profileARN string) (string, wireMessage, error) {
	if index < 0 || index >= len(wirePrompts) || conversationID == "" || profileARN == "" || index == 2 && toolID == "" {
		return "", wireMessage{}, errPlanInvalid
	}
	current := wireCurrent(index, toolID)
	bound := int64(4096+len(profileARN)*6+len(conversationID)*6) + wireMessageBytes(current)*6
	for _, m := range history {
		bound += wireMessageBytes(m) * 6
	}
	if bound > probeMaxRequest {
		return "", wireMessage{}, errBudget
	}
	var req wireRequest
	req.Conversation.ID, req.Conversation.Trigger = conversationID, "MANUAL"
	req.Conversation.History = history
	if history == nil {
		req.Conversation.History = []wireMessage{}
	}
	req.Conversation.Current = current
	req.Profile = profileARN
	req.Controls.MaxTokens = wireMaxTokens
	req.Controls.Thinking.Type = "disabled"
	b, err := json.Marshal(req)
	if err != nil || len(b) > probeMaxRequest {
		return "", wireMessage{}, errBudget
	}
	return string(b), current, nil
}

type wireAssertions struct {
	probeCaseAssertions
	TentativeCompletion *bool `json:"tentative_completion"`
	ControlsRequested   *bool `json:"controls_requested"`
	OutputWithinLimit   *bool `json:"output_within_limit"`
}
type wireCaseResult struct {
	ServiceError        string         `json:"service_error,omitempty"`
	ErrorResponseFormat string         `json:"error_response_format,omitempty"`
	FailureStage        string         `json:"failure_stage,omitempty"`
	TransportFailure    string         `json:"transport_failure,omitempty"`
	HTTPStatus          string         `json:"http_status_category,omitempty"`
	ID                  string         `json:"case"`
	Status              string         `json:"status"`
	Cause               string         `json:"cause,omitempty"`
	Attempt             int            `json:"attempt"`
	ReceivedBytes       int64          `json:"received_bytes"`
	TextEvents          int            `json:"text_events"`
	ToolEvents          int            `json:"tool_events"`
	UnknownEvents       int            `json:"unknown_event_count"`
	UnknownFields       int            `json:"unknown_field_count"`
	Assertions          wireAssertions `json:"assertions"`
}
type wireRunResult struct {
	Cases                       [6]wireCaseResult `json:"cases"`
	Verdict                     string            `json:"verdict"`
	Attempts                    int               `json:"attempts"`
	PeakReservedBytes           int64             `json:"peak_reserved_bytes"`
	InstructionTransformation   string            `json:"instruction_transformation"`
	CompletionPolicy            string            `json:"completion_policy"`
	DistinctSystemRolePreserved bool              `json:"distinct_system_role_preserved"`
}

// A finite label set is shared with plan validation. No response can extend it.
func wireEventFields() map[string][]string {
	return map[string][]string{
		"assistantResponseEvent": {"content", "modelId"},
		"toolUseEvent":           {"toolUseId", "name", "input", "stop"},
		"messageMetadataEvent":   {"conversationId", "utteranceId"},
		"metadataEvent":          {"tokenUsage"},
		"contextUsageEvent":      {"contextUsagePercentage"},
		"meteringEvent":          {"usage", "unit"},
	}
}
func wireUsageFields() []string {
	return []string{"inputTokens", "outputTokens", "totalTokens", "cacheReadInputTokens", "cacheWriteInputTokens", "uncachedInputTokens", "contextUsagePercentage", "normalizedTokenUsage"}
}
func wireKnown(name string, names []string) bool {
	for _, n := range names {
		if name == n {
			return true
		}
	}
	return false
}

type wireTurn struct {
	fixtureTurn
	textEvents        int
	outputWithinLimit *bool
}

func wireString(raw json.RawMessage) (string, error) {
	var s *string
	if json.Unmarshal(raw, &s) != nil || s == nil {
		return "", errContract
	}
	return *s, nil
}
func wireNumber(raw json.RawMessage) (float64, error) {
	var n *float64
	if json.Unmarshal(raw, &n) != nil || n == nil || math.IsNaN(*n) || math.IsInf(*n, 0) || *n < 0 {
		return 0, errContract
	}
	return *n, nil
}

func (s *wireTurn) observe(event string, payload []byte) error {
	fields, ok := wireEventFields()[event]
	if !ok {
		return errNeedsEvidence
	}
	o, err := jsonobject.Parse(payload, probeMaxEvent)
	if err != nil {
		return errContract
	}
	for name := range o {
		if !wireKnown(name, fields) {
			s.unknownFields++
		}
	}
	if s.unknownFields != 0 {
		return errNeedsEvidence
	}
	switch event {
	case "toolUseEvent":
		return s.fixtureTurn.observe(event, payload)
	case "assistantResponseEvent":
		if raw, ok := o["modelId"]; ok {
			model, err := wireString(raw)
			if err != nil {
				return err
			}
			s.assertions.ModelMatch = probeBool(model == probeModel)
			if model != probeModel {
				return errContradicted
			}
		}
		raw, exists := o["content"]
		if !exists {
			return errNeedsEvidence
		}
		content, err := wireString(raw)
		if err != nil {
			return err
		}
		if content != "" {
			s.textEvents++
		}
		return s.memory.appendString(&s.text, content)
	case "messageMetadataEvent":
		for _, raw := range o {
			if _, err := wireString(raw); err != nil {
				return err
			}
		}
	case "metadataEvent":
		raw, exists := o["tokenUsage"]
		if !exists {
			return nil
		}
		usage, err := jsonobject.Parse(raw, probeMaxEvent)
		if err != nil {
			return errContract
		}
		s.assertions.UsagePresent = probeBool(true)
		for name, value := range usage {
			if !wireKnown(name, wireUsageFields()) {
				s.unknownFields++
				return errNeedsEvidence
			}
			n, err := wireNumber(value)
			if err != nil {
				return err
			}
			if name == "outputTokens" {
				if n != math.Trunc(n) {
					return errContract
				}
				s.outputWithinLimit = probeBool(n <= wireMaxTokens)
				if n > wireMaxTokens {
					return errContradicted
				}
			}
		}
	case "contextUsageEvent":
		for _, raw := range o {
			if _, err := wireNumber(raw); err != nil {
				return err
			}
		}
	case "meteringEvent":
		if raw, ok := o["usage"]; ok {
			if _, err := wireNumber(raw); err != nil {
				return err
			}
		}
		if raw, ok := o["unit"]; ok {
			if _, err := wireString(raw); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *wireTurn) validate(index int) error {
	if index != 1 && s.toolID != "" {
		return errContradicted
	}
	if index == 1 {
		if s.toolID == "" || !s.toolStopped {
			return errNeedsEvidence
		}
		return nil
	}
	text := strings.TrimSpace(s.text)
	if text == "" {
		return errNeedsEvidence
	}
	want := fixtureMarker
	if index == 2 {
		want = fixtureToolResult
	}
	if index == 3 {
		want = fixtureFollowup
	}
	s.assertions.MarkerMatch = probeBool(text == want)
	if text != want {
		return errContradicted
	}
	if index == 0 {
		s.assertions.Incremental = probeBool(s.textEvents >= 2)
		if s.textEvents < 2 {
			return errNeedsEvidence
		}
	}
	return nil
}

// runWireCases owns exactly one sequence. Even six observed cases cannot prove
// a distinct system role or a successful upstream model turn.
func runWireCases(p *protocolProbe, conversationID string, memoryLimit int64) (out wireRunResult) {
	out.InstructionTransformation = "instructions_in_user_content"
	out.CompletionPolicy = wireCompletionPolicy
	memory := &probeMemory{limit: memoryLimit}
	for i, id := range fixtureCaseIDs {
		out.Cases[i] = wireCaseResult{ID: id, Status: "unrun"}
	}
	defer func() {
		p.stopped = true
		out.Attempts, out.PeakReservedBytes = p.attempts, memory.peak
		out.Verdict = "limited_candidate_observed"
		for _, c := range out.Cases {
			if c.Status == "contradicted" {
				out.Verdict = "candidate_rejected"
				return
			}
			if c.Status != "observed" {
				out.Verdict = "needs_evidence"
			}
		}
		if p.ctx.Err() != nil {
			out.Verdict = "needs_evidence"
		}
	}()
	if !p.wire || memoryLimit <= 0 || memoryLimit > probeMaxRetained || len(conversationID) > 64 || conversationID == "" {
		out.Cases[0].Status, out.Cases[0].Cause = "inconclusive", "plan_invalid"
		return out
	}
	if err := memory.reserve((256 << 10) + probeSourceAllowance + probeErrorScratch); err != nil {
		out.Cases[0].Status, out.Cases[0].Cause = "inconclusive", err.Error()
		return out
	}
	history := make([]wireMessage, 0, 8)
	toolID := ""
	allowed := wireEventFields()
	for i := range fixtureCaseIDs {
		c := &out.Cases[i]
		c.Status = "inconclusive"
		if p.ctx.Err() != nil {
			c.Cause = probeContextError(p.ctx, p.ctx.Err()).Error()
			return out
		}
		if err := memory.reserve(4 * probeMaxRequest); err != nil {
			c.Cause = err.Error()
			return out
		}
		turn := &wireTurn{fixtureTurn: fixtureTurn{memory: memory}}
		var current wireMessage
		before := p.attempts
		reached, tentative := false, false
		result, err := p.exchangeBuilt(func(snapshot credentials.ProfileSnapshot) (string, error) {
			selectedHistory := history
			if i >= 4 {
				selectedHistory = nil
			}
			body, message, err := wireBody(i, selectedHistory, toolID, conversationID, snapshot.ProfileARN())
			current = message
			return body, err
		}, func(ctx context.Context, cancel context.CancelFunc, input io.Reader, obs *probeObservation) error {
			var cutoff *fixtureCutoffReader
			if i == 5 {
				cutoff = &fixtureCutoffReader{reader: input, remaining: fixtureCutoff}
				input = cutoff
			}
			decodeErr := walkFramedEvents(input, obs, memory, func(event string, payload []byte) error {
				err := turn.observe(event, payload)
				if i != 1 && turn.toolID != "" {
					err = errContradicted
				}
				turn.contradicted = turn.contradicted || errors.Is(err, errContradicted)
				if err != nil {
					return err
				}
				if i == 4 && turn.textEvents == 1 {
					reached = true
					cancel()
					return errInjectedCancel
				}
				return nil
			}, func(event string) bool { _, ok := allowed[event]; return ok })
			if cutoff != nil {
				reached = cutoff.reached
				if reached && errors.Is(decodeErr, errIncomplete) {
					return errInjectedCutoff
				}
			}
			if errors.Is(decodeErr, io.EOF) && i < 4 && ctx.Err() == nil {
				err := turn.validate(i)
				turn.contradicted = turn.contradicted || errors.Is(err, errContradicted)
				if err != nil {
					return err
				}
				tentative = true
				return nil
			}
			return probeReadError(decodeErr)
		})
		memory.release(4 * probeMaxRequest)
		c.ServiceError, c.ErrorResponseFormat = result.ServiceError, result.ErrorResponseFormat
		c.FailureStage, c.TransportFailure, c.HTTPStatus = result.FailureStage, result.TransportFailure, result.HTTPStatus
		if c.FailureStage == "" {
			c.FailureStage = "stream"
		}
		c.ReceivedBytes, c.UnknownFields, c.UnknownEvents = result.ReceivedBytes, turn.unknownFields, result.UnknownEvents
		c.TextEvents, c.ToolEvents = turn.textEvents, result.ToolEvents
		if p.attempts > before {
			c.Attempt = p.attempts
			c.Assertions.ControlsRequested = probeBool(true)
			c.Assertions.InstructionPlacement = probeBool(true)
		}
		c.Assertions.probeCaseAssertions = turn.assertions
		if c.Attempt != 0 {
			c.Assertions.InstructionPlacement = probeBool(true)
		}
		c.Assertions.OutputWithinLimit = turn.outputWithinLimit
		c.Assertions.CleanupCompleted = result.CleanupCompleted
		if err == nil && (result.CleanupCompleted == nil || !*result.CleanupCompleted) {
			err = errTimedOut
		}
		if i >= 4 {
			c.Assertions.InjectionReached = probeBool(reached)
			if i == 4 && errors.Is(err, errInjectedCancel) && reached && p.ctx.Err() == nil && !isFalse(result.CleanupCompleted) {
				err = nil
			}
			if i == 5 && errors.Is(err, errInjectedCutoff) && reached && !isFalse(result.CleanupCompleted) {
				err = nil
			}
		}
		if p.ctx.Err() != nil {
			err = probeContextError(p.ctx, p.ctx.Err())
		}
		if turn.contradicted {
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
		if i < 4 {
			c.Assertions.TentativeCompletion = probeBool(tentative)
			assistant := wireMessage{Assistant: &wireAssistant{Content: turn.text}}
			if i == 1 {
				toolID = turn.toolID
				// Preserve the original validated arguments, including JSON escapes.
				if err := memory.reserve(int64(len(turn.arguments))); err != nil {
					c.Cause = err.Error()
					turn.release()
					return out
				}
				assistant.Assistant.Tools = []wireToolUse{{ID: turn.toolID, Name: turn.toolName, Input: json.RawMessage(turn.arguments)}}
			}
			if err := memory.reserve(wireMessageBytes(current) + wireMessageBytes(assistant)); err != nil {
				c.Cause = err.Error()
				turn.release()
				return out
			}
			history = append(history, current, assistant)
		}
		turn.release()
		c.Status = "observed"
		c.FailureStage = ""
	}
	return out
}
