# Scope: Kiro Gateway

Kiro Gateway lets you use Claude Code with the Claude models available through your Kiro account. The goal is dependable daily coding, with Claude Code controlling the prompts, conversation, permissions, and tool execution.

**Build approach:** Tracer Bullet (prove one real coding loop through every layer, then strengthen it in working slices).
**Workflow:** Beta (`/check verify`, then `/test` after implementation). Credential handling and the first protocol bridge carry a GA override, adding a separate model review and documentation.

These are recommendations you can change. Each feature starts with one suggested entry command. Design fills in the build milestones and verification steps later.

## Product boundary

Your stated priorities are Claude Code as the coding agent, Kiro as the source of model access, support for Opus and Sonnet, and reliability that existing gateways have not given you. A CLI is the primary product. A UI is a possible extension.

**Confirmed choices:** serve your own daily work first, then a free open source release. Start with a local CLI on macOS and one Kiro account, supporting Opus and Sonnet. Prioritize dependable daily coding, including streaming, tool use, long conversations, cancellation, credential renewal, and clear failures. Keep diagnostics local, omit telemetry by default, and use clear English terminal output that remains readable without color. Defer the UI, additional platforms, and advanced capabilities. Remote hosting, billing, account pooling, and other client protocols are outside this first release. Keep this scope free of reference sections. You accepted the recommended scope choices on September 30, 2026.

**Still open:** your Kiro sign in method, concrete failures you have experienced, deadline, budget, and team capacity. The free text question did not have a recommended answer, so these facts remain unknown. Authentication design needs the sign in method before its supported path can be settled. The next step is to explain the proposed solution architecture before implementation.

**Feasibility boundary:** the connection must let Claude Code receive a model tool request, execute that tool under its own permissions, return the result, and continue the conversation. The first design and working slice need evidence that the chosen Kiro access path supports this. Record any changes to instruction roles, context, or model controls. An ordinary text response alone does not establish compatibility. Equivalent behavior to direct Claude access remains something to measure.

**Suggested release evidence:** complete three hour long coding sessions spanning both supported model families, including file edits and test commands, without a gateway caused failure. Repeatable checks should cover credential expiry, interrupted streams, cancellation, and model unavailability. Record upstream failures separately, along with the tested client and model versions. These are proposed acceptance targets, not measured results or an upstream uptime promise.

## At a glance

| # | Feature | Phase | Status |
|---|---------|-------|--------|
| 1 | Stack and architecture | Foundation | done |
| 2 | Coding standards and tooling | Foundation | done |
| 3 | Local configuration and credential data model | Foundation | done |
| 4 | First real Claude Code coding loop | Slice 1 | planned |
| 5 | Sign in and credential renewal | Slice 2 | planned |
| 6 | Long conversations and model selection | Slice 3 | planned |
| 7 | Interrupted sessions and safe recovery | Slice 4 | planned |
| 8 | CLI setup, operation, and diagnostics | Slice 5 | planned |
| 9 | Compatibility checks and dependable releases | Release | planned |
| 10 | Local dashboard | Deferred | planned |
| 11 | Linux and Windows support | Deferred | planned |
| 12 | Broader Claude Code capabilities | Deferred | planned |

## Foundations

### 1. Stack and architecture · done

Choose the project structure and runtime, then scaffold the smallest runnable CLI that the first coding loop needs. Keep the Kiro connection replaceable as its behavior changes.

**Done when:** an architecture spec records the choices and boundaries; the CLI builds and runs locally; the first slice has a defined way to prove client controlled tool use and surface an unsuitable access path.

**Spec:** [0001. Go gateway stack and architecture](../specs/0001-stack-architecture/index.md)

**Confirmed decision:** One foreground Go executable, an internal replaceable Kiro adapter, and reuse of your Kiro CLI session with IAM Identity Center for the first slice. Investigation of an undocumented inference interface is allowed, with a real Claude Code tool loop required before claiming compatibility. You accepted the spec after independent review on September 30, 2026.

- [x] Decide the stack (spec): `/architect stack and architecture`
- [x] Scaffold from the decision: `/develop stack and architecture`
- [x] Verify it: `/check verify stack and architecture`
- [x] Test it: `/test stack and architecture`

**Code:** `cmd/kiro-gateway`, `internal/cli`, and `internal/gateway`. Build and operation instructions are in `README.md`.

### 2. Coding standards and tooling · done

Capture conventions from the actual scaffold, then install the checks that keep later contributions consistent. Keep the initial development setup small.

**Done when:** project instructions reflect the real code; formatting, static checks, and continuous integration run from a clean checkout; contributors have a repeatable development command.

- [x] Capture conventions and tooling: `/audit`

**Code:** `scripts/check`, `.githooks/pre-commit`, and `.github/workflows/check.yml`. Setup and conventions are in `README.md` and `AGENTS.md`. Local checks pass; hosted CI execution is pending publication to GitHub.

### 3. Local configuration and credential data model · done · GA

Define how settings, credential references, model mappings, and diagnostic records relate and persist. Keep secret storage separate from ordinary configuration and make retention explicit.

**Done when:** the model supports one account, settings upgrades, removal of saved credentials, and bounded diagnostic retention; secrets and conversation contents are excluded from routine logs; persistent conversation storage has an explicit need before being added.

**Spec:** [0002. Local configuration and credential data model](../specs/0002-local-configuration-credentials/index.md)

- [x] Design it (spec): `/architect local configuration and credential data model`
- [x] Build it: `/develop local configuration and credential data model`
  - [x] Save and validate settings through the authenticated health server, including private storage and process locking (AC-1, AC-2, AC-5, AC-6, AC-8).
  - [x] Capture and forget the selected Kiro session, with exact fingerprints and atomic model mapping cleanup (AC-3, AC-4, AC-5, AC-6, AC-8, AC-9).
  - [x] Complete explicit upgrade, failure recovery, and restart behavior (AC-2, AC-6, AC-7, AC-8).
- [x] Verify it: `/check verify local configuration and credential data model`
- [x] Test it: `/test local configuration and credential data model`
- [x] Review it (fresh model): `/check review local configuration and credential data model`
- [x] Document it: `/document changelog local configuration and credential data model`

**Code:** `internal/config`, `internal/configstore`, `internal/credentials`, and `internal/cli`, with shared validation in `internal/jsonobject` and `internal/safepath`. Builder checks pass; the [verification checklist](../specs/0002-local-configuration-credentials/verify.md) records the next pass.

## Slice 1: Prove the coding loop

### 4. First real Claude Code coding loop · planned · needs a decision · GA

Connect an authenticated Kiro account to a local gateway and complete a small real coding task in Claude Code with one available Claude model. This is the working skeleton and the first product proof.

**Done when:** Claude Code receives streamed output, runs its own file and shell tools under its normal permissions, sends tool results back, and completes a follow up turn; instructions and tool identifiers survive translation; the local endpoint requires a credential; errors and cancellation are visible; evidence records the exact access path and tested versions.

**Spec:** [0003. First Claude Code loop, protocol feasibility](../specs/0003-first-claude-code-loop/index.md)

**Confirmed preparation:** You accepted the bounded feasibility design on October 1, 2026, after independent review and its four corrections. A development harness will investigate one exact saved Sonnet mapping, with at most six inference attempts and explicit review of the concrete plan before each live run. The initial baseline is Claude Code `2.1.285` and Kiro CLI `2.8.0`. The full bridge still needs protocol evidence and another architecture pass; its design checkbox and feature status remain pending.

**Confirmed profile amendment:** You accepted the selected profile source on October 1, 2026, after independent review and its name validation correction. The next local slice reads the fixed token and `state["api.codewhisperer.profile"]` records in one transaction, derives region from the profile ARN, and pins profile bytes for the run. Synthetic implementation can proceed. Destination and authentication, distinct system instructions, output controls, and successful turn completion still need new evidence before live work.

**Confirmed limited experiment:** After the additional protocol investigation on October 1, 2026, you approved carrying instructions in user context and treating clean stream end as tentative completion for feasibility only. Static evidence now connects the regional runtime operation and bearer token path, and documented model controls supply the bounded request. Even six observed cases yield only `limited_candidate_observed`; the distinct system role and proven model completion guarantees remain unresolved. Your approval covers preparation, not live account access or inference.

- [x] Plan the feasibility milestone: `/architect first real Claude Code coding loop`
- [x] Build the feasibility milestone only: `/develop first real Claude Code coding loop: feasibility milestone only`
  - [x] Prepare the concrete experiment and prove one offline path through the harness (AC-1, AC-2, AC-5, AC-6, AC-7, AC-8, AC-9).
    - [x] Complete the six case synthetic harness, including exact snapshot selection, tool result continuation, local failure controls, and passing repository checks.
    - [x] Build the combined token and selected profile snapshot with synthetic verification: `/develop first real Claude Code coding loop: selected profile snapshot only` (AC-1, AC-2, AC-6, AC-8, AC-9).
    - [x] Complete the limited candidate contract and concrete request plan from evidence and the approved instruction and completion limitations.
  - [x] Complete the safeguards and present the exact plan and clean code commit for live review (AC-1, AC-2, AC-3, AC-5, AC-6, AC-8).
  - [x] After approval and explicit launch, run the bounded cases and record evidence or the unresolved contract (AC-2 through AC-8). One dispatch attempt stopped inconclusively; five dependent cases remained unrun as required.
- [ ] Verify the feasibility milestone: `/check verify first real Claude Code coding loop: feasibility milestone only`
- [ ] Test the feasibility milestone: `/test first real Claude Code coding loop: feasibility milestone only`
- [ ] Review the feasibility milestone (fresh model): `/check review first real Claude Code coding loop: feasibility milestone only`
- [ ] Document the feasibility milestone: `/document changelog first real Claude Code coding loop: feasibility milestone only`
- [ ] Design it (spec): `/architect first real Claude Code coding loop`

**Code:** The test only limited harness is built in `internal/kiro`, with local configuration, session linking, and the exact Sonnet mapping set up. After session renewal and explicit relinking, the latest approved diagnostic run passed local source validation and received HTTP 403 from the reviewed US runtime path on its first request. It stopped as `needs_evidence`; five dependent cases were unrun. The run evidence is in spec 0003's verification record. No raw error body or headers were retained, and no automatic retry occurred. The feasibility build has recorded an unresolved authorization outcome, not compatibility. Further authorization evidence, independent GA verification, tests, review, documentation, and the full Claude Code bridge remain pending.

## Slice 2: Keep access working

### 5. Sign in and credential renewal · planned · needs a decision · GA

Make the proven account connection usable across normal workdays. Support the sign in method you actually use before broadening authentication options.

**Done when:** you can connect, inspect account status, reconnect, and disconnect; expired credentials renew when supported; simultaneous requests do not race renewal; revoked access produces a clear recovery action; account changes cannot silently select different credentials.

- [ ] Design it (spec): `/architect sign in and credential renewal`

## Slice 3: Use it for sustained coding

### 6. Long conversations and model selection · planned · needs a decision

Extend the proven loop to Opus and Sonnet models available to your account, longer histories, and repeated tool exchanges. Publish the limits that affect ordinary Claude Code sessions.

**Done when:** both model families complete representative coding tasks; resolved model identity is visible; unavailable choices fail clearly without silent substitution; long tool histories and client context compaction remain usable within declared limits; token usage is identified as measured, estimated, or unavailable.

- [ ] Design it (spec): `/architect long conversations and model selection`

## Slice 4: Recover predictably

### 7. Interrupted sessions and safe recovery · planned · needs a decision

Handle temporary connection failures, service limits, and broken streams without corrupting the conversation or hiding incomplete work.

**Done when:** retries and waiting are bounded; rate limits and exhausted credits are distinguishable; interrupted output is never reported as complete; retrying cannot silently replay visible tool calls; cancellation stops gateway work promptly; every unrecoverable error explains the next useful action.

- [ ] Design it (spec): `/architect interrupted sessions and safe recovery`

## Slice 5: Make daily operation simple

### 8. CLI setup, operation, and diagnostics · planned · needs a decision

Give you a repeatable way to configure Claude Code, operate the gateway, and find the cause of a failed request. Extend the minimal controls already used by the first slice.

**Done when:** you can set up, launch, inspect, and stop the gateway; configuration changes are previewable and reversible; diagnostics distinguish configuration, authentication, model access, and service failures; timing and request identifiers support troubleshooting; any exported report excludes secrets and conversation content by default.

- [ ] Design it (spec): `/architect CLI setup operation and diagnostics`

## Release: Keep compatibility measurable

### 9. Compatibility checks and dependable releases · planned · needs a decision

Make maintenance part of the product. A release should say what works and carry repeatable evidence for the behavior that commonly breaks.

**Done when:** a supported version matrix distinguishes verified, unsupported, and untested capabilities; recorded protocol cases cover tool exchanges and stream failures; live checks record model access and session outcomes; macOS installation, upgrade, and rollback work; release notes describe compatibility changes and known limits.

- [ ] Design it (spec): `/architect compatibility checks and dependable releases`

## Deferred

These remain outside the proposed first release. Revisit them after the core coding loop has demonstrated daily reliability.

### 10. Local dashboard · planned · needs a decision

Add an optional visual surface for setup, account status, and diagnostics when the CLI has exposed the real usability needs. Its design system belongs with this later feature.

**Done when:** you can perform the established management tasks through an accessible local interface, with keyboard access and clear error states; the CLI remains independently usable.

- [ ] Design it (spec): `/architect local dashboard`

### 11. Linux and Windows support · planned · needs a decision

Extend installation and daily operation to additional operating systems after the initial platform is reliable.

**Done when:** each added platform has verified installation, credentials, start and stop behavior, configuration recovery, and a real Claude Code tool loop; platform limits are documented.

- [ ] Design it (spec): `/architect Linux and Windows support`

### 12. Broader Claude Code capabilities · planned · needs a decision

Expand beyond the declared baseline to images, additional reasoning controls, and parallel agent workloads when the Kiro connection can support their semantics.

**Done when:** each promoted capability has a real acceptance case and a stated support boundary; unsupported requests get a useful response; parallel work cannot mix histories or tool results between sessions.

- [ ] Design it (spec): `/architect broader Claude Code capabilities`

## Legend

**Status:** `planned` means proposed work, `in-progress` means designed or being built, and `done` means the required implementation and checks are complete. `existing` marks work that predates this workflow. `dropped` preserves a removed feature's history.

**Needs a decision:** the entry command recommends `/architect`. Design owns the spec, adds its link, and expands the feature into a few build milestones. Atomic tasks stay in the spec.

**Workflow:** Beta recommends real behavior verification and automated tests. A GA tag adds a separate model review and documentation. A later UI or platform inherits Beta unless its design calls for a different level.

**Next step:** the first unticked box in an active phase. Deferred features are outside the current release even though they retain a planned status. Commands are recommendations you can run or skip.
