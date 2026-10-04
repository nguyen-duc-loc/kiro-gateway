package kiro

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/gateway"
)

// covers: AC-11. Exercise real framing at both semantic limits, both client
// response modes, overflow, and cancellation after an incremental text write.
func TestFragmentedStreamLimitsAndCancellation(t *testing.T) {
	text := strings.Repeat("x", 2<<20)
	input := `{"value":"` + strings.Repeat("y", (256<<10)-12) + `"}`
	var wire bytes.Buffer
	for offset := 0; offset < len(text); offset += 64 {
		wire.Write(probeFrame("assistantResponseEvent", string(bridge.Canonical(map[string]any{"content": text[offset : offset+64]}))))
	}
	for offset := 0; offset < len(input); offset += 64 {
		event := map[string]any{"toolUseId": "fragmented_tool", "input": input[offset : offset+64]}
		if offset == 0 {
			event["name"] = "Read"
		}
		if offset+64 == len(input) {
			event["stop"] = true
		}
		wire.Write(probeFrame("toolUseEvent", string(bridge.Canonical(event))))
	}
	if wire.Len() > 8<<20 {
		t.Fatal("fragmented fixture exceeds wire limit")
	}
	t.Logf("fragmented fixture text_bytes=%d tool_bytes=%d wire_bytes=%d", len(text), len(input), wire.Len())
	for _, mode := range []string{"json", "sse", "cancel", "text overflow", "tool overflow"} {
		t.Run(mode, func(t *testing.T) {
			body := wire.Bytes()
			switch mode {
			case "text overflow":
				var overflow bytes.Buffer
				for range 32 {
					overflow.Write(probeFrame("assistantResponseEvent", string(bridge.Canonical(map[string]any{"content": strings.Repeat("x", 64<<10)}))))
				}
				overflow.Write(probeFrame("assistantResponseEvent", `{"content":"x"}`))
				body = overflow.Bytes()
			case "tool overflow":
				// Overflow before stop, rather than a fragment after a completed tool.
				var overflow bytes.Buffer
				for i := 0; i < 5; i++ {
					event := map[string]any{"toolUseId": "oversize", "input": strings.Repeat("x", 64<<10)}
					if i == 0 {
						event["name"] = "Read"
					}
					overflow.Write(probeFrame("toolUseEvent", string(bridge.Canonical(event))))
				}
				body = overflow.Bytes()
			}
			home, _ := probeHome(t)
			adapter := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				_, _ = w.Write(body)
			})
			handler := gateway.NewExperimentalHandler("synthetic-bearer", "fragmented", slog.New(slog.NewTextHandler(io.Discard, nil)), adapter)
			stream := mode != "json"
			raw := fmt.Sprintf(`{"model":"claude-opus-5.5","max_tokens":4096,"stream":%t,"messages":[{"role":"user","content":"synthetic"}],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`, stream)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(raw)).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer synthetic-bearer")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Anthropic-Version", "2023-06-01")
			recorder := httptest.NewRecorder()
			var writer http.ResponseWriter = recorder
			if mode == "cancel" {
				writer = &fragmentCancelWriter{recorder, cancel}
			}
			handler.ServeHTTP(writer, req)
			if mode == "cancel" || strings.Contains(mode, "overflow") {
				if strings.Contains(recorder.Body.String(), "event: message_stop") || strings.Contains(recorder.Body.String(), `"type":"tool_use"`) {
					t.Error("failed fragmented response exposed successful terminal or tool")
				}
				if mode == "cancel" && ctx.Err() == nil {
					t.Error("fragmented cancellation trigger absent")
				}
				if mode == "text overflow" && !strings.Contains(recorder.Body.String(), "event: error") {
					t.Error("text overflow did not signal stream error")
				}
				if mode == "tool overflow" && recorder.Code != http.StatusBadGateway {
					t.Errorf("tool overflow status=%d, want 502", recorder.Code)
				}
				return
			}
			var gotText, gotInput string
			var outputTokens int
			if !stream {
				var message struct {
					Content []struct {
						Type, Text string
						Input      map[string]any
					}
					Usage struct {
						OutputTokens int `json:"output_tokens"`
					}
				}
				if json.Unmarshal(recorder.Body.Bytes(), &message) != nil || len(message.Content) != 2 {
					t.Fatal("fragmented JSON invalid")
				}
				gotText = message.Content[0].Text
				gotInput = string(bridge.Canonical(message.Content[1].Input))
				outputTokens = message.Usage.OutputTokens
			} else {
				var accumulated strings.Builder
				scanner := bufio.NewScanner(bytes.NewReader(recorder.Body.Bytes()))
				scanner.Buffer(make([]byte, 4096), 1<<20)
				for scanner.Scan() {
					line := scanner.Text()
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					var event struct {
						Delta struct {
							Type, Text  string
							PartialJSON string `json:"partial_json"`
						}
						Usage struct {
							OutputTokens int `json:"output_tokens"`
						}
					}
					if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) != nil {
						t.Fatal("fragmented SSE invalid")
					}
					if event.Delta.Type == "text_delta" {
						accumulated.WriteString(event.Delta.Text)
					}
					if event.Delta.Type == "input_json_delta" {
						gotInput = event.Delta.PartialJSON
					}
					if event.Usage.OutputTokens != 0 {
						outputTokens = event.Usage.OutputTokens
					}
				}
				if scanner.Err() != nil {
					t.Fatal(scanner.Err())
				}
				gotText = accumulated.String()
				if !strings.Contains(recorder.Body.String(), "event: message_stop") {
					t.Error("fragmented SSE terminal absent")
				}
			}
			wantTokens := bridge.OutputTokens([]bridge.Block{{Type: "text", Text: text}, {Type: "tool_use", ID: "fragmented_tool", Name: "Read", Input: map[string]any{"value": strings.Repeat("y", (256<<10)-12)}}})
			if gotText != text || gotInput != input || outputTokens != wantTokens {
				t.Errorf("fragmented output text_equal=%t input_equal=%t tokens=%d, want exact content and %d tokens", gotText == text, gotInput == input, outputTokens, wantTokens)
			}
		})
	}
}

type fragmentCancelWriter struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (w *fragmentCancelWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseRecorder.Write(data)
	if bytes.Contains(data, []byte(`"type":"text_delta"`)) {
		w.cancel()
	}
	return n, err
}
