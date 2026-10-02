# Reasoning for the experimental coding bridge

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

No assumption is made that the four beta names are harmless on arbitrary bodies. In particular, `interleaved-thinking-2025-05-14` cannot admit thinking history, and `mid-conversation-system-2026-04-07` cannot admit system messages within history. The complete body contract remains the gate. If later client turns need those features, development returns to architecture with structural evidence instead of stripping them.

### Verification impact

The amended matrix checks both endpoints, all repeated header values, malformed and unknown controls, and absence of account reads on failure. It also proves that the tolerated fields do not affect normalized content, estimates, upstream semantic requests, or fixed response labels. The first real client success and full tool exchange remain pending. Review and acceptance of this amendment do not authorize a live run.

### Independent amendment review

At your request, `gpt-6-sol` reviewed the amendment for decision completeness and soundness. It found no material decision gaps. The literal effort shape, beta token set, parser bounds, validation order, discard boundaries, upstream omissions, fixed errors and policy headers, launch flag, and return to architecture for future unsupported shapes are specified.

The reviewer noted that the newer exact enum and token observations are retained here, while the original JSON fixture records presence only. A second sanitized fixture could improve auditability, but the reviewer did not consider it a blocker: the implementation contract is explicit and the full offline exercise through production handlers remains required. No new capture or fixture was created by the reviewer. You then accepted the amendment on October 2, 2026. That acceptance confirms its design and verification plan. The feature linked spec remains `Proposed` until implementation begins; live execution still requires its separate concrete run review.

## References

**Project sources**

1. [Project context](../../../AGENTS.md), stack, boundaries, diagnostics, and checks.
2. [Scope](../../scope/scope.md), first coding loop and Tracer Bullet delivery.
3. [Spec 0001](../0001-stack-architecture/index.md), accepted architecture and stronger compatibility goals.
4. [Spec 0002](../0002-local-configuration-credentials/index.md), settings and exact credential snapshot rules.
5. [Spec 0003](../0003-first-claude-code-loop/index.md), [live evidence](../0003-first-claude-code-loop/verify.md), and [request plan](../../../internal/kiro/testdata/probe-plan.json), observed candidate and limitations.
6. [Go security skill](../../../.agents/skills/golang-security/SKILL.md) and [Go concurrency skill](../../../.agents/skills/go-concurrency/SKILL.md), bounded input, fixed authority, stream ownership, and cleanup.

**Practices**

Explicit trust boundaries, bounded resource ownership, deterministic local verification separated from live compatibility evidence, and no replay of visible tool work.

**Verified official links**

1. [Claude Code environment variables](https://code.claude.com/docs/en/env-vars), checked October 2, 2026.
2. [Claude Code settings](https://code.claude.com/docs/en/settings), checked October 2, 2026.
3. [Streaming Messages](https://platform.claude.com/docs/en/build-with-claude/streaming), checked October 2, 2026.
4. [Define tools](https://platform.claude.com/docs/en/agents-and-tools/tool-use/define-tools), checked October 2, 2026. The auto and none selection boundary uses this page's described model restrictions. (basis: official tool definition reference)
5. [Effort](https://platform.claude.com/docs/en/build-with-claude/effort), checked for the client contract amendment on October 2, 2026.
6. [Beta headers](https://platform.claude.com/docs/en/api/beta-headers), checked for the client contract amendment on October 2, 2026.

These links are retained for human inspection. Later build and review work uses this recorded evidence and the offline client exercise rather than fetching them again by default.
