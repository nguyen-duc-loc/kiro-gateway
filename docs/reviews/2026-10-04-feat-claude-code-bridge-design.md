# Review, feat/claude-code-bridge-design, 2026-10-04

**Reviewed by**: GPT-6 Sol (author on GPT-6 Astra)
**Scope**: 46 files, branch versus main at `816d310b7f33af26f1f7e061ab627edf965bbf52`
**Verdict**: Changes requested

## Summary

The branch adds an authenticated experimental Messages bridge, a bounded Kiro adapter, offline client exercises, and a separately approved live coding proof. The protocol boundaries and failure handling are well covered. Two issues need correction: valid fragmented output can cause excessive copying, and the live runner can award its experimental verdict without proving the required followup test execution or boundary assertion. This review does not dispute the separately recorded human observations from the October 2 run.

## Major

### 🟠 Accumulate streamed text without repeated full copies, `internal/bridge/response.go:35`

**Problem**: Each text event appends to an immutable string, copying all prior output. The 2 MiB text limit and 8 MiB response limit do not bound cumulative copying. For example, 41,943 valid frames with 50 text bytes each occupy about 5.83 MiB in the repository's EventStream fixture format, but this line copies about 43.98 GiB while building the response. The tool input accumulator has the same pattern at `internal/kiro/events.go:164`, within its smaller per tool limit.

**Why it matters**: Both ordinary JSON and SSE pass every text event through this accumulator. One valid fragmented response can consume substantial CPU and allocation capacity while holding the sole inference slot. The two minute context deadline cannot interrupt an individual string copy.

**Suggested fix**: Accumulate text and tool input with a bounded builder or chunk buffer, then materialize each final string once. Add a deterministic fragmented stream case near the resource limits to guard the behavior.

### 🟠 Require the followup test behavior in the live verdict, `internal/kiro/live_bridge_test.go:392`

**Problem**: `codingEvidence.complete` accepts one successful Bash result from the initial turn and any second user turn. The later check runs `go test` from the runner itself, while the AST check at line 249 accepts any function named `TestClampUpperBoundary`. Thus a client that fixes the original bug, runs tests once, and then adds an empty function with that name can satisfy these automated coding checks without running the suite on the followup turn or testing the upper boundary.

**Why it matters**: Spec 0004 requires Claude Code to add a boundary regression test and run tests again on the subsequent turn. The runner can label an incomplete loop `experimental_loop_observed`. The retained October 2 record also cites direct operator observation, so this finding concerns the verdict condition, not a claim that the observed run failed.

**Suggested fix**: Track each user turn and matching Bash result separately, require a second test command result after the followup turn, and verify the new test exercises `Clamp(11, 0, 10)` with the expected result. A check that the test fails against the original upper boundary defect would establish its regression value.

## Minor

### 🟡 Assert the retry comparison's control case, `internal/gateway/client_retry_offline_test.go:80`

**Problem**: The cases without `CLAUDE_CODE_MAX_RETRIES` pass after one request because the assertion only requires at least one attempt. The 429 and 502 cases log the expected second attempt but never require it.

**Why it matters**: If the installed client stops retrying in the absent setting case, this test remains green while no longer showing that the zero setting changed retry behavior.

**Suggested fix**: Require the observed second attempt in the absent setting cases for 429 and 502. Keep the interrupted stream expectation tied to its separately observed behavior.

## Strengths

- The parser rejects ambiguous input before account access, and the adapter uses one fresh combined credential snapshot for each admitted inference.
- Complete tool calls are withheld until framing, semantic validation, and transport cleanup succeed. The tests cover cancellation, source changes, response modes, and synthetic TLS behavior.
- The live runner has an explicit launch gate, attempt budget, deadline, and fixed diagnostic categories. Its recorded plan and result do not retain conversation or credential values.

## Test coverage

The main agent ran `rtk proxy ./scripts/check`; formatting, vet, build, and race tests passed after loopback access was approved for the sandbox. I inspected the governing spec, changed production files, selected tests, and retained synthetic evidence. A read only arithmetic check using the repository's frame format established the fragmentation example above. The 43.98 GiB figure is cumulative bytes copied by the append pattern, not a measured memory peak or elapsed time; I did not run a runtime benchmark. I did not run installed client exercises, access the operator account, or launch live inference. Existing tests exercise many failure paths, but no test measures fragmented accumulation cost or requires a Bash result from the followup turn.

## Repair review, 2026-10-04

**Reviewed by**: GPT-6 Sol (author on GPT-6 Astra)
**Scope**: 28 files, repair commits and working tree versus `f3335a6e3f9bb6387b5b186e0087d6831e602b23`
**Verdict**: Approve

### Summary

The repair bounds fragmented accumulation, ties the coding verdict to two completed client turns and a meaningful boundary test, and requires the absent retry controls to show their expected second attempt. I found no remaining blocker, major, or minor in this repair slice. This approval closes the three findings above for the experimental bridge; it does not establish the separate GA protocol gates.

### Closure of earlier findings

- **Fragmented accumulation, closed.** `internal/bridge/response.go:30` and `internal/kiro/events.go:225` check remaining capacity before appending to request owned builders. Final text is materialized once. The fixed allocation tests reject the old copying pattern, and the larger adapter tests cover exact content, limits, both response modes, cancellation, and incomplete tools.
- **Coding verdict, closed.** `internal/kiro/coding_evidence_test.go:78` requires a successful terminal write with the matching request ID and stop reason. The runner matches distinct test commands and returned results to each turn, checks boundary test timing, and requires the final test to fail against the isolated upper branch mutation. The prior runner check based only on the test function name has been replaced.
- **Retry control, closed.** `internal/gateway/client_retry_offline_test.go:80` now requires two attempts without the retry setting for 429 and 502, and one with zero. Interrupted SSE retains its independent one attempt expectation.

### Strengths

- Final write errors, short writes, cancellation, and failed SSE flushes keep the coding verdict incomplete.
- The offline coding exercise runs the pinned client through the authenticated handler, production adapter, synthetic SQLite source, and local TLS service. The mutation check leaves the original fixture untouched.

### Test coverage

The main agent ran `rtk proxy ./scripts/check` with approved loopback access. Formatting, vet, build, and all ordinary race tests passed. I inspected the retained `2.1.289` offline result and its focused tests, but did not independently launch the installed client, access real credentials, or run live inference. The earlier live result remains evidence for `2.1.287`; interactive permission behavior and a live account loop on `2.1.289` remain unverified. The four GA protocol gates remain open by design.
