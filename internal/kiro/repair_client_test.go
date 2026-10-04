//go:build clientbridge

package kiro

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/gateway"
)

var repairClient = flag.Bool("repair-client", false, "exercise the pinned client against synthetic loopback services only")

func repairClientBinary(t *testing.T) string {
	t.Helper()
	if !*repairClient {
		t.Skip("requires explicit -repair-client flag")
	}
	binary, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal("client unavailable")
	}
	version, err := exec.Command(binary, "--version").Output()
	if err != nil || !strings.HasPrefix(string(version), "2.1.289 ") {
		t.Fatal("client version mismatch")
	}
	artifact, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal("client artifact unavailable")
	}
	sum := sha256.Sum256(artifact)
	if hex.EncodeToString(sum[:]) != "03d66745e3bb69ec727d66023696f3820bc0a00a8a5ba725eb6706d0c67cbe69" {
		t.Fatal("client artifact mismatch")
	}
	return binary
}

// The host approves only exact invented fixture operations, never live tools.
func runRepairClient(t *testing.T, binary, repo, url string, calls []bridge.Block, prompts []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-p", "--no-session-persistence", "--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--permission-prompt-tool", "stdio")
	cmd.Dir = repo
	cmd.Env = liveClientEnv(url, "synthetic-bearer", t.TempDir())
	cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOENV=off", "GOFLAGS=-mod=readonly")
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Start() != nil {
		t.Fatal("offline client start failed")
	}
	waited := false
	defer func() {
		cancel()
		_ = stdin.Close()
		if !waited {
			_ = cmd.Wait()
		}
	}()
	send := func(value any) {
		if json.NewEncoder(stdin).Encode(value) != nil {
			t.Error("fixture host write failed")
		}
	}
	user := func(text string) {
		send(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}})
	}
	send(map[string]any{"type": "control_request", "request_id": "fixture_init", "request": map[string]any{"subtype": "initialize", "hooks": map[string]any{}}})
	user(prompts[0])
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	turns := 0
	for scanner.Scan() {
		var message map[string]any
		if json.Unmarshal(scanner.Bytes(), &message) != nil {
			t.Error("fixture client output invalid")
			break
		}
		switch message["type"] {
		case "control_request":
			request, _ := message["request"].(map[string]any)
			allow := false
			if request["subtype"] == "can_use_tool" {
				for _, call := range calls {
					if request["tool_name"] == call.Name && repairInputMatches(request["input"], call.Input) {
						allow = true
					}
				}
			}
			response := map[string]any{"behavior": "deny", "message": "Not an approved fixture operation."}
			if allow {
				response = map[string]any{"behavior": "allow", "updatedInput": request["input"]}
			}
			send(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": message["request_id"], "response": response}})
		case "result":
			turns++
			if message["is_error"] == true {
				t.Error("fixture client turn failed")
				_ = stdin.Close()
			} else if turns < len(prompts) {
				user(prompts[turns])
			} else {
				_ = stdin.Close()
			}
		}
	}
	if scanner.Err() != nil {
		t.Error("fixture client output incomplete")
	}
	err = cmd.Wait()
	waited = true
	if err != nil || turns != len(prompts) {
		t.Errorf("offline client success=%t turns=%d, want success and %d turns", err == nil, turns, len(prompts))
	}
}
func repairInputMatches(value any, want map[string]any) bool {
	got, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for key, value := range want {
		if !bytes.Equal(bridge.Canonical(got[key]), bridge.Canonical(value)) {
			return false
		}
	}
	for key, value := range got {
		if _, ok := want[key]; ok {
			continue
		}
		if (key == "replace_all" || key == "run_in_background" || key == "dangerouslyDisableSandbox") && value == false {
			continue
		}
		return false
	}
	return true
}

// covers: AC-12. Process exit status is characterized before the runner relies
// on the client's is_error bit. Both cases use the production wire adapter.
func TestRepairClientTestResultEncoding(t *testing.T) {
	binary := repairClientBinary(t)
	for _, passes := range []bool{false, true} {
		name := "fail"
		if passes {
			name = "pass"
		}
		t.Run(name, func(t *testing.T) {
			repo, plan := codingFixture(t)
			if passes {
				writeCodingFile(t, repo, "clamp.go", strings.Replace(plan.Files["clamp.go"], "if value < lower { return upper }", "if value < lower { return lower }", 1))
			}
			call := bridge.Block{Type: "tool_use", ID: "fixture_test", Name: "Bash", Input: map[string]any{"command": codingTestCommand, "description": "Test disposable fixture"}}
			home, _ := probeHome(t)
			var requests atomic.Int64
			adapter := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				if requests.Add(1) == 1 {
					_, _ = w.Write(toolFrame(call))
				} else {
					_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"Fixture result received."}`))
				}
			})
			var matched atomic.Bool
			generator := repairGenerateFunc(func(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
				for _, m := range r.Messages {
					for _, b := range m.Content {
						if b.Type == "tool_result" && b.ToolUseID == call.ID && b.IsError == !passes {
							matched.Store(true)
						}
					}
				}
				return adapter.Generate(ctx, r, emit)
			})
			handler := gateway.NewExperimentalHandler("synthetic-bearer", "offline", slog.New(slog.NewTextHandler(io.Discard, nil)), generator)
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)
			runRepairClient(t, binary, repo, server.URL, []bridge.Block{call}, []string{"Run the disposable fixture tests."})
			if !matched.Load() || requests.Load() != 2 {
				t.Errorf("client result passing=%t matched=%t requests=%d, want matching is_error and two requests", passes, matched.Load(), requests.Load())
			}
			t.Logf("suite_passes=%t matching_client_error_flag=%t", passes, matched.Load())
		})
	}
}

type repairGenerateFunc func(context.Context, bridge.Request, func(bridge.Event) error) (bridge.End, error)

func (f repairGenerateFunc) Generate(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
	return f(ctx, r, emit)
}
func toolFrame(call bridge.Block) []byte {
	return probeFrame("toolUseEvent", string(bridge.Canonical(map[string]any{"toolUseId": call.ID, "name": call.Name, "input": string(bridge.Canonical(call.Input)), "stop": true})))
}

// covers: AC-12. Real pinned client, authenticated handler, production adapter,
// synthetic SQLite credentials and TLS, two edits, two tests and two end turns.
func TestRepairClientCodingLoop(t *testing.T) {
	binary := repairClientBinary(t)
	repo, plan := codingFixture(t)
	calls := codingCalls(repo, plan)
	home, _ := probeHome(t)
	var requests atomic.Int64
	adapter := adapterFixture(t, home, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		n := int(requests.Add(1))
		switch n {
		case 1, 2, 3, 4:
			_, _ = w.Write(toolFrame(calls[n-1]))
		case 6, 7:
			_, _ = w.Write(toolFrame(calls[n-2]))
		default:
			_, _ = w.Write(probeFrame("assistantResponseEvent", `{"content":"Fixture turn complete."}`))
		}
	})
	control := newRunControl(t.Context(), time.Now().Add(90*time.Second))
	t.Cleanup(func() { control.cancel(context.Canceled) })
	tracking := newCodingEvidence(adapter, control, repo, plan.InitialPrompt, plan.FollowupPrompt)
	handler := gateway.NewObservedExperimentalHandler("synthetic-bearer", "offline", slog.New(slog.NewTextHandler(io.Discard, nil)), tracking, tracking)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	runRepairClient(t, binary, repo, server.URL, calls, []string{plan.InitialPrompt, plan.FollowupPrompt})
	if !tracking.complete() {
		t.Errorf("coding evidence complete=false cause=%s, want two completed turns", control.cause)
	}
	if _, err := fixtureTest(t.Context(), repo, "./..."); err != nil {
		t.Error("final fixture suite failed")
	}
	if !verifyBoundaryRegression(t.Context(), repo) {
		t.Error("boundary regression incomplete")
	}
	if requests.Load() != 8 {
		t.Errorf("offline requests=%d, want 8", requests.Load())
	}
	t.Logf("coding_loop_complete=%t synthetic_dispatches=%d live_dispatches=0", tracking.complete(), requests.Load())
}
