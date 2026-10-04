//go:build clientbridge

package gateway

import (
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
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kiro-gateway/internal/bridge"
)

var clientBridge = flag.Bool("client-bridge", false, "explicitly exercise the installed client using only a dummy local adapter")

type clientScript struct {
	calls     atomic.Int64
	result    atomic.Bool
	path      string
	wantError bool
}

func (s *clientScript) Generate(ctx context.Context, r bridge.Request, emit func(bridge.Event) error) (bridge.End, error) {
	n := s.calls.Add(1)
	if n == 1 {
		if err := emit(bridge.Event{Text: "I will read the fixture."}); err != nil {
			return bridge.End{}, err
		}
		if err := emit(bridge.Event{Tool: &bridge.Block{Type: "tool_use", ID: "offline_read_1", Name: "Read", Input: map[string]any{"file_path": s.path}}}); err != nil {
			return bridge.End{}, err
		}
	} else {
		for _, m := range r.Messages {
			for _, b := range m.Content {
				if b.Type == "tool_result" && b.ToolUseID == "offline_read_1" && b.IsError == s.wantError {
					s.result.Store(true)
				}
			}
		}
		if err := emit(bridge.Event{Text: "The fixture was read."}); err != nil {
			return bridge.End{}, err
		}
	}
	return bridge.End{Basis: bridge.InferredCleanEOF}, nil
}

func TestInstalledClientOffline(t *testing.T) {
	installedClientReadOffline(t, false)
}

// covers: AC-3, AC-4, AC-9. A missing fixture file returns a matching error result.
func TestInstalledClientReadErrorOffline(t *testing.T) {
	installedClientReadOffline(t, true)
}

func installedClientReadOffline(t *testing.T, missing bool) {
	t.Helper()
	if !*clientBridge {
		t.Skip("requires explicit -client-bridge flag")
	}
	binary, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal("installed Claude Code is unavailable")
	}
	version, err := exec.Command(binary, "--version").Output()
	if err != nil || !strings.HasPrefix(string(version), "2.1.289 ") {
		t.Fatal("installed client does not match version 2.1.289")
	}
	artifact, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal("could not digest client")
	}
	digest := sha256.Sum256(artifact)
	dir := t.TempDir()
	repo := filepath.Join(dir, "fixture")
	cfg := filepath.Join(dir, "client")
	for _, d := range []string{repo, cfg} {
		if err := os.Mkdir(d, 0700); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(repo, "value.go")
	if err := os.WriteFile(file, []byte("package fixture\nfunc Value() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if missing {
		file = filepath.Join(repo, "absent.go")
	}
	script := &clientScript{path: file, wantError: missing}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	token := strings.Repeat("offline-token-", 4)
	handler := NewExperimentalHandler(token, "offline", logger, script)
	var attempts, invalid atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		if r.URL.Path == "/v1/messages" {
			b, _ := io.ReadAll(io.LimitReader(r.Body, bridge.MaxBody+1))
			r.Body = io.NopCloser(bytes.NewReader(b))
			_, parseErr := bridge.Parse(b, false)
			t.Logf("messages headers_valid=%t body_valid=%t bearer_matches=%t", validateHeaders(r) == nil, parseErr == nil, r.Header.Get("Authorization") == "Bearer "+token)
			if parseErr != nil {
				offlineShape(t, b)
			}
		} else {
			t.Log("non Messages request observed")
		}
		rec := &offlineStatusWriter{ResponseWriter: w, status: http.StatusOK}
		handler.ServeHTTP(rec, r)
		if rec.status >= http.StatusBadRequest && r.URL.Path == "/v1/messages" {
			invalid.Add(1)
			t.Logf("client response status=%d", rec.status)
		}
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	args := []string{"-p", "--no-session-persistence", "--model", bridge.Model, "--effort", "high", "--safe-mode", "--tools", "Read,Edit,Bash", "--permission-mode", "manual", "--prompt-suggestions", "false", "Read value.go and explain its return value."}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = repo
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Env = offlineClientEnv(srv.URL, token, cfg)
	err = cmd.Run()
	t.Logf("client_version=2.1.289 client_sha256=%s requests=%d validated=%d result_matched=%t invalid=%d client_success=%t kiro_dispatches=0", hex.EncodeToString(digest[:]), attempts.Load(), script.calls.Load(), script.result.Load(), invalid.Load(), err == nil)
	if err != nil || script.calls.Load() != 2 || !script.result.Load() || invalid.Load() != 0 {
		t.Error("installed client read exchange did not complete")
	}
}

func offlineShape(t *testing.T, b []byte) {
	t.Helper()
	var root map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if d.Decode(&root) != nil {
		return
	}
	known := map[string]bool{}
	for _, k := range []string{"model", "messages", "system", "tools", "metadata", "max_tokens", "output_config", "stream", "thinking", "tool_choice", "stop_sequences"} {
		known[k] = true
		if v, ok := root[k]; ok {
			t.Logf("root %s shape=%T", k, v)
		}
	}
	n := 0
	for k := range root {
		if !known[k] {
			n++
		}
	}
	t.Logf("unknown_root_fields=%d", n)
	if ts, ok := root["tools"].([]any); ok {
		for _, v := range ts {
			if m, ok := v.(map[string]any); ok {
				n := 0
				for k := range m {
					if k != "name" && k != "description" && k != "input_schema" && k != "cache_control" {
						n++
					}
				}
				t.Logf("tool unknown_fields=%d", n)
			}
		}
	}
	if ms, ok := root["messages"].([]any); ok {
		for _, v := range ms {
			if m, ok := v.(map[string]any); ok {
				n := 0
				for k := range m {
					if k != "role" && k != "content" {
						n++
					}
				}
				t.Logf("message user=%t assistant=%t system=%t developer=%t content_shape=%T unknown_fields=%d", m["role"] == "user", m["role"] == "assistant", m["role"] == "system", m["role"] == "developer", m["content"], n)
				if bs, ok := m["content"].([]any); ok {
					for _, v := range bs {
						if b, ok := v.(map[string]any); ok {
							n := 0
							for k := range b {
								if k != "type" && k != "text" && k != "cache_control" && k != "id" && k != "name" && k != "input" && k != "tool_use_id" && k != "content" && k != "is_error" {
									n++
								}
							}
							t.Logf("block text=%t tool_use=%t tool_result=%t unknown_fields=%d", b["type"] == "text", b["type"] == "tool_use", b["type"] == "tool_result", n)
						}
					}
				}
			}
		}
		// This is a diagnostic counterfactual only. It is never forwarded to a
		// handler or adapter and does not define a supported transformation.
		withoutSystem := make([]any, 0, len(ms))
		removed := 0
		for _, v := range ms {
			if m, ok := v.(map[string]any); ok && m["role"] == "system" {
				removed++
				continue
			}
			withoutSystem = append(withoutSystem, v)
		}
		root["messages"] = withoutSystem
		_, err := bridge.Parse(bridge.Canonical(root), false)
		t.Logf("diagnostic_only removed_system_messages=%d remainder_valid=%t dispatched=false", removed, err == nil)
	}
}

type offlineStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *offlineStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *offlineStatusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func offlineClientEnv(url, token, cfg string) []string {
	var env []string
	for _, k := range []string{"HOME", "PATH", "TMPDIR", "USER", "LOGNAME", "SHELL"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	for k, v := range map[string]string{
		"CLAUDE_CONFIG_DIR": cfg, "ANTHROPIC_BASE_URL": url, "ANTHROPIC_AUTH_TOKEN": token,
		"ANTHROPIC_MODEL": bridge.Model, "ANTHROPIC_DEFAULT_OPUS_MODEL": bridge.Model, "ANTHROPIC_DEFAULT_SONNET_MODEL": bridge.Model, "ANTHROPIC_DEFAULT_HAIKU_MODEL": bridge.Model,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "CLAUDE_CODE_DISABLE_TERMINAL_TITLE": "1", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1", "CLAUDE_CODE_DISABLE_THINKING": "1", "CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK": "1", "DISABLE_PROMPT_CACHING": "1", "CLAUDE_CODE_MAX_OUTPUT_TOKENS": "4096", "CLAUDE_CODE_MAX_RETRIES": "0",
	} {
		env = append(env, k+"="+v)
	}
	return env
}
