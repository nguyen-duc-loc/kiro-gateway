# 0004. Experimental Claude Code coding bridge

**Date**: 2026-10-02
**Status**: Proposed

## Summary

You enable an experimental local API and use Claude Code to fix a small Go bug with its own tools and permissions. The bridge uses the Opus mapping exercised by the feasibility harness. The confirmed client amendment tolerates the observed effort field and four beta markers while explicitly ignoring their requested behavior. Instructions, completion, reasoning, token counts, and output controls have explicit compatibility limits. A successful coding task establishes this narrow experiment, while GA compatibility remains pending.

## Requirements

As the local operator, you want a real file edit, test command, tool result exchange, and followup turn through Kiro before expanding the product.

1. **AC-1**: Plain `serve` retains authenticated health behavior and never opens Kiro's store. `serve --experimental-bridge` enables the specified Messages surfaces only with a valid linked configuration and the exact `claude-opus-5.5` to `claude-opus-5.5` mapping. Startup and health do not read credentials or dispatch inference.
2. **AC-2**: Every local route authenticates before credential access. Each inference uses one fresh combined snapshot matching the startup session reference and the process profile pin. Changed, expired, unavailable, or invalid sources fail before dispatch, with no renewal, relinking, fallback, or settings changes.
3. **AC-3**: The gateway supports streaming and ordinary JSON Messages responses plus local token estimation. Instructions, text, tool definitions, tool IDs, arguments, error results, and followup history obey the declared transformations. Unsupported input fails before account access rather than disappearing silently.
4. **AC-4**: Claude Code receives incremental text and complete tool calls, executes its own `Read`, `Edit`, and `Bash` tools under manual permissions, returns matching results, passes the disposable repository's tests, and completes a subsequent user turn. The gateway never executes a model supplied tool.
5. **AC-5**: Only a validated clean upstream stream end can produce the explicitly inferred `end_turn` or `tool_use` result. Cancellation, malformed frames, exceptions, unfinished tools, resource exhaustion, and detected truncation never produce a successful terminal client event. Missing whole frames at a clean boundary remain an acknowledged limitation.
6. **AC-6**: Exactly one inference request can be active. Overlap gets a clear busy error without queuing or account access. Client disconnect and process shutdown cancel work, cleanup is bounded, and neither the gateway nor its HTTP transport retries or replays an upstream request.
7. **AC-7**: Experimental behavior is visible in startup output, response headers, and documentation. Instructions become user context, known reasoning is discarded, usage is estimated, and `max_tokens` is advisory. No resolved model identity, exact token cap, faithful reasoning continuation, or GA compatibility is claimed.
8. **AC-8**: The gateway stores no conversation or new account metadata. Logs, fixtures, and retained verification evidence exclude credentials, profile values, fingerprints, machine fingerprints, prompts, tool arguments, results, and raw upstream errors. Existing settings and health checks continue to pass.
9. **AC-9**: An isolated offline exercise establishes the actual traffic and tool exchange of Claude Code `2.1.287`, before live work. Deterministic verification uses synthetic credentials and loopback services; the repository check script passes.
10. **AC-10**: A separately reviewed live acceptance plan permits at most 20 inference dispatches within 20 minutes, with exact versions, destinations, code, and task recorded. Its outcome distinguishes an observed experimental coding loop from incomplete evidence and from GA acceptance.

## Decision

**Chosen option**: Build an explicitly enabled experimental bridge on the existing Go stack, using the successful feasibility request as the upstream baseline.

Keep the standard library HTTP and JSON stack, the existing SQLite reader, and the accepted package boundaries. Add no runtime dependency, provider framework, credential source, schema migration, skill, or MCP server. (basis: your choices, [project context](../../../AGENTS.md), specs [0001](../0001-stack-architecture/index.md), [0002](../0002-local-configuration-credentials/index.md), and [0003](../0003-first-claude-code-loop/index.md))

**Implementation skills**: `golang-security` (`samber/cc-skills-golang`, [SKILL.md](../../../.agents/skills/golang-security/SKILL.md)) and `go-concurrency` (`cxuu/golang-skills`, [SKILL.md](../../../.agents/skills/go-concurrency/SKILL.md)).

**Original decision confirmed**: You accepted the original design on October 2, 2026, after independent review, the three approved corrections, and the cleanup deadline clarification. The lifecycle status remains `Proposed` because implementation has not begun; live execution still needs the concrete run review described below.

**Client contract amendment, confirmed October 2, 2026**: The pinned client sends `output_config.effort: "high"` and four beta tokens despite the original launch controls. Accept only the effort shape and beta tokens specified below as explicitly ignored compatibility inputs. Pin `--effort high` in the client launch. Keep the evidenced Kiro wire request unchanged. You accepted this amendment after an independent review found no material decision gaps. It ratifies these exceptions for the experimental build; it does not authorize live execution. The observations and alternatives are in [rationale.md](rationale.md#client-contract-amendment-october-2-2026).

This new spec builds on 0003 and preserves its experiment record. It governs an experimental milestone within scope feature 4. Your design choices do not authorize a live run. The feature keeps its GA verification workflow, including independent review, while the release promise of this milestone is experimental.

The following exceptions apply only with the experimental flag. They amend the stronger instruction role and terminal state requirements in 0001 for this milestone; they do not redefine the eventual GA contract.

| Behavior | Selected experimental rule |
|---|---|
| System instructions | Preserve text and order, prepend once to the first user message in each translated request history. No distinct system role or priority guarantee. |
| Completion | Derive `end_turn` or `tool_use` from a validated clean EOF and complete semantic content. This is an inference, not an upstream stop reason. |
| Reasoning | Validate and discard known `reasoningContentEvent` objects in all turns. Do not display, save, sign, or replay reasoning. |
| Usage | Supply the local estimates defined below. Upstream usage is not a verified measurement in this baseline. |
| Output controls | Accept `max_tokens` as advisory and only the observed `output_config.effort: "high"` as ignored. Omit upstream model control fields, preserve the successful request baseline, and enforce independent local bounds. No effort level is promised. |
| Model identity | Echo the requested client model in the client response. The configured target is known; the actual serving model remains unverified without a matching upstream echo. |
| Beta requests | Tolerate only the four named tokens below. They enable no beta feature and are never forwarded upstream. All body validation still applies. |

## Feature design

### Data and ownership

| Entity | Fields and identity | Lifetime and relationships |
|---|---|---|
| Settings snapshot | Existing version 1 document, required session and exact mapping for experimental startup | Loaded once under the existing process lock. One process owns one immutable copy; other saved mappings remain untouched but are unsupported by this milestone. |
| Profile pin | Optional 32 byte digest before the first valid inference read, required afterward | One pin per process, held only in memory. The admitted inference owner establishes and compares it. No automatic reset. |
| Request | Required random local request ID, exact model, normalized system text, ordered messages and tools, response mode, advisory controls | One accepted HTTP request owns its history and response state. Nothing is reused as a conversation cache. |
| Message and block | Ordered user or assistant messages containing text, tool calls, or tool results | Many messages per request and many blocks per message. Tool IDs identify calls within the request history. |
| Tool call | Required ID, name, and JSON object input | Each historical call has exactly one matching result in the next user message. Each new upstream call belongs only to the current response. |
| Credential snapshot | Existing private token, expiry, profile ARN, region, and digests | Read for each admitted inference in one transaction, then released after that request. Never exposed to the bridge or gateway transport. |

There is no persistent entity, index, foreign key, or schema change. The configuration reference still identifies token bytes rather than a person. Profile selection is pinned only for this process, as you confirmed.

`internal/cli` loads settings and wires the server, bridge, and adapter. `internal/gateway` authenticates, bounds HTTP input, owns response writes, and implements the local routes. `internal/bridge` owns client protocol validation, normalized messages, estimates, and response construction. It imports neither HTTP transport nor credential storage. `internal/kiro` implements upstream encoding, destination selection, snapshot use, frame decoding, and cancellation. Its credential interface is defined at that consuming boundary.

The bridge consumes a synchronous operation shaped as `Generate(ctx, request, emit) (end, error)`. Its explicit request and event types live at the consuming bridge boundary. Events are text deltas or complete tool calls; `end` carries the named completion basis. A nil error without `inferred_clean_eof` is invalid. The adapter may emit text before returning an error; callers must preserve that distinction. No raw Kiro frame or credential appears in an inner type.

### Local surface and enablement

| Surface | Inputs | Output | Authentication and key failures |
|---|---|---|---|
| `serve --experimental-bridge` | Existing token, saved session and exact mapping, optional existing `--listen` | Foreground server and one fixed compatibility notice | Missing setup fails startup without reading Kiro. Plain `serve` retains its existing behavior. |
| `GET /healthz` | No body | Existing process health JSON | Existing bearer authentication and health meaning; no readiness or account test. |
| `POST /v1/messages` | JSON body with exact `model`, positive integer `max_tokens`, ordered `messages`; optional supported fields below | SSE when `stream: true`, otherwise one Messages JSON object | Bearer required. 400 unsupported or invalid input, 409 source changed, 503 source unavailable or expired, 529 busy. |
| `POST /v1/messages/count_tokens` | Same supported conversation fields, without requiring `max_tokens` or accepting `stream` | `{"input_tokens": N}` from the local estimator | Bearer required. No credential read, upstream call, or inference admission slot. 400 invalid input. |

All routes authenticate before routing or processing a body. Accept exactly one `Authorization: Bearer <gateway token>` using the existing constant time comparison. Do not add `x-api-key` authentication in this slice. For Messages routes, reject a simultaneous `x-api-key` header as ambiguous after bearer validation. Accept `Content-Type: application/json` with an optional UTF 8 charset. Reject encoded request bodies. The supported API version is `anthropic-version: 2023-06-01`; missing or different values fail with 400. A query of `beta=true`, as used by client libraries, is tolerated without enabling extra semantics; other query keys fail. Both Messages routes apply the beta header contract below after authentication and before admission or account access.

With the flag absent, inference paths remain 404 after authentication. No `/v1/models`, management endpoint, token refresh, or external discovery is added. Wrong methods return 405 with `Allow`; unknown paths return 404. The client baseline keeps gateway model discovery disabled.

The experimental startup notice names every compatibility limit from the decision table, including ignored effort and unsupported beta features. Inference and token count responses, including their authenticated errors, add `X-Kiro-Gateway-Compatibility: experimental`, `X-Kiro-Gateway-Usage: estimated`, `X-Kiro-Gateway-Completion: inferred`, `X-Kiro-Gateway-Controls: advisory`, `X-Kiro-Gateway-Effort: ignored`, and `X-Kiro-Gateway-Betas: unsupported`. These fixed labels describe policy even when the input omits effort or beta headers; they do not claim a failed request was accepted. Preserve `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`.

### Beta header contract

`internal/gateway` reads every `anthropic-beta` header value and applies the byte limit below before splitting or trimming. Absence, or a single empty value after trimming ASCII space and tab, means no beta tokens. Otherwise, split every value on commas, trim only surrounding ASCII space and tab, and accept any subset of the exact tokens below in any order. Limit the combined values to 512 bytes, counting one comma between repeated header values, and at most four tokens. Empty elements, duplicate tokens across any header values, unknown tokens, different case, parameters, and malformed values fail with 400 `invalid_request_error` and the fixed message `Unsupported or invalid beta header.` before inference admission or account access. The existing incoming header cap still applies.

| Exact tolerated token | Local meaning |
|---|---|
| `claude-code-20250219` | Observed client marker only; grants no additional route, tool, or field. |
| `interleaved-thinking-2025-05-14` | No thinking input or reasoning continuation is enabled. The existing reasoning discard rule remains. |
| `mid-conversation-system-2026-04-07` | No system role inside `messages`, changing tool catalogue inside history, or per message controls are enabled. Only the top level system transformation is supported. |
| `effort-2025-11-24` | No effort control is enabled. Only the narrow body shape below is tolerated. |

These are explicit experimental exceptions, not implementations of the named Anthropic features. Header presence cannot relax body validation. The accepted header set and accepted effort shape are independent: neither requires the other. Discard accepted beta values at the local HTTP boundary; do not send them to the bridge request, Kiro, logs, estimates, or response echoes. An unsupported payload still fails even with all four tokens. Ordinary product diagnostics retain only fixed categories, while explicit offline characterization may retain these public protocol constants.

### Request contract

Validate valid UTF 8, exact JSON names, one root object, no duplicate members at any depth, a maximum nesting depth of 64 containers, and no trailing JSON. Apply length limits before allocating from untrusted lengths. Preserve JSON numbers without a float64 round trip when translating arguments and schemas. Return fixed errors rather than echoing unknown names or input values.

| Field | Supported shape and behavior |
|---|---|
| `model` | Required string exactly `claude-opus-5.5`. Resolve through the frozen saved mapping, requiring the same target. No alias expansion, substitution, or alternate model fallback. |
| `max_tokens` | Required integer from 1 through 65536 for Messages. Retained as an advisory request value; neither forwarded nor used to invent a token stop reason. |
| `output_config` | Optional on either Messages route. When present, it must be exactly an object with one member, `"effort": "high"`. Validate, then discard it before constructing the normalized request. Empty objects, null, other effort levels or types, `format`, and every extra member fail with 400 `invalid_request_error` and fixed message `Unsupported or invalid output configuration.` No per message or content block `output_config` protocol field is supported. This does not reserve that name inside arbitrary tool schemas, tool inputs, or text. |
| `stream` | Optional boolean, default false. Both modes use the same normalized request and completion rules. |
| `system` | Optional string or ordered array of text blocks. Each block has `type: text` and string `text`; concatenate blocks with two newline characters. Empty system text adds no prefix. |
| `messages` | Required nonempty array starting with user and ending with user, alternating user and assistant roles. Each content is a string or a nonempty array of the supported blocks. Reject assistant prefills and adjacent equal roles in this first baseline. |
| Text block | `type: text`, required string `text`. Preserve bytes, order, and whitespace. |
| Assistant tool block | `type: tool_use`, required nonempty string `id`, valid tool `name`, and JSON object `input`. No duplicate IDs across the request history. |
| User result block | `type: tool_result`, required `tool_use_id`, optional boolean `is_error` default false, required string `content` or an array of text blocks. An empty string or empty array is a valid empty result. |
| `tools` | Optional array of unique client tool definitions with `name`, optional string `description`, and required object `input_schema`. The schema root must declare `type: object`. Preserve the schema structurally, including `additionalProperties`; no general JSON Schema engine or silent keyword removal. |
| `tool_choice` | Absent or `{ "type": "auto" }`, or `{ "type": "none" }`. Auto may include boolean `disable_parallel_tool_use`; when true, a response with several calls fails. None requires omission of tool definitions upstream and rejects any returned call. Forced `any` or named tool selection is unsupported. |
| `metadata` | Optional object containing only optional string `user_id`. Validate and discard it. It is not a conversation ID, account selector, or log field. |
| `cache_control` | On a system text block, message text block, or tool definition only: `{ "type": "ephemeral" }`, optionally with `ttl: "5m"` or `"1h"`. Validate and discard this optimization hint, without claiming cache behavior. |
| `thinking` | Absent or `{ "type": "disabled" }`. This controls the accepted client shape, not Kiro's behavior. Enabled or adaptive requests, thinking history, and signatures are unsupported. |
| `stop_sequences` | Absent or an empty array. An empty array adds no stopping condition; nonempty sequences are unsupported. |

All other fields or block types fail before credential access, including images, documents, server tools, deferred tool references, structured output, unsupported `output_config` shapes, sampling controls, nonempty stop sequences, and explicit reasoning budgets. Do not broadly discard unknown fields to make a newer client work. If the pinned client cannot operate inside this contract, the offline milestone records the exact structural mismatch and returns it to architecture before live work. That is an evidence outcome, not permission to invent field semantics. The effort exception does not allow adaptive thinking, thinking blocks, system messages inside history, or mid conversation control updates. It supplies no reasoning budget and is not converted into prompt text or a Kiro model control.

Tool names contain 1 through 128 ASCII letters, digits, underscores, or hyphens. Tool IDs contain 1 through 256 visible ASCII bytes. Assistant text may precede tool blocks; text after the first tool block is unsupported. In the following user message, result blocks must come first, exactly once for each immediately preceding call, before any ordinary text. A result for an unknown, already satisfied, or older call fails. A historical call need not still appear in the current tool catalogue. Returning no ordinary text with results is supported. Errors remain results with `is_error: true`, not successful tool output.

The gateway validates that arguments are an unambiguous JSON object, not that they satisfy every JSON Schema keyword. Claude Code owns tool permission and tool argument validation. New upstream calls must name an offered current tool, and their IDs cannot collide with a call already present in client history. No placeholder IDs, rewritten names, invented results, or repaired arguments are allowed.

### Upstream request translation

Use the fixed regional destinations and application header contract exercised by 0003's successful run, as frozen in `internal/kiro/testdata/probe-plan.json`. Port production logic deliberately; do not link production code to test helpers or silently change the existing live harness.

1. After authentication, complete local validation and acquire the one inference slot. Read a fresh `Reader.ReadProfileSnapshot` against the frozen session reference. The first successful read establishes the in memory profile digest; later reads require equality. Failure after establishing the pin does not clear it.
2. Route `us-east-1` to `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` and `eu-central-1` to the corresponding `runtime.eu-central-1.kiro.dev` URL. No other region, port, path, override, redirect, or fallback is supported. The successful live evidence covers the first region; the second remains a supported candidate needing its own live evidence.
3. Use the access token as bearer authentication and the same snapshot's ARN as `profileArn`. Send `Content-Type: application/x-amz-json-1.0` and `X-Amz-Target: AmazonCodeWhispererStreamingService.GenerateAssistantResponse`. Preserve the plan's fixed IDE compatibility user agents, opt out value, agent mode, SDK headers, `Accept: */*`, and `Accept-Encoding: identity`.
4. Derive the application fingerprint from SHA 256 of hostname, a hyphen, username, and `-kiro-gateway`, using the username precedence already specified in 0003. Generate a fresh UUID v4 invocation ID and a fresh UUID v4 `conversationId` for each HTTP inference request with `crypto/rand`. Full supplied history makes the request self contained; neither ID is a persistent conversation key. Fail before dispatch if required metadata or randomness cannot be obtained.
5. Form `conversationState` with `chatTriggerType: MANUAL`, historical messages, and the final user message as `currentMessage.userInputMessage`. Omit empty history. All user messages use `origin: AI_EDITOR` and the exact model target. Prepend joined system text and two newline characters only to the first user's ordinary text, including when that first user is in history. Do not mutate the client history or accumulate the prefix across requests.
6. Join ordinary text blocks within each message with two newline characters. Preserve their internal bytes. Historical assistant messages become `assistantResponseMessage`; tool blocks become `toolUses` with exact IDs, names, and decoded object inputs. Empty assistant text uses the reference's literal `(empty placeholder)` only on the upstream wire.
7. Put current tool definitions in `userInputMessageContext.tools` as `toolSpecification` entries with `name`, `description`, and `inputSchema.json`. Omitted descriptions become the empty string. Historical user contexts omit tool definitions. Unlike the probe's deliberately reduced fixture schema, preserve every supplied schema keyword. This general schema path requires new coding loop evidence.
8. Map results to `userInputMessageContext.toolResults` with exact `toolUseId`, `status: success` or `error`, and `content` as text objects. Preserve result block order; each text block becomes one text object. An empty array becomes one empty text object. An otherwise empty current user content remains an empty string. Omit an empty context object.
9. Omit `additionalModelRequestFields`, `output_config`, effort, and client beta headers. No timeout, estimated count, model echo, or short response is evidence that the upstream enforced `max_tokens` or the client's requested effort. Removing the accepted compatibility inputs from an otherwise identical request must leave its normalized content and Kiro semantic request unchanged, apart from independently generated request metadata.

The fixed account source, schema, ownership, descriptor checks, record sizes, expiry rules, busy timeout, and digest formulas remain those of 0002 and 0003. There is no early real source inspection for startup, health, counts, malformed requests, or busy requests.

### Stream state and client output

Require HTTP 200 and the EventStream content type before interpreting the upstream body. Validate AWS EventStream lengths, header types, prelude checksum, and complete frame checksum before consuming JSON. Error or exception frames fail. Retain 0003's finite event vocabulary: `assistantResponseEvent`, `toolUseEvent`, `messageMetadataEvent`, `metadataEvent`, `contextUsageEvent`, and `meteringEvent`, extended by the already observed `reasoningContentEvent` discard rule.

Use the existing known semantic fields and numeric validation from the feasibility decoder. Ignore only extra top level metadata members after bounded duplicate free JSON validation. Unknown semantic fields or event names fail. Validate known reasoning as a bounded JSON object, discard every value, and count it; it supplies neither output nor completion. Optional upstream model echoes must match the configured target. Missing echoes do not establish identity. Validate recognized usage metadata but do not promote it to measured usage in this milestone.

The request state progresses through `validated`, `admitted`, `source_validated`, `dispatched`, `text`, optional `tools_pending`, then `inferred_complete` or `failed`. Cancellation can fail any active state. Metadata and reasoning do not advance semantic state. EOF with no nonempty text and no complete tool call fails.

Text can stream immediately after frame validation. At the first tool fragment, stop exposing semantic output until the entire upstream response validates. A later nonempty text event fails as unsupported ordering. Assemble up to 16 tool calls by exact stable `toolUseId`, keeping first appearance order. A first fragment requires ID and name; continuations require the same ID and cannot change the name. Append only string `input` fragments. Require `stop: true` exactly once per call, no fragments after stop, valid object JSON at stop, and no duplicate ID reuse. Interleaved fragments for different IDs are allowed within these rules.

At EOF, require an error free body end at a complete validated frame boundary, no pending call, no cancellation, no deadline or resource violation, and successful upstream cleanup. Only then emit complete tool calls to the bridge. This prevents a subsequently rejected upstream stream from exposing an executable partial call. Text already delivered remains partial text on failure. A missing whole frame at EOF cannot be detected and remains part of the accepted experimental limitation.

For SSE, use `Content-Type: text/event-stream` and flush each event. Emit `message_start` lazily just before the first normalized output event, after the adapter has accepted upstream status and headers. Use a locally generated `msg_` plus 32 lowercase random hex digits, `type: message`, `role: assistant`, empty `content`, requested `model`, null `stop_reason` and `stop_sequence`, and estimated input usage with initial output usage zero. Open a text block lazily, emit `text_delta` fragments, and close it before any tool blocks. Block indexes are consecutive from zero. The adapter does not need to expose transport headers to the bridge. A failure before the first output event remains an ordinary JSON error.

After validated completion, emit each complete tool block in order: `content_block_start` with exact `id`, `name`, and empty object `input`; one `input_json_delta` containing the complete encoded input object; then `content_block_stop`. Finish with one `message_delta` containing `stop_reason: tool_use` when any call exists, otherwise `end_turn`, null `stop_sequence`, and cumulative estimated output usage. Finally emit `message_stop`. Do not emit `max_tokens`, `refusal`, or a measured terminal claim without evidence. (basis: [official streaming reference](https://platform.claude.com/docs/en/build-with-claude/streaming), accepted experimental completion choice)

For ordinary JSON, buffer within the same limits and return the corresponding complete Message object only on inferred completion. A failure returns the mapped error, with no partial successful object. The normalized content and estimates must match the streaming path.

Do not add SSE heartbeat workers in this first implementation. The two minute overall deadline also bounds a reasoning period with no visible text. If client compatibility requires heartbeats, add their explicit single writer design through an amendment rather than a concurrent writer on `ResponseWriter`.

### Token estimates

Use one deterministic estimator, with no tokenizer dependency. Canonically encode an object containing only normalized `system`, `messages`, and active `tools`, using compact Go JSON encoding with deterministic object key order and HTML escaping disabled. Strip cache hints and metadata before this encoding. Define estimated input tokens as `max(1, (UTF8_bytes + 3) / 4)` with integer division. Do not include request IDs, model, controls, accepted effort or beta inputs, or the extra upstream instruction prefix in this input object.

For output, apply the same formula to the canonical content array of delivered text and complete tool blocks; empty initial content has zero output tokens. The final SSE count is cumulative and equals the ordinary JSON count. Include `input_tokens` and `output_tokens`; omit optional cache and billing fields rather than fabricate measured values. `/count_tokens` returns the same input estimate for the same normalized conversation. These values are approximate compatibility values, not a context capacity, pricing, billing, or output limit guarantee.

### Bounds, concurrency, and failures

| Resource | Limit and behavior |
|---|---|
| Incoming JSON | 4 MiB, at most 256 messages, 1024 content blocks in total, 128 tool definitions, and 64 KiB per schema |
| Generated upstream JSON | 8 MiB, checked before dispatch |
| Upstream response | 16 KiB response headers, 8 MiB total body, 1 MiB per complete frame and decoded event |
| Semantic output | 2 MiB text, 256 KiB arguments per tool, at most 16 calls, all still within the total body bound |
| Request timing | Two minutes from inference admission through completion, shortened by client or process cancellation |
| Upstream inactivity | 30 seconds from dispatch, including DNS, connect, and TLS; reset only by received response bytes |
| HTTP lifecycle | Existing five second header and request body read limits, 16 KiB incoming header cap, 60 second idle connection limit; per write deadline of five seconds for Messages |
| Cancellation cleanup | At most five seconds to close and join owned work; process shutdown retains its five second outer bound |

These are resource bounds, not model context limits or an exact heap usage promise. Exceeding them fails instead of trimming input, producing an apparently complete answer, or replaying a shorter request.

Use one nonblocking admission token around the complete inference lifetime, including credential access and cleanup. The admitted owner alone touches the profile pin. Busy requests return 529 immediately. Health and counts remain independent; each count request retains the input bound. Keep stream processing synchronous where possible. Any transport worker must have cancellation, an owner that waits for it, and bounded handoff. A failure to finish cleanup disables further inference for that process rather than releasing the slot onto unresolved work. (basis: project `go-concurrency` skill)

The existing five second server `WriteTimeout` cannot govern a two minute stream. In experimental mode, use per response write deadlines with one response writer owner; retain a five second deadline for health and local error writes. Close failed client connections and cancel upstream promptly. A response write failure is never a completed request even if upstream reached EOF.

Use an explicit transport with TLS verification, no environment proxy, no redirects, no transparent decompression, and no automatic replay. Use a fresh HTTP/1.1 connection for each request, disable connection reuse, and provide no request body rewind callback. Preserve the feasibility destination policy, including rejecting nonpublic resolved destinations, binding connections to the validated resolution, and retaining the intended TLS server name. Do not forward client authorization, cookies, arbitrary headers, or URLs upstream. Keep literal SDK retry metadata from the reference as metadata only. A dispatched request counts even when dial or HTTP status fails.

Messages errors use `{ "type": "error", "error": { "type": TYPE, "message": FIXED_MESSAGE }, "request_id": LOCAL_ID }`. Generate the local request ID as `req_` plus 32 lowercase random hex digits and add `request-id` to the HTTP response. If randomness fails, return a fixed 500 error with no request ID and no account access. Authentication errors on Messages routes use 401 `authentication_error`; validation uses 400 `invalid_request_error`, oversized local input 413 `request_too_large`, unsupported model 400 `invalid_request_error`, busy 529 `overloaded_error`, upstream throttle 429 `rate_limit_error`, upstream authentication or forbidden responses 502 `api_error`, other upstream or protocol failures 502 `api_error`, and deadline expiry 504 `api_error`. Source failures use the complete table below. Preserve existing health error formatting. Never forward raw service error text or a service authentication status as a local bearer failure.

Every source failure prevents dispatch. Map errors by their typed identity with `errors.Is`, never by matching raw error text. The adapter adds a typed profile digest mismatch result. All client errors in this table use `api_error`; the fixed category is for diagnostics and runner outcomes, not an echoed source value.

| Source result | HTTP status before streaming | Fixed category | Fixed client message |
|---|---|---|---|
| `credentials.ErrSource`, including an unrecognized source failure | 503 | `source_unavailable` | Saved Kiro source is unavailable or unsupported. Check Kiro CLI sign in and local file access. |
| `credentials.ErrRecord` | 503 | `credential_invalid` | Saved Kiro token record is invalid or unsupported. Stop the gateway, sign in through Kiro CLI, link again, restore the model mapping, and restart. |
| `credentials.ErrExpired` | 503 | `credential_expired` | Saved Kiro credential has expired. Stop the gateway, sign in through Kiro CLI, link again, restore the model mapping, and restart. |
| `credentials.ErrBusy` | 503 | `source_busy` | Saved Kiro source is busy. Wait for Kiro CLI, then retry explicitly. |
| `credentials.ErrTimeout` | 504 | `source_timeout` | Reading the saved Kiro source timed out. Check local source availability before retrying. |
| `credentials.ErrChanged` | 409 | `session_changed` | Saved Kiro session has changed. Stop the gateway, link again, restore the model mapping, and restart. |
| `credentials.ErrProfileInvalid` | 503 | `profile_invalid` | Saved Kiro profile is invalid. Check the selected profile through Kiro CLI, then restart the gateway. |
| `credentials.ErrProfileUnsupported` | 503 | `profile_unsupported` | Saved Kiro profile is unsupported by this gateway. Check the supported profile and region before restarting. |
| Adapter profile digest mismatch | 409 | `profile_changed` | Selected Kiro profile has changed. Check the selected profile through Kiro CLI, then restart the gateway. |
| `credentials.ErrCanceled` when the client remains connected | 503 | `source_canceled` | Reading the saved Kiro source was canceled. Retry only when the gateway is ready. |

Cancellation has explicit precedence. A disconnected client receives no further write; record `canceled` and clean up. A live runner or inference deadline that caused cancellation uses 504 with `budget_exhausted` or `timed_out`, respectively, if the client is still writable. Process shutdown or a runner stop caused by another failure uses 503 `api_error` with a fixed stopping message and category `stopping` or `run_stopped`. An independent canceled source operation uses the table entry. Do not interpret every `ErrCanceled` as proof that the client disconnected. After SSE has begun, these conditions use the existing stream error and closure rule instead of changing the HTTP status.

Once SSE headers are committed, attempt one fixed `event: error` with the same error type and safe message, then close without a successful terminal update or `message_stop`. On disconnect, perform cleanup without trying to write. Diagnostics distinguish cancellation, timeout, incomplete stream, unsupported contract, source failure, busy, and response write failure. The gateway does not claim that local cancellation stops remote billing or computation.

Recover changed tokens by stopping the gateway, explicitly relinking, restoring the mapping, and restarting. Recover profile changes by checking the selected profile outside the gateway and restarting. Do not adopt changed bytes during a process run. A transport or model error leaves the profile pin intact and does not trigger a corrective request. Client initiated retries are separate requests and are not promised absent merely because the gateway has no retry loop.

### Configuration and client baseline

No new secret or saved setting is introduced. `KIRO_GATEWAY_TOKEN` retains its current role and validation. Add only the experimental serve flag to the product command. Model mappings remain explicit manual JSON configuration under the existing stop, edit, check, and restart workflow; do not add a model setter command in this slice.

The proposed baseline is macOS arm64, Go `1.27.1`, Kiro CLI `2.8.0`, and installed Claude Code `2.1.287`. The latter two versions and the client help were inspected on October 2, 2026. Freeze exact binaries or their digests in the later acceptance plan; version drift needs review, not an automatic update.

Use a disposable repository and a temporary private `CLAUDE_CONFIG_DIR`, without changing `HOME`. Supply `ANTHROPIC_BASE_URL` as the numeric loopback listener, and `ANTHROPIC_AUTH_TOKEN` as the gateway token. Clear competing API keys, provider selection variables, effort overrides, and custom header variables in the child environment. Select `--model claude-opus-5.5`, `--effort high`, `--safe-mode`, `--tools Read,Edit,Bash`, `--permission-mode manual`, and `--prompt-suggestions false`. The effort flag pins the observed client serialization; it does not choose Kiro reasoning depth. Keep the normal Claude Code system prompt. Do not use `--bare`, because its documented local help describes different authentication rules. No global client configuration is edited.

Set `ANTHROPIC_MODEL`, `ANTHROPIC_DEFAULT_OPUS_MODEL`, `ANTHROPIC_DEFAULT_SONNET_MODEL`, and `ANTHROPIC_DEFAULT_HAIKU_MODEL` to the same exact mapping. This directs helper choices to the declared model; the gateway still rejects any other received model. Set `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`, `CLAUDE_CODE_DISABLE_TERMINAL_TITLE=1`, `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1`, `CLAUDE_CODE_DISABLE_THINKING=1`, `CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK=1`, `DISABLE_PROMPT_CACHING=1`, and `CLAUDE_CODE_MAX_OUTPUT_TOKENS=4096`. Leave gateway model discovery and tool search disabled. Keep the beta suppression candidate even though this binary still emits the four tolerated tokens; it is not an assurance of empty headers. Other effort settings remain unsupported. These controls still need the full offline baseline; the preliminary capture established only the initial request shape. (basis: inspected local help, recorded offline observations in [rationale.md](rationale.md#client-contract-amendment-october-2-2026), [environment reference](https://code.claude.com/docs/en/env-vars), [settings reference](https://code.claude.com/docs/en/settings))

The documentation describes `CLAUDE_CODE_MAX_RETRIES` but does not establish whether zero disables retries in this installed version. The offline capture explicitly tests zero as a candidate setting and records actual attempts after a synthetic 429, 502, and interrupted stream. Use it in the accepted launch recipe only if observed to prevent retries. Otherwise record the observed behavior and rely on the live runner's dispatcher latch below; do not invent a second undocumented retry switch.

### Verification and live evidence

Ordinary tests use synthetic SQLite stores, isolated settings, fake credentials, and local test servers. No inherited environment flag may make repository checks contact Kiro. The explicit client characterization uses only a dummy local bearer and a scripted upstream. It preserves normal client tools and permissions within the disposable directory. Record safe field shapes, status classes, request counts, and test outcomes; construct protocol fixtures from invented values rather than saving raw client traffic.

The live acceptance runner is development tooling, excluded from the product executable and ordinary tests, for example under an explicit `livebridge` test entry point with a launch gate. Start its monotonic 20 minute budget immediately when that entry point accepts the explicit live launch flag, before version and commit preflight, server or client setup, and every source read. Preparation of the reviewed artifacts happens before launch; runtime preflight, setup, human permission waiting, task execution, and cleanup all consume this one budget. Reserve the final five seconds for cleanup: stop new admission and cancel active work at 19 minutes 55 seconds, with the absolute run deadline at 20 minutes. Setup and request contexts inherit the earlier work cutoff. Cleanup is bounded by the earlier of five seconds after cancellation or the absolute run deadline, never a fresh extension. If cleanup is still incomplete at the absolute deadline, record failed cleanup rather than success; close owned connections and terminate owned client processes without starting more work. The disabled entry point exits without starting setup or accessing the source. Count every upstream dispatch up to 20. A two minute per request limit remains in force.

The runner starts the production handlers and adapter with one run controller. It does not replace the adapter with a mock for the live task. A dispatcher wrapper remains the final attempt counter and deadline guard, but it is not the only source of failure observations. Inject a small typed observer at the local Messages boundary, with fixed request ID, phase, category, and cleanup outcome fields. It receives validation, admission, source, dispatch, stream, client write, and terminal outcomes. The runner's assertion driver reports case failures to this same controller. No observer field contains request or response content, credentials, account metadata, or raw error text. Ordinary product serving uses no live run controller.

The controller owns the parent context and a synchronized open or stopped state. Check it when admitting authenticated Messages and count requests, immediately before a source operation, and immediately before dispatch. Report a detected failure synchronously at the decision point, before writing the error or releasing inference admission; a client write or final cleanup failure reports when detected. Do not rely on a deferred log callback that runs after another request could start. Serialize admission and stopping through the same controller state. Operations admitted before stopping receive cancellation; no source operation or dispatch can be newly admitted after stopping. Health remains available.

On the first unexpected validation mismatch, admission failure, source failure, upstream error, stream failure, client write failure, failed assertion, or exhausted budget, close the controller to further inference and cancel its active child work. Later client requests receive a fixed 503 `api_error` with category `run_stopped`, without credential access or dispatch. Preserve the first failure as the run cause; cancellation outcomes caused by that stop do not replace it. This makes a local 400 or a failure after upstream EOF as effective at stopping the run as a dispatch failure.

For deliberate cancellation and injected interruption, the controller records the current case ID and the one permitted fault category before the trigger is armed. Match that expected fault only to the case's admitted request ID. While the assertion driver checks the trigger and cleanup, keep admission closed; a client retry receives a local 503 and cannot consume another attempt. Reopen only when the named expected assertions and cleanup pass, the run budget remains, and the driver explicitly advances to the next case. A fault that differs from the armed category, an unreached trigger, or failed cleanup stops the run. The inference slot is released only after cleanup. Do not use a dollar budget based on estimated usage as the spending control.

Before a live launch, prepare the exact synthetic Go repository, prompts, client environment and arguments, allowed tools and manual permission procedure, fault triggers, version and artifact digests, permitted two destination URLs, mapping, clean code commit, plan digest, and passing offline evidence. Present that concrete plan for one run review. Existing feasibility approvals are consumed and do not authorize this run. After implementation, your explicit instruction for a reviewed run supplies authorization; do not ask again for the same authorized launch.

The coding case starts with a deterministic failing Go test in a repository with no external modules. Claude Code reads the source, edits it, runs `go test ./...`, consumes the result, and explains the change. The next user turn asks it to add a boundary regression test, run tests again, and report the result. Separate checks cancel after visible text and cut an upstream frame after a validated text event. The exact byte trigger is selected in the concrete plan from the deterministic local fixture; if it is not reached live, record incomplete evidence rather than inventing a pass or starting another run.

The result is `experimental_loop_observed` only when all required coding, followup, permission ownership, cancellation, failure, and cleanup assertions pass within the budget. Otherwise it is `needs_evidence` with completed and unrun assertions distinguished. Neither value means GA acceptance. The [verification plan](verify.md) gives the evidence matrix.

### Value sourcing

| Value produced or used | Source |
|---|---|
| Client and upstream model names | Exact HTTP `model`, frozen configuration mapping, and equality checks against this baseline |
| Response model and identity claim | Requested client model for the response; only a validated optional upstream echo can add identity evidence |
| Instructions, history, schemas, arguments, results | Validated current client body and the explicit transformations above; no gateway history store |
| Accepted effort shape and beta tokens | Exact top level client object and all local beta header values, checked against the literal rules above and discarded; the pinned offline observations motivated these exceptions |
| Effort and beta response labels | Fixed local policy constants `ignored` and `unsupported`, independent of client input or upstream output |
| Token, expiry, profile ARN, routing region | One combined snapshot through the existing reader; none becomes a client or diagnostic value |
| Profile pin | Existing domain separated SHA 256 digest of the first valid snapshot's exact profile bytes |
| Destination and application headers | Fixed regional map and pinned feasibility plan; invocation UUID from local randomness |
| Client request ID, message ID, upstream conversation ID | Separate fresh values from `crypto/rand`; request ID and message ID are local response metadata |
| Machine fingerprint | The existing hostname and username formula, only for the adapter header, never retained in evidence |
| Content block index and tool ordering | Local consecutive counter and first appearance order of validated upstream calls |
| Tool handoff and final stop reason | Complete validated calls and inferred clean EOF policy, never a guessed upstream stop event |
| Usage and count endpoint result | Canonical normalized content byte counts and the fixed estimator formula |
| Busy, timeout, outcome, elapsed time | Admission state, contexts, monotonic clocks, decoder result, and response write result |
| Live verdict | Named acceptance assertions against fixture tests, client observations, dispatcher counters, and cleanup results |

Logs retain only existing allowed request fields plus fixed `compatibility: experimental`, `usage_source: estimated`, `completion_basis: inferred_clean_eof` when achieved, and a count of discarded reasoning events. Do not log token counts, request field names from unknown input, client paths, contents, account metadata, or raw errors. Headers and documentation carry estimate labels; diagnostics need only their source label. Same user or root compromise remains outside the inherited local threat boundary. No additional compliance regime is introduced by this disposable coding proof.

### Critical test scenarios

The detailed [verification matrix](verify.md) covers all acceptance criteria. The essential whole paths are a real client file and shell exchange through a synthetic upstream (**AC-3**, **AC-4**, **AC-9**), authentication and a changed snapshot stopping before dispatch (**AC-1**, **AC-2**, **AC-8**), and an interrupted response after visible text withholding every pending tool call (**AC-5**, **AC-6**). The bounded live task then proves the selected experimental contract rather than replacing these deterministic checks (**AC-7**, **AC-10**).

## Build plan

Follow the Tracer Bullet approach. No layer is built as a disconnected framework, and no schema migration is needed.

1. **Prove the client through one local vertical path.** Add the experimental command wiring, authenticated Messages routes, normalized text request, response encoder, and injected adapter boundary. Implement and test the narrow effort and beta exceptions with their rejection cases and visible labels. Drive Claude Code `2.1.287` with `--effort high` against a scripted loopback adapter with a dummy token. Establish actual paths, fields, controls, both response modes, and retry behavior. The preliminary shape captures do not complete this task. Preserve the current plain server. Satisfies **AC-1**, **AC-3**, **AC-7**, **AC-9**.
2. **Connect the real wire path with synthetic account data.** Implement the production Kiro adapter from the evidenced request and decoder, through the existing combined reader and fixed transport rules, to incremental client output. Use a synthetic SQLite source and TLS test service for text, source changes, clean completion, errors, and cancellation. Satisfies **AC-2**, **AC-5**, **AC-6**, **AC-8**, **AC-9**.
3. **Complete the tool loop and failure boundaries.** Extend the same path through exact tool definitions, multiple complete calls, success and error results, followup history, ordinary JSON, local counts, limits, and one active request. Prove a real Claude Code file and shell exchange against the scripted adapter before live access. Complete deterministic failure and leak checks, run the repository checks, and prepare the concrete live plan. Satisfies **AC-3** through **AC-10**.
4. **Review and execute the bounded coding proof.** Commit the implementation and plan with a clean checkout, present them for your run review, then execute only the authorized run. Retain the allowed evidence and all compatibility limits. Finish behavior verification, tests, independent code review, and documentation for this milestone. Satisfies **AC-1** through **AC-10**.

## Consequences

You get a concrete coding proof using the project's real credential reader, HTTP server, translator, and adapter. An explicit flag keeps experimental inference distinct from existing health operation.

The cost is weaker instruction priority, an inferred terminal state, lost reasoning continuity, approximate usage, and no verified output token cap. The client can display high effort even though the gateway ignores it, and accepted beta names do not mean the corresponding features work. Fixed headers, startup output, and documentation must disclose these limits. Unsupported current client fields can still stop the offline milestone. One active request can reject helper traffic, and token changes still require relinking. None of these limits is hidden by a successful small task.

## Follow-up

1. GA promotion still owes distinct instruction semantics or a separately accepted replacement contract, authoritative completion or a stronger detection mechanism, model identity and usage evidence, and verified model controls.
2. Credential renewal, long conversations, both model families, retries and recovery, and broader Claude Code features remain with their existing scope features.
3. If offline capture exposes an essential unsupported field, or live use rejects preserved tool schemas, record the structural evidence and amend this design before broadening it. Do not silently adopt the reference gateway's lossy transformations.
4. Reconcile durable context through `/sync` after implementation. The current comments limiting the combined reader to feasibility will need a focused update when it is used by the product adapter.
5. Spec 0003 retains historical instructions about extending that file and earlier Sonnet or client baselines. This separately chosen spec is the current bridge design; those older directions do not override this milestone's explicit model and version choices.

## Rationale

Reasoning, alternatives, and verified sources: see [rationale.md](rationale.md).
