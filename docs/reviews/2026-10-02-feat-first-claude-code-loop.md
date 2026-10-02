# Review, feat/first-claude-code-loop, 2026-10-02

**Reviewed by**: GPT-5.6 Sol (author on GPT-6/Codex)
**Scope**: 28 feasibility milestone files, branch versus `8d692a4`
**Verdict**: Changes requested

## Summary

The milestone adds the combined credential/profile snapshot reader and a bounded, opt-in feasibility harness for six protocol cases. The source selection, destination restrictions, framing checks, observation filtering, and limitation reporting are unusually thorough, and the approved prior live record is consistent with the reviewed code. One timing defect violates the experiment's explicit 30-second inactivity bound before a connection is established; I also found a narrower preflight cancellation edge involving descendant-held command pipes.

## Major

### 🟠 The stream idle budget does not cover DNS or dialing, `internal/kiro/probe_offline_test.go:295`

**Problem**: The attempt gets only the two-minute request context before calling `p.dial` or `net.Dialer`. The 30-second idle deadline is first installed by `probeIdleConn` after dialing succeeds, even though the accepted contract starts that budget at dispatch and resets it only when response bytes arrive. A scratch test with a 50 ms idle limit, a 400 ms request limit, and a synthetic dial blocked on `ctx.Done()` returned after about 430 ms, demonstrating that the request deadline wins while no bytes are received.

**Why it matters**: Slow or stuck DNS and connect work can hold an attempt for up to two minutes, four times the approved inactivity limit. This breaks AC-3's bounded-run guarantee and delays automatic termination of a failed live destination path.

**Suggested fix**: Apply the initial idle deadline to resolution and dialing as well as socket reads, while preserving the existing reset on each received byte. Add deterministic stalled-resolver and stalled-dial tests that prove the idle limit wins over the request limit before a connection exists.

## Minor

### 🟡 A descendant holding stdout can outlive the preflight context, `internal/kiro/probe_preflight_test.go:135`

**Problem**: `realProbeCommand` uses `exec.CommandContext` with a custom stdout writer but does not set `Cmd.WaitDelay` or check `ctx.Err()` after `Run`. If the direct command exits while a descendant retains the stdout pipe, `Run` waits for that pipe after the context deadline and may eventually return `nil`. A 40 ms context around `sh -c 'sleep 0.6 & exit 0'` took about 614 ms and returned a nil command error with `context deadline exceeded` already set.

**Why it matters**: A changed or malfunctioning Git or version wrapper can make the live preflight ignore cancellation and overrun its intended bound before account access. If the last version command emits the expected text, the expired preflight context is also not rechecked before the harness continues.

**Suggested fix**: Give the command a finite `WaitDelay`, treat context expiry or `exec.ErrWaitDelay` as the existing fixed preflight failure category, and add a regression test with a descendant-held stdout pipe. Ensure any process cleanup stays bounded and does not expose raw command errors.

## Strengths

- The combined SQLite read validates the fixed token and selected profile in one transaction, checks the saved fingerprint before profile use, and pins exact profile bytes in memory between attempts.
- The live path freezes a finite region-to-host map, rejects private DNS results, verifies TLS for the original host, disables proxies, redirects, connection reuse, HTTP/2, and replay, and consumes the attempt before transport dispatch.
- EventStream lengths and CRCs, JSON ambiguity, tool identity and complete arguments, metadata exceptions, reasoning disposal, output labels, and tentative completion are all constrained by focused synthetic tests.
- The result keeps the accepted experimental limits visible: instructions remain in user content, authoritative completion and output-token compliance remain unknown, and the best verdict is only `limited_candidate_observed`.

## Test coverage

Configured Go tests cover the credential/profile transaction, profile drift, regional routing, six-case TLS flow, request examples, tool continuation, malformed framing, cancellation, cutoff injection, diagnostic filtering, and live-gate ordering. The supplied independent ledger records 203 harness test/subtest passes, 68 profile passes, a separate six-case scratch flow, zero-dispatch source drift, and repository checks; the approved six-case remote evidence is historical, and this review did not run live inference. The two timing paths above lack repository regression tests; targeted scratch reproductions confirmed both defects.
