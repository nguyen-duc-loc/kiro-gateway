# Reasoning for the first loop feasibility milestone

## Context

> Premise note: The current evidence does not establish Kiro CLI 2.8.0's complete inference contract. Treating historical SDK source as that contract would leave authentication, instruction roles, and completion semantics for the builder to guess. You chose to establish those facts through a bounded experiment before completing the production bridge design.

The Go gateway already serves authenticated health and supports saved configuration and explicit session capture. The code does not contain an inference adapter or a protocol bridge. Scope feature 4 requires a real coding task with Claude Code owning tools, permissions, history, and continuation. Its GA workflow calls for independent review and explicit evidence.

You chose a new spec extending 0001 and 0002, one available Sonnet model, and a small Go bug fix in a disposable repository as the eventual product proof. You selected the installed Claude Code `2.1.285` and Kiro CLI `2.8.0` as the initial baseline. A proposed data model preserves the existing configuration and keeps runtime conversation state in memory; you confirmed metadata and reviewed synthetic examples as retained evidence.

Preflight found 18 Go source and test files, a clean working tree, no nested project context files, and zero commits behind `origin/main` after fetching. This work uses the existing single repository spec location and its directory convention. It does not revise either accepted foundation.

## Options considered

### Option 1: Specify the full translator from historical source

Use the public Amazon Q SDK shapes as the current Kiro contract. **Benefit**: The request, history, and tool types provide a concrete starting point. **Cost**: They do not prove the current destination, credential use, instruction semantics, or successful terminal condition. This would conceal important assumptions. (basis: historical generated client and types, installed binary observations, spec 0001 feasibility gate)

### Option 2: Bounded feasibility harness

Prepare a concrete candidate plan, prove its controls locally, then seek review before an explicitly launched live experiment. **Benefit**: Observed behavior drives the bridge design and a failed hypothesis has a useful stopping point. **Cost**: Another design pass is needed, and the experiment may remain inconclusive. You chose this option. (basis: your choices, Tracer Bullet approach, explicit resource ownership)

### Option 3: Continue live investigation during this design session

Prepare a specific live request plan now and settle the contract before writing the full bridge spec. **Benefit**: Could shorten the next architecture pass. **Cost**: The current evidence is insufficient to review the complete plan, and running real inference would expand this session beyond the selected design milestone. You chose a separately implemented harness first. (basis: your selection after the research findings, spec 0002 credential boundary)

### Option 4: Document a manual experiment

Record a procedure without a harness. **Benefit**: Less development work. **Cost**: Credential safeguards, stream inspection, and exact budgets would depend more on manual execution, reducing repeatability. The reusable product probe command was also considered; it would expose an experimental interface as a supported operator command too early. (basis: your development harness choice, existing standard library testing conventions)

## Rationale

The uncertainty is the connection's behavior, so production translation is premature. An explicit development entry point lets the ordinary test suite remain isolated while a small experiment exercises the chosen credential and one model. A reviewed plan makes the hypotheses visible before any token is sent. The plan digest and clean code commit bind the invocation to reviewed inputs; they do not replace your approval. The same local user can create any unsigned approval file, so human consent remains a recorded workflow decision rather than a receipt the harness claims to authenticate. (basis: your selected review gate and approved review corrections, Go security skill, explicit authorization and bounded resource use)

You selected six inference attempts, ten minutes per run, two minutes per request, a 30 second stream idle limit, and stopping when evidence is missing. Serial execution makes the accounting and cancellation behavior easier to assess. The exact saved mapping carries operator intent from spec 0002 into the experiment. Model discovery would require a second private operation and was not selected. (basis: your answers, spec 0002 model ownership and snapshot pinning)

The finite candidate plan is a review artifact rather than a general provider framework or arbitrary HTTP script. Standard library transport and bounded observation are sufficient starting tools; a production decoder or SDK choice is not justified until the wire contract is known. If new dependencies become necessary, their choice returns to architecture rather than being added silently. (basis: project standard library preference, smallest working slice)

Runtime output cannot be saved safely by merely redacting selected strings from arbitrary upstream payloads. Retain a fixed summary and write artificial regression examples from reviewed schema observations. The selected disposable Go task remains the eventual Claude Code proof; the harness's invented tool results deliberately establish only the lower level prerequisite. (basis: your evidence retention choice, installed Go security guidance, scope product boundary)

## Evidence and limits

The following observations were made on October 1, 2026. No real credential store, gateway configuration, live model, or signed in account status was accessed during this design.

| Observation | Established | Not established |
|---|---|---|
| `claude --version` returns `2.1.285 (Claude Code)` | Installed client baseline | Which gateway endpoints and optional features a real coding session will exercise. |
| `kiro-cli --version` returns `kiro-cli 2.8.0` | Installed CLI baseline | Stability of its private protocol or access by the linked account. |
| Local Claude Code help exposes model selection, explicit tools, settings controls, and normal permission modes | Controls available for later client verification | An appropriate full launch profile; the bridge design still owes that decision. |
| Local Kiro help exposes chat and model listing | Those commands exist | Permission to call them during a probe or proof that they avoid refresh and account changes. |
| Static inspection of `/Applications/Kiro CLI.app/Contents/MacOS/kiro-cli-chat` finds `AmazonCodeWhispererStreamingService.GenerateAssistantResponse`, `application/x-amz-json-1.0`, and `x-amz-target` | A candidate operation is embedded in the installed artifact | Its selected runtime path or accepted authentication for this session. |
| The same artifact contains conversation state, history, tool result, tool ID, and tool event names | Current artifact clues align with parts of the historical schema | Complete field serialization, lossless instruction handling, or a successful terminal indicator. |
| The artifact contains `runtime.us-east-1.kiro.dev`, `runtime.eu-central-1.kiro.dev`, management hosts, and `q` service hosts | Several candidate service destinations exist in this version | Which one applies to IAM Identity Center inference. None is an approved destination. |
| Inspection did not find the searched literal `/v1/messages` or `anthropic-version` strings | Those exact strings were absent in that search | Absence of a compatible service; compilation and unrelated client code make negative string results weak evidence. |
| Existing source returns only a credential fingerprint from `Capture` | The saved selection contract is implemented | A token retrieval API for inference; it still needs the same snapshot guarantee. |

The official Amazon Q Developer CLI repository states that it is no longer actively maintained and that Kiro CLI is closed source. Its generated client shows a historical POST operation using the named content type and target header. Its conversation and tool types are evidence of past shapes, not a current version contract. (basis: [official repository](https://github.com/aws/amazon-q-developer-cli), [historical operation](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-streaming-client/src/operation/generate_assistant_response.rs), [conversation type](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-streaming-client/src/types/_conversation_state.rs), [tool type](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-streaming-client/src/types/_tool_use.rs))

Current Kiro CLI documentation describes CLI usage and authentication, including IAM Identity Center and API key operation. The reviewed pages did not establish a direct Anthropic compatible inference endpoint. The infrastructure security page describes AWS API security at a broader level; it cannot establish that the selected SQLite access token can be used with a particular signing or bearer scheme. (basis: [CLI documentation](https://kiro.dev/docs/cli/), [CLI authentication](https://kiro.dev/docs/cli/authentication/), [infrastructure security](https://kiro.dev/docs/cli/privacy-and-security/infrastructure-security/))

The read only research helper used five searches and eight page opens. No current Anthropic wire contract was established in that bounded pass. The prior architecture remains the source of required client semantics; a later full bridge design needs concrete client evidence. References are for a human reader, not instructions for another agent to fetch the same pages again.

## Review and ratification

At your request, `gpt-6-sol` independently reviewed the draft against the accepted foundations, scope, and relevant implementation on October 1, 2026. It read local files without editing them, fetching references, accessing credentials, or running inference. It found the bounded feasibility scope coherent and identified four decisions that need clarification before ratification:

1. Human approval is a workflow record, while the harness can enforce only concrete launch inputs. The review recommends binding one approved run to the exact plan, mapping, destinations, baseline, and reviewed code identity.
2. Operator cancellation must be distinct from the deliberate cancellation of attempt 5. Expected stream interruption and an unreached cutoff also need explicit run verdict rules.
3. The plan and configuration need an explicit immutable run snapshot. The process lock does not prevent manual configuration edits.
4. Protocol observations need a precise allowed output model, so the builder does not invent how structural evidence is retained without saving raw traffic.

You approved the recommended corrections on October 1, 2026. Approval now belongs to the human workflow for one launch, while the harness checks a clean reviewed commit, exact plan digest, mapping, and baseline. Plan and configuration values are frozen at startup; later manual edits are ignored, and every attempt still validates a fresh credential snapshot against the frozen reference. Deliberate child request cancellation and interruption have explicit required assertions and run verdicts. The output model permits only reviewed labels, local measurements, and computed assertions, with fixed labels for unknown input.

The same independent reviewer checked the four corrections and found them closed, with no new material contradiction or remaining blocker within that review scope. The author clarified that local rejection before dispatch uses no inference slot, local tests may use synthetic credentials, and raw upstream IDs remain observations in memory rather than retained evidence. The example live test command uses verbose output so an allowed summary remains visible after a passing test.

You accepted the assembled revised feasibility spec on October 1, 2026. The scope links its preparation milestones while keeping the full bridge design and coding loop pending. The spec remains `Proposed` under the feature lifecycle convention. No candidate plan or live run has been approved. No implementation or live compatibility result exists.

## References

**Project sources**

1. [Project context](../../../AGENTS.md), package ownership, standard library preference, and local operation rules.
2. [Scope](../../scope/scope.md), feature 4, Tracer Bullet delivery, and GA workflow.
3. [Architecture](../0001-stack-architecture/index.md), tool ownership and feasibility gate.
4. [Configuration and credentials](../0002-local-configuration-credentials/index.md), snapshot selection and exact model mappings.
5. [Credential capture](../../../internal/credentials/capture.go) and [health server](../../../internal/gateway/server.go), current implementation boundaries.
6. [Go security skill](../../../.agents/skills/golang-security/SKILL.md), trust boundaries and sensitive output.
7. Your design answers and the local version, help, and static artifact observations recorded above.

**Practices**

1. A thin working experiment before expanding the implementation.
2. Explicit authorization, resource ownership, and bounded execution.
3. Positive evidence for successful completion and no automatic replay.
4. Allowed diagnostic fields and artificial regression fixtures.

**Verified external sources**

1. [Official Amazon Q Developer CLI repository](https://github.com/aws/amazon-q-developer-cli), repository status and Kiro source availability.
2. [Historical generated streaming operation](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-streaming-client/src/operation/generate_assistant_response.rs), candidate method and headers.
3. [Historical conversation state](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-streaming-client/src/types/_conversation_state.rs) and [tool use type](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/amzn-codewhisperer-streaming-client/src/types/_tool_use.rs), candidate field evidence.
4. [Kiro CLI documentation](https://kiro.dev/docs/cli/) and [authentication](https://kiro.dev/docs/cli/authentication/), supported operator surfaces.
5. [Kiro infrastructure security](https://kiro.dev/docs/cli/privacy-and-security/infrastructure-security/), general network security description and its inference limits.
