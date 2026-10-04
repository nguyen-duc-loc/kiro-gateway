package kiro

import (
	"bytes"
	"context"
	"sync"

	"kiro-gateway/internal/bridge"
)

const codingTestCommand = "rtk proxy go test ./..."

type codingCall struct {
	block          bridge.Block
	turn           int
	test, returned bool
}
type codingTurn struct{ read, edit, test, complete bool }

// codingEvidence is runner memory only. Its mutex serializes admission, file
// inspection, result matching and terminal writes with the next task turn.
// Lock order is evidence then controller, never the reverse.
type codingEvidence struct {
	mu                         sync.Mutex
	generator                  bridge.Generator
	control                    *runControl
	repo                       string
	prompts                    [2]string
	turns                      [2]codingTurn
	turn                       int
	calls                      map[string]*codingCall
	history                    []bridge.Message
	activeID                   string
	activeTurn                 int
	output                     *bridge.Response
	terminal, failed, finished bool
}

func newCodingEvidence(g bridge.Generator, control *runControl, repo, initial, followup string) *codingEvidence {
	return &codingEvidence{generator: g, control: control, repo: repo, prompts: [2]string{initial, followup}, turn: -1, activeTurn: -1, calls: map[string]*codingCall{}}
}
func (c *codingEvidence) reject() error {
	c.failed = true
	c.control.fail("coding_assertion_failed")
	return stoppedFailure()
}
func (c *codingEvidence) Before(o bridge.Observation) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.control.Before(o); err != nil {
		return err
	}
	if c.finished {
		return nil
	}
	if c.failed {
		return stoppedFailure()
	}
	if o.Phase == "admission" {
		if c.activeID != "" || o.RequestID == "" {
			return c.reject()
		}
		c.activeID, c.activeTurn = o.RequestID, -1
		c.terminal, c.output = false, nil
	}
	return nil
}
func (c *codingEvidence) Observe(o bridge.Observation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.control.Observe(o)
	if c.finished || c.failed {
		return
	}
	switch o.Phase {
	case "failure":
		c.failed = true
	case "terminal":
		if o.RequestID != c.activeID || c.activeTurn != c.turn || c.turn < 0 || c.output == nil || c.terminal || o.Category != "success" || !o.Cleanup || o.StopReason != c.output.StopReason() {
			c.reject()
			return
		}
		if c.output.Complete(bridge.End{Basis: bridge.InferredCleanEOF}) != nil {
			c.reject()
			return
		}
		c.history = append(c.history, bridge.Message{Role: "assistant", Content: c.output.Content})
		c.terminal = true
		if o.StopReason == "end_turn" {
			turn := &c.turns[c.turn]
			if !turn.edit || !turn.test || !c.turns[0].read || c.outstanding() {
				c.reject()
				return
			}
			if c.turn == 0 {
				present, _, err := boundaryAssertion(c.repo)
				if err != nil || present {
					c.reject()
					return
				}
			}
			turn.complete = true
		}
	case "cleanup":
		if o.RequestID != c.activeID || !o.Cleanup || !c.terminal {
			c.reject()
			return
		}
		c.activeID, c.activeTurn, c.output = "", -1, nil
	}
}
func (c *codingEvidence) outstanding() bool {
	for _, call := range c.calls {
		if call.turn == c.turn && !call.returned {
			return true
		}
	}
	return false
}
func (c *codingEvidence) accept(r bridge.Request) error {
	if c.activeID == "" || c.activeTurn != -1 || len(r.Messages) <= len(c.history) {
		return c.reject()
	}
	// Every previously observed message must remain unchanged. Replayed requests
	// cannot append a turn or count a result again.
	if !c.historyMatches(r.Messages) {
		return c.reject()
	}
	next := r.Messages[len(c.history):]
	users := 0
	for i, m := range next {
		if m.Role == "system" {
			if i == 0 || next[i-1].Role != "user" {
				return c.reject()
			}
			continue
		}
		if m.Role != "user" {
			return c.reject()
		}
		users++
		if users != 1 {
			return c.reject()
		}
		hasResult := false
		for _, b := range m.Content {
			hasResult = hasResult || b.Type == "tool_result"
		}
		if !hasResult {
			if c.turn >= 1 || c.turn >= 0 && !c.turns[c.turn].complete || !taskPrompt(m, c.prompts[c.turn+1]) {
				return c.reject()
			}
			c.turn++
		} else {
			if c.turn < 0 || c.turns[c.turn].complete {
				return c.reject()
			}
			for _, b := range m.Content {
				if b.Type != "tool_result" {
					continue
				}
				call := c.calls[b.ToolUseID]
				if call == nil || call.returned || call.turn != c.turn {
					return c.reject()
				}
				call.returned = true
				if b.IsError {
					continue
				}
				turn := &c.turns[c.turn]
				switch call.block.Name {
				case "Read":
					turn.read = true
				case "Edit":
					if c.turn == 1 {
						present, assertion, err := boundaryAssertion(c.repo)
						if err != nil || !present || !assertion {
							return c.reject()
						}
					}
					turn.edit = true
				case "Bash":
					if call.test {
						if !turn.edit {
							return c.reject()
						}
						turn.test = true
					}
				}
			}
		}
	}
	if users != 1 {
		return c.reject()
	}
	c.history = append([]bridge.Message(nil), r.Messages...)
	c.activeTurn = c.turn
	c.output = bridge.NewResponse(r)
	return nil
}
func (c *codingEvidence) Generate(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
	c.mu.Lock()
	bypass := c.finished
	var err error
	if !bypass {
		err = c.accept(r)
	}
	c.mu.Unlock()
	if err != nil {
		return bridge.End{}, err
	}
	if bypass {
		return c.generator.Generate(ctx, r, emit)
	}
	return c.generator.Generate(ctx, r, func(e bridge.Event) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.failed {
			return stoppedFailure()
		}
		var call *codingCall
		if e.Tool != nil {
			b := *e.Tool
			if c.calls[b.ID] != nil {
				return c.reject()
			}
			test := b.Name == "Bash" && b.Input["command"] == codingTestCommand
			if test && b.Input["run_in_background"] != nil && b.Input["run_in_background"] != false {
				return c.reject()
			}
			// The matching Edit result must have arrived in an earlier request.
			if test && !c.turns[c.turn].edit {
				return c.reject()
			}
			call = &codingCall{block: b, turn: c.turn, test: test}
		}
		if err := c.output.Add(e); err != nil {
			return c.reject()
		}
		// The handler can report a write failure synchronously from emit.
		c.mu.Unlock()
		err := emit(e)
		c.mu.Lock()
		if err != nil {
			return err
		}
		if call != nil {
			c.calls[call.block.ID] = call
		}
		return nil
	})
}
func (c *codingEvidence) complete() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.failed && c.activeID == "" && c.turns[0].complete && c.turns[1].complete
}
func (c *codingEvidence) finishCoding() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.finished = true
}

// Client context can precede the exact reviewed prompt in the same user message.
// All blocks remain part of the immutable history comparison.
func taskPrompt(m bridge.Message, prompt string) bool {
	matches := 0
	for _, block := range m.Content {
		if block.Type != "text" {
			return false
		}
		if block.Text == prompt {
			matches++
		}
	}
	return matches == 1 && m.Content[len(m.Content)-1].Text == prompt
}

func (c *codingEvidence) historyMatches(messages []bridge.Message) bool {
	if len(c.history) == 0 {
		return true
	}
	last := len(c.history) - 1
	if !bytes.Equal(bridge.Canonical(messages[:last]), bridge.Canonical(c.history[:last])) {
		return false
	}
	got, want := messages[last], c.history[last]
	if got.Role != want.Role || len(got.Content) != len(want.Content) {
		return false
	}
	for i, b := range got.Content {
		expected := want.Content[i]
		if b.Type != "tool_use" || expected.Type != "tool_use" {
			if !bytes.Equal(bridge.Canonical(b), bridge.Canonical(expected)) {
				return false
			}
			continue
		}
		if b.ID != expected.ID || b.Name != expected.Name {
			return false
		}
		for key, value := range expected.Input {
			if !bytes.Equal(bridge.Canonical(b.Input[key]), bridge.Canonical(value)) {
				return false
			}
		}
		for key, value := range b.Input {
			if _, ok := expected.Input[key]; ok {
				continue
			}
			// The pinned client explicitly inserts these false tool defaults before
			// returning its assistant history. No command or argument may change.
			if value == false && (b.Name == "Edit" && key == "replace_all" || b.Name == "Bash" && (key == "run_in_background" || key == "dangerouslyDisableSandbox")) {
				continue
			}
			return false
		}
	}
	return true
}
