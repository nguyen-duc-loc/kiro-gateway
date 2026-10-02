//go:build livebridge

package kiro

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kiro-gateway/internal/bridge"
	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/credentials"
	"kiro-gateway/internal/gateway"
)

var liveBridge = flag.Bool("live-bridge", false, "launch only the separately reviewed bounded live coding proof")
var liveBridgePlan = flag.String("bridge-plan", "internal/kiro/testdata/bridge-plan.json", "reviewed plan path")
var liveBridgePlanHash = flag.String("bridge-plan-sha256", "", "reviewed plan digest required for launch")

type bridgePlan struct {
	ClientVersion   string            `json:"client_version"`
	ClientSHA       string            `json:"client_sha256"`
	KiroVersion     string            `json:"kiro_version"`
	KiroSHA         string            `json:"kiro_sha256"`
	CodeCommit      string            `json:"code_commit"`
	Files           map[string]string `json:"files"`
	InitialPrompt   string            `json:"initial_prompt"`
	FollowupPrompt  string            `json:"followup_prompt"`
	CancelPrompt    string            `json:"cancel_prompt"`
	InterruptPrompt string            `json:"interrupt_prompt"`
}

// TestLiveBridge is excluded from ordinary checks and exits before setup unless
// explicitly launched. Run its compiled test binary in a terminal for manual
// Claude Code permissions. All account access uses the production adapter.
func TestLiveBridge(t *testing.T) {
	if !*liveBridge {
		t.Skip("requires reviewed plan digest and explicit -live-bridge")
	}
	started := time.Now()
	absolute := started.Add(20 * time.Minute)
	cutoff := absolute.Add(-5 * time.Second)
	work, stop := context.WithDeadlineCause(t.Context(), cutoff, &bridge.Failure{Status: 504, Type: "api_error", Message: "Live run budget exhausted.", Category: "budget_exhausted"})
	defer stop()
	control := newRunControl(work, cutoff)
	defer control.cancel(context.Canceled)
	verdict := "needs_evidence"
	defer func() {
		control.mu.Lock()
		defer control.mu.Unlock()
		t.Logf("verdict=%s dispatches=%d cause=%s cleanup_within_deadline=%t", verdict, control.attempts, control.cause, time.Now().Before(absolute))
	}()
	fail := func(category string) { control.fail(category); t.Error(category) }
	planBytes, err := os.ReadFile(*liveBridgePlan)
	if err != nil {
		fail("plan_unavailable")
		return
	}
	sum := sha256.Sum256(planBytes)
	if hex.EncodeToString(sum[:]) != *liveBridgePlanHash {
		fail("plan_digest_mismatch")
		return
	}
	var plan bridgePlan
	if json.Unmarshal(planBytes, &plan) != nil || plan.ClientVersion != "2.1.287" || plan.KiroVersion != "2.8.0" || len(plan.CodeCommit) != 40 {
		fail("plan_invalid")
		return
	}
	// Artifact and clean code checks run inside the same work budget.
	client, err := exec.LookPath("claude")
	if err != nil {
		fail("client_unavailable")
		return
	}
	kiro, err := exec.LookPath("kiro-cli")
	if err != nil {
		fail("kiro_unavailable")
		return
	}
	for _, artifact := range []struct{ path, digest, version string }{{client, plan.ClientSHA, "2.1.287 "}, {kiro, plan.KiroSHA, "kiro-cli 2.8.0"}} {
		data, err := os.ReadFile(artifact.path)
		if err != nil {
			fail("artifact_unavailable")
			return
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != artifact.digest {
			fail("artifact_changed")
			return
		}
		v, err := exec.CommandContext(control.ctx, "rtk", "proxy", artifact.path, "--version").Output()
		if err != nil || !strings.HasPrefix(string(v), artifact.version) {
			fail("version_changed")
			return
		}
	}
	status, err := exec.CommandContext(control.ctx, "rtk", "proxy", "git", "status", "--porcelain").Output()
	if err != nil || len(status) != 0 {
		fail("checkout_not_clean")
		return
	}
	if exec.CommandContext(control.ctx, "rtk", "proxy", "git", "merge-base", "--is-ancestor", plan.CodeCommit, "HEAD").Run() != nil {
		fail("code_commit_changed")
		return
	}
	if exec.CommandContext(control.ctx, "rtk", "proxy", "git", "diff", "--quiet", plan.CodeCommit, "--", "cmd", "internal", "go.mod", "go.sum", ":(exclude)internal/kiro/testdata/bridge-plan.json").Run() != nil {
		fail("code_changed_since_review")
		return
	}
	dir, err := os.MkdirTemp("", "kiro-bridge-live-")
	if err != nil {
		fail("fixture_setup")
		return
	}
	defer os.RemoveAll(dir)
	if os.Chmod(dir, 0700) != nil {
		fail("fixture_setup")
		return
	}
	repo, cfg := filepath.Join(dir, "fixture"), filepath.Join(dir, "client")
	for _, p := range []string{repo, cfg} {
		if os.Mkdir(p, 0700) != nil {
			fail("fixture_setup")
			return
		}
	}
	for name, body := range plan.Files {
		if name != "go.mod" && name != "clamp.go" && name != "clamp_test.go" {
			fail("fixture_plan_invalid")
			return
		}
		if os.WriteFile(filepath.Join(repo, name), []byte(body), 0600) != nil {
			fail("fixture_setup")
			return
		}
	}
	if len(plan.Files) != 3 {
		fail("fixture_plan_invalid")
		return
	}
	check := func() bool {
		cmd := exec.CommandContext(control.ctx, "rtk", "proxy", "go", "test", "./...")
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run() == nil
	}
	if check() {
		fail("fixture_did_not_fail_initially")
		return
	}
	if control.ctx.Err() != nil {
		fail("budget_exhausted")
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fail("home_unavailable")
		return
	}
	store, err := configstore.OpenExisting(home)
	if err != nil {
		fail("configuration_unavailable")
		return
	}
	defer store.Close()
	document, _, err := store.Load()
	if err != nil {
		fail("configuration_invalid")
		return
	}
	adapter, err := New(credentials.Reader{Home: home}, document)
	if err != nil {
		fail("configuration_invalid")
		return
	}
	adapter.responseReader = func(r io.Reader) io.Reader { return &cutAfterText{r: r, control: control, left: 13} }
	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		fail("randomness")
		return
	}
	bearer := hex.EncodeToString(tokenBytes[:])
	tracking := &codingEvidence{generator: adapter}
	handler := gateway.NewObservedExperimentalHandler(bearer, "live-proof", slog.New(slog.NewTextHandler(io.Discard, nil)), tracking, control)
	server := httptest.NewUnstartedServer(handler)
	server.Config.BaseContext = func(net.Listener) context.Context { return control.ctx }
	server.Config.ReadHeaderTimeout = 5 * time.Second
	server.Config.ReadTimeout = 5 * time.Second
	server.Config.IdleTimeout = 60 * time.Second
	server.Config.MaxHeaderBytes = 16 << 10
	server.Start()
	defer func() { stop(); server.CloseClientConnections(); _ = server.Config.Close() }()
	// The absolute watchdog never grants a fresh cleanup budget.
	watchdogDone := make(chan struct{})
	watchdog := time.AfterFunc(time.Until(absolute), func() {
		defer close(watchdogDone)
		stop()
		server.CloseClientConnections()
		_ = server.Config.Close()
	})
	defer func() {
		if !watchdog.Stop() {
			<-watchdogDone
		}
	}()
	args := []string{"--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", plan.InitialPrompt}
	cmd := exec.CommandContext(control.ctx, client, args...)
	cmd.Dir = repo
	cmd.Env = liveClientEnv(server.URL, bearer, cfg)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// This is interactive client output, never copied into the evidence record.
	t.Log("In Claude Code, review each Read, Edit, and Bash permission. After the first answer, enter the followup prompt from the reviewed plan. After the second answer, exit Claude Code.")
	if runInteractiveClient(cmd) != nil {
		fail("client_incomplete")
		return
	}
	if !tracking.complete() || !check() {
		fail("coding_assertion_failed")
		return
	}
	tests, err := os.ReadFile(filepath.Join(repo, "clamp_test.go"))
	if err != nil || string(tests) == plan.Files["clamp_test.go"] {
		fail("boundary_test_not_added")
		return
	}
	parsed, parseErr := parser.ParseFile(token.NewFileSet(), "clamp_test.go", tests, 0)
	boundary := false
	if parseErr == nil {
		for _, decl := range parsed.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "TestClampUpperBoundary" {
				boundary = true
			}
		}
	}
	if !boundary {
		fail("boundary_test_not_added")
		return
	}
	for _, fault := range []struct{ name, category, prompt string }{{"cancel", "canceled", plan.CancelPrompt}, {"interrupt", "incomplete_stream", plan.InterruptPrompt}} {
		if !control.arm(fault.name, fault.category) {
			fail("fault_setup")
			return
		}
		ctx, cancel := context.WithCancel(control.ctx)
		raw := bridge.Canonical(map[string]any{"model": bridge.Model, "max_tokens": 4096, "stream": true, "messages": []any{map[string]any{"role": "user", "content": fault.prompt}}})
		req, err := http.NewRequestWithContext(ctx, "POST", server.URL+"/v1/messages", strings.NewReader(string(raw)))
		if err != nil {
			cancel()
			fail("fault_setup")
			return
		}
		req.Header.Set("Authorization", "Bearer "+bearer)
		req.Header.Set("Anthropic-Version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")
		local := &http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, Timeout: 2 * time.Minute}
		response, err := local.Do(req)
		if err != nil {
			cancel()
			fail("fault_request_failed")
			return
		}
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 4096), 4<<20)
		visible, terminal, streamError := false, false, false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: message_stop") {
				terminal = true
			}
			if strings.HasPrefix(line, "event: error") {
				streamError = true
			}
			if strings.HasPrefix(line, "data: ") {
				var data struct{ Delta struct{ Type, Text string } }
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data) == nil && data.Delta.Type == "text_delta" && data.Delta.Text != "" {
					visible = true
					if fault.name == "cancel" {
						cancel()
						break
					}
				}
			}
		}
		_ = response.Body.Close()
		cancel()
		local.CloseIdleConnections()
		// Await the handler's synchronous failure and cleanup observation.
		cleanupDeadline := time.Now().Add(5 * time.Second)
		if absolute.Before(cleanupDeadline) {
			cleanupDeadline = absolute
		}
		for time.Now().Before(cleanupDeadline) {
			control.mu.Lock()
			done := control.cleaned || control.stopped
			control.mu.Unlock()
			if done {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !visible || terminal || fault.name == "interrupt" && !streamError || !control.advance() {
			fail("fault_assertion_failed")
			return
		}
	}
	control.mu.Lock()
	healthy := !control.stopped && control.attempts <= 20
	control.mu.Unlock()
	if !healthy || !time.Now().Before(cutoff) {
		fail("run_incomplete")
		return
	}
	// Stop new work, cancel all children, and synchronously close owned serving.
	stop()
	server.CloseClientConnections()
	if server.Config.Close() != nil || !time.Now().Before(absolute) {
		fail("cleanup_failed")
		return
	}
	verdict = "experimental_loop_observed"
}

type codingEvidence struct {
	mu        sync.Mutex
	generator bridge.Generator
	calls     map[string]string
	results   map[string]bool
	userTurns int
}

func (c *codingEvidence) Generate(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
	c.mu.Lock()
	if c.calls == nil {
		c.calls = map[string]string{}
		c.results = map[string]bool{}
	}
	last := bridge.Message{}
	for _, m := range r.Messages {
		if m.Role == "user" {
			last = m
		}
		for _, b := range m.Content {
			if b.Type == "tool_result" && !b.IsError {
				if name := c.calls[b.ToolUseID]; name != "" {
					c.results[name] = true
				}
			}
		}
	}
	resultTurn := false
	for _, b := range last.Content {
		if b.Type == "tool_result" {
			resultTurn = true
		}
	}
	if !resultTurn {
		c.userTurns++
	}
	c.mu.Unlock()
	return c.generator.Generate(ctx, r, func(e bridge.Event) error {
		if e.Tool != nil {
			c.mu.Lock()
			c.calls[e.Tool.ID] = e.Tool.Name
			c.mu.Unlock()
		}
		return emit(e)
	})
}
func (c *codingEvidence) complete() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.userTurns >= 2 && c.results["Read"] && c.results["Edit"] && c.results["Bash"]
}

type cutAfterText struct {
	r       io.Reader
	control *runControl
	left    int
}

func (r *cutAfterText) Read(b []byte) (int, error) {
	r.control.mu.Lock()
	armed := r.control.caseID == "interrupt" && r.control.textSeen
	r.control.mu.Unlock()
	if armed {
		if r.left == 0 {
			return 0, io.ErrUnexpectedEOF
		}
		if len(b) > r.left {
			b = b[:r.left]
		}
	}
	n, err := r.r.Read(b)
	if armed {
		r.left -= n
	}
	return n, err
}
func liveClientEnv(url, token, cfg string) []string {
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

func TestLiveBridgeCutoffHelper(t *testing.T) {
	c := newRunControl(t.Context(), time.Now().Add(time.Minute))
	defer c.cancel(context.Canceled)
	if !c.arm("interrupt", "incomplete_stream") {
		t.Fatal("arm failed")
	}
	if err := c.Before(bridge.Observation{RequestID: "req_fixture", Phase: "admission"}); err != nil {
		t.Fatal(err)
	}
	// This exact synthetic frame boundary fixes the live trigger: 13 bytes of
	// the next frame after a validated nonempty text event.
	text := probeFrame("assistantResponseEvent", `{"content":"visible"}`)
	second := probeFrame("assistantResponseEvent", `{"content":"withheld"}`)
	reader := &cutAfterText{r: strings.NewReader(string(append(text, second...))), control: c, left: 13}
	observed := 0
	err := readFrames(reader, func(string, []byte) error {
		observed++
		c.Observe(bridge.Observation{RequestID: "req_fixture", Phase: "text", Category: "success"})
		return nil
	})
	if err == nil || observed != 1 || reader.left != 0 {
		t.Errorf("cutoff err=%v events=%d remaining=%d, want one validated event and incomplete frame", err, observed, reader.left)
	}
}
