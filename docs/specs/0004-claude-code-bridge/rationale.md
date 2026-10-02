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

No real credential store, profile, conversation, or account configuration was read during this design. No live request or native Kiro chat was launched. Version and help commands were the only client executions. The repository had 42 source files and no commits missing from `origin/main` after the freshness check.

## Independent review, October 2, 2026

At your request, `gpt-6-sol` performed a read only cross check for decision completeness and soundness. It identified three gaps: an ambiguous start for the live budget, incomplete source error mappings, and a dispatcher wrapper unable to observe every failure that must stop the run. It found no further material contradiction in the request, tool, or streaming transformations against the local feasibility contract.

You selected the recommended fixes. The revised spec starts the clock at the accepted launch flag before runtime preflight or setup, maps every source error and cancellation cause, and supplies a typed boundary observer and shared run controller alongside the final dispatcher guard. Expected faults are bound to their case and request so client retries cannot be mistaken for an authorized next case. The verification plan now checks these rules. This approval resolves the review findings; final acceptance of the whole spec is a separate step.

The same reviewer confirmed the source mapping and observer corrections, then identified a cleanup edge in the clock amendment. Canceling active work only at the absolute deadline would leave no time for its already specified five second cleanup bound. The author reserved the last five seconds of the approved 20 minute total, stopping work at 19 minutes 55 seconds, and added the corresponding boundary test. This preserves the chosen total rather than adding a cleanup extension.

You then accepted the complete revised spec on October 2, 2026. The design is confirmed and linked to scope feature 4, with the experimental build milestones ready. This ratification neither claims implementation completion nor authorizes a live run. The spec remains `Proposed` until development begins, and the full feature retains its separate GA completion requirements.

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

These links are retained for human inspection. Later build and review work uses this recorded evidence and the offline client exercise rather than fetching them again by default.
