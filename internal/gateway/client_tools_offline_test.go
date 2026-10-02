//go:build clientbridge

package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"

	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/bridge"
)

// This host answers only the exact invented fixture permissions. It is not
// product code and does not establish the later live human permission evidence.
func TestInstalledClientToolLoopOffline(t *testing.T) {
	if !*clientBridge {
		t.Skip("requires explicit -client-bridge flag")
	}
	binary, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal("installed client unavailable")
	}
	version, err := exec.Command(binary, "--version").Output()
	if err != nil || !strings.HasPrefix(string(version), "2.1.287 ") {
		t.Fatal("client version mismatch")
	}
	dir := t.TempDir()
	repo, cfg := filepath.Join(dir, "fixture"), filepath.Join(dir, "client")
	for _, p := range []string{repo, cfg} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(repo, "value.go")
	for name, body := range map[string]string{
		"go.mod":        "module fixture\n\ngo 1.27.1\n",
		"value.go":      "package fixture\nfunc Value() int { return 1 }\n",
		"value_test.go": "package fixture\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 2 { t.Fatal(\"wrong value\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := []bridge.Block{
		{Type: "tool_use", ID: "offline_read", Name: "Read", Input: map[string]any{"file_path": file}},
		{Type: "tool_use", ID: "offline_edit", Name: "Edit", Input: map[string]any{"file_path": file, "old_string": "return 1", "new_string": "return 2"}},
		{Type: "tool_use", ID: "offline_bash", Name: "Bash", Input: map[string]any{"command": "rtk proxy go test ./...", "description": "Test disposable fixture"}},
	}
	var generated, invalid, matched atomic.Int64
	gen := generateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
		n := int(generated.Add(1))
		if n > 1 && n <= 4 {
			found := false
			for _, m := range r.Messages {
				for _, b := range m.Content {
					if b.Type == "tool_result" && b.ToolUseID == calls[n-2].ID && !b.IsError {
						found = true
					}
				}
			}
			if !found {
				return bridge.End{}, bridge.ProtocolFailure()
			}
			matched.Add(1)
		}
		if n <= len(calls) {
			if err := emit(bridge.Event{Tool: &calls[n-1]}); err != nil {
				return bridge.End{}, err
			}
		} else {
			if err := emit(bridge.Event{Text: "The fixture check completed."}); err != nil {
				return bridge.End{}, err
			}
		}
		return bridge.End{Basis: bridge.InferredCleanEOF}, nil
	})
	handler := NewExperimentalHandler(strings.Repeat("dummy", 8), "offline", slog.New(slog.NewTextHandler(io.Discard, nil)), gen)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/messages" {
			raw, _ := io.ReadAll(io.LimitReader(r.Body, bridge.MaxBody+1))
			r.Body = io.NopCloser(strings.NewReader(string(raw)))
			if _, err := bridge.Parse(raw, false); err != nil {
				invalid.Add(1)
				offlineShape(t, raw)
			}
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	args := []string{"-p", "--no-session-persistence", "--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--permission-prompt-tool", "stdio"}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = repo
	cmd.Env = offlineClientEnv(srv.URL, strings.Repeat("dummy", 8), cfg)
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal("client start failed")
	}
	// CommandContext terminates the child on deadline; the test always joins it.
	waited := false
	defer func() {
		cancel()
		_ = stdin.Close()
		if !waited {
			_ = cmd.Wait()
		}
	}()
	send := func(v any) {
		if json.NewEncoder(stdin).Encode(v) != nil {
			t.Error("offline host write failed")
		}
	}
	user := func(s string) {
		send(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": s}})
	}
	send(map[string]any{"type": "control_request", "request_id": "offline_init", "request": map[string]any{"subtype": "initialize", "hooks": map[string]any{}}})
	user("Read value.go, change Value to return 2, and run the fixture tests.")
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	turns, permissions := 0, 0
	for scanner.Scan() {
		var msg map[string]any
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			t.Error("invalid client host message")
			break
		}
		switch msg["type"] {
		case "control_request":
			request, _ := msg["request"].(map[string]any)
			allow := false
			if request["subtype"] == "can_use_tool" {
				for _, call := range calls {
					if request["tool_name"] == call.Name && fixtureInputMatches(request["input"], call.Input) {
						allow = true
					}
				}
			}
			response := map[string]any{"behavior": "deny", "message": "Not an approved fixture operation."}
			if allow {
				permissions++
				response = map[string]any{"behavior": "allow", "updatedInput": request["input"]}
			}
			send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": msg["request_id"], "response": response}})
		case "result":
			turns++
			if msg["is_error"] == true {
				t.Error("client reported an error result")
				_ = stdin.Close()
			} else if turns == 1 {
				user("Confirm the completed fixture change on this subsequent turn.")
			} else {
				_ = stdin.Close()
			}
		}
	}
	err = cmd.Wait()
	waited = true
	got, readErr := os.ReadFile(file)
	t.Logf("tool_loop generator_calls=%d matched_results=%d invalid=%d completed_turns=%d host_permissions=%d client_success=%t kiro_dispatches=0", generated.Load(), matched.Load(), invalid.Load(), turns, permissions, err == nil)
	if err != nil || readErr != nil || !strings.Contains(string(got), "return 2") || matched.Load() != 3 || turns != 2 || generated.Load() != 5 || invalid.Load() != 0 {
		t.Error("offline coding and followup exchange incomplete")
	}
}

func fixtureInputMatches(value any, want map[string]any) bool {
	got, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	for k, v := range got {
		if _, ok := want[k]; ok {
			continue
		}
		if (k == "replace_all" || k == "run_in_background" || k == "dangerouslyDisableSandbox") && v == false {
			continue
		}
		return false
	}
	return true
}
