// Package kiro implements the experimental inference adapter.
package kiro

import (
	"kiro-gateway/internal/bridge"
	"strings"
)

// encodeRequest constructs fresh wire values, leaving normalized history intact.
func encodeRequest(r bridge.Request, profile, conversation string) ([]byte, error) {
	history := []any{}
	var current map[string]any
	first := true
	for i, m := range r.Messages {
		if m.Role == "system" {
			continue
		}
		texts := []string{}
		for _, b := range m.Content {
			if b.Type == "text" {
				texts = append(texts, b.Text)
			}
		}
		content := strings.Join(texts, "\n\n")
		if m.Role == "assistant" {
			uses := []any{}
			for _, b := range m.Content {
				if b.Type == "tool_use" {
					uses = append(uses, map[string]any{"toolUseId": b.ID, "name": b.Name, "input": b.Input})
				}
			}
			if content == "" {
				content = "(empty placeholder)"
			}
			assistant := map[string]any{"content": content}
			if len(uses) > 0 {
				assistant["toolUses"] = uses
			}
			history = append(history, map[string]any{"assistantResponseMessage": assistant})
			continue
		}
		if i+1 < len(r.Messages) && r.Messages[i+1].Role == "system" {
			s := r.Messages[i+1].Content[0].Text
			if s != "" {
				content += "\n\n" + s
			}
		}
		if first && r.System != "" {
			content = r.System + "\n\n" + content
		}
		first = false
		user := map[string]any{"content": content, "modelId": r.Model, "origin": "AI_EDITOR"}
		context := map[string]any{}
		results := []any{}
		for _, b := range m.Content {
			if b.Type != "tool_result" {
				continue
			}
			texts := []any{}
			for _, s := range b.Content {
				texts = append(texts, map[string]any{"text": s})
			}
			if len(texts) == 0 {
				texts = append(texts, map[string]any{"text": ""})
			}
			status := "success"
			if b.IsError {
				status = "error"
			}
			results = append(results, map[string]any{"toolUseId": b.ToolUseID, "status": status, "content": texts})
		}
		if len(results) > 0 {
			context["toolResults"] = results
		}
		last := i == len(r.Messages)-1 || i == len(r.Messages)-2 && r.Messages[i+1].Role == "system"
		if last && len(r.Tools) > 0 {
			tools := []any{}
			for _, t := range r.Tools {
				tools = append(tools, map[string]any{"toolSpecification": map[string]any{"name": t.Name, "description": t.Description, "inputSchema": map[string]any{"json": t.InputSchema}}})
			}
			context["tools"] = tools
		}
		if len(context) > 0 {
			user["userInputMessageContext"] = context
		}
		message := map[string]any{"userInputMessage": user}
		if last {
			current = message
		} else {
			history = append(history, message)
		}
	}
	if current == nil {
		return nil, bridge.Invalid()
	}
	state := map[string]any{"conversationId": conversation, "chatTriggerType": "MANUAL", "currentMessage": current}
	if len(history) > 0 {
		state["history"] = history
	}
	body := bridge.Canonical(map[string]any{"profileArn": profile, "conversationState": state})
	if len(body) == 0 || len(body) > 8<<20 {
		return nil, bridge.Invalid()
	}
	return body, nil
}
