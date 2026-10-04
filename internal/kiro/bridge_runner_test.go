package kiro

import "os"
import "kiro-gateway/internal/bridge"

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
