package bridge

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// covers: AC-3. Tool results must match the immediately preceding calls once.
func TestParseRejectsAmbiguousToolContinuations(t *testing.T) {
	call := `{"type":"tool_use","id":"c","name":"Read","input":{}}`
	result := `{"type":"tool_result","tool_use_id":"c","content":""}`
	for _, tc := range []struct{ name, assistant, user, suffix string }{
		{"duplicate call", call + "," + call, result, ""},
		{"duplicate result", call, result + "," + result, ""},
		{"changed result ID", call, strings.Replace(result, `"c"`, `"other"`, 1), ""},
		{"missing result with system", call, `{"type":"text","text":"ordinary"}`, `,{"role":"system","content":"cannot satisfy result"}`},
		{"text after tool", call + `,{"type":"text","text":"late"}`, result, ""},
		{"result after text", call, `{"type":"text","text":"early"},` + result, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":"start"},{"role":"assistant","content":[` + tc.assistant + `]},{"role":"user","content":[` + tc.user + `]}` + tc.suffix + `]}`
			for _, count := range []bool{false, true} {
				if _, err := Parse([]byte(body), count); err == nil {
					t.Errorf("Parse(%s, count=%t) succeeded, want invalid request", tc.name, count)
				}
			}
		})
	}
}

// covers: AC-3, AC-9. Check inclusive limits with real serialized request sizes.
func TestParseInclusiveResourceLimits(t *testing.T) {
	for _, size := range []int{MaxBody, MaxBody + 1} {
		body := strings.Replace(textRequest, "hello", strings.Repeat("x", size-len(textRequest)+len("hello")), 1)
		if _, err := Parse([]byte(body), false); (err != nil) != (size > MaxBody) {
			t.Errorf("Parse(body bytes=%d) error=%v, want rejection=%t", len(body), err, size > MaxBody)
		}
	}
	for _, n := range []int{128, 129} {
		tools := make([]string, n)
		for i := range tools {
			tools[i] = fmt.Sprintf(`{"name":"T%d","input_schema":{"type":"object"}}`, i)
		}
		body := strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"tools":[`+strings.Join(tools, ",")+`]`, 1)
		if _, err := Parse([]byte(body), false); (err != nil) != (n > 128) {
			t.Errorf("Parse(tools=%d) error=%v, want rejection=%t", n, err, n > 128)
		}
	}
	for _, size := range []int{64 << 10, (64 << 10) + 1} {
		base := `{"description":"","type":"object"}`
		schema := strings.Replace(base, `"description":""`, `"description":"`+strings.Repeat("x", size-len(base))+`"`, 1)
		body := strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"tools":[{"name":"Read","input_schema":`+schema+`}]`, 1)
		if _, err := Parse([]byte(body), false); (err != nil) != (size > 64<<10) {
			t.Errorf("Parse(schema bytes=%d) error=%v, want rejection=%t", len(schema), err, size > 64<<10)
		}
	}
}

// covers: AC-3. Compatibility metadata is discarded but arbitrary schemas survive.
func TestParseMetadataAndSchemaBoundaries(t *testing.T) {
	schema := `{"type":"object","$defs":{"v":{"enum":[9007199254740993,"text"]}},"properties":{"output_config":{"$ref":"#/$defs/v"}},"oneOf":[{"required":["output_config"]}],"additionalProperties":false}`
	body := strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"tools":[{"name":"Read","input_schema":`+schema+`}],"metadata":{"user_id":"discard"},"thinking":{"type":"disabled"},"stop_sequences":[]`, 1)
	r, err := Parse([]byte(body), false)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	d := json.NewDecoder(strings.NewReader(schema))
	d.UseNumber()
	if err := d.Decode(&want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Tools[0].InputSchema, want) {
		t.Errorf("Parse(schema).input_schema = %v, want %v", r.Tools[0].InputSchema, want)
	}
	if strings.Contains(string(Canonical(r)), "discard") {
		t.Error("Parse(metadata) retained discarded user ID, want no metadata")
	}
	for _, field := range []string{`"thinking":{"type":"adaptive"}`, `"stop_sequences":["stop"]`, `"metadata":{"extra":"value"}`, `"tool_choice":{"type":"any"}`} {
		invalid := strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,`+field, 1)
		if _, err := Parse([]byte(invalid), false); err == nil {
			t.Errorf("Parse(%s) succeeded, want unsupported semantics error", field)
		}
	}
}

const textRequest = `{"model":"claude-opus-5.5","max_tokens":4096,"messages":[{"role":"user","content":"hello"}]}`

func TestParseRejectedContracts(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"leading system role", `{"model":"claude-opus-5.5","max_tokens":4096,"messages":[{"role":"system","content":"invented context"},{"role":"user","content":"invented"}]}`},
		{"duplicate root", strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"max_tokens":1`, 1)},
		{"case variant", strings.Replace(textRequest, `"model"`, `"Model"`, 1)},
		{"trailing JSON", textRequest + `{}`},
		{"invalid UTF8", strings.Replace(textRequest, "hello", string([]byte{0xff}), 1)},
		{"unknown field", strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"temperature":0`, 1)},
		{"assistant prefill", strings.Replace(textRequest, `"role":"user"`, `"role":"assistant"`, 1)},
		{"unmatched result", `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"absent","content":"result"}]}]}`},
		{"duplicate nested", `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"Read","input_schema":{"type":"object","properties":{"x":{"type":"string","type":"number"}}}}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.body), false); err == nil {
				t.Errorf("Parse(%s) succeeded, want rejection", tc.name)
			}
		})
	}
}

func TestEffortIsValidatedAndDiscarded(t *testing.T) {
	base, err := Parse([]byte(textRequest), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []bool{false, true} {
		body := strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"output_config":{"effort":"high"}`, 1)
		got, err := Parse([]byte(body), count)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, base) || InputTokens(got) != InputTokens(base) {
			t.Error("accepted effort changed normalized content or estimate")
		}
		for _, value := range []string{`null`, `{}`, `[]`, `{"effort":"low"}`, `{"effort":1}`, `{"Effort":"high"}`, `{"effort":"high","format":{}}`} {
			body := strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"output_config":`+value, 1)
			_, err := Parse([]byte(body), count)
			if err == nil || SafeFailure(err).Message != "Unsupported or invalid output configuration." {
				t.Errorf("Parse(output_config=%s, count=%t) = %v, want fixed output configuration error", value, count, err)
			}
		}
	}
}

func TestParseToolHistoryAndEstimates(t *testing.T) {
	body := `{"model":"claude-opus-5.5","max_tokens":1,"system":[{"type":"text","text":"first"},{"type":"text","text":"second","cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[{"role":"user","content":"start"},{"role":"assistant","content":[{"type":"text","text":"calling"},{"type":"tool_use","id":"call_1","name":"OldTool","input":{"number":9007199254740993}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":[],"is_error":true},{"type":"text","text":"continue"}]}],"tools":[{"name":"Read","input_schema":{"type":"object","additionalProperties":false},"cache_control":{"type":"ephemeral"}}]}`
	r, err := Parse([]byte(body), false)
	if err != nil {
		t.Fatal(err)
	}
	if r.System != "first\n\nsecond" || r.Messages[1].Content[1].Input["number"] != json.Number("9007199254740993") || !r.Messages[2].Content[0].IsError {
		t.Error("Parse(tool history) lost system order, exact number, or error result")
	}
	if r.Tools[0].InputSchema["additionalProperties"] != false {
		t.Error("Parse(schema) lost additionalProperties")
	}
	canonical := string(Canonical(r.Messages))
	if strings.Contains(canonical, "cache_control") {
		t.Error("canonical content retained cache hint")
	}
	count, err := Parse([]byte(body), true)
	if err != nil {
		t.Fatal(err)
	}
	if InputTokens(count) != InputTokens(r) {
		t.Error("count route estimate differs")
	}
	if _, err := Parse([]byte(strings.Replace(body, `"max_tokens":1`, `"max_tokens":1,"stream":false`, 1)), true); err == nil {
		t.Error("count route accepted stream")
	}
}

func TestRequestDepthLimit(t *testing.T) {
	for _, n := range []int{58, 66} {
		body := strings.Replace(textRequest, `"max_tokens":4096`, `"max_tokens":4096,"tools":[{"name":"Read","input_schema":{"type":"object","x":`+strings.Repeat("[", n)+`0`+strings.Repeat("]", n)+`}}]`, 1)
		_, err := Parse([]byte(body), false)
		if (err != nil) != (n == 66) {
			t.Errorf("Parse(nesting=%d) error=%v, want rejection=%t", n, err, n == 66)
		}
	}
}

func TestResponseRequiresCompleteOfferedTools(t *testing.T) {
	r, _ := Parse([]byte(textRequest), false)
	s := NewResponse(r)
	if s.Complete(End{Basis: InferredCleanEOF}) == nil {
		t.Error("empty response completed")
	}
	if err := s.Add(Event{Text: "hello"}); err != nil {
		t.Fatal(err)
	}
	if s.Complete(End{}) == nil {
		t.Error("response completed without inferred EOF")
	}
	if s.Add(Event{Tool: &Block{Type: "tool_use", ID: "call", Name: "Read", Input: map[string]any{}}}) == nil {
		t.Error("response accepted an unoffered tool")
	}
	if err := s.Complete(End{Basis: InferredCleanEOF}); err != nil {
		t.Fatal(err)
	}
	if s.StopReason() != "end_turn" || OutputTokens(s.Content) == 0 {
		t.Error("complete text response has wrong stop or estimate")
	}
}
