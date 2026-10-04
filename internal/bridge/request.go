package bridge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"kiro-gateway/internal/jsonobject"
)

// Parse validates the entire client contract before account access. count selects
// the count_tokens shape, which never accepts stream or requires max_tokens.
func Parse(data []byte, count bool) (Request, error) {
	r, err := parse(data, count)
	if err != nil {
		return Request{}, err
	}
	return r, nil
}

func parse(data []byte, count bool) (Request, error) {
	r := Request{Messages: []Message{}, Tools: []Tool{}}
	if _, err := jsonobject.Parse(data, MaxBody); err != nil {
		return r, Invalid()
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var root map[string]any
	if d.Decode(&root) != nil || !depthOK(root, 0) || !keys(root, "model", "max_tokens", "stream", "system", "messages", "tools", "tool_choice", "metadata", "thinking", "stop_sequences", "output_config") {
		return r, Invalid()
	}
	r.Model, _ = root["model"].(string)
	if r.Model != Model {
		return r, Invalid()
	}
	if v, ok := root["output_config"]; ok {
		m, ok := v.(map[string]any)
		if !ok || len(m) != 1 || m["effort"] != "high" {
			return r, &Failure{http.StatusBadRequest, "invalid_request_error", "Unsupported or invalid output configuration.", "output_configuration"}
		}
	}
	if v, ok := root["max_tokens"]; ok {
		n, ok := v.(json.Number)
		if !ok {
			return r, Invalid()
		}
		x, err := n.Int64()
		if err != nil || x < 1 || x > 65536 {
			return r, Invalid()
		}
		r.MaxTokens = int(x)
	} else if !count {
		return r, Invalid()
	}
	if v, ok := root["stream"]; ok {
		b, valid := v.(bool)
		if count || !valid {
			return r, Invalid()
		}
		r.Stream = b
	}
	blocks := 0
	if v, ok := root["system"]; ok {
		ss, valid := textContent(v, true, &blocks)
		if !valid {
			return r, Invalid()
		}
		r.System = strings.Join(ss, "\n\n")
	}
	if v, ok := root["metadata"]; ok {
		m, ok := v.(map[string]any)
		if !ok || !keys(m, "user_id") {
			return r, Invalid()
		}
		if x, ok := m["user_id"]; ok {
			if _, ok := x.(string); !ok {
				return r, Invalid()
			}
		}
	}
	if v, ok := root["thinking"]; ok {
		m, ok := v.(map[string]any)
		if !ok || len(m) != 1 || m["type"] != "disabled" {
			return r, Invalid()
		}
	}
	if v, ok := root["stop_sequences"]; ok {
		a, ok := v.([]any)
		if !ok || len(a) != 0 {
			return r, Invalid()
		}
	}
	names := map[string]bool{}
	if v, ok := root["tools"]; ok {
		a, ok := v.([]any)
		if !ok || len(a) > 128 {
			return r, Invalid()
		}
		for _, v := range a {
			m, ok := v.(map[string]any)
			if !ok || !keys(m, "name", "description", "input_schema", "cache_control") || !cache(m) {
				return r, Invalid()
			}
			t := Tool{}
			t.Name, _ = m["name"].(string)
			if !ValidName(t.Name) || names[t.Name] {
				return r, Invalid()
			}
			names[t.Name] = true
			if v, ok := m["description"]; ok {
				var valid bool
				t.Description, valid = v.(string)
				if !valid {
					return r, Invalid()
				}
			}
			t.InputSchema, ok = m["input_schema"].(map[string]any)
			if !ok || t.InputSchema["type"] != "object" || len(Canonical(t.InputSchema)) > 64<<10 {
				return r, Invalid()
			}
			r.Tools = append(r.Tools, t)
		}
	}
	if v, ok := root["tool_choice"]; ok {
		m, ok := v.(map[string]any)
		if !ok {
			return r, Invalid()
		}
		switch m["type"] {
		case "none":
			if len(m) != 1 {
				return r, Invalid()
			}
			r.Tools = []Tool{}
		case "auto":
			if !keys(m, "type", "disable_parallel_tool_use") {
				return r, Invalid()
			}
			if v, ok := m["disable_parallel_tool_use"]; ok {
				var valid bool
				r.DisableParallel, valid = v.(bool)
				if !valid {
					return r, Invalid()
				}
			}
		default:
			return r, Invalid()
		}
	}
	ms, ok := root["messages"].([]any)
	if !ok || len(ms) == 0 || len(ms) > 256 {
		return r, Invalid()
	}
	seen := map[string]bool{}
	pending := map[string]bool{}
	conversationCount := 0
	for _, v := range ms {
		m, ok := v.(map[string]any)
		if !ok || !keys(m, "role", "content") {
			return r, Invalid()
		}
		if m["role"] == "system" {
			s, valid := m["content"].(string)
			if !valid || len(r.Messages) == 0 || r.Messages[len(r.Messages)-1].Role != "user" {
				return r, Invalid()
			}
			blocks++
			if blocks > 1024 {
				return r, Invalid()
			}
			r.Messages = append(r.Messages, Message{Role: "system", Content: []Block{{Type: "text", Text: s}}})
			continue
		}
		role := "user"
		if conversationCount%2 == 1 {
			role = "assistant"
		}
		if m["role"] != role {
			return r, Invalid()
		}
		conversationCount++
		out := Message{Role: role, Content: []Block{}}
		var a []any
		if s, ok := m["content"].(string); ok {
			a = []any{map[string]any{"type": "text", "text": s}}
		} else {
			a, ok = m["content"].([]any)
			if !ok || len(a) == 0 {
				return r, Invalid()
			}
		}
		textSeen, callSeen := false, false
		for _, v := range a {
			blocks++
			if blocks > 1024 {
				return r, Invalid()
			}
			b, ok := v.(map[string]any)
			if !ok {
				return r, Invalid()
			}
			block := Block{}
			block.Type, _ = b["type"].(string)
			switch block.Type {
			case "text":
				if callSeen || !keys(b, "type", "text", "cache_control") || !cache(b) {
					return r, Invalid()
				}
				block.Text, ok = b["text"].(string)
				if !ok {
					return r, Invalid()
				}
				textSeen = true
			case "tool_use":
				if role != "assistant" || !keys(b, "type", "id", "name", "input") {
					return r, Invalid()
				}
				callSeen = true
				block.ID, _ = b["id"].(string)
				block.Name, _ = b["name"].(string)
				block.Input, ok = b["input"].(map[string]any)
				if !ok || !ValidID(block.ID) || !ValidName(block.Name) || seen[block.ID] {
					return r, Invalid()
				}
				seen[block.ID] = true
				pending[block.ID] = true
			case "tool_result":
				if role != "user" || textSeen || !keys(b, "type", "tool_use_id", "is_error", "content") {
					return r, Invalid()
				}
				block.ToolUseID, _ = b["tool_use_id"].(string)
				if !pending[block.ToolUseID] {
					return r, Invalid()
				}
				delete(pending, block.ToolUseID)
				if v, exists := b["is_error"]; exists {
					block.IsError, ok = v.(bool)
					if !ok {
						return r, Invalid()
					}
				}
				block.Content, ok = textContent(b["content"], false, &blocks)
				if !ok {
					return r, Invalid()
				}
			default:
				return r, Invalid()
			}
			out.Content = append(out.Content, block)
		}
		if role == "user" && len(pending) != 0 {
			return r, Invalid()
		}
		r.Messages = append(r.Messages, out)
	}
	if conversationCount%2 == 0 {
		return r, Invalid()
	}
	return r, nil
}

func keys(m map[string]any, allowed ...string) bool {
	for k := range m {
		found := false
		for _, a := range allowed {
			if k == a {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func cache(m map[string]any) bool {
	v, ok := m["cache_control"]
	if !ok {
		return true
	}
	c, ok := v.(map[string]any)
	if !ok || !keys(c, "type", "ttl") || c["type"] != "ephemeral" {
		return false
	}
	if ttl, ok := c["ttl"]; ok {
		return ttl == "5m" || ttl == "1h"
	}
	return true
}

func textContent(v any, hints bool, count *int) ([]string, bool) {
	if s, ok := v.(string); ok {
		return []string{s}, true
	}
	a, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := []string{}
	for _, v := range a {
		*count++
		if *count > 1024 {
			return nil, false
		}
		m, ok := v.(map[string]any)
		if !ok || m["type"] != "text" {
			return nil, false
		}
		if hints {
			if !keys(m, "type", "text", "cache_control") || !cache(m) {
				return nil, false
			}
		} else if !keys(m, "type", "text") {
			return nil, false
		}
		s, ok := m["text"].(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func depthOK(v any, depth int) bool {
	switch x := v.(type) {
	case map[string]any:
		if depth >= 64 {
			return false
		}
		for _, v := range x {
			if !depthOK(v, depth+1) {
				return false
			}
		}
	case []any:
		if depth >= 64 {
			return false
		}
		for _, v := range x {
			if !depthOK(v, depth+1) {
				return false
			}
		}
	}
	return true
}

// ValidName reports whether a name fits the supported ASCII tool vocabulary.
func ValidName(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// ValidID reports whether an ID is bounded visible ASCII.
func ValidID(s string) bool {
	if len(s) < 1 || len(s) > 256 {
		return false
	}
	for _, c := range s {
		if c < '!' || c > '~' {
			return false
		}
	}
	return true
}
