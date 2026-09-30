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
| 2 | Coding standards and tooling | Foundation | planned |
| 3 | Local configuration and credential data model | Foundation | planned |
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

### 2. Coding standards and tooling · planned

Capture conventions from the actual scaffold, then install the checks that keep later contributions consistent. Keep the initial development setup small.

**Done when:** project instructions reflect the real code; formatting, static checks, and continuous integration run from a clean checkout; contributors have a repeatable development command.

- [ ] Capture conventions and tooling: `/audit`

### 3. Local configuration and credential data model · planned · needs a decision · GA

Define how settings, credential references, model mappings, and diagnostic records relate and persist. Keep secret storage separate from ordinary configuration and make retention explicit.

**Done when:** the model supports one account, settings upgrades, removal of saved credentials, and bounded diagnostic retention; secrets and conversation contents are excluded from routine logs; persistent conversation storage has an explicit need before being added.

- [ ] Design it (spec): `/architect local configuration and credential data model`

## Slice 1: Prove the coding loop

### 4. First real Claude Code coding loop · planned · needs a decision · GA

Connect an authenticated Kiro account to a local gateway and complete a small real coding task in Claude Code with one available Claude model. This is the working skeleton and the first product proof.

**Done when:** Claude Code receives streamed output, runs its own file and shell tools under its normal permissions, sends tool results back, and completes a follow up turn; instructions and tool identifiers survive translation; the local endpoint requires a credential; errors and cancellation are visible; evidence records the exact access path and tested versions.

- [ ] Design it (spec): `/architect first real Claude Code coding loop`

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
