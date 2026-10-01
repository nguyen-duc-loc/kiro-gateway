# Verify: First loop protocol feasibility, spec 0003

The approved run at `4fd0b70773ca6bdba87d73be1ac6c07ab8d77659` observed all six live cases and returned `limited_candidate_observed` on October 1, 2026. The launch records below retain both the successful bounded proof and its preceding investigations. This is builder evidence; it does not claim independent GA verification or full Claude Code compatibility.

The final candidate uses the fixed combined token and profile snapshot, the reference request baseline, bounded metadata extension handling, and reasoning discard only in independent failure cases. Instructions remain translated into user context and EOF completion remains tentative. Model identity and token usage are unverified, and no upstream token cap is requested.

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

## Corrected reader run, October 1, 2026

You approved the corrected reader and one run at clean commit `3323e609a9404f27549944b3b8965b43f10991d8`, with unchanged plan SHA 256 `be490b8a73271fb6af757bcbdc448ad53374529a1c1b82f22184ae15647f7278`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `44871b03-d69e-455e-ad62-e3ff6289e440` |
| Started | `2026-10-01T08:46:46.141392Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-sonnet-5` |
| Selected destination | `https://runtime.us-east-1.kiro.dev:443/` |
| Attempts | One dispatch attempt |
| First case | `text`: `inconclusive`, cause `needs_evidence`, attempt one, cleanup completed |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Response observations | Zero counted stream bytes, text events, tool events, unknown events, and unknown fields |
| Local request assertions | Instruction placement and controls requested true; these describe the constructed request, not service acceptance |
| Protocol assertions | Marker match, model match, usage, and completion remain unknown |
| Verdict and elapsed time | `needs_evidence`, 1162 milliseconds |
| Peak reservation | 2621440 bytes |

The combined snapshot passed and the approved regional destination was selected. One attempt was counted immediately before dispatch. The original summary does not distinguish DNS, connection, TLS, or HTTP rejection at this point. Zero counted stream bytes does not prove the service sent no response body: non 200 bodies are not decoded or counted. There is no retained status or raw error from which to recover the missing distinction, and no claim that authentication or model acceptance succeeded. No second request or automatic replay occurred.

### Prepared diagnostic followup

The next candidate adds the finite diagnostic labels in the spec. Synthetic HTTP responses prove that status categories remain distinct from transport failures and that private header and body sentinels are never emitted. Synthetic DNS and connection failures, rejected private destinations, and an untrusted local TLS certificate exercise the transport labels. Successful case behavior and all stop conditions remain unchanged. No extra real account read or network experiment was used to test this change.

The diagnostic plan has a new digest because its allowed output policy changes. It requires a new exact code and plan review before a launch. The completed run above remains inconclusive; additional diagnostics cannot retroactively identify its failure.

## Diagnostic run stopped on expiry, October 1, 2026

You approved one diagnostic launch at clean commit `b2490a92125e147b53a2eb6615ce17d952c61eea`, with plan SHA 256 `137a0c1d0999e5700cec06a998d860ba151d844eaecd6ec27d4a7cf9efe0496b`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `ec074214-3df6-40b6-a1f7-2554470aefb0` |
| Started | `2026-10-01T08:59:57.380217Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-sonnet-5` |
| Destination | None selected |
| Attempts and received bytes | Zero attempts, zero response bytes |
| First case | `text`: `inconclusive`, failure stage `source`, cause `credential_expired`, attempt zero, cleanup completed |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Protocol and request assertions | Unknown; no inference request was constructed or dispatched |
| Verdict and elapsed time | `needs_evidence`, 1 millisecond |
| Peak reservation | 2621440 bytes |

The source reader rejected the expired credential before destination selection and dispatch. No renewal, sign in, fallback, relink, corrective request, or second launch followed. This result does not identify the earlier dispatch failure or establish model compatibility. No expiry timestamp, token, fingerprint, account value, or raw source error was retained.

The operator must renew the IAM Identity Center session through the normal Kiro CLI flow before further live work. The gateway must then explicitly link the current snapshot and restore the exact Sonnet mapping, because changed credential bytes produce a new reference and relinking clears mappings. This remediation is separate from inference; the harness does not perform it automatically. A further launch still needs its own reviewed clean commit and plan digest after the session is ready.

## Renewed session diagnostic run returned forbidden, October 1, 2026

You reported a completed IAM Identity Center sign in. The gateway explicitly relinked the renewed fixed token snapshot and restored `claude-sonnet-5` to the same exact target under the configuration lock. Configuration validation passed. Claude Code and Kiro CLI still matched the diagnostic baseline. No source fingerprint, token, or account metadata was printed or retained in this record.

You then approved one run at clean commit `b07b0d6add6dcf85c74d66357853d00271696b35`, with plan SHA 256 `137a0c1d0999e5700cec06a998d860ba151d844eaecd6ec27d4a7cf9efe0496b`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `9b1ca2ad-2e28-425f-9199-802a6edc896c` |
| Started | `2026-10-01T09:10:41.388901Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-sonnet-5` |
| Selected destination | `https://runtime.us-east-1.kiro.dev:443/` |
| Attempts | One dispatch attempt |
| First case | `text`: `inconclusive`, cause `needs_evidence`, failure stage `http_status`, HTTP status category `forbidden`, attempt one |
| Transport diagnostic | Absent; the HTTP client returned a response rather than a transport error |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Response observations | Zero counted stream bytes and zero decoded events; the error response body was not read |
| Local request assertions | Instruction placement and controls requested true; no assertion of service acceptance |
| Model, usage, and completion | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 902 milliseconds |
| Peak reservation | 2621440 bytes |

The fixed `forbidden` category maps only to HTTP 403. The combined source read, local expiry check, profile selection, and TLS verified request path passed far enough to receive that response. This establishes a service denial for this candidate request, not successful authentication, model access, or inference compatibility. The response could originate from the service or its front end; the retained observations do not identify which authorization check failed.

No error body, raw headers, upstream request ID, or raw error text was retained. No retry, second request, model substitution, endpoint switch, renewal, or automatic relink followed. The prior dispatch attempt without these diagnostics cannot be assumed to have failed for the same reason.

### Feasibility disposition

The bounded feasibility build has produced an explicit unresolved outcome. The candidate is not supported, and a 403 alone does not contradict the instruction or tool protocol assertions, so the verdict remains `needs_evidence`, not `candidate_rejected`. Full Claude Code compatibility remains unproven. The next investigation should establish why the selected token, profile, model, and operation were denied before proposing another inference run. Repeating the same request without new evidence is not recommended. Independent GA verification and review remain pending.

## Prepared error discriminator observation

The authorization research and static addresses are recorded in `rationale.md`. No live request or account read was made during that investigation. It did not prove a cause for the recorded 403 and did not justify changing the outbound operation or authentication headers.

The next local candidate retains the same six requests, model, destinations, source rules, and stop conditions. It adds finite `service_error` and `error_response_format` observations from the protocol's named error discriminator, preferring the header and permitting a bounded JSON inspection only if the header is absent. Raw values, namespaces, suffixes, messages, and bodies remain unprinted and unpersisted. Unknown, duplicate, conflicting, malformed, oversized, or unreadable inputs yield fixed labels and never a success assertion or replay.

Synthetic tests cover header precedence without a body read, decorated type names, unknown private sentinels, duplicate headers and JSON keys, conflicting discriminators, malformed values, non JSON responses, oversized bodies, read failures, and HTTP 403 with private message and account fields. The run remains inconclusive after one rejected request. All new data comes from invented fixtures. This candidate awaits review of its changed diagnostic policy before live use.

## Bounded error classifier run returned access denied, October 1, 2026

You approved one run at clean commit `3511f2a4aac847f8582ff49cfc88139de7600ea6`, with plan SHA 256 `1b53928cc89c8216a4e90ff1aff28144895806aa082c96e4723492e86ae0affb`, including the bounded error discriminator read.

| Run metadata | Observed value |
|---|---|
| Local run ID | `a37d86e9-4020-470e-b42a-036487fad8ff` |
| Started | `2026-10-01T09:40:49.131469Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-sonnet-5` |
| Selected destination | `https://runtime.us-east-1.kiro.dev:443/` |
| Attempts | One dispatch attempt |
| First case | `text`: `inconclusive`, cause `needs_evidence`, failure stage `http_status`, HTTP status category `forbidden` |
| Error observation | `service_error: access_denied`, `error_response_format: json` |
| Received bytes | 119 error response bytes inspected in memory; zero decoded text or tool events |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Local request assertions | Instruction placement and controls requested true; these do not establish service acceptance |
| Protocol assertions | Marker match, tool behavior, model identity, usage, and completion remain unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 983 milliseconds |
| Peak reservation | 3735552 bytes |

The error classifier recognized an allowlisted access denial discriminator in the bounded JSON response. The summary retains the class only and does not distinguish the two mapped access denial spellings. No raw body, discriminator decoration, message, header, request ID, or account value was printed or saved. The byte count supplies no evidence about the discarded message and must not be used to guess it.

The result confirms an access denial response for the candidate request. It does not establish whether the failure concerns the token, profile, account entitlement, model, client restrictions, or another service rule. It does not retroactively classify earlier attempts. The run stopped without retry, fallback, or a second request, and does not establish inference compatibility.

### Next evidence needed

Do not repeat this unchanged request solely to obtain the same class again. A controlled comparison with the official client under the same account, selected profile, and model could help distinguish account access from a candidate request mismatch. Such a comparison would need its own concrete review: the official client may refresh credentials, read additional state, send telemetry, or retry, all outside this harness's current boundary. No official client inference, renewal, profile discovery, entitlement query, or further gateway run is authorized by this completed launch.

## Native model clarification and proposed comparison

The operator reported successful native use, supplied a session reference, and then clarified that the test used Opus 5.5. A bounded read of only `session.json` confirmed the recognized `modelId` value `claude-opus-5.5`. The separate conversation and history files were not read. No session identifier, title, workspace path, conversation, or account metadata is retained here.

The earlier probe used `claude-sonnet-5`, so these are different model observations. Opus success does not establish Sonnet availability or prove a cause for its denial. The next prepared candidate pins the confirmed Opus identifier while retaining the exact request protocol and limits. Explicit relinking of the current fixed snapshot, the exact Opus mapping addition, and one new run require review of the changed model, code, and plan. Relinking must retain the existing rule that changed token bytes clear old mappings. No Opus gateway request has occurred. Controls remain explicit hypotheses for Opus; unsupported controls stop rather than being dropped.

## Approved Opus comparison returned access denied, October 1, 2026

You approved explicit relinking, saving the exact `claude-opus-5.5` mapping, and one bounded run at commit `d1fdd1a05ad7b6b9a112ed47ff756b14fa960bbe`, with plan SHA 256 `2eefec0fec9e8bd600bfac2f3867538049a64f05e51f960c64d2fa26771f5600`.

The existing `account link` command captured the current fixed IAM Identity Center snapshot and reported that mappings were empty. A temporary local helper saved and reloaded the exact Opus mapping under the existing lock, preserving the new reference. It was removed before launch. `config check` passed, and the code commit and plan digest matched the approved clean checkpoint. No sign in or automatic renewal occurred during setup.

| Run metadata | Observed value |
|---|---|
| Local run ID | `376971aa-a8de-4e68-8d7d-49cf4bd0edd8` |
| Started | `2026-10-01T11:35:16.00959Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Selected destination | `https://runtime.us-east-1.kiro.dev:443/` |
| Attempts | One dispatch attempt |
| First case | `text`: `inconclusive`, cause `needs_evidence`, failure stage `http_status`, HTTP status category `forbidden` |
| Error observation | `service_error: access_denied`, `error_response_format: json` |
| Received bytes | 119 error response bytes inspected in memory; zero decoded text or tool events |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Local request assertions | Instruction placement and controls requested true; no service acceptance claim |
| Model identity, usage, and completion | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 1017 milliseconds |
| Peak reservation | 3735552 bytes |

The exact model string now matches the operator confirmed native Opus test, but the gateway request still received an access denial. Changing the model did not resolve this candidate's denial. This does not prove the two clients used the same profile, authentication branch, operation, or controls, nor identify the service's failed access rule. Matching byte counts do not establish identical discarded error messages.

No raw message, body, header, upstream identifier, token, fingerprint, or account value was retained. No retry, model fallback, endpoint change, renewal, or second request followed. The next productive investigation is the native client's actual authentication and request path, using the successful native session as operator evidence. Another unchanged gateway request is not recommended. Any native live comparison or changed inference candidate still needs its own concrete review.

## Bundled operation target comparison, October 1, 2026

You instructed, "yes, test them", after reviewing the two operation header values and the single bounded comparison. The new target was prepared at clean commit `5c6e8c6fc2221e9194c7945f6caf1309556c24db`, with plan SHA 256 `eecbd193c8aec5ab93ce6ed3c77d33a0160a7f6af7980070ade6c2f66b6b621d`. Both were recorded before launch. Full deterministic checks passed, including race tests and the synthetic header assertion. The tagged live test compiled and skipped with live probing disabled before the authorized launch.

| Run metadata | Observed value |
|---|---|
| Local run ID | `1e5d79ab-5220-41d3-94d9-2bf6e7c72900` |
| Started | `2026-10-01T12:13:33.293265Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Operation target | `KiroRuntimeService.GenerateAssistantResponse` |
| Selected destination | `https://runtime.us-east-1.kiro.dev:443/` |
| Attempts | One dispatch attempt |
| First case | `text`: `inconclusive`, cause `needs_evidence`, failure stage `http_status`, HTTP status category `forbidden` |
| Error observation | `service_error: access_denied`, `error_response_format: json` |
| Received bytes | 119 error response bytes inspected in memory; zero decoded text or tool events |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Local request assertions | Instruction placement and controls requested true; no service acceptance claim |
| Model identity, usage, and completion | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 1620 milliseconds |
| Peak reservation | 3735552 bytes |

The header change did not resolve the denial. The previous old target result remains the historical comparison, not a repeated request in this run. This does not establish native credential equivalence or identify the failed service rule. No renewal, relink, mapping change, native client launch, second request, or raw response retention occurred. This one run authorization is consumed.

## Prepared reference path comparison, October 1, 2026

The requested source review of `jwadow/kiro-gateway` is recorded in `rationale.md`, pinned to commit `a5292ca04c7c6231e0b47673ac3f981f5a706e1e`. Its explicit dispatch path supplies a new candidate: `/generateAssistantResponse` with `AmazonCodeWhispererStreamingService.GenerateAssistantResponse`. The prepared plan SHA 256 is `9c5f55aebe373d20ec46334faf97e0f380aceaf2bf673bd398373f99baf17b31`. Synthetic bodies, exact Opus mapping, account source, controls, diagnostic output, and budgets are unchanged.

`scripts/check` passed formatting, vet, compilation, and all tests with the race detector. Existing six case tests assert the new request URI and operation header against local TLS servers. The destination checks reject root path, trailing slash, encoded path, query, and fragment variants. The tagged live entry point compiled and skipped with `KIRO_GATEWAY_LIVE_PROBE=0`. No reference application code, account access, refresh, or inference ran during preparation. This changed destination still needs one concrete live approval; the prior run's permission is consumed.

## Approved reference path launch stopped on expiry, October 1, 2026

You approved one bounded live run at clean commit `a9fc79db09457461e9d97eea311c3b7634a17dea` and plan SHA 256 `9c5f55aebe373d20ec46334faf97e0f380aceaf2bf673bd398373f99baf17b31`. Both matched before launch.

| Run metadata | Observed value |
|---|---|
| Local run ID | `c1ebfb5d-4695-4885-b002-38f8be2b08be` |
| Started | `2026-10-01T12:37:58.850424Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Destination | Not selected |
| Attempts | Zero dispatch attempts |
| First case | `text`: `inconclusive`, cause `credential_expired`, failure stage `source` |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Received bytes and events | Zero |
| Request and protocol assertions | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 1 millisecond |
| Peak reservation | 3735552 bytes |

This is a local expiry result, not a denial from the new path. No inference request, renewal, relink, mapping change, or retry occurred. No credential value was retained. The candidate remains untested remotely; the operator must renew the Kiro session before explicit relinking and another approved run. This launch consumed its one run approval.

## Renewed reference path run returned access denied, October 1, 2026

After the requested session refresh was reported complete, the gateway relinked the fixed snapshot and restored the exact Opus mapping using its existing lock and atomic save. The temporary setup helper was removed before launch, configuration validation passed, and the working tree was clean. No probe code or plan changed since the prior approval; the intervening commit recorded the expiry result.

The run used commit `1860d2b65caff84e95f8de980cae2e4b400ece3b` and plan SHA 256 `9c5f55aebe373d20ec46334faf97e0f380aceaf2bf673bd398373f99baf17b31`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `36876f90-1267-423a-af49-8724858359bc` |
| Started | `2026-10-01T12:49:30.243654Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Selected destination | `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| Operation target | `AmazonCodeWhispererStreamingService.GenerateAssistantResponse` |
| Attempts | One dispatch attempt |
| First case | `text`: `inconclusive`, cause `needs_evidence`, stage `http_status`, HTTP status category `forbidden` |
| Error observation | `service_error: access_denied`, `error_response_format: json` |
| Received body bytes | Zero read; the classifier recognized the error header and did not inspect the body |
| Text, tool, and unknown events | Zero |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Local assertions | Instruction placement and controls requested true; no service acceptance claim |
| Model, usage, and completion | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 2285 milliseconds |
| Peak reservation | 3735552 bytes |

This demonstrates a denial at the reference path after the renewed fixed snapshot passed local validation. It does not establish successful remote authentication or native account equivalence. Header classification with zero body bytes is not evidence that the response body was empty. No raw header, body, account value, or credential was retained. No second request, automatic renewal, fallback, or native client launch occurred. The bounded run is complete and its authorization consumed.

## Prepared complete reference request baseline, October 1, 2026

Following the instruction to proceed with the complete reference comparison, the candidate now matches independently generated application header and body examples from reference commit `a5292ca04c7c6231e0b47673ac3f981f5a706e1e`. All inputs are synthetic and optional fake reasoning, truncation recovery additions, and trimming are disabled. The fixture and extraction boundary are documented in `rationale.md`.

| Local evidence | Result |
|---|---|
| Six independent reference JSON bodies | Exact semantic matches |
| Application headers and synthetic fingerprint | Exact matches, plus explicit Accept and identity encoding transport choices |
| Tool schema sanitizer difference | Detected by reference fixture; corrected by omitting wire `additionalProperties` while preserving strict local arguments |
| Local TLS sequence | Text, tool, result, followup, cancellation, and cutoff cases passed |
| Metadata validation | Malformed fingerprints and invocation IDs rejected; fresh UUID v4 values validated |
| Output guarantees | Controls requested false after dispatch; output within limit remains null; no metadata leakage in summaries |
| Repository checks | Formatting, vet, build, and all race tests passed |
| Live tag check | Compiled and skipped with `KIRO_GATEWAY_LIVE_PROBE=0` |
| Real account access and inference during preparation | None |

Prepared plan SHA 256: `fe17da293c0bfebf56410f824fa20cd0234012cc13ef116f3399d7c85f476b28`. Independent fixture SHA 256: `2867ae2429efc6cfdef8e2937ae0b6a2c56a29ae67b9c1563a4ea9e6170e6fcd`. No live acceptance result exists for this new request baseline. Prior denials concern materially different requests and cannot establish this candidate's result.

## Approved complete baseline received HTTP 200, October 1, 2026

You approved one run at clean commit `4de4fe96e54eee5f918d947029047421937af64b` with plan SHA 256 `fe17da293c0bfebf56410f824fa20cd0234012cc13ef116f3399d7c85f476b28`. The identity and clean state checks passed before launch.

| Run metadata | Observed value |
|---|---|
| Local run ID | `04e1b6c5-266a-435c-b6ce-c0c75b516ece` |
| Started | `2026-10-01T13:16:18.878299Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Selected destination | `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| Attempts | One dispatch attempt |
| HTTP outcome | `ok`, HTTP 200 |
| First case | `text`: `inconclusive`, cause `needs_evidence`, failure stage `stream` |
| Received bytes | 380 |
| Events | Two text events, zero tool events, zero unknown event names, one unknown field |
| Remaining cases | `tool`, `result`, `followup`, `cancel`, and `interrupt`: `unrun` |
| Local request assertions | Instruction placement true; controls requested false |
| Marker, incremental assertion, model identity, usage, and completion | Unknown; case validation did not finish |
| Output within limit | Unknown; no upstream token cap requested |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 2110 milliseconds |
| Peak reservation | 3808522 bytes |

The complete reference baseline obtained acceptance and streamed text. It did not receive the previous access denial. The harness stopped at its first unreviewed field as designed, so no text content, unknown spelling, client fingerprint, or upstream identifier was retained. The byte count does not establish which event or field caused the stop. No retry, renewal, fallback, or later case occurred.

The static metering schema review and prepared optional `unitPlural` decoder correction are recorded in `rationale.md`. Their next plan digest is `7f251348b878c311b777b556bfccf4fef9913826046c3526c841c87f644e0678`. That correction awaits a separate live review; it cannot retroactively identify the discarded unknown field or claim a completed first case.

The corrected decoder passed formatting, vet, build, all race tests, and the complete local TLS sequence with synthetic plural unit metadata. The live tagged entry point compiled and skipped with its gate disabled. No additional account access or live request occurred during the correction.

## Approved metering decoder run still stopped, October 1, 2026

You approved one bounded run at clean commit `906e14a351792bf691b57ffb5b95d769baad7146`, plan SHA 256 `7f251348b878c311b777b556bfccf4fef9913826046c3526c841c87f644e0678`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `cac31e82-a7c4-42a0-83ca-26547374e477` |
| Started | `2026-10-01T13:26:18.991175Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Model and destination | `claude-opus-5.5`, `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| Attempts and HTTP outcome | One attempt, HTTP 200 |
| First case | `text`: `inconclusive`, `needs_evidence`, stage `stream` |
| Received bytes and events | 380 bytes, two text events, zero tool events, zero unknown event names, one unknown field |
| Remaining cases | Five unrun |
| Local assertions | Instruction placement true; controls requested false |
| Marker, incremental assertion, model, usage, completion, token limit | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 2377 milliseconds |
| Peak reservation | 3808522 bytes |

The added metering member did not resolve the unknown field stop. Neither the event containing that field nor its spelling was retained, and equal counts do not prove equal discarded contents. No later request, retry, refresh, raw response retention, or account mutation occurred. This run's approval is consumed.

The prepared structural diagnostic and finite vocabulary are described in `rationale.md`. Its plan digest is `ffed93d9e9089568d095506f490e9924d5c96c683ad9a8e1eeb387af6d0d3ef5`. It preserves the request and stop behavior while adding bounded event, location, known name hint, and JSON kind labels. It awaits a new exact run review.

## Approved diagnostic identified metadata extension, October 1, 2026

You approved one run at clean commit `f7870e86dc6f782b886899ccefa5a6a974785c83`, plan SHA 256 `ffed93d9e9089568d095506f490e9924d5c96c683ad9a8e1eeb387af6d0d3ef5`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `25f46680-aa1b-4704-8ffe-0bb28d9f5f24` |
| Started | `2026-10-01T13:36:47.170438Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Model and destination | `claude-opus-5.5`, `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| Attempts and HTTP outcome | One attempt, HTTP 200 |
| First case | `text`: `inconclusive`, `needs_evidence`, stage `stream` |
| Received bytes and events | 380 bytes, two text events, zero tool events, zero unknown events, one unknown field |
| Structural detail | `event: metadataEvent`, `location: event`, `known_name: unlisted`, `kind: string` |
| Remaining cases | Five unrun |
| Local assertions | Instruction placement true; controls requested false |
| Marker, incremental assertion, model, usage, completion, token limit | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 2221 milliseconds |
| Peak reservation | 3808522 bytes |

The diagnostic locates a string extension in the top level of a metadata event. Neither its actual name nor its value was retained. There was no retry, refresh, fallback, account mutation, or second request. This approval is consumed.

The prepared correction ignores only additional top level `metadataEvent` members while retaining their fixed diagnostics and validating known usage. Its rationale and regression boundaries are recorded in `rationale.md`; plan SHA 256 is `4ad27507d9c8e63592c67dc41dbcd23afb034405d171a79a2b4a2bfd0de5aba3`. It has no live result yet and needs a new exact review.

## Approved metadata compatibility launch stopped before dispatch, October 1, 2026

You approved one run at clean commit `542947fe3fd1e49b8c13abce0bc084174d35ef0d`, with plan SHA 256 `4ad27507d9c8e63592c67dc41dbcd23afb034405d171a79a2b4a2bfd0de5aba3`. Both matched before launch.

| Run metadata | Observed value |
|---|---|
| Local run ID | `53b2a383-1cb9-4af3-beae-944a481ec289` |
| Started | `2026-10-01T14:05:04.452795Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Destination | Not selected |
| Attempts | Zero |
| First case | `text`: `inconclusive`, cause `session_changed`, stage `source` |
| Remaining cases | Five unrun |
| Received bytes and events | Zero |
| Request and protocol assertions | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 1 millisecond |
| Peak reservation | 3735552 bytes |

The selected token record differed from the linked fingerprint. No inference request, remote rejection, renewal, relinking, mapping mutation, or retry occurred. No fingerprint or credential value was retained. This does not establish why the record changed or require another sign in by itself. The metadata acceptance correction remains untested remotely. Explicit relinking, restoration of the exact Opus mapping, and a new bounded launch approval are the next prerequisites. This launch consumed its approval.

## Approved relink and run observed the tool roundtrip, October 1, 2026

You approved relinking, restoring the exact `claude-opus-5.5` mapping, and one unchanged bounded run. Setup used the existing lock and atomic save, configuration validation passed, the temporary helper was removed, and the working tree was clean. The run used commit `b27b3c2c3de208baa4667e79026cb9b9f492dfb8` and plan SHA 256 `4ad27507d9c8e63592c67dc41dbcd23afb034405d171a79a2b4a2bfd0de5aba3`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `3ac8306e-4483-48ac-be87-6eb263126825` |
| Started | `2026-10-01T14:18:02.52139Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Model and destination | `claude-opus-5.5`, `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| Attempts | Five; all received HTTP 200 |
| Verdict and elapsed time | `needs_evidence`, 14526 milliseconds |
| Peak reservation | 3818172 bytes |

| Case | Outcome | Allowed observations |
|---|---|---|
| Text | `observed` | 695 bytes, two text events, incremental and marker assertions true |
| Tool | `observed` | 1359 bytes, five tool events, valid arguments and matching tool name and ID true |
| Result | `observed` | 578 bytes, one text event, marker assertion true |
| Followup | `observed` | 576 bytes, one text event, marker assertion true |
| Cancellation | `inconclusive`, `needs_evidence`, stage `stream` | 121 bytes, no text or tool events, one unknown event, cancellation trigger false |
| Interruption | `unrun` | No dispatch |

The first four cases each counted one metadata extension, with fixed labels `metadataEvent`, `event`, `unlisted`, and `string`. All four reached tentative completion and cleanup. The cancellation attempt completed cleanup after stopping at the unknown event, but that is not evidence that the planned cancellation trigger worked. Instruction placement was true and controls requested false for all five dispatched cases. Model identity, token usage, authoritative completion, and output token limit remained unknown. No raw text, arguments, IDs, unknown names, account metadata, or device fingerprint was retained.

No retry, renewal, fallback, or sixth request followed. The first four protocol cases have positive live evidence. Cancellation and interrupted stream handling still require live evidence before the limited six case result can be declared observed. This run's approval is consumed.

The next prepared diagnostic reports only a finite unsupported event hint from the inspected SDK stream vocabulary or `unlisted`. It changes no request or acceptance behavior. Its plan SHA 256 is `a188bf63b275f919445cf5b7a01eef3b789027cd1d14b5f7d762441ff38f9a4f`; it awaits another exact run review.

## Approved event diagnostic stopped on session change, October 1, 2026

You approved one diagnostic run at clean commit `67ae516e305f18dae4fe68816a3123a0db710bba`, plan SHA 256 `a188bf63b275f919445cf5b7a01eef3b789027cd1d14b5f7d762441ff38f9a4f`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `3ae598e4-6b1f-4782-94ee-c6729a4749e9` |
| Started | `2026-10-01T14:27:09.001676Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Destination and attempts | Not selected, zero attempts |
| First case | `text`: `inconclusive`, `session_changed`, stage `source` |
| Remaining cases | Five unrun |
| Received bytes and events | Zero |
| Request and protocol assertions | Unknown |
| Cleanup | Completed |
| Verdict and elapsed time | `needs_evidence`, 1 millisecond |
| Peak reservation | 3735552 bytes |

No request, renewal, relink, mapping change, or retry occurred. The event hint has not yet been exercised live, and the cause of the saved token record change is unknown. The previously observed text and tool roundtrip remain historical evidence; cancellation and interruption remain pending. This one run approval is consumed.

## Approved setup and event diagnostic identified reasoning, October 1, 2026

You approved recurring setup before each remaining separately approved feasibility run, plus one current diagnostic run. Relinking, exact Opus mapping restoration, and configuration validation succeeded. No agent driven renewal occurred. The temporary helper was removed and the working tree was clean before launch at `8dbeeae529a401747dbdb7e42558fa5355a51e15`, with plan SHA 256 `a188bf63b275f919445cf5b7a01eef3b789027cd1d14b5f7d762441ff38f9a4f`.

| Run metadata | Observed value |
|---|---|
| Local run ID | `73507d2a-b5ef-42df-b3c3-086c97dd5e2a` |
| Started | `2026-10-01T14:35:16.97726Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Model and destination | `claude-opus-5.5`, `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| Attempts | Five, all HTTP 200 |
| Verdict and elapsed time | `needs_evidence`, 16095 milliseconds |
| Peak reservation | 3818172 bytes |

| Case | Outcome | Allowed observations |
|---|---|---|
| Text | `observed` | 695 bytes, two text events, incremental and marker true |
| Tool | `observed` | 1177 bytes, four tool events, arguments and tool name and ID true |
| Result | `observed` | 578 bytes, one text event, marker true |
| Followup | `observed` | 576 bytes, one text event, marker true |
| Cancellation | `inconclusive`, `needs_evidence`, stage `stream` | 119 bytes, no text or tool events, one unknown event, hint `reasoningContentEvent`, trigger false |
| Interruption | `unrun` | No dispatch |

The first four cases again counted one unlisted string metadata extension each and completed tentative EOF and cleanup assertions. Instruction placement was true and controls requested false for all dispatched cases. Model identity, usage, authoritative completion, and token limit remained unknown. Cleanup completed for cancellation after the unsupported event stop, but the planned cancellation itself was not observed. No payload, reasoning, account value, device fingerprint, raw tool argument, or identifier was retained.

The run consumed its approval and did not retry. Standing permission for the existing pre-run relink and exact Opus mapping restoration remains in force for separately approved remaining feasibility tests.

The prepared correction discards bounded reasoning objects only in the independent failure cases, preserving the text trigger and byte cutoff. Local regression and race checks passed and the tagged live entry point compiled and skipped while disabled. Its plan SHA 256 is `21d76d39ca83de75df9dbefcbeae8e43c016e856a1ebc3bab9b83c47936cf4f6`; the correction has not run live yet.

## Approved six case proof passed, October 1, 2026

You approved one bounded run at clean commit `4fd0b70773ca6bdba87d73be1ac6c07ab8d77659` and plan SHA 256 `21d76d39ca83de75df9dbefcbeae8e43c016e856a1ebc3bab9b83c47936cf4f6`. Standing setup confirmed the session was already linked, verified the exact Opus mapping under lock, and passed configuration validation. The temporary helper was removed before launch.

| Run metadata | Observed value |
|---|---|
| Local run ID | `2d18b2f6-ad2d-465d-b81f-5b5af4146612` |
| Started | `2026-10-01T14:45:33.478676Z` |
| Platform and baseline | `darwin/arm64`, Claude Code `2.1.286`, Kiro CLI `2.8.0` |
| Requested model | `claude-opus-5.5` |
| Destination | `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| Attempts and HTTP outcomes | Six, all HTTP 200 |
| Verdict and elapsed time | `limited_candidate_observed`, 19969 milliseconds |
| Peak reservation | 3867910 bytes |

| Case | Outcome | Allowed observations |
|---|---|---|
| Text | `observed` | 695 bytes, two text events, incremental and marker true |
| Tool | `observed` | 1359 bytes, five tool events, valid arguments and matching tool name and ID true |
| Result | `observed` | 578 bytes, one text event, marker true |
| Followup | `observed` | 576 bytes, one text event, marker true |
| Cancellation | `observed` | 2666 bytes, 13 discarded reasoning events, one text event, cancellation trigger and cleanup true |
| Interruption | `observed` | Exactly 256 bytes, two complete discarded reasoning events, zero text events, cutoff trigger and cleanup true |

The first four cases each retained one fixed metadata diagnostic (`metadataEvent`, `event`, `unlisted`, `string`) and reached tentative completion. Cancellation and interruption reported no tentative completion and did not contribute to history. All six completed cleanup, had instruction placement true, and reported controls requested false. Distinct system role preservation was false. Model identity, token usage, authoritative completion, and output token limit remained unknown.

No raw conversation, tool arguments, tool IDs, unknown metadata names or values, reasoning payload, credential, account metadata, or device fingerprint was retained. No seventh attempt, retry, renewal, fallback, or source adoption during the run occurred. The live test passed. This is the completed bounded feasibility proof and builder evidence, not independent GA verification or a completed Claude Code bridge.

## Later Claude Code acceptance gate

After `/architect` completes the bridge design, the original scope still needs a real Claude Code session against the implemented local API, using the model and client baseline selected for that milestone. This feasibility proof requested Opus 5.5 and recorded Claude Code `2.1.286` and Kiro CLI `2.8.0`, but did not run Claude Code through a gateway inference endpoint. The later task must read and fix a disposable Go bug, execute Claude Code file and shell tools under normal permissions, return results, finish a followup user turn, and demonstrate cancellation and incomplete stream handling. Record exact versions, the access path, and every instruction or model control difference. That full coding task remains unverified.
