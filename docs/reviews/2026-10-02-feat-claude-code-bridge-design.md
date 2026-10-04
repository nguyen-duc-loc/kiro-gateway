# Review, feat/claude-code-bridge-design, 2026-10-02

**Reviewed by**: GPT-6 Sol (author on GPT-6 Astra)
**Scope**: 12 files, uncommitted changes against HEAD `4edd00256e45536a31741e9bc2ceddaaa2b52878`
**Verdict**: Approve with nits

## Summary

This review covers the current cleanup changes and their tests, not the complete experimental branch. The transport now assigns connection closing to one worker and bounds its cleanup wait. The adapter disables later inference after failed cleanup, and the gateway preserves that failure through cancellation and response writes. I found one test assertion that should better support the offline retry comparison.

## Minor

### 🟡 Require the absent setting control to show a retry, `internal/gateway/client_retry_offline_test.go:80`

**Problem**: The absent `CLAUDE_CODE_MAX_RETRIES` cases pass with one attempt because the assertion requires only `attempts.Load() >= 1`. The code logs whether a second attempt occurred, but does not assert it for the 429 and 502 cases that are expected to retry.

**Why it matters**: A client behavior change that makes the absent setting behave like zero would leave this comparison green. The test would no longer demonstrate that the zero setting is responsible for stopping those retries.

**Suggested fix**: For the 429 and 502 absent setting cases, require the observed second attempt and explain the expected cancellation after it. Keep the interrupted stream case tied to its observed behavior.

## Strengths

- The single transport worker owns connection closing, and its timeout produces a fixed failure that disables subsequent inference before another source read.
- The tests exercise real TLS and HTTP over synthetic connections, compare JSON and SSE content, and check that cancellation never produces a successful terminal event.
- The gateway records `cleanup_failed` with unsuccessful cleanup before releasing admission, including when the original error was a canceled request or failed client write.

## Test coverage

I applied the check review rubric and the Go security, Go concurrency, and Go testing skill guidance. I read the governing bridge spec and the 12 changed files. I ran `rtk proxy go test -race ./internal/bridge ./internal/gateway ./internal/kiro`; all three packages passed from cache. The main verification evidence also reports a passing `scripts/check` run, both explicit installed client offline suites, and the scratch executable checks. This review did not run live inference or access operator credentials. The full experimental feature review remains open because this assessment covers only the 12 current files.
