# Verify: First loop protocol feasibility, spec 0003

All cases are pending. This file is a verification plan, not a record of passing tests. The full Claude Code coding loop remains a later gate in the same scope feature.

## Local checks before live review

| Scenario | Expected evidence | Criteria |
|---|---|---|
| Normal tests with inherited live controls, absent live gate, invalid gate, or mismatched plan digest | Live entry point excluded without its build tag; otherwise skip or reject before any real credential read or external network access | AC-1, AC-8 |
| Plan missing destination, auth source, schema, framing, completion assertion, or generated value source | Explicit stopped state; no guessed default and no credential read | AC-1, AC-7 |
| Wrong code commit, dirty or untracked inputs, or baseline drift | Refusal before credential access; no claim that an environment value proves human approval | AC-1, AC-8 |
| Human workflow lacks approval for this exact launch | Operator or agent does not invoke the live harness; reviewed plan, commit, model, destinations, versions, and one run authorization are recorded in the conversation | AC-1 |
| Missing configuration or mapping, different target ID, occupied process lock | Refusal without changing settings or calling the service | AC-2, AC-8 |
| Synthetic snapshot changes, expires, is busy, or has unsafe metadata | Same snapshot validation and token selection; no source fallback or refresh | AC-2, AC-8 |
| Source changes after its fingerprint is checked | The token used belongs to the checked snapshot; no second token read | AC-2 |
| Plan or saved mapping is manually edited after the run's startup snapshot | All attempts use the original verified plan and mapping; edits are ignored for this run | AC-1, AC-2 |
| Saved session reference is manually changed while the credential source also changes | The active run still compares against its frozen reference and rejects the new source | AC-2 |
| Redirect, unknown region, environment proxy, unapproved host, or additional profile requirement | Refusal without forwarding a credential | AC-1, AC-2, AC-8 |
| Service error, connection failure, or an apparently retryable result | Exactly one counted attempt and no transport or application replay | AC-3, AC-7 |
| Sixth attempted dispatch followed by another proposed dispatch | Seventh dispatch refused, including after failed responses | AC-3 |
| Slow headers, idle stream, byte trickle, expired request deadline, expired run deadline | All respective deadlines terminate work without extending the total budget | AC-3, AC-8 |
| Cancellation during source read, dial, frame read, or blocked consumer | Context cancellation and cleanup complete within five seconds | AC-3, AC-5, AC-8 |
| Case 5 deliberately cancels its child request while the parent remains active | Expected assertions pass, cleanup finishes, then case 6 may dispatch | AC-3, AC-5 |
| Operator cancels the parent during or after case 5 | No further dispatch; `needs_evidence` unless a prior protocol contradiction already establishes rejection | AC-3, AC-5, AC-7 |
| Case 6 reaches its planned cutoff, or completes before that cutoff | First is an observed incomplete response assertion; second is inconclusive and makes the run `needs_evidence` | AC-5, AC-7 |
| Required assertion fails, required evidence is absent, or only optional metadata is absent | Exact verdict table applied; absent model echo or usage alone does not fail the run | AC-4, AC-5, AC-7 |
| Split synthetic tool arguments, escaped strings, missing IDs, mismatched results | Exact reassembly and matching, or explicit rejected case with no corrective model call | AC-4, AC-8 |
| EOF before a frame completes, after text, or after a tool becomes visible | Incomplete result without successful terminal state or replay | AC-5, AC-8 |
| Oversize body, header, event length, or accumulated history | Rejected before excessive allocation; no success from truncated data | AC-3, AC-5, AC-8 |
| Sentinel tokens, fingerprint, account values, upstream errors, and real conversation text in every input position | None appears in stdout, stderr, saved summaries, or fixtures | AC-6, AC-8 |
| Secret sentinel appears as an event name, JSON member name, tool ID, model string, or terminal label | Emit only reviewed labels, fixed unknown labels, local counts, and assertion results; no raw spelling or value | AC-6, AC-8 |
| Run end with unknown model identity, usage, terminal semantics, or instruction precedence | Unknown remains explicit; unsupported claims are absent | AC-4, AC-5, AC-7 |
| Existing repository checks | Health, configuration, source capture, and native builds still pass | AC-8 |

## Required live plan review

Before a live invocation, you review the committed `probe-plan.json`, its digest, clean code commit, exact destination and regional rule, authentication source, synthetic bodies, stream decoder assertions, output labels, model mapping, attempt sequence, resource limits, and offline results. Approval in the current conversation covers one explicitly started bounded run. A second launch needs fresh approval. A changed plan, model mapping, code, destination set, or baseline needs review again. Record approval and results in the spec after the run so the reviewed checkout remains clean for launch.

The harness verifies the explicit launch controls, clean commit, frozen plan digest, mapping equality, and baseline. It cannot infer human consent from these values or enforce a single approval across separate processes. Check that boundary through the review workflow rather than inventing a machine `plan_unreviewed` state.

This spec contains no approved live plan. Do not substitute the candidate strings in its rationale for one.

## Live cases

| Attempt | Case | Record |
|---|---|---|
| 1 | Incremental text and instruction observation | Framing, field placement, control behavior, model evidence, terminal evidence |
| 2 | Synthetic tool request | Name, exact ID, schema, complete arguments, event ordering |
| 3 | Synthetic result continuation | Result matching, preserved history, continued model response |
| 4 | Follow up user input | Conversation continuity and instruction handling |
| 5 | Deliberate request cancellation | Reached child cancellation point, local resource closure, elapsed cleanup time, parent still active |
| 6 | Controlled stream interruption | Reached cutoff point, expected incomplete outcome, no replay; otherwise an inconclusive case |

Stop at a missing required contract or unexpected failure. Mark later cases `unrun`. The expected case 5 cancellation and case 6 cutoff are successful assertions when their triggers and cleanup are observed; they are not unexpected run failures. A local truncation test is required even if the corresponding live cutoff cannot be reached, and does not turn that inconclusive live case into a pass. No case executes model supplied file or shell operations.

## Evidence record

For each run, record only its local ID, plan digest, date, checked clean code commit, platform, validated client versions, exact requested model from the plan, whether a matching model identity was evidenced, public service destination from the plan, access method, case outcomes, elapsed times, named instruction or control assertions, and unresolved contracts. Structural observations are restricted to the exact allowed output model in `index.md`. Do not include account identifiers, selected credential metadata, fingerprints, tokens, arbitrary upstream strings or field names, raw traffic, or actual prompt and response bodies.

Reviewed synthetic fixtures may be referenced by case label. A fixture is written from invented content and allowed schema facts; it is not an automatically redacted recording. `candidate_supported` requires every mandatory assertion in all six live cases to be observed. An observed contradiction yields `candidate_rejected`; otherwise any missing required evidence yields `needs_evidence`. Optional model echo and usage observations limit claims without changing a passing verdict. No verdict means that Claude Code compatibility was verified.

## Later Claude Code acceptance gate

After `/architect` completes the bridge design, the original scope still needs a real Claude Code `2.1.285` session against the gateway using the recorded available Sonnet model. Claude Code must read and fix a disposable Go bug, execute its file and shell tools under normal permissions, return results, finish a follow up user turn, and demonstrate cancellation and incomplete stream handling. Record Kiro CLI `2.8.0`, the actual access path, and every instruction or model control difference. None of that is claimed by this feasibility checklist.
