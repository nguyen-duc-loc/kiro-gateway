# Review, feat/first-claude-code-loop, 2026-10-02

**Reviewed by**: GPT-5.6 Sol (author on GPT-6/Codex)
**Scope**: 28 feasibility milestone files, branch versus `8d692a4`
**Verdict**: Approve
**Initial verdict**: Changes requested

## Summary

The milestone adds the combined credential/profile snapshot reader and a bounded, opt-in feasibility harness for six protocol cases. The initial review found one violation of the experiment's inactivity bound and one narrower preflight cancellation edge. The October 2 follow-up verified both fixes and their regression coverage without finding a new blocker, major, minor, or nit; the current verdict is Approve.

## Major

### 🟠 The stream idle budget does not cover DNS or dialing, `internal/kiro/probe_offline_test.go:295`

**Status**: Resolved in the October 2 follow-up.

**Problem**: The attempt gets only the two-minute request context before calling `p.dial` or `net.Dialer`. The 30-second idle deadline is first installed by `probeIdleConn` after dialing succeeds, even though the accepted contract starts that budget at dispatch and resets it only when response bytes arrive. A scratch test with a 50 ms idle limit, a 400 ms request limit, and a synthetic dial blocked on `ctx.Done()` returned after about 430 ms, demonstrating that the request deadline wins while no bytes are received.

**Why it matters**: Slow or stuck DNS and connect work can hold an attempt for up to two minutes, four times the approved inactivity limit. This breaks AC-3's bounded-run guarantee and delays automatic termination of a failed live destination path.

**Suggested fix**: Apply the initial idle deadline to resolution and dialing as well as socket reads, while preserving the existing reset on each received byte. Add deterministic stalled-resolver and stalled-dial tests that prove the idle limit wins over the request limit before a connection exists.

## Minor

### 🟡 A descendant holding stdout can outlive the preflight context, `internal/kiro/probe_preflight_test.go:135`

**Status**: Resolved in the October 2 follow-up.

**Problem**: `realProbeCommand` uses `exec.CommandContext` with a custom stdout writer but does not set `Cmd.WaitDelay` or check `ctx.Err()` after `Run`. If the direct command exits while a descendant retains the stdout pipe, `Run` waits for that pipe after the context deadline and may eventually return `nil`. A 40 ms context around `sh -c 'sleep 0.6 & exit 0'` took about 614 ms and returned a nil command error with `context deadline exceeded` already set.

**Why it matters**: A changed or malfunctioning Git or version wrapper can make the live preflight ignore cancellation and overrun its intended bound before account access. If the last version command emits the expected text, the expired preflight context is also not rechecked before the harness continues.

**Suggested fix**: Give the command a finite `WaitDelay`, treat context expiry or `exec.ErrWaitDelay` as the existing fixed preflight failure category, and add a regression test with a descendant-held stdout pipe. Ensure any process cleanup stays bounded and does not expose raw command errors.

## Follow-up, 2026-10-02

The remediation was reviewed against `8eb9b1ffd766693ff9d673053a8fc081fd2bb1b3`. The per-attempt dial path now derives a fresh context from the original request context with one absolute `start + idle` deadline. DNS and connect share that deadline, a shorter request deadline still wins, and the existing connection wrapper continues to renew the idle deadline only after received bytes. Wrapped `context.DeadlineExceeded` is reduced to the fixed `timed_out` category.

The preflight command now uses a 100 ms `Cmd.WaitDelay`, discards output on any command or context failure, and rejects an expired context after the final matching version output. This bounds the caller and its pipe cleanup; it does not claim to terminate every descendant process.

Focused race tests passed for stalled resolution, stalled dialing, the shorter request deadline, stream progress beyond the initial idle window, descendant-held stdout, a valid short command, and cancellation after matching baseline output. The independent reproductions changed from approximately 400 ms to 52 ms for a 50 ms idle bound and from approximately 614 ms with a nil error to 107 ms with `code_changed` for descendant-held stdout. The full check script passed formatting, vet, build, and all race tests; the independent six-case TLS sequence still returned `limited_candidate_observed` with unchanged settings and no synthetic data leakage. No live request or real credential access was used for this follow-up.

## Strengths

- The combined SQLite read validates the fixed token and selected profile in one transaction, checks the saved fingerprint before profile use, and pins exact profile bytes in memory between attempts.
- The live path freezes a finite region-to-host map, rejects private DNS results, verifies TLS for the original host, disables proxies, redirects, connection reuse, HTTP/2, and replay, and consumes the attempt before transport dispatch.
- EventStream lengths and CRCs, JSON ambiguity, tool identity and complete arguments, metadata exceptions, reasoning disposal, output labels, and tentative completion are all constrained by focused synthetic tests.
- The result keeps the accepted experimental limits visible: instructions remain in user content, authoritative completion and output-token compliance remain unknown, and the best verdict is only `limited_candidate_observed`.

## Test coverage

Configured Go tests cover the credential/profile transaction, profile drift, regional routing, six-case TLS flow, request examples, tool continuation, malformed framing, cancellation, cutoff injection, diagnostic filtering, live-gate ordering, pre-connection idle deadlines, shorter request deadline precedence, stream deadline renewal, and bounded preflight pipe cleanup. The supplied independent ledger records 203 harness test/subtest passes, 68 profile passes, a separate six-case scratch flow, zero-dispatch source drift, and repository checks; the approved six-case remote evidence is historical, and this review did not run live inference. The remediation's focused regressions, full check script, original scratch reproductions, six-case TLS flow, and gate-zero tagged entry check all passed.
