package kiro

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/gateway"
)

type evidenceGenerator struct{ events []bridge.Event }

func (g *evidenceGenerator) Generate(_ context.Context, _ bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
	for _, event := range g.events {
		if err := emit(event); err != nil {
			return bridge.End{}, err
		}
	}
	return bridge.End{Basis: bridge.InferredCleanEOF}, nil
}

type evidenceFixture struct {
	t         *testing.T
	repo      string
	plan      bridgePlan
	calls     []bridge.Block
	tracking  *codingEvidence
	generator *evidenceGenerator
	handler   http.Handler
	history   []bridge.Message
	stream    bool
}

func newEvidenceFixture(t *testing.T, stream bool) *evidenceFixture {
	t.Helper()
	repo, plan := codingFixture(t)
	control := newRunControl(t.Context(), time.Now().Add(time.Minute))
	t.Cleanup(func() { control.cancel(context.Canceled) })
	gen := &evidenceGenerator{}
	tracking := newCodingEvidence(gen, control, repo, plan.InitialPrompt, plan.FollowupPrompt)
	return &evidenceFixture{t: t, repo: repo, plan: plan, calls: codingCalls(repo, plan), tracking: tracking, generator: gen, handler: gateway.NewObservedExperimentalHandler("synthetic-bearer", "test", slog.New(slog.NewTextHandler(io.Discard, nil)), tracking, tracking), stream: stream}
}
func (f *evidenceFixture) request() *http.Request {
	f.t.Helper()
	tools := []bridge.Tool{}
	for _, name := range []string{"Read", "Edit", "Bash"} {
		tools = append(tools, bridge.Tool{Name: name, InputSchema: map[string]any{"type": "object"}})
	}
	raw := bridge.Canonical(map[string]any{"model": bridge.Model, "max_tokens": 4096, "stream": f.stream, "messages": f.history, "tools": tools})
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(raw)).WithContext(f.tracking.control.ctx)
	r.Header.Set("Authorization", "Bearer synthetic-bearer")
	r.Header.Set("Anthropic-Version", "2023-06-01")
	r.Header.Set("Content-Type", "application/json")
	return r
}
func (f *evidenceFixture) send(events ...bridge.Event) bool {
	f.t.Helper()
	f.generator.events = events
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, f.request())
	if f.tracking.failed {
		return false
	}
	if response.Code != 200 {
		f.t.Errorf("fixture response=%d, want 200", response.Code)
		return false
	}
	blocks := []bridge.Block{}
	for _, event := range events {
		if event.Tool != nil {
			blocks = append(blocks, *event.Tool)
		} else {
			blocks = append(blocks, bridge.Block{Type: "text", Text: event.Text})
		}
	}
	f.history = append(f.history, bridge.Message{Role: "assistant", Content: blocks})
	return true
}
func (f *evidenceFixture) user(text string) {
	f.history = append(f.history, bridge.Message{Role: "user", Content: []bridge.Block{{Type: "text", Text: text}}})
}
func (f *evidenceFixture) result(call int, failed bool) {
	f.history = append(f.history, bridge.Message{Role: "user", Content: []bridge.Block{{Type: "tool_result", ToolUseID: f.calls[call].ID, IsError: failed, Content: []string{"synthetic result"}}}})
}
func (f *evidenceFixture) initial() {
	f.t.Helper()
	f.user(f.plan.InitialPrompt)
	for i := 0; i < 4; i++ {
		if !f.send(bridge.Event{Tool: &f.calls[i]}) {
			f.t.Fatal("initial fixture handoff failed")
		}
		if i == 2 {
			writeCodingFile(f.t, f.repo, "clamp.go", strings.Replace(f.plan.Files["clamp.go"], "if value < lower { return upper }", "if value < lower { return lower }", 1))
		}
		f.result(i, false)
	}
	if !f.send(bridge.Event{Text: "Initial turn complete."}) {
		f.t.Fatal("initial fixture completion failed")
	}
}
func (f *evidenceFixture) followupEdit() {
	f.t.Helper()
	f.user(f.plan.FollowupPrompt)
	if !f.send(bridge.Event{Tool: &f.calls[4]}) {
		f.t.Fatal("followup fixture handoff failed")
	}
	writeCodingFile(f.t, f.repo, "clamp_test.go", f.plan.Files["clamp_test.go"]+boundaryTest)
	f.result(4, false)
}

// covers: AC-12. Both response modes reach the same two completed task turns.
func TestCodingEvidenceRequiresCompleteTurns(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			f := newEvidenceFixture(t, stream)
			f.initial()
			if f.tracking.complete() {
				t.Error("complete(after initial)=true, want false")
			}
			f.followupEdit()
			if !f.send(bridge.Event{Tool: &f.calls[5]}) {
				t.Fatal("second test handoff failed")
			}
			if f.tracking.complete() {
				t.Error("complete(tool handoff)=true, want false")
			}
			f.result(5, false)
			if !f.send(bridge.Event{Text: "Followup complete."}) || !f.tracking.complete() {
				t.Error("complete(two successful turns)=false, want true")
			}
		})
	}
}

func TestCodingEvidenceRejectsIncompleteProof(t *testing.T) {
	for _, mode := range []string{"duplicate request", "changed history", "early boundary", "missing boundary", "empty boundary", "unrelated bash", "background test", "composed command", "failed test", "missing test result", "combined edit and test", "reused ID", "unmatched result", "third turn", "tool handoff only"} {
		t.Run(mode, func(t *testing.T) {
			f := newEvidenceFixture(t, true)
			if mode == "early boundary" {
				writeCodingFile(t, f.repo, "clamp_test.go", f.plan.Files["clamp_test.go"]+boundaryTest)
			}
			// Build the first turn explicitly because early test presence must fail it.
			f.user(f.plan.InitialPrompt)
			for i := 0; i < 4; i++ {
				if !f.send(bridge.Event{Tool: &f.calls[i]}) {
					t.Fatal("initial handoff failed")
				}
				f.result(i, false)
			}
			first := f.send(bridge.Event{Text: "Initial complete."})
			if mode == "early boundary" {
				if first {
					t.Error("early boundary accepted")
				}
				return
			}
			if !first {
				t.Fatal("initial completion failed")
			}
			if mode == "duplicate request" {
				f.history = f.history[:len(f.history)-1]
				f.send(bridge.Event{Text: "duplicate"})
			} else {
				f.user(f.plan.FollowupPrompt)
				if mode == "changed history" {
					f.history[0].Content = []bridge.Block{{Type: "text", Text: "different"}}
				}
				if mode == "combined edit and test" {
					f.send(bridge.Event{Tool: &f.calls[4]}, bridge.Event{Tool: &f.calls[5]})
				} else if mode == "reused ID" {
					call := f.calls[4]
					call.ID = f.calls[0].ID
					f.send(bridge.Event{Tool: &call})
				} else {
					accepted := f.send(bridge.Event{Tool: &f.calls[4]})
					if mode == "changed history" {
						if accepted {
							t.Error("changed history accepted")
						}
						return
					}
					if !accepted {
						t.Fatal("followup edit handoff failed")
					}
					test := boundaryTest
					if mode == "missing boundary" {
						test = ""
					}
					if mode == "empty boundary" {
						test = "\nfunc TestClampUpperBoundary(t *testing.T) {}\n"
					}
					writeCodingFile(t, f.repo, "clamp_test.go", f.plan.Files["clamp_test.go"]+test)
					f.result(4, false)
					if mode == "unmatched result" {
						f.history[len(f.history)-1].Content[0].ToolUseID = "unmatched"
					}
					call := f.calls[5]
					if mode == "unrelated bash" {
						call.Input = map[string]any{"command": "rtk proxy go version"}
					}
					if mode == "background test" {
						call.Input = map[string]any{"command": codingTestCommand, "run_in_background": true}
					}
					if mode == "composed command" {
						call.Input = map[string]any{"command": codingTestCommand + "; true"}
					}
					accepted = f.send(bridge.Event{Tool: &call})
					if accepted {
						if mode == "tool handoff only" {
							if f.tracking.complete() {
								t.Error("tool handoff completed proof")
							}
							return
						}
						if mode == "missing test result" {
							f.user("not a result")
						} else {
							f.result(5, mode == "failed test")
						}
						f.send(bridge.Event{Text: "Followup complete."})
						if mode == "third turn" {
							f.user("A third task")
							f.send(bridge.Event{Text: "third"})
						}
					}
				}
			}
			if f.tracking.complete() || !f.tracking.failed {
				t.Errorf("incomplete proof accepted: complete=%t failed=%t", f.tracking.complete(), f.tracking.failed)
			}
		})
	}
}

type evidenceWriteFailure struct {
	*httptest.ResponseRecorder
	stream bool
	mode   string
	cancel context.CancelFunc
}

func (w *evidenceWriteFailure) Write(data []byte) (int, error) {
	if !w.stream || bytes.Contains(data, []byte("event: message_stop")) {
		switch w.mode {
		case "short":
			return len(data) - 1, nil
		case "cancel":
			n, err := w.ResponseRecorder.Write(data)
			w.cancel()
			return n, err
		default:
			return 0, io.ErrClosedPipe
		}
	}
	return w.ResponseRecorder.Write(data)
}
func TestCodingEvidenceFailedTerminalWrite(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, mode := range []string{"error", "short", "cancel"} {
			t.Run(fmt.Sprintf("stream=%t/%s", stream, mode), func(t *testing.T) {
				f := newEvidenceFixture(t, stream)
				f.initial()
				f.followupEdit()
				if !f.send(bridge.Event{Tool: &f.calls[5]}) {
					t.Fatal("test handoff failed")
				}
				f.result(5, false)
				f.generator.events = []bridge.Event{{Text: "Completed upstream only."}}
				ctx, cancel := context.WithCancel(f.tracking.control.ctx)
				defer cancel()
				writer := &evidenceWriteFailure{ResponseRecorder: httptest.NewRecorder(), stream: stream, mode: mode, cancel: cancel}
				func() {
					defer func() {
						if p := recover(); p != nil && p != http.ErrAbortHandler {
							panic(p)
						}
					}()
					f.handler.ServeHTTP(writer, f.request().WithContext(ctx))
				}()
				if f.tracking.complete() || f.tracking.turns[1].complete {
					t.Error("failed final write completed proof")
				}
			})
		}
	}
}

func TestCodingEvidenceRejectsMismatchedTerminal(t *testing.T) {
	for _, mode := range []string{"wrong request", "absent stop reason", "tool handoff", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			f := newEvidenceFixture(t, false)
			f.initial()
			f.followupEdit()
			if !f.send(bridge.Event{Tool: &f.calls[5]}) {
				t.Fatal("test handoff failed")
			}
			f.result(5, false)
			c := f.tracking
			if err := c.Before(bridge.Observation{RequestID: "pending", Phase: "admission"}); err != nil {
				t.Fatal(err)
			}
			f.generator.events = []bridge.Event{{Text: "Upstream complete."}}
			r, err := bridge.Parse(mustReadRequest(t, f.request()), false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Generate(t.Context(), r, func(bridge.Event) error { return nil }); err != nil {
				t.Fatal(err)
			}
			observation := bridge.Observation{RequestID: "pending", Phase: "terminal", Category: "success", Cleanup: true, StopReason: "end_turn"}
			switch mode {
			case "wrong request":
				observation.RequestID = "other"
			case "absent stop reason":
				observation.StopReason = ""
			case "tool handoff":
				observation.StopReason = "tool_use"
			case "canceled":
				observation.Phase = "failure"
				observation.Category = "canceled"
				observation.StopReason = ""
			}
			c.Observe(observation)
			if c.complete() || !c.failed || c.turns[1].complete {
				t.Errorf("terminal(%s) completed=%t failed=%t, want incomplete and failed", mode, c.turns[1].complete, c.failed)
			}
		})
	}
}
func mustReadRequest(t *testing.T, r *http.Request) []byte {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
