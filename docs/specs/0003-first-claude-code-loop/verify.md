# Verify: First loop protocol feasibility, spec 0003

No live inference case has dispatched a request. This file contains the verification plan and the launch records below, including the local source failure; it does not claim independent verification passed. The full Claude Code coding loop remains a later gate in the same scope feature.

The accepted profile amendment adds AC-9 and the profile cases below. Its local implementation is complete. Builder checks cover synthetic profile and wire cases; independent verification and live compatibility remain pending. The limited experiment amendment permits explicit user context instructions and tentative EOF continuation only.

## Local checks before live review

| Scenario | Expected evidence | Criteria |
|---|---|---|
| Normal tests with inherited live controls, absent live gate, invalid gate, or mismatched plan digest | Live entry point excluded without its build tag; otherwise skip or reject before any real credential read or external network access | AC-1, AC-8 |
| Plan missing destination, auth source, schema, framing, explicit tentative completion policy, or generated value source | Explicit stopped state; no guessed default and no credential read | AC-1, AC-7 |
| Wrong code commit, dirty or untracked inputs, or baseline drift | Refusal before credential access; no claim that an environment value proves human approval | AC-1, AC-8 |
| Human workflow lacks approval for this exact launch | Operator or agent does not invoke the live harness; reviewed plan, commit, model, destinations, versions, and one run authorization are recorded in the conversation | AC-1 |
| Missing configuration or mapping, different target ID, occupied process lock | Refusal without changing settings or calling the service | AC-2, AC-8 |
| Synthetic snapshot changes, expires, is busy, or has unsafe metadata | Same snapshot validation and token selection; no source fallback or refresh | AC-2, AC-8 |
| Source changes after its fingerprint is checked | The token used belongs to the checked snapshot; no second token read | AC-2 |
| Plan or saved mapping is manually edited after the run's startup snapshot | All attempts use the original verified plan and mapping; edits are ignored for this run | AC-1, AC-2 |
| Saved session reference is manually changed while the credential source also changes | The active run still compares against its frozen reference and rejects the new source | AC-2 |
| Redirect, unknown profile region, environment proxy, unapproved host, or a requirement for any further account source | Refusal without forwarding a credential | AC-1, AC-2, AC-8, AC-9 |
| Service error, connection failure, or an apparently retryable result | Exactly one counted attempt and no transport or application replay | AC-3, AC-7 |
| Sixth attempted dispatch followed by another proposed dispatch | Seventh dispatch refused, including after failed responses | AC-3 |
| Slow headers, idle stream, byte trickle, expired request deadline, expired run deadline | All respective deadlines terminate work without extending the total budget | AC-3, AC-8 |
| Cancellation during source read, dial, frame read, or blocked consumer | Context cancellation and cleanup complete within five seconds | AC-3, AC-5, AC-8 |
| Case 5 deliberately cancels its child request while the parent remains active | Expected assertions pass, cleanup finishes, then case 6 may dispatch | AC-3, AC-5 |
| Operator cancels the parent during or after case 5 | No further dispatch; `needs_evidence` unless a prior protocol contradiction already establishes rejection | AC-3, AC-5, AC-7 |
| Case 6 reaches its planned cutoff, or completes before that cutoff | First is an observed incomplete response assertion; second is inconclusive and makes the run `needs_evidence` | AC-5, AC-7 |
| Required assertion fails, required evidence is absent, or only optional metadata is absent | Exact verdict table applied; absent model echo or usage alone does not fail the run | AC-4, AC-5, AC-7 |
| Split synthetic tool arguments, escaped strings, missing IDs, mismatched results | Exact reassembly and matching, or explicit rejected case with no corrective model call | AC-4, AC-8 |
| EOF inside a frame or with incomplete tool arguments | Incomplete result without tentative completion or replay | AC-5, AC-8 |
| Oversize body, header, event length, or accumulated history | Rejected before excessive allocation; no success from truncated data | AC-3, AC-5, AC-8 |
| Sentinel tokens, fingerprint, account values, upstream errors, and real conversation text in every input position | None appears in stdout, stderr, saved summaries, or fixtures | AC-6, AC-8 |
| Secret sentinel appears as an event name, JSON member name, tool ID, model string, or terminal label | Emit only reviewed labels, fixed unknown labels, local counts, and assertion results; no raw spelling or value | AC-6, AC-8 |
| Run end with unknown model identity, usage, terminal semantics, or instruction precedence | Unknown remains explicit; unsupported claims are absent | AC-4, AC-5, AC-7 |
| Existing repository checks | Health, configuration, source capture, and native builds still pass | AC-8 |

## Selected profile checks

| Scenario | Expected evidence | Criteria |
|---|---|---|
| Synthetic `auth_kv` token and `state` profile with the two exact keys | One connection and one read transaction return the pinned token and exact `arn` member; no other rows or settings are read | AC-2, AC-9 |
| Profile value column is declared BLOB but stores bounded text JSON | Accept the same fixed record; actual BLOB storage, generated columns, and token schema changes still fail | AC-2, AC-8, AC-9 |
| Token region differs from profile ARN region | The profile's fourth ARN component selects the matching synthetic plan destination; token region and start URL do not affect routing | AC-9 |
| Either supported profile region | Its corresponding finite plan entry is selected; absent entry or a host outside that plan stops before dispatch | AC-1, AC-9 |
| Missing, duplicate, nontext, oversize, invalid UTF 8, ambiguous JSON, absent or wrong case `arn` | Fixed `profile_invalid` failure, no value in output, and size checked before allocation | AC-6, AC-8, AC-9 |
| Exactly one string valued `profileName` or `profile_name`, including an empty string | Both spellings work individually; the decoded name is discarded and cannot affect routing or identity | AC-6, AC-9 |
| ARN alone, neither name alias, both aliases even with equal values, null or nonstring name, or only a case variant | `profile_invalid` before dispatch; no alias precedence or fallback is invented | AC-8, AC-9 |
| State table is a view, has generated key/value columns, wrong declared types, or lacks the sole key primary key | `source_unavailable` before reading record values; additive ordinary columns remain allowed | AC-8, AC-9 |
| Unsupported ARN partition, service, region, account syntax, or resource syntax | `profile_unsupported` without guessing a host, trimming, or falling back to token region | AC-9 |
| Profile changes between attempts with token unchanged, including only whitespace or ignored name data | Exact byte digest mismatch produces `profile_changed`; the original destination is not replaced and no slot is consumed | AC-2, AC-9 |
| Token changes while profile remains unchanged | Existing `session_changed` behavior and saved fingerprint formula remain intact | AC-2, AC-9 |
| A writer changes one or both rows between metadata and value reads in rollback journal and WAL fixtures | The reader observes one consistent SQLite snapshot or a bounded busy failure; no mixed transaction reads and no repair | AC-2, AC-8, AC-9 |
| Source read is busy or canceled while obtaining the second record | Both reads share the five second deadline and one second busy budget; all resources close and no dispatch occurs | AC-3, AC-8, AC-9 |
| Profile ARN, name, account component, or profile digest contains synthetic sentinel data | No values in summary, fixed errors, formatting, JSON output, or saved configuration; only approved public destination labels may be emitted | AC-6, AC-9 |
| New approved run with a different selected profile but unchanged token | First valid combined read establishes a new in memory pin; no hidden persistent pin, forced relink, or settings mutation | AC-2, AC-9 |
| Ordinary `account link`, `ReadSnapshot`, health, or tests with inherited live controls | Original source boundary remains unchanged; no real profile read or network dispatch | AC-1, AC-8, AC-9 |
| Profile snapshot is valid but the concrete limited plan is incomplete or differs from implemented semantics | Reject before dispatch; a passing local profile test cannot enable an unspecified contract | AC-1, AC-7, AC-9 |

## Limited experiment checks

| Case | Expected result | Criteria |
|---|---|---|
| Clean EOF after all required text or tool observations | Tentative completion only; `observed_completion` remains null and distinct system role preservation is false | AC-4, AC-5, AC-6 |
| Valid text followed by an error frame, unknown field, or bad CRC | Stop with no tentative completion or automatic replay | AC-5, AC-7, AC-8 |
| Six limited cases observed | `limited_candidate_observed`, never `candidate_supported`; full coding loop remains pending | AC-4, AC-7 |
| Changed request examples, destination, instruction policy, controls, or completion policy | Plan validation fails even if its launch digest is recomputed | AC-1, AC-8 |
| Output usage exceeds 1024, is malformed, or is absent | Above limit is contradicted, malformed is rejected, absent remains unknown; raw counts are never emitted | AC-4, AC-6 |
| DNS returns local or reserved addresses, or the selected public address fails | Reject local destinations; one connection attempt without address fallback | AC-3, AC-8 |

A clean transport end after dropping complete frames can remain indistinguishable from a complete response. Verify that the evidence states this limitation rather than claiming the harness detects it.

## Required live plan review

Before a live invocation, you review the committed `probe-plan.json`, its digest, clean code commit, exact destination and regional rule, authentication source, synthetic bodies, stream decoder assertions, output labels, model mapping, attempt sequence, resource limits, and offline results. Approval in the current conversation covers one explicitly started bounded run. A second launch needs fresh approval. A changed plan, model mapping, code, destination set, or baseline needs review again. Record approval and results in the spec after the run so the reviewed checkout remains clean for launch.

The harness verifies the explicit launch controls, clean commit, frozen plan digest, mapping equality, and baseline. It cannot infer human consent from these values or enforce a single approval across separate processes. Check that boundary through the review workflow rather than inventing a machine `plan_unreviewed` state.

This spec contains no approved live plan. Do not substitute the candidate strings in its rationale for one.

For a plan covering multiple profile regions, review every exact permitted destination and the rule selecting one from the saved profile ARN. Approval selects the profile present at the first attempt and freezes it only within that run. No actual ARN is included in the plan, printed for review, or persisted. This does not provide approval bound to an exact account across launches. A change after the first read stops the run.

## Live cases

| Attempt | Case | Record |
|---|---|---|
| 1 | Incremental text and instruction observation | Framing, explicit user content transformation, control placement, optional usage comparison, model evidence, tentative completion |
| 2 | Synthetic tool request | Name, exact ID, schema, complete arguments, event ordering |
| 3 | Synthetic result continuation | Result matching, preserved history, continued model response |
| 4 | Follow up user input | Conversation continuity and instruction handling |
| 5 | Deliberate request cancellation | Reached child cancellation point, local resource closure, elapsed cleanup time, parent still active |
| 6 | Controlled stream interruption | Reached cutoff point, expected incomplete outcome, no replay; otherwise an inconclusive case |

Stop at a missing required contract or unexpected failure. Mark later cases `unrun`. The expected case 5 cancellation and case 6 cutoff are successful assertions when their triggers and cleanup are observed; they are not unexpected run failures. A local truncation test is required even if the corresponding live cutoff cannot be reached, and does not turn that inconclusive live case into a pass. No case executes model supplied file or shell operations.

## Evidence record

For each run, record only its local ID, plan digest, date, checked clean code commit, platform, validated client versions, exact requested model from the plan, whether a matching model identity was evidenced, public service destination from the plan, access method, case outcomes, elapsed times, named instruction or control assertions, and unresolved contracts. Structural observations are restricted to the exact allowed output model in `index.md`. Do not include account identifiers, selected credential metadata, fingerprints, tokens, arbitrary upstream strings or field names, raw traffic, or actual prompt and response bodies.

Reviewed synthetic fixtures may be referenced by case label. A fixture is written from invented content and allowed schema facts; it is not an automatically redacted recording. `limited_candidate_observed` requires every mandatory limited assertion in all six cases to be observed. It never establishes a distinct system role or proven model completion; `observed_completion` stays null. An observed contradiction yields `candidate_rejected`; otherwise any missing required evidence yields `needs_evidence`. Optional model echo and usage observations limit claims without changing a passing verdict. No verdict means that Claude Code compatibility was verified.

## Approved launch stopped in preflight, October 1, 2026

You approved and explicitly launched one experiment at clean commit `3253ffcfca9733e84a2963e0a20f68ce60796aa2`, with plan SHA 256 `26070279796930d28c27718b5b56994780b565b8cb0e4fae73e159ab0282fbf9`. The reviewed model was `claude-sonnet-5`, with at most six requests over ten minutes to the regional destination selected from the saved profile.

The harness passed the code and plan checks, then reported `baseline_changed` at `probe_live_test.go:51`. The test failed after 0.08 seconds. It stopped before run ID creation, opening configuration, acquiring the account snapshot, selecting a destination, or dispatching a request. All six cases are `unrun`; inference attempts are zero. No protocol verdict or run summary was emitted.

Separate fixed version diagnostics then returned Claude Code `2.1.286`, Kiro CLI `2.8.0`, and Go `go1.27.1`. The reviewed Claude Code baseline was `2.1.285`. This identifies the mismatch without reading an account or querying a service. It supplies no evidence against the candidate inference protocol.

The replacement candidate updates only the Claude Code baseline to `2.1.286`; request bodies, destinations, model, limitations, and budgets remain unchanged. The original approval has been used for its one launch. The new plan digest and clean commit must be reviewed before any further launch, including after this zero request preflight stop.

## Replacement launch stopped at configuration, October 1, 2026

You approved and explicitly launched one replacement experiment at clean commit `40ea75a17a129e5789d464d5816e5bba469d7e1d`, with plan SHA 256 `be490b8a73271fb6af757bcbdc448ad53374529a1c1b82f22184ae15647f7278`. It retained the same model, destination set, synthetic requests, limitations, and six request budget, using Claude Code `2.1.286`.

The harness passed the code, plan, and baseline checks, then reported `configuration_invalid` at `probe_live_test.go:78`. The test failed after 0.17 seconds. It stopped in `openProbeState`, before reading the Kiro token or selected profile and before selecting a network destination or dispatching a request. All six cases are `unrun`; inference attempts are zero. No protocol verdict or run summary was emitted.

The existing read only `config check` command then reported `No saved configuration; defaults are valid.` This identifies the missing prerequisite without opening Kiro's credential store. The experiment requires an existing linked configuration and the exact saved mapping, so it correctly did not initialize settings itself.

### Proposed prerequisite setup

The next proposed action is local setup followed by one separately approved bounded run. Create the existing versioned gateway configuration, capture the current fixed IAM Identity Center token reference through `account link`, and save exactly `models["claude-sonnet-5"] = "claude-sonnet-5"` while holding the stable configuration lock. Keep token bytes in Kiro's store; save only the existing session reference and fingerprint. Preserve the default listener, validate the resulting configuration, and release the lock before launch. No sign in, renewal, profile discovery, or model listing is part of this setup.

This setup is outside the inference harness and requires explicit approval because the two launch approvals covered use of an existing selection, not creating one. It does not change the plan bytes or the inference request budget. Once approved, setup failure stops before inference. Any subsequent launch must use its freshly reviewed clean commit and the unchanged plan digest above. Neither of the completed launch approvals is reused automatically.

## Setup completed and profile source rejected, October 1, 2026

You approved local initialization, linking the current fixed Kiro CLI IAM Identity Center session, saving the exact Sonnet mapping, and one subsequent bounded run. `config init` and `account link` completed successfully. A temporary local helper used the existing configuration store's exclusive lock and atomic save to set `models["claude-sonnet-5"] = "claude-sonnet-5"`, preserving the saved reference. It reloaded and validated the result without printing account data. The helper was removed before launch; `config check` passed and the reviewed checkout was clean.

| Run metadata | Observed value |
|---|---|
| Reviewed commit | `468cda48fe2497f65841a1319a68238d996dc6da` |
| Plan SHA 256 | `be490b8a73271fb6af757bcbdc448ad53374529a1c1b82f22184ae15647f7278` |
| Local run ID | `8f55a6d3-2869-4840-a6a0-67faabb62272` |
| Started | `2026-10-01T08:32:56.261269Z` |
| Platform and versions | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-sonnet-5` |
| Destination | None selected |
| Attempts and received bytes | Zero attempts, zero response bytes |
| First case | `text`: `inconclusive`, `source_unavailable`, attempt zero, cleanup completed |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Verdict and elapsed time | `needs_evidence`, 1 millisecond in the run summary |
| Peak reservation | 2621440 bytes, a local reservation count rather than process heap usage |

The run stopped while obtaining the combined snapshot, before dispatch. No model acceptance, text, tool, usage, or completion assertion was established. The two approved experiment limitations remain in force. No second run followed this failure.

### Local schema diagnosis and prepared correction

A separate read only diagnostic inspected only `main.state` schema metadata and the fixed selected profile row's storage type and bounded size. It did not load token or profile contents, scan other account records, or contact a service. Fixed booleans established an ordinary table, a plain declared `TEXT` key as the sole primary key, and a plain declared `BLOB` value column. The selected profile exists once, has storage type `text`, and meets the existing size bound.

The reader had required a declared `TEXT` value column and therefore rejected this source before reading profile bytes. A synthetic reproduction with `CREATE TABLE state(key TEXT PRIMARY KEY,value BLOB)` and invented text JSON failed before the correction. The prepared correction accepts declared `BLOB` only for that profile value column. Actual BLOB values, null, oversized text, generated columns, nontext keys, and the token table's BLOB declaration remain rejected. The existing text profile declaration also remains supported.

The correction uses the same record, transaction, size checks, JSON validation, ARN policy, and digest. It does not modify Kiro's database or change the plan bytes. Synthetic regression and full repository checks cover the correction; a real combined read under the corrected code has not been attempted. A new review of the code commit is required before one further live run.

## Later Claude Code acceptance gate

After `/architect` completes the bridge design, the original scope still needs a real Claude Code `2.1.285` session against the gateway using the recorded available Sonnet model. Claude Code must read and fix a disposable Go bug, execute its file and shell tools under normal permissions, return results, finish a follow up user turn, and demonstrate cancellation and incomplete stream handling. Record Kiro CLI `2.8.0`, the actual access path, and every instruction or model control difference. None of that is claimed by this feasibility checklist.
