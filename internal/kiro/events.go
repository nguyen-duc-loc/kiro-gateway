package kiro

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/jsonobject"
)

type pendingTool struct {
	id, name, input string
	stopped         bool
	value           map[string]any
}
type streamState struct {
	request              bridge.Request
	emit                 func(bridge.Event) error
	tools                []*pendingTool
	byID                 map[string]*pendingTool
	textBytes, reasoning int
}

func (s *streamState) observe(event string, payload []byte) error {
	o, err := jsonobject.Parse(payload, 1<<20)
	if err != nil {
		return bridge.ProtocolFailure()
	}
	if event == "reasoningContentEvent" {
		s.reasoning++
		return nil
	}
	fields, ok := map[string][]string{
		"assistantResponseEvent": {"content", "modelId"}, "toolUseEvent": {"toolUseId", "name", "input", "stop"},
		"messageMetadataEvent": {"conversationId", "utteranceId"}, "metadataEvent": {"tokenUsage"},
		"contextUsageEvent": {"contextUsagePercentage"}, "meteringEvent": {"usage", "unit", "unitPlural"},
	}[event]
	if !ok {
		return bridge.ProtocolFailure()
	}
	for k := range o {
		if !slices.Contains(fields, k) && event != "metadataEvent" {
			return bridge.ProtocolFailure()
		}
	}
	switch event {
	case "assistantResponseEvent":
		if raw, ok := o["modelId"]; ok {
			model, err := eventString(raw)
			if err != nil || model != s.request.Model {
				return bridge.ProtocolFailure()
			}
		}
		text, err := eventString(o["content"])
		if err != nil {
			return err
		}
		if text == "" {
			return nil
		}
		if len(s.tools) > 0 || s.textBytes+len(text) > 2<<20 {
			return bridge.ProtocolFailure()
		}
		s.textBytes += len(text)
		return s.emit(bridge.Event{Text: text})
	case "toolUseEvent":
		return s.tool(o)
	case "messageMetadataEvent":
		for _, raw := range o {
			if _, err := eventString(raw); err != nil {
				return err
			}
		}
	case "metadataEvent":
		if raw, ok := o["tokenUsage"]; ok {
			usage, err := jsonobject.Parse(raw, 1<<20)
			if err != nil {
				return bridge.ProtocolFailure()
			}
			for k, v := range usage {
				if !slices.Contains([]string{"inputTokens", "outputTokens", "totalTokens", "cacheReadInputTokens", "cacheWriteInputTokens", "uncachedInputTokens", "contextUsagePercentage", "normalizedTokenUsage"}, k) {
					return bridge.ProtocolFailure()
				}
				n, err := eventNumber(v)
				if err != nil || k == "outputTokens" && n != math.Trunc(n) {
					return bridge.ProtocolFailure()
				}
			}
		}
	case "contextUsageEvent":
		for _, raw := range o {
			if _, err := eventNumber(raw); err != nil {
				return err
			}
		}
	case "meteringEvent":
		if raw, ok := o["usage"]; ok {
			if _, err := eventNumber(raw); err != nil {
				return err
			}
		}
		for _, k := range []string{"unit", "unitPlural"} {
			if raw, ok := o[k]; ok {
				if _, err := eventString(raw); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *streamState) tool(o map[string]json.RawMessage) error {
	id, err := eventString(o["toolUseId"])
	if err != nil || !bridge.ValidID(id) {
		return bridge.ProtocolFailure()
	}
	if s.byID == nil {
		s.byID = map[string]*pendingTool{}
	}
	tool, exists := s.byID[id]
	if !exists {
		name, err := eventString(o["name"])
		if err != nil || !bridge.ValidName(name) || len(s.tools) >= 16 || s.request.DisableParallel && len(s.tools) > 0 {
			return bridge.ProtocolFailure()
		}
		offered := false
		for _, t := range s.request.Tools {
			if t.Name == name {
				offered = true
			}
		}
		for _, m := range s.request.Messages {
			for _, b := range m.Content {
				if b.Type == "tool_use" && b.ID == id {
					return bridge.ProtocolFailure()
				}
			}
		}
		if !offered {
			return bridge.ProtocolFailure()
		}
		tool = &pendingTool{id: id, name: name}
		s.byID[id] = tool
		s.tools = append(s.tools, tool)
	} else {
		if tool.stopped {
			return bridge.ProtocolFailure()
		}
		if raw, ok := o["name"]; ok {
			name, err := eventString(raw)
			if err != nil || name != tool.name {
				return bridge.ProtocolFailure()
			}
		}
	}
	if raw, ok := o["input"]; ok {
		part, err := eventString(raw)
		if err != nil || len(tool.input)+len(part) > 256<<10 {
			return bridge.ProtocolFailure()
		}
		tool.input += part
	}
	if raw, ok := o["stop"]; ok {
		var stop *bool
		if json.Unmarshal(raw, &stop) != nil || stop == nil {
			return bridge.ProtocolFailure()
		}
		if *stop {
			if _, err := jsonobject.Parse([]byte(tool.input), 256<<10); err != nil {
				return bridge.ProtocolFailure()
			}
			d := json.NewDecoder(bytes.NewBufferString(tool.input))
			d.UseNumber()
			if d.Decode(&tool.value) != nil {
				return bridge.ProtocolFailure()
			}
			tool.stopped = true
		}
	}
	return nil
}

// complete validates all calls before exposing any executable tool event.
func (s *streamState) complete() error {
	if s.textBytes == 0 && len(s.tools) == 0 {
		return bridge.ProtocolFailure()
	}
	for _, t := range s.tools {
		if !t.stopped {
			return bridge.ProtocolFailure()
		}
	}
	return nil
}
func (s *streamState) emitTools() error {
	for _, t := range s.tools {
		if err := s.emit(bridge.Event{Tool: &bridge.Block{Type: "tool_use", ID: t.id, Name: t.name, Input: t.value}}); err != nil {
			return err
		}
	}
	return nil
}
func eventString(raw json.RawMessage) (string, error) {
	var s *string
	if json.Unmarshal(raw, &s) != nil || s == nil {
		return "", bridge.ProtocolFailure()
	}
	return *s, nil
}
func eventNumber(raw json.RawMessage) (float64, error) {
	var n *float64
	if json.Unmarshal(raw, &n) != nil || n == nil || math.IsNaN(*n) || math.IsInf(*n, 0) || *n < 0 {
		return 0, bridge.ProtocolFailure()
	}
	return *n, nil
}
