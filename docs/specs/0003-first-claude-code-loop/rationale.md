# Reasoning for the first loop feasibility milestone

## Context

> Premise note: The current evidence does not establish Kiro CLI 2.8.0's complete inference contract. Treating historical SDK source as that contract would leave authentication, instruction roles, and completion semantics for the builder to guess. You chose to establish those facts through a bounded experiment before completing the production bridge design.

The Go gateway already serves authenticated health and supports saved configuration and explicit session capture. The code now contains a six case synthetic feasibility harness, but no live inference dispatcher or protocol bridge. Scope feature 4 requires a real coding task with Claude Code owning tools, permissions, history, and continuation. Its GA workflow calls for independent review and explicit evidence.

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

You accepted the assembled revised feasibility spec on October 1, 2026. At that point its status was `Proposed`, and no implementation or live result existed. Development subsequently advanced it to `In Progress` and implemented the synthetic harness. No candidate plan or live run has been approved.

## Profile sourcing amendment, October 1, 2026

The resumed architecture pass found 26 Go source and test files, a clean checkout at `9d911dc`, and zero commits behind `origin/main` after fetching. That checkpoint records passing repository checks and a closed live gate. No real credential, profile, or gateway configuration was read in this architecture pass, and no Kiro process or live inference was launched. Existing verified links were reused without fetching them again.

You selected a read only lookup of Kiro's selected profile record over supplying the inference region in each experiment plan. After independent review and the correction below, you accepted the complete amendment and its new AC-9.

### Options for the additional source

| Option | Benefit | Cost and disposition |
|---|---|---|
| Extend the existing reader with one combined token and selected profile snapshot | Reuses the path and transaction safeguards, follows the saved selection, and avoids operator transcription | Adds dependence on another private record; chosen for the amendment. The original token capture remains unchanged. |
| Put the region explicitly in every reviewed plan | Avoids reading another account record and makes the destination easy to review | Can disagree with the saved profile and still leaves a required `profileArn` without a source; you did not choose it. |
| Add a new profile discovery adapter alongside the reader | Could use a provider API to establish profile ownership and availability | Requires another operation, network budget, and authentication contract before the first inference; deferred. |
| Replace the reader with a Kiro subprocess | Delegates selection details to the installed client | May renew credentials, mutate state, or own conversation and tools; inconsistent with this bounded harness, not chosen. |

The recommendation is a targeted extension because the failure is an omitted routing input, not an unmaintainable store adapter. A new SDK, service, profile catalogue, or settings schema would not solve the remaining protocol uncertainty. (basis: existing `internal/credentials/capture.go`, spec 0002, your selected profile choice)

### Static source evidence

The following addresses refer to the installed arm64 `kiro-cli-chat` artifact with SHA 256 `2118bd89d96830a4f0e0884e4f4071fb94e4b6036c757126afe9e3f80c9d90c6`. These are binary inspection findings, not reads of the operator's database. Two read only helpers investigated profile and protocol paths; the author then resolved the profile constants and decoder comparisons directly with LLDB without launching the target.

| Observation | Evidence and implication |
|---|---|
| Selected profile key | `Database::get_auth_profile` at `0x1022fb854` calls `get_entry` at `0x1022fb88c`, passing table discriminator `0` and 25 key bytes. Reading constant address `0x11b85f55e` yields `api.codewhisperer.profile`. |
| Profile table | `get_entry` formats its table through `Table::fmt` at `0x102349b48`. Its discriminator `0` branch uses five bytes at `0x11b85f087`, which read `state`. The gateway's ordinary table, primary key, and size requirements are chosen validation policy, not an observation of the operator's schema. |
| Profile fields | `get_auth_profile` calls serde `from_trait` at `0x102619a90`, which calls the specialized struct decoder at `0x102694d80`. Comparisons at `0x102694ec8` through `0x102694ee8` recognize `arn`; comparisons at `0x102694f34` through `0x102694ff0` recognize `profileName` and `profile_name`. The recognized values use a string decoder. Missing field paths at `0x10269521c` and `0x10269526c` require the ARN and profile name respectively. Both name aliases share the same field and duplicate check. No region or token association field is established by this path. |
| Endpoint input | `Endpoint::configured_value` at `0x10259ee70` calls that profile reader at `0x10259ef18`. The ensuing colon search at `0x10259ef7c`, `0x10259efa0`, `0x10259efc4`, and `0x10259efec` selects the fourth component before regional endpoint lookup at `0x10259f11c`. This supports using the profile ARN region. It does not settle the operation's auth branch or all endpoint override behavior. |
| Candidate hosts | The existing plan records `Endpoint::krs_for_region` at `0x10259ec14` mapping `us-east-1` and `eu-central-1` to their corresponding runtime hosts. They remain candidates, not approved destinations. |
| Completion gap | The previous inspected message metadata decoder recognizes `conversationId` and `utteranceId`. A further bounded static pass did not establish a positive model completion signal, distinct system wire field, or generation output limit. Negative string searches are not proof that the service lacks them. |

The selected profile is a global saved selection. The inspected read does not bind it to the token's `start_url`. A consistent SQLite transaction and an in memory digest prevent accidental mixing or mid run adoption; they do not establish ownership, historical atomic writes by Kiro, or server permission. A reviewed launch therefore selects the saved profile at its first attempt and relies on the eventual service response for authorization evidence. Reject rather than repair any unusable pair. (basis: static call trace, spec 0002's snapshot rather than identity guarantee)

The official region documentation says inference uses the Kiro profile region, which may differ from the Identity Center instance region. Agent prompt and headless output documentation concern CLI behavior and do not close the installed 2.8.0 runtime contract. The strict instruction and completion requirements are retained. This amendment resolves local source ownership and consistency; it does not declare the remaining service facts true by architectural choice. (basis: [supported regions](https://kiro.dev/docs/enterprise/supported-regions/), [agent configuration](https://kiro.dev/docs/custom-agents/configuration-reference/), [headless output](https://kiro.dev/docs/cli/headless/), prior evidence checkpoint)

The accepted ARN subset is intentionally a gateway policy: commercial `aws` partition, `codewhisperer` service, two candidate regions, and bounded profile resource syntax. It permits deterministic validation without a new ARN library. It may reject a valid but unreviewed format, which is preferable to deriving a network destination from arbitrary account text. The finite reviewed plan still owns all network destinations. (basis: standard library preference, Go security guidance, bounded experiment)

The architectural outcome has two parts. A local combined profile read has a concrete source and verification plan. Live preparation remains `needs_evidence` until new protocol evidence establishes destination and authentication together, distinct instruction roles, controls, and positive completion. There is no recommendation to repeat unchanged investigations or to disguise synthetic successes as live compatibility.

### Amendment review

At your request, `gpt-6-sol` independently read the amendment and its verification plan. It found one material source shape gap: requiring only `arn` would accept a profile record that the installed Kiro decoder rejects for a missing name. It found no other material gap in the local AC-9 slice and confirmed that the draft preserves the unresolved live gates.

You approved the recommended correction. The reader design now requires exactly one string valued `profileName` or `profile_name` field and discards the decoded name after validation. Empty strings remain allowed because this is a type and presence check, not a new semantic use of the name. Missing, wrong type, and conflicting aliases have explicit rejection cases. The same independent reviewer confirmed that the gap was closed.

You then accepted the revised amendment on October 1, 2026. This ratifies the local source design and the explicit disposition of unresolved protocol facts. The lifecycle status stays `In Progress`; profile implementation, live feasibility, and the full Claude Code proof are not claimed complete. No live run is authorized.

## References

**Project sources**

1. [Project context](../../../AGENTS.md), package ownership, standard library preference, and local operation rules.
2. [Scope](../../scope/scope.md), feature 4, Tracer Bullet delivery, and GA workflow.
3. [Architecture](../0001-stack-architecture/index.md), tool ownership and feasibility gate.
4. [Configuration and credentials](../0002-local-configuration-credentials/index.md), snapshot selection and exact model mappings.
5. [Credential capture](../../../internal/credentials/capture.go) and [health server](../../../internal/gateway/server.go), current implementation boundaries.
6. [Go security skill](../../../.agents/skills/golang-security/SKILL.md), trust boundaries and sensitive output.
7. Your design answers and the local version, help, and static artifact observations recorded above.
8. [Feasibility candidate plan](../../../internal/kiro/testdata/probe-plan.json), existing static provenance and the closed live contract at checkpoint `9d911dc`.

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
6. [Kiro supported regions](https://kiro.dev/docs/enterprise/supported-regions/), distinction between profile and Identity Center regions, verified during the preceding development evidence pass on October 1, 2026.
7. [Agent configuration reference](https://kiro.dev/docs/custom-agents/configuration-reference/) and [headless mode](https://kiro.dev/docs/cli/headless/), CLI level instruction and output concepts, verified in that same evidence pass. These do not establish the required runtime wire fields.
