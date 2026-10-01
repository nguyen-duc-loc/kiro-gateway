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

## Additional protocol investigation, October 1, 2026

You explicitly authorized further public research and static inspection of the installed binary, without credential access or live requests. This extends the earlier research boundary only. No Kiro process was launched, no credential or profile store was opened, and no inference endpoint was contacted. Public documentation and historical AWS source were fetched. The checkout began clean at `7a66898`.

**Result:** The current artifact now supplies a connected candidate for destination, operation, and bearer authentication. Current official documentation also supplies a named output control, with a matching generic serialization path in the binary. Distinct system instruction semantics and positive successful turn completion remain unresolved. This is new preparation evidence, not a completed plan, a service acceptance result, or a change to the accepted requirements.

### Artifact and method

The inspected file is `/Applications/Kiro CLI.app/Contents/MacOS/kiro-cli-chat`. Its SHA 256 remains `2118bd89d96830a4f0e0884e4f4071fb94e4b6036c757126afe9e3f80c9d90c6`, matching the recorded 2.8.0 baseline. Addresses below are arm64 file virtual addresses, before process relocation. The arm64 slice starts at file offset `0x229e0000`.

Inspection used `nm`, LLDB with startup files disabled, and direct reads of the Mach O file. LLDB created a target for disassembly but never launched or attached to a process. Function calls described below are static instructions in that file, not calls made during research. Constants were resolved from instruction operands and checked against file bytes; nearby strings alone were not treated as proof of a field's use.

### Destination and authentication chain

| Link | Current artifact evidence |
|---|---|
| Profile region enters client construction | `chat_cli_v2::api_client::ApiClient::new` starts at `0x1010904fc`. It calls `Endpoint::configured_value` at `0x1010905a8`. The previously traced selected profile ARN supplies that endpoint's region. |
| Runtime endpoint selection | In the absence of its endpoint setting override, the same constructor calls `Endpoint::krs_for_region` at `0x101090a04` and stores the result. The streaming client reads that result's URL at `0x101091858` and passes it to its endpoint resolver at `0x1010918c4`. This connects the runtime selector to the streaming client rather than merely finding host strings. |
| Exact regional candidates | `krs_for_region` at `0x10259ec14` selects constant record `0x11e9b5230` for `us-east-1` and `0x11e9b5260` for `eu-central-1`. Their URL bytes are `https://runtime.us-east-1.kiro.dev` and `https://runtime.eu-central-1.kiro.dev`. Other regions, overrides, and the CLI's fallback behavior are outside the gateway's finite destination policy. |
| Operation and HTTP request | `RealApiClient::send_message` calls the GenerateAssistantResponse builder at `0x10238a410` and its operation plugins at `0x10238ba90`. Its request serializer at `0x10081fea8` uses `POST` (constant `0x105275878`, four bytes), path `/` (constant `0x11b9ca5f8`, one byte), content type `application/x-amz-json-1.0`, and `x-amz-target: AmazonCodeWhispererStreamingService.GenerateAssistantResponse`. Header values are passed at `0x1008203f0` and `0x1008206b8`. |
| Resolver installed on that streaming client | At `0x10109175c`, the constructor registers `httpBearerAuth` with the resolver vtable at `0x11e938830`. That table points directly to `UnifiedBearerResolver::resolve_identity` at `0x1022679a4`. The scheme string is at `0x11b7f5f4a`, length 14. |
| Current operation's auth scheme | Streaming `Client::from_conf` installs `BearerAuthScheme` at `0x100817d1c`, then creates `DefaultAuthSchemeResolver` at `0x100817d24`. The latter starts at `0x100872d38` and installs the `httpBearerAuth` default. The scheme vtable at `0x11e8b3c00` identifies its resolver and signer. This is current client wiring, not reliance on the historical operation's auth plugin. |
| Fixed token branch | The unified resolver's async body at `0x102267a44` includes a branch that passes the exact `kirocli:odic:token` key, length 18, to `Database::get_secret` at `0x10226b590`. The key is at `0x11b84971c`. That branch decodes the record at `0x10226b7f8` and constructs an HTTP token identity at `0x10226ba20`. This is the existing selected record source; no additional credential source is proposed. |
| Header construction | `BearerAuthSigner::sign_http_request` at `0x100d6553c` formats the token with `Bearer `, then inserts the header at `0x100d65760`. The format prefix is at `0x1052c265c`, length seven. The header name uses standard header index 16; the `StandardHeader::as_str` tables resolved through `0x100d8a534` map that index to `authorization` at `0x1052c747e`, length 13. |

The normal client also contains settings reads, credential renewal, alternative credential branches, and retries. Inspecting those instructions does not authorize using them. A future gateway experiment would retain its own pinned snapshot, exact destination restrictions, and no renewal or retry policy. These observations establish a candidate request path for the selected record type, not that the operator's actual token and profile are authorized by the service. That latter fact still needs the separately reviewed live experiment.

### Output control evidence

The current [Kiro reasoning effort reference](https://kiro.dev/docs/models/effort/), updated September 30, 2026, explicitly describes `max_tokens` as a response output limit. It lists the Sonnet 5 range as 1024 through 128000 and shows per model configuration through `chat.modelDefaults`. This supplies a documented parameter and range that the earlier agent prompt and headless pages did not establish.

The installed client provides the following path for additional model parameters:

1. `RtsState::apply_model_defaults` at `0x1027a8300` reads settings and calls `AdditionalModelFields::apply_overrides` at `0x1027a85b8`.
2. `RtsModel::make_conversation_state` reads `RtsState::additional_fields` at `0x1027a7200`.
3. `RealApiClient::send_message` converts additional values through `value_to_document` at `0x10238a464` and calls `set_additional_model_request_fields` at `0x10238a4b8`.
4. The input serializer at `0x100810d48` writes the `additionalModelRequestFields` member through its generic document serializer. The member name is at `0x10527fdce`; its key writer call is at `0x100810e4c`.

Together, the documentation and this path support `additionalModelRequestFields.max_tokens` as a concrete candidate to test. They do not prove that this exact service and model accept or honor it. The inspected `metadataEvent` decoder recognizes `tokenUsage`, and its nested decoder at `0x1008228a0` recognizes `outputTokens` among usage fields. That is a possible observation source to assess when completing the control assertion. A short answer alone is not proof of enforcement. The plan still needs the exact chosen limit and an assertion consistent with the existing optional usage and output retention rules. No runtime output policy is expanded by this research.

### Distinct instruction role remains unresolved

The input serializer writes `conversationState`, `additionalModelRequestFields`, `profileArn`, and `agentMode`. The full conversation serializer at `0x100861b10` has no distinct system instruction member. The message union serializer at `0x10085bf08` selects `userInputMessage` or `assistantResponseMessage`. The user context serializer at `0x100841890` includes `additionalContext`, `toolResults`, and `tools`, alongside environment and editor context.

These are observations of the typed request path. They do not rule out an undocumented field inside the generic additional model document or a different service operation. However, no inspected primary source establishes such a field or its instruction priority. The documented agent prompt cannot establish that wire contract. Sending Claude Code's system instructions as user text or `additionalContext` would remain an unapproved semantic change.

### Successful turn completion remains unresolved

Inspection followed the current `ChatResponseStreamUnmarshaller::unmarshall` at `0x10082eb58` into the relevant payload decoders, rather than searching for a stop string anywhere in the executable.

| Examined payload | What its decoder establishes |
|---|---|
| `assistantResponseEvent`, decoder `0x1008771e4` | Recognizes `content` and `modelId`; other payload keys reach the skip path. These fields do not indicate turn completion. |
| `messageMetadataEvent`, decoder `0x100869d10` | Previously established `conversationId` and `utteranceId`; identifiers do not indicate turn completion. |
| `metadataEvent`, decoder `0x1008686d8` | Recognizes `tokenUsage`, with nested usage fields. No terminal semantics or required final ordering was established for this event. |
| `reasoningContentEvent`, decoder `0x100876650` | Recognizes `text`, `signature`, and `redactedContent`. These describe reasoning content, not a successful turn end. |
| Dispatcher | Separates event and error or exception handling. Its `dryRunSucceedEvent` branch is not evidence that an ordinary inference turn completed. No positive normal turn completion event was identified in this pass. |

Unknown server fields may be discarded by the installed decoder. Absence from these typed paths therefore does not prove the service can never supply a terminal signal. Equally, an error free EOF, a tool stop flag, model identity, or usage metadata cannot be promoted to the positive completion guarantee required by the current spec without new evidence or a changed requirement.

### Practical disposition

The research narrows the problem from unidentified connection details to two unresolved compatibility guarantees, plus completing the control experiment's assertion. The candidate path and output parameter can inform the next architecture pass. The recommendation is to keep the current compatibility requirements intact and record `needs_evidence`; do not present a working transport as a verified Claude Code bridge.

If you want to proceed without evidence for those guarantees, the next decision is whether to permit an explicitly limited experiment with changed instruction or completion semantics. That would be a requirement change, not more implementation detail, and is not authorized by this research request. No build checkbox, live launch permission, candidate plan readiness, or feature status changes here.

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

## Limited experiment approval, October 1, 2026

After the additional investigation, you agreed to an experiment that carries system instructions as user context and treats clean stream end as tentative completion. This explicitly changes those two feasibility requirements only. It does not authorize a live run or claim that the production bridge preserves the original semantics. The implementation will use the documented `max_tokens` and disabled thinking controls, record optional usage comparison only as a boolean, and retain all existing budgets and account safeguards. A complete synthetic sequence can establish local behavior; the best live verdict is `limited_candidate_observed`.

The advantage is a buildable, concrete experiment that can test tools and continuation. The cost is weaker instruction priority and no reliable way to detect an upstream truncation that looks like a clean transport end. Requiring the original guarantees would keep the experiment blocked on evidence; you selected the limited experiment instead. The full bridge design remains pending.

### Local implementation evidence

The limited request and response path is implemented in test files under `internal/kiro`. Synthetic TLS tests cover six cases, exact tool identity and result continuity, EOF distinctions, malformed framing and JSON, unknown data, profile drift, output limit observations, plan mutation, and DNS restrictions. The existing fixture suite still covers source consistency, locking, time limits, and byte budgets. A cancellation test server initially waited without consuming its request body; consuming the synthetic body fixed its teardown, and the focused cases passed with the race detector.

The complete repository check subsequently passed formatting, vet, compilation, and race tests with fake live launch controls inherited. The `liveprobe` build also compiled and skipped `TestProtocolProbe` with `KIRO_GATEWAY_LIVE_PROBE=0`, before configuration or version command access. A read only implementation cross check found no actionable issue in the snapshot, transport, retention, and completion paths; it does not replace the separate GA review. No real account access or inference occurred.

## First approved launch and baseline replacement

On October 1, 2026, you approved one live launch of commit `3253ffcfca9733e84a2963e0a20f68ce60796aa2` and plan digest `26070279796930d28c27718b5b56994780b565b8cb0e4fae73e159ab0282fbf9`. The launch stopped at `baseline_changed` before account access or inference. Fixed version diagnostics identified installed Claude Code `2.1.286`, differing from the reviewed `2.1.285`. Kiro CLI and Go still match. The verification file records the precise preflight evidence; no raw network traffic or account metadata exists for this launch.

A replacement candidate pins the installed Claude Code patch version instead of changing the operator's installation or weakening the version guard. It retains the same requests, Kiro binary baseline, model, account sources, budgets, and two explicit limitations. This harness sends synthetic requests itself, so the replacement changes its recorded client baseline rather than a Claude Code request translation. Full Claude Code compatibility remains untested. Preparing this replacement does not authorize a second run; its exact commit and digest await review.

## Replacement launch and missing setup

You approved a replacement launch at `40ea75a17a129e5789d464d5816e5bba469d7e1d` with plan digest `be490b8a73271fb6af757bcbdc448ad53374529a1c1b82f22184ae15647f7278`. It passed the version gate and stopped with `configuration_invalid`. The existing read only configuration check identified absent saved settings. No Kiro credential or profile record was read, and no inference attempt occurred. The verification record contains both launch identities and the proposed local setup.

The recommendation is to establish the configuration through the existing initialization, session capture, and locked save mechanisms, then conduct one newly approved run. Weakening the harness to initialize or choose credentials implicitly would change its account boundary. No harness code, candidate request, or plan byte needs to change for this prerequisite.

## Profile declaration correction after approved setup

The approved setup created valid settings, linked the fixed token reference, and saved the exact Sonnet mapping. The subsequent approved launch at `468cda48fe2497f65841a1319a68238d996dc6da` stopped with `source_unavailable`, with zero inference attempts. Its allowed summary is recorded in `verify.md`.

Read only metadata diagnostics established that Kiro declares `main.state.value` as `BLOB` while storing the selected profile as bounded text. The original profile amendment copied the token table's declared `TEXT` rule without evidence that the profile table used the same declaration. That policy was too restrictive for the actual source. SQLite declaration and the selected value's storage type are separate checks here.

The proposed correction permits either `TEXT` or `BLOB` declarations for the plain profile value column, preserving the requirement that its selected value is text. Keeping the old declaration restriction would require changing Kiro's database or abandoning the selected source. Accepting actual binary values or broad column affinities would expand the source contract unnecessarily. The narrow correction keeps all token schema, path, key, bounds, transaction, JSON, ARN, and fingerprint checks. It is prepared with synthetic regression coverage and awaits the next exact live review. No account values were retained, and no post correction live read was performed.

## First dispatch and diagnostic gap

You approved the corrected reader at `3323e609a9404f27549944b3b8965b43f10991d8` with plan digest `be490b8a73271fb6af757bcbdc448ad53374529a1c1b82f22184ae15647f7278`. The combined snapshot passed and selected the US runtime destination. One dispatch attempt stopped with `needs_evidence`, before any decoded response event. Its summary is retained in `verify.md`; no raw response or error was saved.

The original sanitized category combined transport failure and non 200 HTTP status. This protected account data but omitted a safe distinction needed to localize the failure. The prepared correction emits only finite local stage, transport, and HTTP status labels. It leaves error bodies unread and preserves the existing stop and verdict rules. Raw error logging would expose unbounded upstream content and is not proposed. A fresh diagnostic run needs its own review because both code and allowed output policy changed. The prior attempt is not replayed automatically and its exact cause remains unknown.

## Approved diagnostic launch stopped on credential expiry

You approved the diagnostic plan at `b2490a92125e147b53a2eb6615ce17d952c61eea` with digest `137a0c1d0999e5700cec06a998d860ba151d844eaecd6ec27d4a7cf9efe0496b`. The run stopped at the source stage with `credential_expired`, before selecting a destination or dispatching any request. Its allowed summary is recorded in `verify.md`. This is an expected source boundary, not a new protocol finding or evidence that the earlier dispatch failed for the same reason.

No code change is indicated by this result. The next prerequisite is operator renewal through Kiro CLI's IAM Identity Center flow, then explicit relinking and restoration of the exact model mapping. Automatic renewal and relinking remain outside the harness. No further live run occurred.

## Renewed session reached HTTP 403

After you reported successful IAM Identity Center sign in, the gateway explicitly relinked the renewed snapshot and restored the exact Sonnet mapping. You approved one diagnostic run at `b07b0d6add6dcf85c74d66357853d00271696b35`, with digest `137a0c1d0999e5700cec06a998d860ba151d844eaecd6ec27d4a7cf9efe0496b`. It passed local source validation and received HTTP 403 from the reviewed US runtime path. The first case was inconclusive and five dependent cases were unrun. The allowed summary is recorded in `verify.md`.

This resolves the diagnostic ambiguity for this attempt only: the HTTP client received a denial, rather than failing before a response. It does not establish which service restriction failed, whether the selected profile is authorized for the token, whether the requested model is accessible, or whether an additional operation detail is required. No error body or raw headers were retained. A sign in succeeding and a locally valid token cannot establish remote authorization for this operation.

The recommendation is to investigate the authorization contract using new evidence before another request. Do not repeat the same candidate, change accounts or models, or try another endpoint automatically. The feasibility milestone records `needs_evidence`; the full bridge design remains pending. No further inference run occurred.

## Authorization investigation after HTTP 403

You authorized investigation of the denial using public sources and static inspection. The installed artifact still has SHA 256 `2118bd89d96830a4f0e0884e4f4071fb94e4b6036c757126afe9e3f80c9d90c6`. This pass did not launch Kiro, read account stores, change settings, or send inference. LLDB inspected arm64 file addresses without starting a process.

### Request differences and their limits

| Finding | Evidence and implication |
|---|---|
| Normal token mode has no extra token type header | `TokenTypeInterceptor::modify_before_signing` at `0x102731fd4` returns without insertion for discriminator zero. `AuthMode::fmt` at `0x102035fec` resolves zero to `Normal`, one to `ExternalIdp`, and two to `ApiKey`. The saved token branch in `ApiClient::new` stores zero at `0x10109089c`; the streaming client registers this interceptor at `0x1010916cc`. Values one and two insert `TokenType: EXTERNAL_IDP` or `TokenType: API_KEY`. The reviewed saved token request should not acquire either header merely because it received 403. |
| CLI user agent differs | The registered `UserAgentOverrideInterceptor` at `0x1027bb210` obtains the AWS user agent and inserts `user-agent` at `0x1027bbce8`. `apply_additional_metadata` at `0x10279da94` adds CLI metadata. The probe uses Go's ordinary user agent. This is an evidenced difference, not evidence that the service requires spoofing the CLI identity or that the difference caused the denial. |
| Opt out header differs | `OptOutInterceptor::modify_before_signing` at `0x102731ec8` inserts the fixed header name `x-amzn-codewhisperer-optout` from constant `0x11b87c174`, length 27. Its boolean comes from client settings or an override. The probe omits it. No evidence connects its absence to this 403, and this research did not read operator settings. |
| Agent mode header is conditional | The operation header serializer at `0x10086a614` skips an absent agent mode and otherwise writes `x-amzn-kiro-agent-mode`, constant `0x1052840f3`, length 22. This does not establish a required value for the current experiment. |
| Inspected connector preserves the URI | `ReqwestConnector::call` async body at `0x102277ed8` copies the existing URI and passes it to `Client::request` at `0x1022780cc`, copies headers at `0x1022782a8`, and dispatches at `0x102278474`. The inspected `StaticEndpointResolver` at `0x10240b770` copies the configured endpoint string. No operation path rewrite was identified in those functions. |

The [AWS JSON 1.0 specification](https://smithy.io/2.0/aws/protocols/aws-json-1_0-protocol.html) defines root path POST requests selected by `X-Amz-Target`, consistent with the inspected serializer and connector. This supports the candidate framing; it is not live proof that the private endpoint accepts this client. The [historical official connector](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/chat-cli/src/aws_common/http_client.rs) and [user agent interceptor](https://github.com/aws/amazon-q-developer-cli/blob/main/crates/chat-cli/src/aws_common/user_agent_override_interceptor.rs) are consistent with these narrow static findings but remain historical sources.

### Evidence that would distinguish the denial

The installed error metadata decoder at `0x10085a5a4` looks up `x-amzn-errortype`, whose bytes are at `0x105277180`. The operation error decoder at `0x10086a89c` recognizes modeled error classes including `AccessDeniedError`, `InternalServerError`, `ServiceQuotaExceededError`, `ThrottlingError`, and `ServiceUnavailableException`. The AWS JSON specification also names `code` and `__type` body members as error discriminators and defines removal of namespace and suffix decorations. These supply concrete sources for a finite error classifier without emitting an arbitrary server message.

[AWS API Gateway's response reference](https://docs.aws.amazon.com/apigateway/latest/developerguide/supported-gateway-response-types.html) documents multiple causes of 403, including access denial, expired tokens, unsupported methods or resources, and filtering. Its [error header example](https://docs.aws.amazon.com/es_en/apigateway/latest/developerguide/set-up-gateway-response-using-the-console.html) includes `MissingAuthenticationTokenException`. These explain why status alone is insufficient; they do not prove which front end Kiro uses or which cause occurred. No operator IAM policy, profile ownership, or model entitlement was inspected.

### Recommended next candidate

Keep the inference request unchanged and observe the reported error discriminator before changing authentication, headers, endpoint, or model. The prepared diagnostic maps a finite set of known error types to fixed labels, treats unknown or ambiguous input as unknown or ambiguous, and discards raw values. It prefers one bounded error type header. If absent and the response declares JSON, it may inspect at most 16 KiB in memory for `code` or `__type`, without retaining the body or its message. This expands the previous no error body read policy and therefore requires exact plan review before any live use.

The tradeoff is a small additional read of a potentially sensitive error response in memory, with no raw output or persistence. Header only classification would avoid that read but miss conforming services that put the discriminator only in JSON. Guessing request headers would change the request without identifying the failed authorization check. The selected recommendation gathers evidence first, preserves all request and credential safeguards, and does not authorize another launch. The cause of the recorded 403 remains unproven.

## Bounded classifier observed access denial

You approved the bounded error classifier run at `3511f2a4aac847f8582ff49cfc88139de7600ea6`, with digest `1b53928cc89c8216a4e90ff1aff28144895806aa082c96e4723492e86ae0affb`. Its first request received HTTP 403 and the fixed `access_denied` label from a JSON error response. The allowed summary is in `verify.md`. Raw messages, bodies, and account fields were discarded. No second request occurred.

The new observation narrows the reported service error class but does not identify the failed access rule. It does not justify assuming a specific missing permission, invalid token, wrong profile, unavailable model, or required request header. The recommendation is a separately designed controlled comparison with the official client, or new primary authorization evidence, before further adapter changes or inference. Native client refresh, account reads, telemetry, and retries must be treated as a different experiment boundary rather than silently folded into this six attempt harness. The feasibility result remains `needs_evidence`.

## Operator native test used Opus 5.5

You reported a successful official Kiro test and supplied its session reference. Inspection was limited to that session's bounded `session.json` metadata. The separate `messages.jsonl` conversation and history files were not opened. The metadata model was the recognized public identifier `claude-opus-5.5`, while the gateway candidate requested `claude-sonnet-5`. You then explicitly confirmed that the successful test used Opus 5.5. The supplied session identifier, title, workspace paths, and conversation contents are not copied into this record.

Static `SessionMetadata::serialize` at `0x1022fa734` identifies the model member, and `write_kas_session_dir` at `0x10262eff8` writes session metadata separately from message records. This supports the limited metadata inspection. It does not prove the native session used the same request operation, backend route, selected profile, model controls, or bearer handling as the gateway. The native success is operator evidence for Opus access, not a controlled Sonnet comparison and not an explanation of the prior 403.

The recommended next candidate requests exactly `claude-opus-5.5`, matching the confirmed native model, while preserving the operation, source boundary, synthetic prompts, six case sequence, response diagnostics, and budgets. This is a prepared model change for review, not a fallback or a live approval. If approved, explicitly relink the current fixed snapshot and add the exact Opus mapping under the configuration lock. Existing link semantics clear old mappings if token bytes changed and preserve them otherwise. This avoids relying on a reference captured before the operator used the native client, which may refresh its token.

The [current Kiro model reference](https://kiro.dev/docs/models/) lists Opus 5.5 for US and EU profiles, at a higher credit multiplier than Sonnet 5. The [reasoning control reference](https://kiro.dev/docs/models/effort/) supplies the existing generic control semantics but its example table does not specifically list Opus 5.5. Keep `max_tokens: 1024` and disabled thinking as explicit candidate hypotheses for this model; support and enforcement are unproven. A rejection stops rather than dropping those controls or substituting another model. The benefit is a comparison with a model you have shown working. The cost is potentially higher credits and remaining uncertainty about native and gateway protocol equivalence. No account setting or inference changed during preparation.

## Approved Opus comparison did not resolve access denial

You approved relinking the current fixed snapshot, adding the exact Opus mapping, and one run at `d1fdd1a05ad7b6b9a112ed47ff756b14fa960bbe`, with plan digest `2eefec0fec9e8bd600bfac2f3867538049a64f05e51f960c64d2fa26771f5600`. Setup and local source validation succeeded. The first `claude-opus-5.5` request received HTTP 403 and the fixed `access_denied` class. Five dependent cases were unrun, and no retry followed. The allowed summary is in `verify.md`.

The named model difference was removed, but the candidate remained denied. This makes the model change insufficient as a remedy; it does not establish a particular authentication defect or prove exact account and profile equivalence with the native session. Raw server messages were not retained. The next investigation should trace the successful native client's actual authentication and request path before another candidate is proposed. Automatic endpoint switches, alternate credentials, header guesses, and repeated unchanged requests remain outside the approved experiment.

## Bundled agent identifies a different operation target

The next investigation inspected installed software files only. It did not launch Kiro, access credentials or conversations, change account settings, or send inference. The CLI binary contains the KAS launch path and references `KIRO_KAS_SERVER_PATH`, `KIRO_KAS_NODE_PATH`, and the ACP server entry point. The extracted software directory is `~/Library/Application Support/kiro-cli/kas/node_modules/`. Its `@kiro/agent/package.json` reports version `0.3.234`. The inspected `@kiro/agent/dist/server/acp-server.js` has SHA 256 `233e4dec77cd538e35b691d1fd0e12ca64a7c90d720c4dbf6979f21b4c45482f`. Line numbers below count newline bytes in that artifact.

The [current CLI V3 documentation](https://kiro.dev/docs/cli/v3/) confirms that V3 uses a shared agent implementation. This supports inspecting the bundled agent as a distinct implementation, but neither that page nor the supplied session metadata proves which code executed the operator's successful turn.

### Follow the bundled client, not the operation schema alone

| Evidence in the ACP server bundle | Finding |
|---|---|
| `createACPQClientFactory`, lines 432418 to 432455 | The default host is `https://runtime.${region}.kiro.dev`. The factory gets a token from its auth provider and constructs `KiroRuntimeClient` with that token and endpoint. |
| Client import at line 430527 and command import at line 431323 | Both resolve to the bundled `require_dist_cjs39()` runtime package, whose exports at lines 144694 onward lead to the inspected client and command. |
| Bundled runtime settings, lines 143951 to 143962 | Bearer authentication and `AwsJson1_0Protocol` are selected. The service target is `KiroRuntimeService`. |
| RPC serializer, lines 127426 and 127461; AWS JSON serializer, lines 137498 to 137504 | The request uses root path POST, `application/x-amz-json-1.0`, and an `x-amz-target` formed from service target and operation name. For this command it is `KiroRuntimeService.GenerateAssistantResponse`. |
| `GenerateAssistantResponse` schema at line 143894 | The schema also contains `/generateAssistantResponse`, but the selected RPC serializer does not use that HTTP binding. The schema alone is insufficient evidence to change the request path. |
| Response middleware, lines 144643 to 144659 | Despite its `addKrsSseMiddleware` name, this operation consumes a binary event stream through `parseBinaryEventStream`. It does not establish that this operation uses text SSE. |
| `AcpCallbackAuthProvider`, lines 432675 onward, especially 432798 | This provider requests the token from its host through `_kiro/auth/getAccessToken` and derives region from the cached profile ARN. This research did not invoke that callback or establish that its selected token and profile equal the gateway snapshot. |

The neighboring standalone `@amzn/kiro-runtime-service-typescript-client` package is materially different: its `dist-cjs/runtimeConfig.shared.js` selects REST JSON and no authentication by default. Its SHA 256 is `943b24680da6794f4351ca4152f03bdd451c3bc7b761fa11bfecb28948ca6fb5`. That package must not be substituted for the bundled code when inferring the ACP server's behavior. Early inspection of the operation schema suggested a path difference; tracing the actual bundled serializer corrected that interpretation.

### Recommended next comparison

The current probe sends `AmazonCodeWhispererStreamingService.GenerateAssistantResponse`. The bundled agent supplies concrete evidence for comparing `KiroRuntimeService.GenerateAssistantResponse` instead, while retaining the reviewed root path, content type, bearer source, exact Opus model, synthetic cases, limits, and no replay policy. This is a specific operation target difference, not evidence that login must be repeated or that the account lacks Opus access.

Prepare that single target change with a synthetic request assertion and an updated plan digest before requesting another live launch. A successful comparison could isolate the target as sufficient to resolve the denial for that run. A further denial would leave native credential selection and other request differences unresolved. The current finding does not prove the cause of the recorded 403, native session engine, or complete protocol equivalence. No probe code or approved run plan changed in this investigation, and no new live launch is authorized by this record.

## Authorized operation target comparison remained denied

After the two header values and bounded comparison were presented, you instructed, "yes, test them". The candidate changed only the outbound operation target, added its static provenance to the plan, and strengthened the existing synthetic request assertion to name the exact bundled target independently of the implementation constant. The full deterministic checks passed, and the tagged live test compiled and skipped with its gate disabled.

One run at clean commit `5c6e8c6fc2221e9194c7945f6caf1309556c24db`, plan digest `eecbd193c8aec5ab93ce6ed3c77d33a0160a7f6af7980070ade6c2f66b6b621d`, received HTTP 403 with `access_denied` on its first request. The summary is recorded in `verify.md`. Five dependent cases were unrun; no retry or credential mutation followed. The older target was not sent again because its prior denial already supplies that side of the comparison.

The target change alone was insufficient to resolve the denial for this snapshot and model. This does not prove that the two operation targets are equivalent or that the bundled agent would fail. The native callback's token and profile selection are still not established as equal to the fixed gateway source. Static tracing of that callback is the next focused investigation before proposing broader account access or another inference experiment. No raw error text was retained or inferred from the byte count.

## Reference gateway comparison and native profile trace

You asked to continue and supplied `https://github.com/jwadow/kiro-gateway` as a reference. Public source was fetched without authentication and pinned to commit `a5292ca04c7c6231e0b47673ac3f981f5a706e1e`. Only selected Python files, tests, and migration history were inspected. No reference code was executed, installed, or copied into the gateway. The installed CLI was inspected through file disassembly without starting its process. No credentials, account configuration, or conversations were read, and no inference was sent during this research.

### What the reference actually does

| Area | Source and comparison |
|---|---|
| HTTP path | Both Anthropic dispatch branches explicitly append `/generateAssistantResponse` to the runtime host. Our completed runs used `/`. See [the dispatch code](https://github.com/jwadow/kiro-gateway/blob/a5292ca04c7c6231e0b47673ac3f981f5a706e1e/kiro/routes_anthropic.py#L720). |
| Operation and headers | It uses `application/x-amz-json-1.0` and `AmazonCodeWhispererStreamingService.GenerateAssistantResponse`. It also supplies IDE identity strings, an opt out value, agent mode, and SDK metadata. These other differences are not proven requirements and are not adopted. See [the header builder](https://github.com/jwadow/kiro-gateway/blob/a5292ca04c7c6231e0b47673ac3f981f5a706e1e/kiro/utils.py#L61). |
| Credential selection | Its SQLite reader searches social, current OIDC, then legacy OIDC token keys. It also reads device registration for refresh. A token's profile ARN takes precedence, with the state profile as fallback; region is derived from the state ARN. Our fixed OIDC record and same transaction profile read remain unchanged. See [the reader](https://github.com/jwadow/kiro-gateway/blob/a5292ca04c7c6231e0b47673ac3f981f5a706e1e/kiro/auth.py#L248). |
| 403 handling | It forces token refresh and retries after 403. Its unit test mocks a 403 followed by 200 and verifies that refresh is invoked. This proves intended retry behavior, not that expiry caused our denial or that renewal would resolve it. See [the client](https://github.com/jwadow/kiro-gateway/blob/a5292ca04c7c6231e0b47673ac3f981f5a706e1e/kiro/http_client.py#L241) and [the test](https://github.com/jwadow/kiro-gateway/blob/a5292ca04c7c6231e0b47673ac3f981f5a706e1e/tests/unit/test_http_client.py#L218). |

The README's claim that SSO does not need a profile ARN is stale relative to its executable route code. The [runtime migration commit](https://github.com/jwadow/kiro-gateway/commit/07d24fc706fce3a40c39a2579bc0dcfdbe238e42) adds state profile loading and profile inclusion for all authentication types. A [followup commit](https://github.com/jwadow/kiro-gateway/commit/90d0509b9ce5aa3f725214ec4e5342673cf7e50e) applies this to another dispatch branch. These establish what the reference changed; they do not independently prove AWS requirements or successful compatibility with the operator's account.

### Native callback findings

The installed callback handlers at `0x101a65f3c` and `0x10279e428` include provider specific coordinated refresh branches for Builder ID, social, and external IdP tokens. This pass did not fully establish branch precedence or which branch served the supplied native session. Do not infer it from the presence of symbols alone.

The callback calls `profile_arn_from_db` at `0x1027503c4`, which calls `Database::get_auth_profile` at `0x1022fb854`. That method passes the 25 byte constant `api.codewhisperer.profile` from `0x11b85f55e` to `get_entry` at call site `0x1022fb88c`, then decodes the record. This is the same named state profile key our harness reads. It narrows the source uncertainty for that branch without proving equal runtime values, selected provider, or token bytes. No broader account scan is justified by this trace.

### Prepared next experiment

Use the reference's explicit `/generateAssistantResponse` path with its older operation target, retaining our existing model, frozen session reference, profile source, controls, six synthetic cases, strict bounds, and no replay policy. This changes only the path relative to the first Opus candidate at `d1fdd1a`, and both path and target relative to the most recent candidate at `5c6e8c6`. It is not an automatic fallback. The allowlist accepts only the two exact regional URLs with this path and rejects root, alternate path, encoded path, query, and fragment variants. Synthetic wire checks exercise the exact request URI and header pair using local TLS servers.

This is evidence from a separate implementation, not proof that the installed native client uses this path or that it will resolve 403. It is stronger support for a path comparison than an unused schema annotation. Copying the reference's credential search or automatic refresh would change account boundaries and obscure this comparison. The recommendation is to test this bounded path candidate first after a separate live review. Current work prepares that concrete candidate only; the prior live approval is consumed.
