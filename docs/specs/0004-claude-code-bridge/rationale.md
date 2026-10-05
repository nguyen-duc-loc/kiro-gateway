# Reasoning for the experimental coding bridge

The original decision and earlier amendments below retain their historical context. The confirmed October 5 assessment after catalogue evidence records the latest GA investigation outcome and recommended next work.

## Context

The repository has a working health server, private versioned configuration, exact session linking, and a combined token and profile reader. Its test only feasibility harness observed all six cases at commit `4fd0b70773ca6bdba87d73be1ac6c07ab8d77659`: streamed text, a tool request, matching result continuation, a followup turn, cancellation, and deliberate interruption. That run requested `claude-opus-5.5` and returned `limited_candidate_observed`.

The evidence does not contain a real Claude Code coding session. It also does not prove distinct system instruction priority, authoritative model completion, serving model identity, measured usage, output token control, or reasoning continuation. Building an apparently equivalent API without naming those limits would turn missing evidence into hidden behavior.

You selected a new bridge spec, an experimental release promise, and the exact Opus mapping from that successful run. You explicitly accepted instruction flattening, inferred completion, discarded reasoning, estimated usage, advisory `max_tokens`, self contained requests, one process profile pin, both response modes, an explicit enable flag, one active inference, and a later budget of 20 requests over 20 minutes. GA completion remains pending.

## Options considered

### Option 1: Experimental bridge with explicit limits

Build the smallest real coding path using the evidenced transport and the exceptions you selected. The benefit is a useful test of the actual product layers and client tools. The cost is a deliberately weaker contract that must not be described as GA compatibility. This is the chosen option. (basis: your design answers, spec 0003's completed live evidence, the project's Tracer Bullet approach)

### Option 2: Resolve every protocol guarantee first

Continue investigation until the adapter can preserve role priority, authoritative completion, usage, and controls. This best matches the stronger architecture contract and reduces ambiguity for daily operation. Its cost is an unknown delay before any real Claude Code task, with no assurance that the private service exposes all those guarantees. (basis: spec 0001's compatibility contract and spec 0003's unresolved evidence)

### Option 3: Reuse a general reference gateway as the product

Adopt an existing broader translator and its compatibility behavior. This may provide a faster initial client connection, but imports credential lifecycle, fallback, schema transformations, and operational assumptions that differ from the accepted Go architecture. Its reference behavior remains useful evidence, while replacing this repository with it would be a different decision. (basis: the reference comparison already recorded in spec 0003, existing package and credential boundaries)

## Rationale

The real question now is whether Claude Code can complete a coding task through the proven candidate. A small isolated task answers that question more directly than adding model families, renewal, a catalogue, or a new provider abstraction. An explicit experimental flag gives the weaker semantics a concrete activation boundary. (basis: your experimental release choice and [accepted architecture](../0001-stack-architecture/index.md))

The standard library remains sufficient for transport, JSON translation, deterministic estimates, and one admission slot. A full Anthropic SDK would not resolve the missing Kiro semantics, and a tokenizer would imply accuracy that has not been established for this service. Preserve complete tool schemas rather than adopt an unexplained sanitizer; a new upstream rejection becomes evidence to assess, not a reason to silently weaken the schema. (basis: project stack, explicit value sourcing, observed limitations in [spec 0003](../0003-first-claude-code-loop/index.md))

Buffering tools until the upstream response validates delays tool handoff but avoids exposing a call before a later frame invalidates the response. Streaming ordinary text still provides early visible progress. The alternative, forwarding tool fragments immediately, would require a stronger argument about when Claude Code may execute a partially delivered response. (basis: client owned tool execution, [streaming reference](https://platform.claude.com/docs/en/build-with-claude/streaming), `go-concurrency` ownership guidance)

Local byte estimates make the wire response usable while retaining a visible label about accuracy. A strict measured usage requirement would block this selected experiment. Zero values labelled as actual usage would be misleading. The formula is a compatibility convention, not a claim about tokenization. (basis: your estimate choice, absent verified usage in the feasibility run)

The first build task exercises the installed client with a dummy token and a synthetic service. This tests request shapes and settings instead of assuming that rolling documentation proves installed behavior. It can also reveal that the selected client requires an unsupported field before any real credential is read. (basis: inspected local version and help, [environment reference](https://code.claude.com/docs/en/env-vars), [settings reference](https://code.claude.com/docs/en/settings))

## Evidence and limits

| Observation | What it establishes | What it does not establish |
|---|---|---|
| Local `claude --version` on October 2, 2026 returned `2.1.287` | Installed proposed client baseline | Compatibility with this unbuilt bridge |
| Local `kiro-cli --version` returned `2.8.0` | Installed source baseline remains unchanged | Current credential usability or account authorization |
| Local client help lists `--safe-mode`, `--tools`, and manual permission mode | Concrete client isolation and tool selection options | Their complete network behavior during a coding session |
| Official environment reference documents gateway URL, bearer token, model controls, and traffic controls | Supported candidate setup controls | Exact emitted fields, absence of every helper request, or zero retries on this binary |
| That reference says Opus 5.5 cannot have thinking disabled by the described switches | A client setting cannot promise no upstream reasoning | The semantics of Kiro's reasoning events or their continuation needs |
| Official streaming page shows message and content block event order, JSON fragments, cumulative usage, and error events | A concrete client stream contract to implement | That Kiro EOF is an authoritative model completion signal |
| Official tool definition page describes schemas and selection modes | The standard client tool input structure | Kiro acceptance of arbitrary Claude Code tool schemas |
| Successful feasibility run used six HTTP 200 responses in about 20 seconds | Positive evidence for its exact limited candidate | Availability, daily reliability, token control, or the later full coding loop |

The research check opened the environment and settings pages plus the streaming and tool definition pages. The large Messages API page and token count page could not be read successfully through the web tool. Integer usage examples were visible, but their complete required schema was not established by that check. The bridge deliberately emits the standard integer input and output fields and must validate consumption in the offline client exercise. Exact tool result consumption is also a required local fixture and client observation, not a newly claimed live fact.

The environment reference names `CLAUDE_CODE_MAX_RETRIES`, but the checked text did not establish whether zero is accepted. The spec therefore tests that candidate offline and contains an independent dispatcher latch for the later live runner. It does not claim a documented switch disables every retry.

During the original design, no real credential store, profile, conversation, or account configuration was read. No live request or native Kiro chat was launched. Version and help commands were the only client executions in that original design pass. The repository had 42 source files and no commits missing from `origin/main` after its freshness check.

## Independent review, October 2, 2026

At your request, `gpt-6-sol` performed a read only cross check for decision completeness and soundness. It identified three gaps: an ambiguous start for the live budget, incomplete source error mappings, and a dispatcher wrapper unable to observe every failure that must stop the run. It found no further material contradiction in the request, tool, or streaming transformations against the local feasibility contract.

You selected the recommended fixes. The revised spec starts the clock at the accepted launch flag before runtime preflight or setup, maps every source error and cancellation cause, and supplies a typed boundary observer and shared run controller alongside the final dispatcher guard. Expected faults are bound to their case and request so client retries cannot be mistaken for an authorized next case. The verification plan now checks these rules. This approval resolves the review findings; final acceptance of the whole spec is a separate step.

The same reviewer confirmed the source mapping and observer corrections, then identified a cleanup edge in the clock amendment. Canceling active work only at the absolute deadline would leave no time for its already specified five second cleanup bound. The author reserved the last five seconds of the approved 20 minute total, stopping work at 19 minutes 55 seconds, and added the corresponding boundary test. This preserves the chosen total rather than adding a cleanup extension.

You then accepted the complete revised spec on October 2, 2026. The design is confirmed and linked to scope feature 4, with the experimental build milestones ready. This ratification neither claims implementation completion nor authorizes a live run. The spec remains `Proposed` until development begins, and the full feature retains its separate GA completion requirements.

## Client contract amendment, October 2, 2026

### Observation

The first `/develop` characterization stopped at an initial client request that violates the original contract. Its [sanitized shape record](../../../internal/gateway/testdata/claude-code-2.1.287-shape.json) records `output_config.effort` as a string and a nonempty beta header. It does not retain the effort value or beta names. No production bridge was built, and no build milestone was completed.

This architecture pass repeated the same isolated characterization to identify those public protocol constants, then repeated it with the proposed `--effort high` launch. Each launch used Claude Code `2.1.287`, an ephemeral numeric loopback receiver, a dummy bearer, a disposable Git directory, and a private temporary `CLAUDE_CONFIG_DIR`. The receiver always returned one fixed HTTP 400 JSON error. It was a shape receiver, not an implementation of the bridge or a successful adapter. Both launches exited with code 1 after one local request each, with zero Kiro dispatches. The checkout was zero commits behind `origin/main` after fetching.

| Evidence item | Observed value or rule |
|---|---|
| Client artifact SHA 256 | `6eab8333fe2121553100d8f40bfada384a3e989b94f947e18ba6677a6fcb41ea`, obtained for the installed executable during the initial characterization |
| Method and route | `POST /v1/messages?beta=true` |
| Authentication | Matching dummy bearer, no `x-api-key`, API version `2023-06-01` |
| Requested model | Exactly `claude-opus-5.5` |
| Top level fields | `model`, `messages`, `system`, `tools`, `metadata`, `max_tokens`, `output_config`, `stream` |
| Output configuration | Exactly one member, `effort`, with string value `high` in both new captures |
| Beta header | One header containing exactly the four tokens in the table below, in that order |
| Reasoning request | No top level `thinking` field in either new capture |
| Difference between launches | The second added `--effort high`; the recorded protocol constants stayed the same |
| Unproved behavior | Complete body validation, successful responses, tool execution, permission ownership, followup, counts, cancellation, and retries after 429, 502, or interrupted streams |

| Observed beta token |
|---|
| `claude-code-20250219` |
| `interleaved-thinking-2025-05-14` |
| `mid-conversation-system-2026-04-07` |
| `effort-2025-11-24` |

To reproduce the shape probe, use the initial record's environment and arguments, without retaining its raw prompt or traffic. That launch uses print mode with `--no-session-persistence` and the normal client system prompt. A synthetic text input suffices. The child environment starts with only `HOME`, `PATH`, `TMPDIR`, `USER`, `LOGNAME`, and `SHELL` when present, then adds the explicit client variables in the record; `HOME` stays unchanged. Add `--effort high` for the amended launch. Observe only known field shapes, exact model equality, bearer match as a boolean, public beta identifiers, and effort from a fixed enum. Do not retain client stdout, stderr, system text, metadata values, tool definitions, or raw request bytes. The temporary receiver and child configuration are removed after exit. These two architecture observations were not added to the historical initial JSON fixture.

The controls suppress neither every beta token nor the effort object in this installed client. That is a narrow observed result, not a finding about all versions or every code path. A returned local 400 establishes neither retry suppression under other failures nor that every emitted field is supported. The coding loop must still pass through the production handlers and scripted adapter before live work.

### Recommended amendment and alternatives

**Recommended, amend in place.** Tolerate only the observed `high` effort object and the four literal beta tokens, validate them strictly, discard them locally, and expose the ignored semantics. Pin the client flag to the observed value. This removes the two initial structural blockers while preserving the successful Kiro request. Its cost is a deliberate API semantic difference: the client can request high effort or advertise a beta without obtaining that behavior. You accepted this extension to the existing experimental exceptions after independent review on October 2, 2026. No migration or new dependency is needed. (basis: the offline observations above, the original experimental control decision, and the Go security skill's bounded validation rules)

**Runner up, keep the original rejection contract and change the client launch or version.** This would avoid a new semantic exception if a supported launch can omit these fields. The prescribed controls already failed to do so on the pinned binary, and no verified omission control was found in this pass. A different version would require a new baseline and its full characterization. It remains a valid choice if honoring effort becomes a requirement. (basis: the two concrete launches and the pinned client requirement)

**Implement equivalent effort and beta features.** This would better match the Anthropic surface, but the Kiro baseline has no evidenced equivalent for these controls. A second adapter or a replacement gateway would not supply that missing evidence and would broaden this decision into provider, state, and reasoning support. Do not invent upstream fields or map `high` into instructions. (basis: spec 0003's wire evidence and the fixed adapter boundary)

The official [effort reference](https://platform.claude.com/docs/en/build-with-claude/effort) describes effort as a behavioral control affecting output, tools, and thinking. The official [beta header reference](https://platform.claude.com/docs/en/api/beta-headers) describes feature opt in and accepts comma separated names or repeated headers. Those are Anthropic semantics, not proof of Kiro support. The confirmed bridge exception knowingly differs and therefore uses explicit `ignored` and `unsupported` labels. The header parser can handle both documented representations while still rejecting unknown names and duplicates. (basis: these two official references, checked October 2, 2026)

At that amendment, no assumption was made that the four beta names were harmless on arbitrary bodies. In particular, `interleaved-thinking-2025-05-14` did not admit thinking history, and `mid-conversation-system-2026-04-07` did not admit system messages within history. The complete body contract remained the gate. The later system history amendment below records the new observation and the narrow replacement for that rejection rule.

### Verification impact

The amended matrix checks both endpoints, all repeated header values, malformed and unknown controls, and absence of account reads on failure. It also proves that the tolerated fields do not affect normalized content, estimates, upstream semantic requests, or fixed response labels. The first real client success and full tool exchange remain pending. Review and acceptance of this amendment do not authorize a live run.

### Independent amendment review

At your request, `gpt-6-sol` reviewed the amendment for decision completeness and soundness. It found no material decision gaps. The literal effort shape, beta token set, parser bounds, validation order, discard boundaries, upstream omissions, fixed errors and policy headers, launch flag, and return to architecture for future unsupported shapes are specified.

The reviewer noted that the newer exact enum and token observations are retained here, while the original JSON fixture records presence only. A second sanitized fixture could improve auditability, but the reviewer did not consider it a blocker: the implementation contract is explicit and the full offline exercise through production handlers remains required. No new capture or fixture was created by the reviewer. You then accepted the amendment on October 2, 2026. That acceptance confirms its design and verification plan. The feature linked spec remains `Proposed` until implementation begins; live execution still requires its separate concrete run review.

## System history amendment, October 2, 2026

### Observation and scope

Development added an initial normalized parser, response construction, and injected Messages handler, then exercised the same installed client. The [sanitized result](../../../internal/gateway/testdata/claude-code-2.1.287-bridge.json) records four isolated diagnostic exercises. In the final exercise, the Messages request had the accepted bearer and headers, a top level system value, and a history containing one user entry with two text blocks followed by one system entry with string content. The body had no unknown top level fields. It received 400 before the scripted generator ran. An in memory diagnostic copy that removed the system entry passed the rest of validation, but that copy was never dispatched. A separate request to another local route received 401; this record does not establish that route's purpose or whether its rejection matters after a successful inference response.

The client version was `2.1.287`, and its executable SHA 256 remained `6eab8333fe2121553100d8f40bfada384a3e989b94f947e18ba6677a6fcb41ea`. There were zero Kiro dispatches. No installed client response success, tool execution, tool result exchange, followup, or retry behavior was established. The tagged reproduction uses a temporary fixture directory, private client configuration, dummy bearer, manual permissions, safe mode, and the normal system prompt. Its raw traffic and client output were not retained. The product command and production Kiro adapter remain unwired. Passing deterministic tests do not resolve this client mismatch.

You asked to resolve this gap, chose an update to spec 0004, and selected preservation as user context. The checkout was zero commits behind `origin/main` after fetching and had 48 Go source files. The partial build remains uncommitted. This architecture pass changes only the spec and its verification plan; it does not run another client experiment or access credentials.

### Meaning of the source protocol

The official [system messages inside a conversation reference](https://platform.claude.com/docs/en/build-with-claude/mid-conversation-system-messages), checked for this amendment, documents system entries inside `messages`. Their instructions apply from that point onward, with later instructions taking precedence over earlier instructions and the top level system value. The observed trailing entry is therefore semantic input. Deleting it merely because validation then passes would lose instructions. The Kiro baseline has no evidenced field that preserves that priority or replacement behavior. Preserving text as user content remains a deliberate weaker contract, including when instructions conflict.

The current [CLI reference](https://code.claude.com/docs/en/cli-reference) describes `--system-prompt-snapshot off` as rebuilding the prompt each request, and `--exclude-dynamic-system-prompt-sections` as moving selected context into a user message. Neither description promises to suppress system entries inside history. The [SDK system prompt guide](https://code.claude.com/docs/en/agent-sdk/modifying-system-prompts) likewise separates prompt construction from context inserted into the conversation. These current references explain the choices; they do not prove how every path in the pinned executable behaves. No omission control was verified in this architecture pass.

### Options and selected rule

**Selected: preserve the observed shape as user context.** Accept exactly one string system entry immediately after a user entry, including a user containing tool results. Preserve it separately in normalized history and estimates; fold it into that user's text only when constructing Kiro JSON. Keep the existing client launch. This addresses the observed blocker with the existing Go boundaries and introduces no dependency, source, migration, or persistent state. Its cost is the explicit lack of system priority and instruction replacement guarantees. A later unsupported shape still returns to architecture. You selected this direction and accepted the written amendment after independent review on October 2, 2026. (basis: the local structural evidence, your selected experimental instruction policy, and bounded input validation from the Go security skill)

**Runner up: keep rejection and investigate client controls.** A verified omission control could avoid supporting this shape, but rebuilding or moving prompt sections is not documented as suppressing it. Trying those controls would need an isolated capture and then the full tool exercise. A different client version would need a separately accepted baseline. This is appropriate if the added semantic limitation is unacceptable, but there is no verified launch change ready to adopt here. (basis: official CLI and SDK references above, pinned client evidence)

**Lift every history instruction into the top level prefix.** This would preserve words but move later instructions ahead of earlier assistant turns. It would also invite unsupported assumptions about replacement and duplication. The chosen suffix retains the observed position relative to the associated user and following assistant. No instruction deduplication or cross request update state is introduced. (basis: self contained history contract and the source protocol's positional semantics)

**Replace or add a second translator.** A parallel or replacement gateway would still need an evidenced Kiro equivalent for system priority. The observed issue does not justify a new adapter framework or replacement of the existing HTTP and credential layers. The local transformation can be added within the experimental flag. (basis: accepted architecture and the absence of an evidenced native Kiro system role)

### Exact boundaries and verification impact

The build spec defines the string shape, immediate user pairing, raw message and block accounting, sequence validation, tool result invariants, exact two newline suffix, empty string handling, top level prefix ordering, current message selection, normalized estimate source, and fixed instruction policy header. The string is never parsed for tags or special phrases. A beta header neither enables nor relaxes this rule. The gateway retains neither instructions nor their fingerprints between requests and makes no model compliance assertion from preserving their bytes.

Verification covers the observed `[user, system]` shape, several user turns with their own entries, a user containing only tool results, repeated text, empty strings, placement failures, extra fields, budget boundaries, both endpoints, and unchanged inputs after translation. The original mismatch fixture remains historical evidence. After implementation, the real client must pass through the handler and scripted adapter for the complete text, tool, followup, and failure exercise. The existing live run review and limits remain in force.

### Independent system history review

At your request, `gpt-6-sol` reviewed the written amendment without changing files or fetching its references. It found no material decision gaps in validation, pairing, tool results, current user selection, limits, estimates, errors, headers, or the unchanged launch. It identified one ambiguous sentence about empty user content. The author clarified that content stays empty only when ordinary text, a nonempty system suffix, and an applicable nonempty top level prefix are all absent, matching the existing exact formula. This was a wording correction, not a new decision. The reviewer also confirmed that the recorded initial request does not establish later client behavior; the full offline exercise remains required. You then accepted the written amendment on October 2, 2026. The feature linked spec remains `In Progress`, with the build and verification milestones incomplete. This confirms the design and permits development to resume; live execution still needs its separate concrete run review.

## GA promotion amendment, October 4, 2026

### Context

> Premise note: A working experimental coding loop does not establish the instruction, completion, identity, usage, and control guarantees that its contract explicitly waived. Promotion requires new evidence for those guarantees as well as closure of the implementation review.

You selected an update to spec 0004 rather than a separate promotion spec. Scope feature 4 remains in progress with a GA workflow. The experiment now has an offline real client loop and a separately approved live coding loop. The scope has checked off the review and documentation activities, but its latest review verdict is `Changes requested`, and several earlier progress sentences still describe the bridge as unbuilt. Those historical sentences are not current acceptance evidence.

This amendment is a confirmed enhancement design for the existing Go CLI and HTTP backend. It uses the current package boundaries, one account, one exact model mapping, and the Tracer Bullet approach. No UI, storage migration, dependency, skill installation, or new service is selected. The references choice for this amendment is named local sources only. Earlier web references remain historical; none was fetched again for this update.

### Options considered

1. **Repair in place and require evidence before promotion, recommended.** Keep the useful experimental path and fix its observed weaknesses, then resolve the wire contract before adding stronger behavior. This avoids replacing working local layers and keeps the requested GA bar honest. Its cost is an uncertain promotion date, and the selected upstream path may never supply every guarantee. (basis: spec 0001, spec 0004's original followup, October 4 review, current adapter and bridge)
2. **Accept a narrower stable replacement contract.** Make user context instructions, inferred completion, estimated usage, and ignored controls permanent declared limits. This could provide a stable integration for an operator who accepts those semantics. It changes the earlier GA promise and still cannot guarantee that a complete frame truncation is detected; it needs a separate explicit decision. The request to update this spec does not itself accept these tradeoffs. (basis: spec 0004's experimental exceptions and recorded run limitations)
3. **Add a second adapter alongside the experiment.** An independently evidenced Kiro access path could supply missing semantics while preserving the existing path for comparison. This isolates migration but duplicates verification and has no proven candidate in this pass. Keep it as a later access path option rather than adding an empty provider abstraction now. (basis: spec 0001's replaceable adapter boundary and the absence of new protocol evidence)
4. **Replace the current bridge directly.** A replacement could simplify the product once a better path is proven. Today it would discard a demonstrated loop without supplying the missing upstream facts, and would require repeating credential, cancellation, and client verification. There is no evidence that the existing local layers are unmaintainable. (basis: the retained offline and live loop records and current code boundaries)

### Rationale

The immediate defects are local and actionable. Fragmented strings repeatedly copy their accumulated prefixes, and the live verdict can succeed without the client's followup test command. These have concrete repairs within the existing path. Protocol guarantees have a different source: the upstream interface must expose and honor them. Treating an optional field's presence or a successful coding task as that evidence would hide a gap from the next builder. (basis: `internal/bridge/response.go`, `internal/kiro/events.go`, `internal/kiro/live_bridge_test.go`, October 4 review)

The recommended gate is deliberately stricter than relabeling the experiment. It preserves the accepted instruction and terminal goals and requires actual sources for model identity, message usage, and controls. The local count endpoint can remain an explicit estimate because its current contract already names that source and denies exact capacity or billing claims. GA protocol implementation remains blocked until the actual wire contract is ratified. This is a bounded repair and promotion decision, not a claim that the missing protocol design is complete. (basis: specs 0001 and 0004, explicit value sourcing)

Bounded builders retain the current byte limits without adding asynchronous processing. The synchronous generator, single stream writer, and cancellation owner remain appropriate. Benchmarking before and after distinguishes the review's arithmetic cost from measured runtime behavior. Runner observations are private test state with exact tool ownership; they never become product conversation storage. (basis: Go security and Go concurrency skills, the current generator interface)

### Current evidence inventory

| Source | Established observation | Remaining limit |
|---|---|---|
| `internal/gateway/testdata/claude-code-2.1.287-offline-loop.json` | Pinned client completed synthetic tool and followup paths, including a production adapter TLS fixture, with zero real Kiro dispatches. | Synthetic output establishes client handling, not upstream semantics. |
| `internal/kiro/testdata/bridge-run-2026-10-02-04.json` | Recorded experimental success, 8 dispatches in 805.64 seconds, operator permissions and followup observations, cancellation, interruption, and bounded cleanup. | Explicitly not GA; the later review limits what the automated coding assertions independently establish. |
| `docs/reviews/2026-10-04-feat-claude-code-bridge-design.md` | Two major findings, repeated accumulation copies and weak followup verdict checks, plus a missing retry control assertion. | Review was performed; fixes and review closure are not established. |
| `internal/bridge/response.go` | Text uses repeated string concatenation; final usage is estimated and model is the request's value. | No measured usage or serving identity source. |
| `internal/kiro/events.go` | Tool fragments use repeated string concatenation; optional model and recognized usage fields are validated but not propagated. | Field names and validation alone do not establish their semantics, completeness, or availability. |
| `internal/kiro/adapter.go` | After validated framing, semantic checks, and cleanup, the adapter returns `inferred_clean_eof`. | No authoritative upstream turn completion indication is consumed. |
| `internal/kiro/request.go` | System text enters user context; no upstream output token or effort control is sent. | No evidence of instruction priority, replacement, or enforced model controls. |
| `internal/kiro/live_bridge_test.go` | The existing observer tracks tool names across the run and increments a request based turn counter; the boundary check accepts a function name. | Repeated history, unrelated Bash success, and an empty followup test can produce insufficient evidence. |
| `internal/kiro/testdata/bridge-plan.json` | Exact initial and followup prompts, command, fixture, versions, and budgets exist. The original upper branch returns the correct upper value. | A new repair changes the candidate and needs a new plan binding before any future live run. |

The review estimates about 43.98 GiB of cumulative text copying for 41,943 valid frames with 50 text bytes each. That is an arithmetic example, not a measured heap peak or benchmark result. The amendment requires measurement rather than repeating that number as performance evidence.

### Promotion gate record

| Gate | State | Missing source or proof |
|---|---|---|
| G1 | open | Concrete instruction role and positional priority contract; current encoder flattens instructions. |
| G2 | open | Concrete terminal indication or equivalent completeness mechanism; current adapter accepts validated EOF. |
| G3 | open | Proven serving identity and message usage semantics, mapping, and availability; current response echoes and estimates. |
| G4 | open | Proven output cap and effort mapping, or a ratified client baseline that omits unsupported effort. |

For each later transition, add the exact source artifact and location, code commit and client baseline, contract amendment, deterministic results, and separately authorized live result. A source or result that is missing stays missing. The retained experimental live record cannot verify these gates retroactively. No new upstream investigation or client run was performed for this amendment.

### Confirmation

You accepted the complete revised GA amendment on October 4, 2026, after choosing an update in place, named local sources, an independent model review, and application of its four recommended corrections. This confirms the repair design and promotion gates. The review repair slice is ready to build; protocol implementation still depends on the later evidenced wire amendment. The lifecycle status remains `In Progress` until feature completion is actually verified. This acceptance does not authorize another live run.

### Independent GA amendment review

At your request, GPT-6 Sol reviewed the draft written by GPT-6 Astra. It read the three spec files and relevant local code without editing files, fetching reference links, running tests, or accessing credentials. It found the explicit G1 through G4 research dependencies honest and found no material gap in evidence ownership, candidate binding, or gate transitions. It identified four gaps in the buildable repair slice.

You selected the recommended fixes. The revised design observes completion only after a successful final client response write with `end_turn`, tied to the issuing request and task turn, with required results and no outstanding calls. It checks the boundary test's absence after the first turn and presence after the followup edit before allowing the second test command. These checks turn timing and completion into concrete runner evidence rather than deductions from a generator return or final file contents.

The mutation now requires an exact patchable upper branch in the final `Clamp` source. Only its return identifier changes in the disposable copy. A refactor outside that shape produces incomplete evidence instead of an invented mutation. This narrows the accepted fixture shape and can reject an otherwise valid coding result, but it gives the regression assertion a reproducible meaning.

The allocation check now measures cumulative allocated bytes per operation through the real production accumulator. It uses exactly 64 KiB in 64 byte fragments, includes initialization and final materialization, and permits at most 1 MiB separately for text and tool input. Fixture construction and protocol parsing are outside that measurement. The old copying pattern exceeds 32 MiB for this input, while the larger stream and CPU profile cases remain supplementary evidence. This is a focused regression budget, not a claim that a complete inference uses only 1 MiB of memory.

These corrections were applied with your approval, and you subsequently accepted the complete revised amendment. They do not close the implementation review findings or verify a GA gate. No second independent review of the corrected text has been recorded.

## GA wire investigation, October 4, 2026

### Context and outcome

You chose an update to spec 0004 and named sources with evidence locations. This is an enhancement investigation on the existing Go backend and Tracer Bullet path. Its outcome is `needs_evidence`: the inspected sources do not supply a complete GA wire contract. You accepted this investigation disposition after independent review; no new protocol behavior is ratified.

The checkout began clean at `d60b949af7ea9d41ced2a3faa5bd7043a9928b00`, with 77 Go files and zero commits behind `origin/main` after fetching. The later repair review in `docs/reviews/2026-10-04-feat-claude-code-bridge-design.md`, section `Repair review, 2026-10-04`, approves the repair slice and closes all three earlier findings. `CHANGELOG.md`, section `Fixed`, records the repairs. Earlier pending review statements in this rationale describe their historical checkpoint.

The offline client record `internal/gateway/testdata/claude-code-2.1.289-offline-loop.json` supplies the current client version, artifact digest, synthetic assertions, and limits. This investigation did not run those checks again. The earlier live evidence stays bound to its original client and runner; it does not verify any GA gate.

### Source identity and inspection boundary

A fresh SHA 256 calculation of `/Applications/Kiro CLI.app/Contents/MacOS/kiro-cli-chat` returned `2118bd89d96830a4f0e0884e4f4071fb94e4b6036c757126afe9e3f80c9d90c6`, matching the recorded Kiro CLI `2.8.0` artifact. Addresses below are arm64 file virtual addresses, not process addresses. The retained disassembly is under `/private/tmp/kiro-protocol-research`; this local scratch directory is not required build input and may disappear. The pinned binary digest, named functions, and addresses identify the binary locations. Reproducing the inspection requires access to that matching binary or adequate retained excerpts; this document does not archive either artifact.

The main thread inspected production event and response handling, reread the prior static call trace, checked retained usage decoding, and resolved input serializer constants directly from binary bytes. A read only helper cross checked the production request and completion paths against the retained disassembly. No client process was launched or attached, no real credential or account settings were read, and no inference endpoint was contacted.

| Gate | Source location | Established fact | Remaining unknown |
|---|---|---|---|
| G1 | `internal/kiro/request.go`, `encodeRequest`; native conversation serializer `0x100861b10`, message union serializer `0x10085bf08`, user context serializer `0x100841890`; prior static trace in spec 0003 | The gateway appends history system text to its paired user and prefixes top level system text to the first user. The inspected native typed shapes supply user and assistant variants and context fields, without an identified distinct system role. | A field and protocol guarantee preserving priority and positional replacement. Generic documents or another operation may expose more, but none was established. |
| G2 | `internal/kiro/adapter.go`, `Generate`; `internal/kiro/events.go`, `complete` and `emitTools`; native stream unmarshaller `0x10082eb58`, assistant decoder `0x1008771e4`, message metadata decoder `0x100869d10`, metadata decoder `0x1008686d8` | The adapter accepts clean frame completion, semantic content, complete tools, and successful cleanup, then returns `InferredCleanEOF`. The identified metadata fields and per tool stop do not supply the required normal generation terminal contract. | A positive generation terminal indication or equivalent mechanism that rejects every earlier valid stream prefix. The native dry run success branch is not such evidence for normal inference. |
| G3 | `internal/kiro/events.go`, `observe`; `internal/bridge/response.go`, `Message`; native assistant decoder `0x1008771e4`, token usage decoder `0x1008228a0` | The gateway checks an optional `modelId` against the request and discards recognized usage. Its client envelope echoes the requested model and computes byte estimates. The native decoder has actual comparisons for `uncachedInputTokens`, `outputTokens`, `totalTokens`, cache counters, and normalized or percentage fields. | Whether identity reports the actual serving model; counter units, measurement basis, totals versus deltas, cache and reasoning accounting, ordering, and availability on this service/model. Field names alone cannot define these. |
| G4 | Native input serializer `0x100810d48`, key writer call `0x100810e4c`; `RealApiClient::send_message` calls at `0x10238a464` and `0x10238a4b8`; spec 0003 output control investigation; `internal/kiro/request.go` | The native path serializes a generic `additionalModelRequestFields` document. Prior documentation makes `max_tokens` a candidate. Production intentionally omits that document and discards the accepted effort hint. | Actual service acceptance and enforcement for the exact Opus mapping, counting basis, cap terminal reason, and an effort equivalent. A request failure with several differing fields cannot isolate a rejected control. |

The usage trace merits care. At `0x100822ab4` through `0x100822af8`, key comparisons recognize `uncachedInputTokens`; at `0x100822c94` through `0x100822cbc` they recognize `outputTokens`. Builder error constants referenced at `0x10082336c`, `0x100823388`, and `0x1008233b0` name required native structure members `uncached_input_tokens`, `output_tokens`, and `total_tokens`. Those underscore names are native error text, not JSON wire names. Native structure validation does not prove that a usage event is always emitted, that these counters mean Anthropic message tokens, or that metadata ends generation. No aggregation formula or client usage mapping is selected from this evidence.

### Public source check

A bounded read only research pass checked official Kiro and AWS material on October 4. Kiro's `Models` documentation describes model selection, Auto routing, and model dependent effort controls. AWS's `Amazon Q Developer permissions reference` identifies the legacy `codewhisperer:GenerateAssistantResponse` action, and its `Prompt log examples` show audit records. These establish product features and historical service context, not a current request and response schema for the selected Kiro runtime operation. None of these findings closes a gate. No result from the public search is used as proof that an undocumented mechanism does not exist.

The earlier spec 0003 `Output control evidence` section remains the named source for the documented `max_tokens` candidate and its connected native serialization path. Bedrock or Anthropic native API semantics cannot be imported into Kiro's generic document without evidence that this operation forwards and honors them. The public research did not establish such a contract. New source names are retained here as requested; prior reference links remain historical.

### Options and recommendation

| Option | Benefit | Cost and disposition |
|---|---|---|
| Fix the current path when new source evidence supplies the missing semantics | Reuses the implemented transport, credential boundary, client tool ownership, and synthetic path. | Completion time is unknown and local code cannot manufacture server guarantees. Recommended continuation, starting with G1 and G2, because it preserves your accepted product and GA requirements. |
| Prove another native Kiro access path alongside the experimental adapter | Could supply a stronger contract while retaining the working experiment for comparison. | No qualifying path is identified. It needs a separate decision for protocol, authentication, destination, and ownership before implementation. Runner up if new evidence identifies a candidate. |
| Replace the adapter directly or weaken the promotion requirements | A genuinely compatible replacement could remove the protocol gap; a weaker requirement could describe today's working experiment. | No compatible replacement is established. Wrapping the native CLI does not by itself preserve Claude Code tool execution, and weakening GA conflicts with the confirmed promotion decision. Neither change is selected. |

(basis: the exact source inventory above, the confirmed G1 through G4 requirements, spec 0001's client tool ownership, and the independent repair review)

### Handoff

All four gate states remain `open`, with source outcomes recorded as `unknown`. No state becomes `contract_recorded`, and no feature, GA design checkbox, or live plan becomes ready from this investigation. The smallest useful next input is an identifiable source for native instruction priority and authoritative completion on this operation. A provider schema, a newly connected binary path, or a concrete alternative access path would change that assessment. Repeating the prior successful coding task would not.

This update adds no dependency, service, storage entity, migration, secret, or implementation skill. Existing experimental interfaces, errors, ownership, and limits remain the runtime contract. When source evidence exists, architecture still owns the complete mapping and failure rules before a builder writes GA protocol code. Content review of this disposition and later ratification of a buildable wire contract are distinct steps.

### Independent wire investigation review

At your request, GPT-6 Sol reviewed the draft written by GPT-6 Astra. It approved the amendment as an honest `needs_evidence` investigation disposition and found no material decision completeness or soundness gap. It checked the spec handoff, scope checkbox, local request and event code, prior source evidence, and binary digest. It performed no tests, client runs, account reads, network access, or edits.

The author applied its two wording corrections: repair review approval closes findings while separate deterministic records support acceptance criteria, and reproducing binary inspection requires the matching artifact or retained excerpts. Neither correction changes the protocol or gate state. You accepted the revised investigation record on October 4, 2026. The GA wire milestone remains incomplete, GA implementation remains blocked, and the feature lifecycle stays `In Progress`. This acceptance grants no live launch authorization.

## Further native protocol evidence, October 4, 2026

You asked to continue after accepting the investigation disposition at `d0ec39b`. This pass adds source observations under that decision. It changes no protocol requirement, gate state, runtime behavior, or live authorization. The inspected native binary still has SHA 256 `2118bd89d96830a4f0e0884e4f4071fb94e4b6036c757126afe9e3f80c9d90c6`, freshly checked again. LLDB ran with startup files disabled and only created a file target for static disassembly. No target was launched or attached. No account data, native client process, public web fetch, or inference request was involved in this continuation.

### G1, local prompt input is not a service role

The internal `agent::agent::agent_loop::protocol::SendRequestArgs` serializer at `0x102336578` writes `messages`, `tool_specs`, and `system_prompt`. Its field accesses are at offsets `0`, `0x18`, and `0x30` respectively; the key and field pairing for `system_prompt` is at `0x102336620` through `0x102336634`. This establishes a local agent request shape. It is not the `GenerateAssistantResponse` input schema.

The selected v2 adapter implements the shared `Model::stream` interface at `0x10284202c`. Its conversation path includes `RtsModel::converse_stream_rts` at `0x102089134`, `make_conversation_state` at `0x1027a69c8`, `format_user_content` at `0x1027a5ce0`, and `extract_tool_results_and_images` at `0x1027a6378`. The service message conversions are at `0x10279b71c` and `0x10279bac0`, before the generated user context serializer at `0x100841890`.

This narrows the next static question to how the local `system_prompt` input is handled at the model adapter boundary. The pass did not establish a field to wire connection for that input, a distinct service role, or instruction priority. It also did not establish that `additionalContext` supplies those semantics. A local field name must not be promoted into a new service field or a claim that native instructions are preserved. G1 remains open.

### G2, complete modeled event dispatch and current receiver

The complete `ChatResponseStreamUnmarshaller::unmarshall` function starts at `0x10082eb58` and ends before `0x10082fe68`. It has 17 payload decoder branches plus one inline dry run event. These are modeled event variants, not 18 successful completion cases.

| Payload decoder type | Decoder address |
|---|---|
| `CodeEvent` | `0x1008500b0` |
| `InteractionComponentsEvent` | `0x10084a294` |
| `InvalidStateEvent` | `0x1008349c4` |
| `CodeReferenceEvent` | `0x100875e94` |
| `ToolResultEvent` | `0x10083fad4` |
| `CitationEvent` | `0x10083e684` |
| `ToolUseEvent` | `0x100860d68` |
| `FollowupPromptEvent` | `0x100835400` |
| `MessageMetadataEvent` | `0x100869d10` |
| `AssistantResponseEvent` | `0x1008771e4` |
| `SupplementaryWebLinksEvent` | `0x10086cfc4` |
| `ContextUsageEvent` | `0x100875800` |
| `MetadataEvent` | `0x1008686d8` |
| `IntentsEvent` | `0x100833130` |
| `ReasoningContentEvent` | `0x100876650` |
| `MeteringEvent` | `0x100833964` |
| `DocumentCitationEvent` | `0x100824aa4` |

The inline comparison at `0x10082f0a4` through `0x10082f0dc` recognizes exactly `dryRunSucceedEvent` and branches to `0x10082f74c`, which constructs a variant without a payload decoder. This does not establish ordinary generation completion. `InvalidStateEvent` is a modeled variant; its name does not make it successful completion. The default branch at `0x10082f5b0` writes a distinct dispatch value. This pass does not classify that value as an error without a further enum or consumer trace. None of these findings expands the gateway's accepted event vocabulary.

For the selected `chat_cli_v2` implementation, `ResponseParser::next` at `0x10239650c` calls `Receiver::next_message` at `0x102396c08`, targeting `0x10239cd24`. That receiver calls `SdkBody::poll_frame` at `0x10239d128`. Its body end branch at `0x10239d418` constructs the stream end result with discriminant `0x8000000000000006`. The parser compares that exact result at `0x102396c18` through `0x102396c24`. The event path separately unmarshals at `0x102396dfc` and converts the model event at `0x102397130`.

This connects HTTP body EOF to the current native parser, rather than relying on the older `chat_cli::SendMessageOutput::recv` implementation at `0x100f75074`. No modeled ordinary generation terminal event was identified. The pass did not prove the higher consumer's final state semantics, arbitrary server extension behavior, or absence of every possible completeness mechanism. G2 remains open; neither this native EOF handling nor the dry run variant meets its guarantee.

### G4, schema selected effort overrides

This pass establishes a concrete native effort selection path that the earlier inventory lacked:

| Step | Static source and observation |
|---|---|
| Model information supplies additional field data | `ModelInfo::from_api_model` at `0x1023f347c` calls `document_to_value` at `0x1023f35dc`. `RtsState::set_model_info` at `0x1027a7d5c` copies the resulting optional `AdditionalModelFields`. The native model builder exposes `set_additional_model_request_fields_schema` at `0x10073c52c`. This does not reveal the selected account's actual model schema. |
| Select an effort property from the schema | `AdditionalModelFields::effort_path` at `0x1022f53ec` tries `output_config.effort` at `0x1022f5420`, then `reasoning.effort` at `0x1022f545c`, through `resolve_schema_node`. Literal bytes at `0x11b85e8ca` (20 bytes) and `0x11b85e8de` (16 bytes) establish those exact paths. |
| Resolve nested schema properties | `resolve_schema_node` at `0x1022f5e7c` splits the path on `.` and looks up `properties`, whose ten bytes are at `0x11b85e8ef`. This is schema lookup, not a literal JSON key containing a dot. |
| Set the native effort override | `RtsState::set_effort` at `0x1027a7aa8` repeats that ordered lookup and calls `set_typed` at `0x1027a7c84`. Its error literals report an absent additional field schema or an unsupported effort configuration. A path's existence alone does not establish that every effort value is accepted. |
| Build nested override JSON | `set_typed` at `0x1022f6260` resolves the schema, then splits the path and constructs nested overrides at `0x1022f69a0` through `0x1022f6afc`. The native `AdditionalModelFields` serializer identifies its separate `schema` and `overrides` members at `0x1022f51a4` and `0x1022f5244`. |
| Carry overrides to the service request | `make_conversation_state` reads additional fields at `0x1027a7200` and selects the optional overrides at `0x1027a724c`. `RealApiClient::send_message` converts their value at `0x10238a464` and sets `additionalModelRequestFields` at `0x10238a4b8`. The previously traced input serializer writes that member at `0x100810e4c`. |

The resulting candidate locations are nested `additionalModelRequestFields.output_config.effort` or `additionalModelRequestFields.reasoning.effort`, conditional on the selected model schema. They are not two fields to send together, a service fallback policy, or an accepted gateway mapping. No selected schema was read, no `high` value was exercised through this service, and no equivalence to the client's effort semantics was established. The `max_tokens` counting basis and terminal reason remain unresolved. G4 stays open.

### Remaining work

G1 now has a concrete local model boundary to trace. G2 has a bounded complete event dispatch inventory and a current native EOF path. G4 has an exact schema selected override path. These are stronger investigation inputs, not gate passes. G3 was not expanded by this continuation. The accepted next decision still requires evidence for native instruction priority and authoritative completion before a buildable GA wire mapping; another ordinary coding run would not settle those source questions.

### Independent source check

GPT-6 Sol checked this supplement against the matching binary using static disassembly. It confirmed the request serializer fields, the 17 payload decoders plus the inline dry run branch, the current receiver's body end path, and the ordered effort property lookup. It found no new decision gap or accidental gate closure. The author applied its wording correction so the index identifies the local prompt field and model boundary without claiming their data flow was proved. This is an evidence supplement to the accepted disposition, not a new wire decision. The review performed no writes, tests, client launches, account reads, or network requests.

## Bundled source and schema investigation, October 5, 2026

You asked to continue the blocked actions. The next pass inspected the already installed software package `@kiro/agent` version `0.3.234`, rather than only native machine instructions. Its `dist/server/acp-server.js` still hashes to `233e4dec77cd538e35b691d1fd0e12ca64a7c90d720c4dbf6979f21b4c45482f`. The full path is `~/Library/Application Support/kiro-cli/kas/node_modules/@kiro/agent/dist/server/acp-server.js`; this is installed software, not account or conversation data. No agent process, authentication callback, or network operation was invoked.

### Connected source findings

| Gate or operation | Exact bundle location | Finding and limit |
|---|---|---|
| G1, custom agent prompt | `getSystemPromptMessages`, lines 415475 through 415487; `ContextChatMessage`, lines 118404 through 118465; `ModelContext.serializeMessage`, lines 326704 through 326751 | The custom system text becomes `ContextChatMessage.fromHuman().withText(...).withForcedRole()`. The flag preserves the human role; serialization constructs `HumanMessage`. |
| G1, service conversion | `QDeveloperConverse.convertToGenerateAssistantMessages`, lines 431969 through 432091 | Human messages become `userInputMessage.content`. Assistant and tool messages have their own branches; unsupported message types throw. No distinct system role is produced by this path. This is concrete native prompt flattening, not proof of global server incapability or the exact Rust path's treatment. |
| G2, completion | `_streamResponseChunksOnce`, lines 431735 through 431940; `AgentExecution.modelStream2`, lines 326161 through 326195 | The adapter consumes the response iterator and reports success after it exhausts. The execution layer closes its local response stream after iteration. No positive ordinary generation terminal signal is required in this inspected path. Local success or turn end notifications cannot be cited as upstream terminal evidence. |
| G3, counters | `parseBedrockChunkEvent`, lines 431465 through 431485 | The native parser maps `uncachedInputTokens` to its local `inputTokens` and carries output and cache counts separately. It defaults missing numeric members to zero and suppresses metadata unless input or output is positive. This is a client policy, not measured counter semantics or a missing value policy adopted by this gateway. |
| G4, effort and reasoning | `parseEffortLevels` and `buildEffortRequestFields`, lines 324846 through 324884; request options at line 326158 | The native bundle selects the first nonempty effort enum from `output_config.effort`, then `reasoning.effort`. The first path also sends `thinking: {type: "adaptive", display: "summarized"}`. Therefore blindly copying that path would introduce reasoning behavior beyond today's accepted disabled thinking baseline. The second path sends a nested reasoning effort value. Neither is verified for this account's exact selected model. |
| Catalogue data source | `listAvailableModels`, lines 324783 through 324842; configuration at lines 455697, 455800 through 455806, and 461785 through 461789 | The installed client obtains `additionalModelRequestFieldsSchema` from the bearer control plane's model catalogue, using the `management` regional host and selected profile. Native pagination and retries are broader than the accepted one request probe. |
| Catalogue wire protocol | Runtime configuration at lines 323944 through 323966; RPC serializer at lines 127412 through 127462; AWS JSON wrapper at lines 137498 through 137513 | The bundled control plane client selects bearer authentication and AWS JSON 1.0. Its actual operation uses root path POST and target `KiroControlPlaneBearerService.ListAvailableModels`, even though its operation schema at line 323890 contains a GET binding. |

These readable source paths give a bounded negative result for native role and terminal guarantees: neither inspected implementation supplies the strict G1 or G2 contract. They do not establish that an undocumented extension or another Kiro operation could never supply it. The Rust followup also traced its consume loop's normal exit at `0x10208bc80` through `0x10208c2fc`, without an emitted terminal reason on that branch. The generic agent finalization path remains distinct from received protocol evidence.

### Synthetic request characterization

The neighboring `@amzn/kiro-control-plane-bearer-client` runtime configuration matches the bundled control plane protocol. Its `dist-cjs/runtimeConfig.shared.js` SHA 256 is `c8911051a36f92d5ed56cb7a9ad6b5f43b418eaf808a08d8eee9b2abb82fd056`; `dist-cjs/schemas/schemas_0.js` is `7f6cb91a745505f434b9b3ba1414d7f9c7c9055f885f61b93ded60bfdafbc619`. This finding does not erase the prior mismatch for the separate runtime inference package recorded in spec 0003.

Node `v24.20.0` ran only the control plane SDK with an injected in memory request handler and invented token and profile. Its permission mode allowed filesystem reads only from the installed `kas/node_modules` software tree. Region, endpoint, token, retry limit, FIPS, dual stack, defaults mode, and retry mode were explicit; no native auth provider was used. The final check verified one bearer header case insensitively, one handler call, and zero network dispatches. It produced `POST`, path `/`, empty query, `application/x-amz-json-1.0`, the exact target above, and body `{"origin":"KIRO_CLI","maxResults":100,"profileArn":"arn:aws:codewhisperer:us-east-1:111111111111:profile/synthetic"}`. The handler returned an invented empty model array. This proves serialization only, not remote acceptance or model access.

### Selected next slice and alternatives

You accepted a development only probe for one selected model schema response, with exact bounds and evidence fields in `index.md`. It acquires new service supplied schema evidence without sending prompts or tool work. The accepted cost is a new authenticated management operation and a narrowly expanded retained evidence policy. It may still return no useful mechanism for G1 or G2. No GA gate is relaxed, and a catalogue schema does not establish actual inference enforcement.

The alternative is to stop account investigation until a provider contract or different operation supplies the missing role and completion guarantees. That avoids another account request but leaves no selected source acquisition step. Another ordinary inference loop is not recommended: successful output cannot establish the missing role hierarchy or detect an arbitrary clean stream prefix. Wrapping the native agent would also change Claude Code's ownership of tools and conversation, and is not selected.

Your acceptance makes offline preparation of the AC-16 slice ready. The final live plan still needs implemented and tested code, a clean commit, a plan digest, independent review, and one explicit launch authorization. No account has been read and no network request has run during this pass.

### Independent proposal review

GPT-6 Sol checked the source evidence and proposed probe without account access, client execution, tests, network requests, or edits. It found one decision gap: the projection must specify whether missing or malformed `properties`, or an unresolved `$ref`, means an absent path or an unknown path. You selected the recommended unknown result on October 5, 2026. The revised rule reports absence only when a valid parent `properties` object lacks the next key. Uninspectable paths remain unknown and can coexist with a structural `schema_observed` outcome, without any capability claim. References are never followed, including when they have sibling properties. The reviewer found the account boundary, fixed operation and hosts, request budget, output projection, and separation between design acceptance and live authorization otherwise coherent. The same reviewer checked the revised rule and verification cases and confirmed the finding closed. You then accepted the complete AC-16 design on October 5, 2026. Offline preparation is ready; implementation, verification, and a separately authorized live run remain pending. The spec and feature stay in progress, and G1 through G4 remain open.

## GA wire assessment after catalogue evidence, October 5, 2026

### Context and source boundary

You chose to update spec 0004 in place while preserving its experimental contract and evidence history. This assessment inspected the repository at `165647c27cb446424f36a2efd9350e04942ee953`, with no commits behind `origin/main` after fetching. It uses the retained report rather than repeating the account operation. No client or native agent was launched, no real account store was read, and no catalogue or inference request was made during this assessment.

The premise requiring correction is that a declared request field completes a GA wire contract. The catalogue supplies candidate controls, while the accepted contract also requires instruction priority, authoritative completion, serving identity, and measured usage. Those facts cannot be manufactured by implementing a serializer.

| Source | What this assessment uses |
|---|---|
| `internal/kiro/testdata/schema-run-2026-10-05-02.json` | `schema_observed`, one dispatch, complete cleanup, exact selected model and region, and the eight bounded field observations |
| `internal/kiro/testdata/schema-plan-2026-10-05-02.json` and the retained verification entry | The report's code commit `7441f88922bb7fb827c52aaf83a346a0859da690`, plan digest `5cbe79883a122980d81cf4ff65fbd4750f4072155058705158440c59a9c50ccc`, and consumed launch provenance |
| Bundled source and schema investigation above | Previously connected prompt flattening, stream exhaustion, candidate counters, and the native effort builder's adaptive thinking addition; these were not freshly inspected native artifacts in this pass |
| `internal/kiro/request.go`, `encodeRequest` | Current production requests still flatten instructions and omit additional model controls |
| `internal/kiro/events.go`, `observe` and `complete`; `internal/kiro/adapter.go`, `Generate` | Current production checks optional model equality and numeric usage shapes, then completes from clean EOF, semantic validation, and cleanup |
| `internal/bridge/request.go` and `response.go` | Current client validation admits advisory limits and only ignored high effort; responses echo the requested model and estimate usage |

The report declares integer `max_tokens` with minimum 1024 and maximum 128000. It retains no default for that field. `output_config.effort` declares `high`, `low`, `max`, `medium`, and `xhigh`, with default `medium`. `thinking.type` declares `adaptive`; `thinking.display` declares `omitted` and `summarized`. The explicit paths `system`, `system_prompt`, `messages`, and `reasoning.effort` are absent under the probe's traversal rule. These observations apply to the selected record in the retained run. They are not a service capability matrix or evidence for another model or region.

The projection intentionally excludes required field lists and general schema constraints. It cannot settle effort without thinking, default application by inference, or valid combinations of controls. A display value named `omitted` does not establish that thinking is disabled. Likewise, the requested catalogue model name cannot supply the serving identity of a later response.

### Options and recommendation

| Option | Benefit | Cost and disposition |
|---|---|---|
| Preserve the current path and seek a source for G1 and G2 | Keeps the working experiment and accepted architecture while testing the prerequisites most likely to determine feasibility. | No buildable GA slice or completion date follows from this report. Recommended. |
| Investigate a replacement adapter alongside the experiment | Could retain Kiro access and Claude Code tool ownership if another operation supplies the missing guarantees. | No qualifying operation is identified yet. Requires a separate access path decision and evidence before implementation. Runner up. |
| Replace the adapter directly or wrap the native agent | Native software already integrates its own service path and model controls. | The inspected native path still flattens instructions and completes on exhaustion; wrapping its conversation and tools would also change the accepted ownership boundary. No evidenced GA benefit justifies that migration here. |
| Implement catalogue controls as the GA candidate now | Provides concrete field names for a smaller control experiment. | Acceptance, enforcement, and field dependencies are unproved, and G1 through G3 remain unresolved. This cannot satisfy AC-14; a separate experimental enhancement would need its own design. |

The recommended disposition remains `needs_evidence`, with every gate `open`. The next useful investigation needs a named source connecting instruction and terminal semantics to an operation, rather than another ordinary successful coding sample. The alternative adapter option becomes actionable only when such a source identifies a concrete candidate. No new provider, endpoint, account operation, or weaker GA requirement is selected.

### Review and confirmation

You accepted this assessment on October 5, 2026. Updating the evidence does not ratify a GA wire contract, change any acceptance criterion, or complete the GA design checkbox. At your request, GPT-6 Sol reviewed the amendment against the retained report, plan, and relevant production code. It found no material decision gap or accidental gate closure. It identified a stale verification overview that still called AC-16 pending; that summary now records completion and consumed authorization. The review performed no edits, tests, client execution, account access, or network requests. Your subsequent content confirmation accepts the assessment and recommended evidence work. The spec and feature remain in progress, with AC-14 incomplete and all four gates open. The existing reference section and historical evidence remain intact; no external reference was fetched for this amendment.

## References

**Project sources**

1. [Project context](../../../AGENTS.md), stack, boundaries, diagnostics, and checks.
2. [Scope](../../scope/scope.md), first coding loop and Tracer Bullet delivery.
3. [Spec 0001](../0001-stack-architecture/index.md), accepted architecture and stronger compatibility goals.
4. [Spec 0002](../0002-local-configuration-credentials/index.md), settings and exact credential snapshot rules.
5. [Spec 0003](../0003-first-claude-code-loop/index.md), [live evidence](../0003-first-claude-code-loop/verify.md), and [request plan](../../../internal/kiro/testdata/probe-plan.json), observed candidate and limitations.
6. [Go security skill](../../../.agents/skills/golang-security/SKILL.md) and [Go concurrency skill](../../../.agents/skills/go-concurrency/SKILL.md), bounded input, fixed authority, stream ownership, and cleanup.
7. `docs/reviews/2026-10-04-feat-claude-code-bridge-design.md`, current review findings and their limits.
8. `internal/gateway/testdata/claude-code-2.1.287-offline-loop.json`, `internal/kiro/testdata/bridge-run-2026-10-02-04.json`, and `internal/kiro/testdata/bridge-plan.json`, recorded client evidence and the exact experimental fixture.
9. `internal/bridge/response.go`, `internal/bridge/bridge.go`, `internal/kiro/request.go`, `internal/kiro/events.go`, `internal/kiro/adapter.go`, `internal/kiro/live_bridge_test.go`, and `internal/gateway/client_retry_offline_test.go`, implementation inspected for the GA amendment.

**Practices**

Explicit trust boundaries, bounded resource ownership, deterministic local verification separated from live compatibility evidence, and no replay of visible tool work.

**Verified official links**

1. [Claude Code environment variables](https://code.claude.com/docs/en/env-vars), checked October 2, 2026.
2. [Claude Code settings](https://code.claude.com/docs/en/settings), checked October 2, 2026.
3. [Streaming Messages](https://platform.claude.com/docs/en/build-with-claude/streaming), checked October 2, 2026.
4. [Define tools](https://platform.claude.com/docs/en/agents-and-tools/tool-use/define-tools), checked October 2, 2026. The auto and none selection boundary uses this page's described model restrictions. (basis: official tool definition reference)
5. [Effort](https://platform.claude.com/docs/en/build-with-claude/effort), checked for the client contract amendment on October 2, 2026.
6. [Beta headers](https://platform.claude.com/docs/en/api/beta-headers), checked for the client contract amendment on October 2, 2026.
7. [System messages inside a conversation](https://platform.claude.com/docs/en/build-with-claude/mid-conversation-system-messages), checked for the system history amendment on October 2, 2026.
8. [Claude Code CLI reference](https://code.claude.com/docs/en/cli-reference), checked for the documented prompt flags on October 2, 2026.
9. [Agent SDK system prompts](https://code.claude.com/docs/en/agent-sdk/modifying-system-prompts), checked for prompt construction and conversation context distinctions on October 2, 2026.

These links are retained for human inspection. Later build and review work uses this recorded evidence and the offline client exercise rather than fetching them again by default.
