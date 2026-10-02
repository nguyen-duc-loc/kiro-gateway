//go:build clientbridge

package kiro

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"io"
	"kiro-gateway/internal/gateway"
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
var wireClientBridge = flag.Bool("client-bridge", false, "explicit offline client through synthetic wire adapter")

func TestInstalledClientWireLoopOffline(t *testing.T) {
	installedClientWireLoop(t, false)
}

func TestInstalledClientInteractiveOffline(t *testing.T) {
	if !*terminalLaunchCheck {
		t.Skip("requires explicit terminal fixture")
	}
	installedClientWireLoop(t, true)
}

func installedClientWireLoop(t *testing.T, interactive bool) {
	t.Helper()
	if !*wireClientBridge {
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
	home, _ := probeHome(t)
	gen := adapterFixture(t, home, func(w http.ResponseWriter, req *http.Request) {
		var root map[string]any
		if json.NewDecoder(req.Body).Decode(&root) != nil {
			t.Error("synthetic wire request decode failed")
			w.WriteHeader(400)
			return
		}
		n := int(generated.Add(1))
		if n > 1 && n <= 4 {
			state, _ := root["conversationState"].(map[string]any)
			current, _ := state["currentMessage"].(map[string]any)
			user, _ := current["userInputMessage"].(map[string]any)
			context, _ := user["userInputMessageContext"].(map[string]any)
			results, _ := context["toolResults"].([]any)
			found := false
			for _, v := range results {
				result, _ := v.(map[string]any)
				if result["toolUseId"] == calls[n-2].ID && result["status"] == "success" {
					found = true
				}
			}
			if !found {
				t.Error("wire tool result not matched")
				w.WriteHeader(400)
				return
			}
			matched.Add(1)
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		if n <= len(calls) {
			call := calls[n-1]
			_, _ = w.Write(probeFrame("toolUseEvent", string(bridge.Canonical(map[string]any{"toolUseId": call.ID, "name": call.Name, "input": string(bridge.Canonical(call.Input)), "stop": true}))))
		} else {
			_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"The fixture check completed."}`))
		}
	})
	handler := gateway.NewExperimentalHandler(strings.Repeat("dummy", 8), "offline", slog.New(slog.NewTextHandler(io.Discard, nil)), gen)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/messages" {
			raw, _ := io.ReadAll(io.LimitReader(r.Body, bridge.MaxBody+1))
			r.Body = io.NopCloser(strings.NewReader(string(raw)))
			if _, err := bridge.Parse(raw, false); err != nil {
				invalid.Add(1)
				t.Error("installed client request contract mismatch")
			}
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	if interactive {
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
		defer cancel()
		args := []string{"--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", "Read value.go, change Value to return 2, and run the fixture tests."}
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = repo
		cmd.Env = wireClientEnv(srv.URL, strings.Repeat("dummy", 8), cfg)
		err := runInteractiveClient(cmd)
		got, readErr := os.ReadFile(file)
		t.Logf("interactive_loop calls=%d matched_results=%d invalid=%d client_success=%t real_kiro_dispatches=0", generated.Load(), matched.Load(), invalid.Load(), err == nil)
		if err != nil || readErr != nil || !strings.Contains(string(got), "return 2") || matched.Load() != 3 || generated.Load() != 5 || invalid.Load() != 0 {
			t.Error("interactive offline coding and followup exchange incomplete")
		}
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	args := []string{"-p", "--no-session-persistence", "--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--permission-prompt-tool", "stdio"}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = repo
	cmd.Env = wireClientEnv(srv.URL, strings.Repeat("dummy", 8), cfg)
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
					if request["tool_name"] == call.Name && wireFixtureInputMatches(request["input"], call.Input) {
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

func wireFixtureInputMatches(value any, want map[string]any) bool {
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

func wireClientEnv(url, token, cfg string) []string {
	env := []string{}
	for _, k := range []string{"HOME", "PATH", "TMPDIR", "USER", "LOGNAME", "SHELL", "TERM"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	for k, v := range map[string]string{"CLAUDE_CONFIG_DIR": cfg, "ANTHROPIC_BASE_URL": url, "ANTHROPIC_AUTH_TOKEN": token, "ANTHROPIC_MODEL": bridge.Model, "ANTHROPIC_DEFAULT_OPUS_MODEL": bridge.Model, "ANTHROPIC_DEFAULT_SONNET_MODEL": bridge.Model, "ANTHROPIC_DEFAULT_HAIKU_MODEL": bridge.Model, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "CLAUDE_CODE_DISABLE_TERMINAL_TITLE": "1", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1", "CLAUDE_CODE_DISABLE_THINKING": "1", "CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK": "1", "DISABLE_PROMPT_CACHING": "1", "CLAUDE_CODE_MAX_OUTPUT_TOKENS": "4096", "CLAUDE_CODE_MAX_RETRIES": "0"} {
		env = append(env, k+"="+v)
	}
	return env
}
