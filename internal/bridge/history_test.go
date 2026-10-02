package bridge

import (
	"reflect"
	"strings"
	"testing"
)

func TestHistorySystemNormalization(t *testing.T) {
	for _, count := range []bool{false, true} {
		body := `{"model":"claude-opus-5.5","max_tokens":1,"system":"top","messages":[{"role":"user","content":"U1"},{"role":"system","content":" S1\n"},{"role":"assistant","content":"A1"},{"role":"user","content":"U2"},{"role":"system","content":""}]}`
		r, err := Parse([]byte(body), count)
		if err != nil {
			t.Fatalf("Parse(history, count=%t) = %v, want success", count, err)
		}
		want := []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: "U1"}}},
			{Role: "system", Content: []Block{{Type: "text", Text: " S1\n"}}},
			{Role: "assistant", Content: []Block{{Type: "text", Text: "A1"}}},
			{Role: "user", Content: []Block{{Type: "text", Text: "U2"}}},
			{Role: "system", Content: []Block{{Type: "text", Text: ""}}},
		}
		if !reflect.DeepEqual(r.Messages, want) || r.System != "top" {
			t.Errorf("Parse(history, count=%t) = %#v, want original text and positions", count, r)
		}
		b := Canonical(map[string]any{"system": "top", "messages": want, "tools": []Tool{}})
		if got := InputTokens(r); got != (len(b)+3)/4 {
			t.Errorf("InputTokens(history) = %d, want %d", got, (len(b)+3)/4)
		}
	}
}

func TestHistorySystemRejectsUnsupportedShapeAndPosition(t *testing.T) {
	u := `{"role":"user","content":"U"}`
	a := `{"role":"assistant","content":"A"}`
	s := `{"role":"system","content":"S"}`
	for _, history := range []string{
		s + "," + u, u + "," + s + "," + s, u + "," + a + "," + s + "," + u,
		u + "," + s + "," + u, u + "," + s + "," + a,
		u + `,{"role":"system","content":[]}`, u + `,{"role":"system","content":null}`,
		u + `,{"role":"system","content":"S","cache_control":{"type":"ephemeral"}}`,
		u + `,{"role":"assistant","content":[{"type":"tool_use","id":"call","name":"Read","input":{}}]},` + u + "," + s,
	} {
		for _, count := range []bool{false, true} {
			body := `{"model":"claude-opus-5.5","max_tokens":1,"messages":[` + history + `]}`
			if _, err := Parse([]byte(body), count); err == nil {
				t.Errorf("Parse(%s, count=%t) succeeded, want rejection", history, count)
			}
		}
	}
}

func TestHistorySystemCountsRawEntriesAndBlocks(t *testing.T) {
	u := `{"role":"user","content":"U"}`
	s := `{"role":"system","content":"S"}`
	a := `{"role":"assistant","content":"A"}`
	history := strings.Repeat(u+","+s+","+a+",", 85) + u
	for _, extra := range []string{"", "," + s} {
		body := `{"model":"claude-opus-5.5","max_tokens":1,"messages":[` + history + extra + `]}`
		if _, err := Parse([]byte(body), false); (err != nil) != (extra != "") {
			t.Errorf("Parse(raw entries %d) error=%v, want rejection=%t", len(strings.Split(history+extra, `},{`)), err, extra != "")
		}
	}
	for _, n := range []int{1023, 1024} {
		blocks := strings.TrimSuffix(strings.Repeat(`{"type":"text","text":"x"},`, n), ",")
		body := `{"model":"claude-opus-5.5","max_tokens":1,"messages":[{"role":"user","content":[` + blocks + `]},` + s + `]}`
		if _, err := Parse([]byte(body), false); (err != nil) != (n == 1024) {
			t.Errorf("Parse(%d text blocks plus system) error=%v, want rejection=%t", n, err, n == 1024)
		}
	}
}
