# CreateResponse discovery probe

**Design review:** Confirmed, October 5, 2026, after independent review and closure of all findings. Governed by spec 0004 and its feature lifecycle.

## Summary

You can prepare a development probe for one explicit `CreateResponse` request hypothesis. A later separately authorized run can observe its HTTP response and bounded structure without executing tools or saving returned text. This supplies discovery evidence when the service binding and response grammar are unknown. It cannot establish a GA protocol guarantee.

## Requirements

This child specifies **AC-17** in [the parent spec](index.md). AC-1 through AC-16 and all four GA gates retain their existing meaning. You confirmed the complete design, so offline preparation is ready. The live entry point remains disabled until a clean implementation, completed checks, independent code review, and exact launch plan receive separate authorization.

## Decision

**Chosen option:** Test the default AWS JSON serialization hypothesis through one development only request, with structural observation instead of a semantic inference decoder.

The alternatives are testing the REST annotation first or waiting for another source artifact. The default serialization has a connected source trace and a synthetic observation, so it is the selected first hypothesis. A rejection cannot trigger a REST request, another target, another model, changed headers, or a second attempt. No SDK or native agent is executed by this probe.

**Implementation skills:** `golang-security` and `go-concurrency`, at the paths listed in the parent spec. Use the existing Go stack, credential reader, locking, transport safeguards, and synchronous ownership. There is no runtime dependency or settings migration.

## Discovery contract

### Exact request hypothesis

| Item | Fixed rule |
|---|---|
| Hypothesis identifier | `createresponse_awsjson_v1` |
| Operation | `POST /`, no query, `X-Amz-Target: KiroRuntimeService.CreateResponse` |
| Destination | Exactly `https://runtime.us-east-1.kiro.dev:443/` or `https://runtime.eu-central-1.kiro.dev:443/`. The reviewed plan selects one `expected_region` before launch; the snapshot region must match it. The two hosts are design candidates, not a claim of service support. |
| Request headers | Bearer token from the combined snapshot; `Content-Type: application/x-amz-json-1.0`, `Accept: */*`, `Accept-Encoding: identity`, `User-Agent: kiro-gateway-response-discovery/1`, `X-Amzn-Codewhisperer-Optout: true`, and the target above. Host, length, and connection headers are transport generated. |
| Body | The exact object below, with only `profileArn` substituted in memory from the same snapshot. JSON object order is immaterial; names, types, values, and absence of extra fields are fixed. |
| Excluded actions | No client session, tool catalogue, tool execution, continuation, prior response ID, credential renewal, fallback, redirect, retry, or settings change. No request to `/v1/responses`. |

```json
{
  "model": "claude-opus-5.5",
  "input": "Return exactly OK.",
  "origin": "KIRO_CLI",
  "instructions": "This is a protocol discovery test. Reply with exactly OK.",
  "stream": true,
  "maxOutputTokens": 1024,
  "profileArn": "<FROM_COMBINED_SNAPSHOT>"
}
```

This is a declared hypothesis, not a proven accepted request. `input` is a string under the generated document member. `origin`, the instructions, and 1024 are explicit experiment choices. The catalogue's `max_tokens` range for the other operation does not establish support for `maxOutputTokens`. The requested limit is not a spending guarantee. Local time and byte limits remain effective if the service ignores it. No control, reasoning, or server retention default is inferred. The probe sends only these fixed synthetic texts and never reuses a returned identifier.

The plain user agent and absence of IDE metadata or a machine fingerprint are deliberate differences from the experimental adapter. The plan records them. A denial cannot identify which request assumption failed.

### Entry point, source, and ownership

Implement only in tagged test tooling under `internal/kiro`, using `responsediscovery`, `TestResponseDiscoveryProbe`, and the explicit boolean flag `-response-discovery-launch`, default false. A disabled entry point skips before preflight, home lookup, configuration, account access, or networking. Environment variables alone cannot enable it. The product executable and ordinary test builds exclude this entry point.

After enabled launch and artifact preflight, acquire `configstore.OpenExisting`, validate the existing settings, and require the linked session and exact `claude-opus-5.5` mapping. Hold its stable exclusive lock through all cleanup. Existing serving or mutation returns `busy` without an account read. Missing settings are not initialized.

Read `Reader.ReadProfileSnapshot` at most once, against the saved reference. Existing descriptor, schema, size, fingerprint, profile, and expiry checks apply. Route only from its profile region, requiring equality with `expected_region`. Recheck the snapshot's token expiry before dispatch without another source read. No credential, profile value, fingerprint, home path, or account selector enters the report. There is no shared process profile pin because this run has one snapshot and excludes serving.

One owner controls preflight, source access, dispatch, decoding, and cleanup. Every transport worker inherits cancellation. Successful cleanup requires joining owned workers; a blocked close follows the failed cleanup rule below. The live entry point runs in a dedicated test process selected only for `TestResponseDiscoveryProbe`, never a reusable runner process.

Start the monotonic budget when the live flag is accepted. The absolute end is 30 seconds after that point, shortened by an earlier parent deadline. The work cutoff is five seconds before that end, at most 25 seconds after launch. If that cutoff has already passed, stop before setup with `timeout`. Existing source reads also retain their five second bound. Body projection and report size validation share the work cutoff. Cleanup can wait at most five seconds from its first trigger, capped by the absolute end; cancellation never restarts this allowance. Manual parent cancellation stops work immediately and starts cleanup within those same bounds.

The 30 seconds bound work and cleanup waiting, not the termination of an unresponsive operating system close. If transport cleanup cannot finish within its allowance, latch failed cleanup, clear the body summary, and perform no further account or network operation. Keep the settings lock owned until the dedicated process exits; do not run a deferred lock release while a transport worker remains unresolved. The worker continues to own the unresolved connection. Never label these resources released or reuse this runner.

Freeze elapsed operation time after the cleanup attempt. On failed cleanup, the live entry point attempts only the bounded failure envelope, allowing at most one additional second for this local output attempt, then exits the dedicated process with nonzero status without waiting again for the worker. A blocked or failed write may leave no complete report; the nonzero exit must not be interpreted as `response_observed`. Process exit releases remaining local descriptors and locks. No report worker may outlive that process. Successful cleanup permits ordinary local report emission outside the live budget with no account or network resource retained. The separate local output allowance never extends inference, source access, or cleanup waiting.

Latch the one allowed dispatch immediately before calling the transport, counting DNS or dial failure as that dispatch. Reuse TLS verification, public IPv4 resolution, one pinned address, numeric address dial, and the original TLS server name. Resolve once, reject any nonpublic address, and use the first validated address. Use a fresh HTTP/1.1 connection, no proxy, redirect, transparent decompression, connection reuse, body rewind, or automatic replay. A test injection may replace these dependencies only in synthetic tests. The live constructor has no URL or header override.

### Bounded response observation

Do not use the SDK's empty response schema or the production semantic event decoder. Buffer at most 256 KiB plus one detection byte, then inspect synchronously. Check the work context while reading and between records. On successful transport cleanup, close the body and connection, join owned workers, and release the settings lock before publishing the report. On failed cleanup, follow the dedicated process exit rule instead.

| Resource | Bound |
|---|---|
| First HTTP response header block | 16 KiB, enforced by transport; no second header block is parsed |
| Response body | 256 KiB, including framing, with no decompression |
| JSON document | Object root, valid UTF 8, no duplicate member at any depth, nesting at most 64, no trailing JSON |
| SSE line and joined event data | 16 KiB per line; 64 KiB per event data value |
| EventStream frame and headers | 64 KiB total frame; 8 KiB headers; validate lengths and both CRC checks before payload inspection |
| Stream records | At most 256 data events or frames, including `[DONE]` records |
| Retained event sequence | First 32 records, with a separate truncation flag; later records still validate and contribute to bounded aggregates |
| Encoded report | At most 16 KiB |

Read one HTTP response header block. Reject every 1xx status, including 100, 101, and 103, before classifying headers or inspecting a body. Use `response_format` unless an earlier cause is latched, with `body_failure: informational_response`. Keep `http_status`, `media_kind`, `transport_complete`, `decode_complete`, and `body_summary` null. Do not read onward to a final response, upgrade the connection, retry, or dispatch again. Perform cleanup normally, including the failed cleanup rule if needed. This deliberately limits the first probe and preserves the single header block bound.

For a noninformational response, record its final status and require exactly one valid `Content-Type`. Permit no content encoding or one case insensitive `identity` value. Duplicate content type or encoding headers, unexpected MIME parameters, malformed types, and an excessive declared length fail observation. JSON and SSE may have only an optional UTF 8 charset. EventStream permits no MIME parameters. Classify only these media types:

1. `application/json` and `application/x-amz-json-1.0`: one bounded JSON object.
2. `text/event-stream`: the SSE rules below.
3. `application/vnd.amazon.eventstream`: the binary rules below.
4. Everything else: report `media_kind: other`, do not sniff or inspect the body, and finish cleanup with incomplete evidence.

SSE accepts LF or CRLF lines, rejects bare CR and invalid UTF 8, and joins repeated `data` fields with newline characters. Remove at most one leading space after a field colon. A blank line ends a record. At most one `event` field is allowed per record. Comments, `id`, `retry`, and unknown field names are ignored within the byte limits and never saved or acted upon. Records without data do not yield a JSON observation. Data exactly equal to `[DONE]` yields a fixed marker observation; every other data value must be a JSON object. An unfinished line or pending `event` or `data` fields at EOF fail framing. Ignored fields and comments alone create no pending record. `[DONE]` never stops reading or grants success. At least one JSON record is required.

For EventStream, validate every header, reject duplicate names, and require string `:message-type` equal to `event`, `error`, or `exception`. An event frame requires string `:event-type`; if present on other frames it must also be a string. Every frame payload must be a JSON object. Record the message kind and only the finite event label below. Unknown event names become `other` after framing validation; they are not a production vocabulary extension. EOF must be between complete frames, with at least one frame. This probe may observe error or exception frames without treating them as model success. Keep the production decoder's rejection behavior intact.

Transport EOF and complete framing are local observations only. Neither supplies an authoritative generation terminal signal. Observe all bounded records through body end; do not stop on a candidate completion or error label. A body read error, including HTTP truncation, discards the body summary even if its prefix looked complete.

### Retained report and finite projection

The sole retained runtime payload is a JSON report. AC-17 permits this narrow structural extension to AC-8. Static synthetic request texts may appear in the reviewed plan; returned text and echoes may not appear in runtime evidence. No raw body, arbitrary key or event name, upstream ID, URL, header value, error message, tool argument, reasoning, signature, credential or profile digest, or response digest is retained. The local run ID and code and plan provenance below are permitted.

| Field | Source and handling |
|---|---|
| `report_version`, `hypothesis_id`, `requested_model` | Fixed `1`, `createresponse_awsjson_v1`, and `claude-opus-5.5` |
| `code_commit`, `plan_digest` | Successful artifact preflight; null until established |
| `run_id` | Fresh local UUID after preflight; null if UUID generation was not reached or failed |
| `elapsed_millis`, `dispatch_count` | Monotonic elapsed time and local attempt latch, 0 or 1 |
| `region` | Validated snapshot region; null until available, including on source failure |
| `http_status` | Numeric noninformational response status, null until received; rejecting any 1xx leaves it null |
| `media_kind` | `json`, `sse`, `eventstream`, `other`, or null before classification |
| `transport_complete` | Null before a body read is attempted, including a body rejected from headers; otherwise true only on an error free body EOF, false on an interrupted or bounded read |
| `decode_complete` | Null if no decoding was attempted; otherwise the complete framing and JSON result |
| `body_failure` | Null or a fixed decoder category below, never raw error text |
| `body_summary` | The finite projection below, or null unless full decoding and all cleanup succeeded |
| `outcome`, `failure_category`, `cleanup_outcome` | Fixed local result rules below |

The body summary holds `record_count`, `json_record_count`, the first 32 `event_sequence` entries, `sequence_truncated`, `path_kinds`, `status_labels`, `error_labels`, and `model_comparisons`. The last three fields are ordered arrays of unique labels; `path_kinds` maps each fixed path below to its ordered array of observed kinds. Plain JSON counts as one record. Each sequence entry contains only `channel` (`json`, `sse`, `eventstream`), `message_kind` (null or `event`, `error`, `exception`), `transport_label`, `payload_type`, and a boolean `done_marker`. Use null for absent labels, `invalid` for a present nonstring payload type, and `other` for an unrecognized string. SSE event labels and binary event types never enter the report verbatim unless equal to a fixed label.

The label allowlist is `message`, `error`, `response.created`, `response.queued`, `response.in_progress`, `response.completed`, `response.incomplete`, `response.failed`, `response.output_item.added`, `response.output_item.done`, `response.content_part.added`, `response.content_part.done`, `response.output_text.delta`, `response.output_text.done`, `response.function_call_arguments.delta`, `response.function_call_arguments.done`, `assistantResponseEvent`, `toolUseEvent`, `reasoningContentEvent`, `metadataEvent`, and `messageMetadataEvent`. Root `type` supplies `payload_type`. These are inspection constants, not claimed Kiro event types or semantic rules.

Inspect only these literal JSON paths in each JSON record:

```text
/type /status /model /output /usage /usage/input_tokens /usage/output_tokens
/response /response/status /response/model /response/output
/response/usage /response/usage/input_tokens /response/usage/output_tokens
/error /error/type /error/code /code /__type
```

For each path, aggregate a set of kinds in this fixed order: `absent`, `unreachable`, `null`, `boolean`, `number`, `string`, `array`, `object`. A missing member gives `absent`; a present nonobject intermediate gives `unreachable`. Do not traverse arrays or retain sizes, scalar values, or keys below these paths. Each record contributes one kind per path, and repeated kinds are deduplicated.

For string values at `/status` and `/response/status`, aggregate only `queued`, `in_progress`, `completed`, `incomplete`, `failed`, `cancelled`, or `other`, in that order. Nonstring and absent values contribute only their path kinds. Model comparisons at `/model` and `/response/model` aggregate `match` or `different` for strings, using exact equality with the fixed requested model; other types contribute no comparison. No differing model string is saved, and a match is not an actual serving identity guarantee.

For string values at `/__type`, `/code`, `/error/type`, and `/error/code`, compare the final component after the last `#` with this fixed list: `AccessDeniedException`, `ValidationException`, `UnknownOperationException`, `ResourceNotFoundException`, `ThrottlingException`, `InternalServerException`, `ServiceUnavailableException`, `ServiceQuotaExceededException`. Aggregate those labels or `other` in the listed order. Multiple differing labels are retained as a set, with no inferred precedence or service cause. Numeric error codes and all other values contribute only path kinds. Do not inspect error headers or messages. The generic `__type` path never bypasses these projection rules.

No input or output token counts are retained, only field kinds. Their units and accuracy remain unknown. Unknown keys and values are validated for JSON safety but otherwise discarded. Every string in the body summary comes from the fixed local vocabulary; report object keys also come only from this design.

### Outcomes and failure precedence

`response_observed` requires final HTTP 200, a supported media type, error free body end, complete framing and JSON validation, at least one JSON record, all bounds met, no cancellation, successful report encoding, and complete cleanup. Its `failure_category` and `body_failure` are null. This outcome means only that a structural response was observed. A failed status label or exception frame can be observed; it never becomes a successful inference verdict. No marker or output text is necessary or sufficient for GA acceptance.

All other runs return `needs_evidence`. Fixed failure categories are `preflight`, `config_missing`, `config_invalid`, `busy`, `source_changed`, `source_expired`, `source_unavailable`, `unsupported_region`, `region_mismatch`, `transport`, `http_status`, `response_format`, `response_limit`, `invalid_response`, `timeout`, `canceled`, `cleanup`, and `report_limit`. One owner latches the first established cause and never overwrites it. Source errors retain the existing AC-16 classifications only when the cancellation rules below do not apply. Snapshot region mismatch uses `region_mismatch` before dispatch.

Apply one cancellation rule before starting and after returning from every phase: preflight, configuration and lock access, source access, metadata construction, DNS and dialing, transport, decoding, report size validation, and cleanup. Once cancellation or failure is known, start no further operational work; only the explicitly permitted non 200 body observation below, cleanup, and bounded failure reporting may continue. Cancellation also stops that body observation. With no cause yet latched, first inspect the controlling parent cancellation or deadline and the current phase deadline. A parent canceled without a deadline cause gives `canceled`; an expired applicable work, parent, or absolute deadline gives `timeout`. If neither is observed, latch the phase's specific error when it fails. A cancellation observed at the same boundary takes precedence over a phase error caused by it. An error already latched at an earlier boundary remains the cause when later cancellation or timeout follows. Tests must control this observation order rather than assume the time a background error occurred.

Once work has completed before its cutoff, that work cutoff expiring during the allowed cleanup period is not a new failure. Cleanup still observes parent cancellation, the absolute end, and its own five second wait cap. Expiry of that cleanup cap alone gives `cleanup` when no earlier cause exists. Parent cancellation or the absolute deadline first observed at that boundary uses `canceled` or `timeout`, respectively, with `cleanup_outcome: failed` if resources remain unresolved. A connection, body close, or lock release failure uses `cleanup` only if no cause is already latched. An earlier cleanup failure also remains the first cause if parent cancellation arrives later.

After the 1xx rejection rule and cancellation check, latch a final non 200 as `http_status` immediately. A supported bounded response body may still be projected for diagnostics, without another request. Later format or body failure does not replace `http_status`; it sets `body_failure`. Body categories are `informational_response`, `unsupported_media`, `invalid_headers`, `limit`, `invalid_utf8`, `invalid_json`, `invalid_sse`, `invalid_eventstream`, `empty_body`, `transport`, `timeout`, and `canceled`. For HTTP 200, media or header rejection uses `response_format`, a bound uses `response_limit`, invalid syntax or empty body uses `invalid_response`, and read failures or cancellation use the corresponding transport, timeout, or cancellation category. Body observation remains subject to cancellation and the work deadline even when `http_status` was already latched.

Set `cleanup_outcome` to `complete` only when all acquired resources are released and owned workers joined, including when none were acquired; otherwise `failed`. Cleanup failure always prevents `response_observed` and clears the body summary, while preserving the first cause. When transport cleanup fails, retain the settings lock until process exit. If lock release itself fails after transport cleanup succeeds, report failed cleanup without claiming that the lock is still held or released; exit the dedicated process under the same failure rule. An overlong encoded report clears the body summary and returns the small fixed envelope with `needs_evidence` and first cause or `report_limit`. Missing observations remain null. No raw SDK, HTTP, DNS, filesystem, or decoder error is printed.

## Build plan

Every milestone satisfies **AC-17**. Follow the existing Tracer Bullet approach.

1. Prove one complete synthetic path through plan preflight, temporary settings, one synthetic combined snapshot, a fixed local TLS request, JSON observation, cleanup, and report encoding. Assert the exact candidate body and headers, one source read, one dispatch, and unchanged settings bytes.
2. Extend that path to SSE and EventStream, then interim response rejection, unknown labels, every bound, cancellation, cleanup, report limits, and redaction. Cover every value source and failure precedence. Use an isolated synthetic subprocess to prove a permanently blocked close, retained settings lock, bounded failure output attempt, and nonzero process exit. An injected fake close must never block the ordinary test process. Refactor only small existing test helpers needed by both probes; preserve AC-16 and production behavior.
3. Add the disabled live entry point and concrete plan validation. Add vet and the tagged synthetic race suite to `scripts/check`, forcing `-response-discovery-launch=false` even under hostile inherited environment flags. Run ordinary checks plus `go vet -mod=readonly -tags=responsediscovery ./internal/kiro` and `go test -mod=readonly -race -tags=responsediscovery ./internal/kiro -run '^TestResponseDiscovery' -args -response-discovery-launch=false`, with the required `rtk proxy` prefix and existing toolchain constraints. Finish separate offline behavior verification, test review, independent code review, and documentation.
4. After implementation review, prepare a concrete launch artifact with clean code commit, all fixed request and observation constants, expected region, exact destination, synthetic request template, body and report budgets, pinned software digests, passing checks, review reference, and exact command tokens. Its live approval field remains null. After a separate explicit authorization, one invocation may run and retain the projected report; that approval is consumed even if preflight or source access stops it before dispatch.

The plan schema is a separate version 1 discovery plan, with no compatibility fallback to a schema or coding plan. Reuse the existing `schemaPlan` field structure: `version`, `code_commit`, `contract`, `software`, `synthetic_checks`, `independent_review`, `command`, and `live_approval`. The discovery contract contains this child's exact hypothesis, one selected `expected_region`, destination, request template, headers, observation paths and vocabularies, budgets, and policies. Compare the entire contract with the compiled discovery contract for that region. Reject unknown plan fields, duplicate keys, invalid UTF 8, depth over 64, and plan bytes over 16 KiB. Require exact synthetic labels `scripts/check:pass` and `response_discovery_race:pass`, a nonempty independent review reference, and two software entries in the existing order `native_binary`, `bundled_source`.

Reuse the existing clean code and plan digest preflight pattern. Add `-response-discovery-plan`, `-response-discovery-plan-sha256`, and `-response-discovery-code-commit` as explicit test flags. Validate the exact reviewed command tokens, including tag `responsediscovery`, test selection `^TestResponseDiscoveryProbe$`, count 1, and the live flag, with `<PLAN_SHA256>` in the plan command to avoid self hashing. The actual command supplies the digest of the final plan bytes. `live_approval` remains null during preparation; only the separately approved launch uses `approved_for_one_response_discovery_run`. Bind the native binary and agent bundle digests by bounded hashing, never execution; existing 2 GiB software file and fixed buffer rules apply. No approved plan is present in this design.

## Consequences

You get evidence about one exact request hypothesis without relaxing the product's GA requirements. A rejection is still useful as a bounded HTTP and structure observation. It cannot isolate the cause, and the finite projection can omit the very field needed for the next decision. That outcome calls for another architecture amendment, not wider logging or another automatic attempt.

The future run may incur model usage even if decoding or cleanup fails. Requesting 1024 tokens does not prove enforcement. Synthetic input limits conversation exposure; it does not establish provider retention policy or remote cancellation. No returned tool is executed, and no generated text becomes a Claude Code response.

## Follow-up

The retained outcome returns to architecture. A complete supported wire contract still needs service binding, instruction semantics, terminal grammar, tool continuation, serving identity, usage, and controls before GA implementation. G1 through G4 remain open after every possible discovery outcome unless a later ratified contract and its required evidence close them.

## Rationale

Source trace, alternatives, and the confirmed prerequisite amendment are in [rationale.md](rationale.md#bounded-createresponse-discovery-design-october-5-2026). The parent [verification plan](verify.md#createresponse-discovery-verification-october-5-2026) supplies the acceptance matrix.
