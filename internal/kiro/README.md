# Limited feasibility harness

This directory contains test code only. Nothing here is linked into the gateway.
The comparison plan selects `claude-opus-5.5` as both the saved mapping
and requested model. The approved run at `4fd0b70` observed all six cases with
verdict `limited_candidate_observed`. That run is complete; further live
launches require separate review.

On October 1, 2026 you accepted two limitations for this experiment only:

1. Instructions are prefixed to user content. A distinct system role is not
   preserved, and instruction priority may change.
2. Clean HTTP body EOF after valid frames and the case assertions permits
   tentative continuation. It never proves successful model completion. A
   truncation that drops whole frames and still looks like clean EOF may escape
   detection.

Even six observed cases yield only `limited_candidate_observed`. The runner never
reports `candidate_supported`, and `observed_completion` remains null. The full
Claude Code bridge and real coding task still need their own design and evidence.

## Candidate contract

`testdata/probe-plan.json` contains the exact six prompts, synthetic request
examples, fields, limits, assertions, and provenance. The code checks the plan
against its implemented contract. Placeholders in the examples stand for the
random local conversation ID, selected profile ARN, and observed tool ID.

The prepared reference comparison uses these fixed destinations:

| Profile region | Destination |
|---|---|
| `us-east-1` | `https://runtime.us-east-1.kiro.dev:443/generateAssistantResponse` |
| `eu-central-1` | `https://runtime.eu-central-1.kiro.dev:443/generateAssistantResponse` |

Each request is POST with `application/x-amz-json-1.0` and
`X-Amz-Target: AmazonCodeWhispererStreamingService.GenerateAssistantResponse`.
Bearer authentication and `profileArn` come from the same fresh combined snapshot.
The prepared reference baseline omits `additionalModelRequestFields`, so it
requests no token cap or thinking control. `controls_requested` is false after
dispatch and `output_within_limit` stays null. Local byte and time limits remain.
It uses `AI_EDITOR`, prepends instructions once per conversation, omits empty
history and historical tool definitions, and normalizes empty assistant history.
Local tool argument validation remains strict even though the reference sanitizer
omits `additionalProperties` from the wire schema.
The path and header pair comes from `jwadow/kiro-gateway` at commit
`a5292ca04c7c6231e0b47673ac3f981f5a706e1e`, inspected as source only.
Earlier root path requests using either operation target were denied. This
operation path also returned HTTP 403 after session renewal and relinking. The reference
project's refresh, retries, and credential fallback are not part of this
candidate. The new prepared baseline includes its application identity headers. See the spec rationale for evidence and limitations.

The approved launch at `a9fc79db09457461e9d97eea311c3b7634a17dea`
stopped with `credential_expired` before dispatch. After the operator reported
refreshing the session, relinking and exact Opus mapping restoration succeeded.
The run at `1860d2b65caff84e95f8de980cae2e4b400ece3b` dispatched once and
received `access_denied` in the error header. The body was not read. Five cases
were unrun; no retry or automatic refresh followed.

| Attempt | Assertion or trigger |
|---|---|
| 1 | Assemble `PROBE_MARKER` from at least two nonempty text events. |
| 2 | Receive one `probe_lookup` call with stable ID and complete `{"key":"alpha"}` arguments. |
| 3 | Return the fixed result to that exact tool ID and receive `probe-value-alpha`. |
| 4 | Retain the exchange and receive `probe-followup`. |
| 5 | Cancel the attempt after its first nonempty text event, then check cleanup. |
| 6 | Cut input at 256 bytes and report incomplete output. An unreached cutoff is inconclusive. |

No model supplied command is executed. Cases stop at the first unexpected failure
and never trigger corrective requests. Unknown events or semantic fields, stream errors,
malformed frames, duplicate JSON members, incomplete tools, and wrong identifiers
cannot become tentative completion. Reasoning is not explicitly disabled; an
unsupported reasoning event stops as inconclusive outside the independent
failure checks described below. The old synthetic fixture decoder remains
separate; its invented `probeFixtureComplete` event is rejected by the wire decoder.

## Local checks

The approved complete request baseline at `4de4fe96e54eee5f918d947029047421937af64b`
received HTTP 200 and two text events, then stopped on one unknown field. The
remaining five cases were unrun. Static SDK evidence supports adding the optional
string `meteringEvent.unitPlural`; the prepared decoder validates and discards it.
This is a documented schema correction, not proof of the discarded field's name.
The outbound request baseline remains unchanged and another live run needs review.

The approved metering update at `906e14a` again returned HTTP 200 and two text
events but stopped on one unknown field. The next prepared plan adds bounded
structural diagnostics: recognized event, location, JSON kind, and a field hint
only if it matches the fixed schema vocabulary. Unmatched names become `unlisted`;
values never enter the summary. Unknown fields still stop the sequence. This
diagnostic change is prepared for review, with no further live run yet.

The approved structural run at `f7870e8` located an unlisted string at the top
level of `metadataEvent`. The next prepared policy counts and discards only
extensions in that metadata object, matching the reference's unused metadata
handling. Known usage, nested usage fields, text, tools, error frames, duplicate
JSON checks, and resource limits remain strict. Diagnostics remain bounded and
values remain private. This policy has passed synthetic verification but still
needs a live review.

The approved launch at `542947f` stopped with `session_changed` before any
dispatch. The fixed saved token record no longer matched the linked reference.
The metadata correction remains untested remotely. Explicitly relink the current
saved session and restore the exact Opus mapping before another approved run;
this source result alone does not call for signing in again.

After approved relinking, the run at `b27b3c2` observed text, tool call, matching
tool result response, and followup. Cancellation then stopped on an unsupported
event before its trigger; interruption was unrun. The next prepared diagnostic
reports a hint only from the inspected SDK stream event names, or `unlisted`.
It does not accept new events or expose their payloads. The first four cases
have positive live evidence; the two failure checks remain pending.

The event diagnostic launch at `67ae516` stopped with `session_changed` before
dispatch. Its plan is unchanged and still awaits a live result. The spec proposes
authorizing the existing relink and exact Opus mapping restoration before each
separately approved feasibility run, while continuing to reject changes during
a run. The operator subsequently approved this setup before each separately approved
remaining feasibility run; it does not authorize renewal, replay, or changes
to the linked reference during a run.

You can run the synthetic harness with:

```sh
rtk proxy env GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=1 GOFLAGS= go test -mod=readonly -race ./internal/kiro ./internal/credentials ./internal/configstore
```

It uses temporary homes, synthetic SQLite records, and local TLS servers.
`probe_wire_checks_test.go` exercises the concrete request and response path,
including the complete tool exchange, tentative completion, error precedence,
plan drift, profile drift, DNS restrictions, output filtering, and budgets.

`probe_reference_checks_test.go` compares all six JSON bodies and application
headers with `testdata/reference-requests.json`, generated independently by the
pinned reference's converter and header builder using synthetic inputs. Optional
fake reasoning, truncation recovery additions, and payload trimming are disabled
in that baseline. Fixture generation is not part of ordinary tests.

The prepared metadata policy includes the reference's IDE compatibility user
agent strings, opt out true, agent mode `vibe`, and SDK metadata. A SHA 256 digest
of hostname and username supplies its client fingerprint; a fresh random UUID v4
identifies each request. These are resolved only after live launch gates and are
never logged. The fixed `attempt=1; max=3` header does not enable retries. The
reviewed transport differences remain explicit: identity encoding, strict TLS
and framing, fixed source selection, local limits, and no refresh or replay.
The earlier fixture checks still exercise deadlines, cancellation, locking,
source consistency, framing bounds, and malformed tool arguments.

Ordinary tests cannot select `TestProtocolProbe`, even with inherited live
variables. You can compile the opt in entry point while explicitly disabling it:

```sh
rtk proxy env GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=1 GOFLAGS= KIRO_GATEWAY_LIVE_PROBE=0 go test -mod=readonly -tags=liveprobe ./internal/kiro -run '^TestProtocolProbe$' -count=1 -v
```

That command skips before reading operator configuration or running client
version commands. It makes no inference request.

## Resource and account safeguards

The runner holds the existing configuration lock through cleanup. It freezes the
saved mapping and token reference. Each attempt reads only the fixed token and
selected profile records in one SQLite transaction. The first profile digest is
pinned in memory. Changed tokens or profiles stop before another dispatch. Normal
capture and health behavior is unchanged, and settings are never written.

The finite live destination map rejects overrides. DNS resolves once per attempt,
rejects local and reserved IPv4 results, and chooses one public address without
fallback. TLS still validates the fixed service hostname. Environment proxies,
redirects, compression, connection reuse, HTTP/2, and request replay are disabled.
Local tests use a separate factory restricted to numeric IPv4 loopback TLS.

The run has at most six sequential attempts in ten minutes, with two minutes per
request, 30 seconds of received byte inactivity, and five seconds for cleanup.
Requests are at most 64 KiB, HTTP and frame headers 16 KiB, response bodies 8 MiB,
and individual event payloads 64 KiB. A conservative 16 MiB reservation budget
covers source copies, requests, decoding, history, and observations.
`peak_reserved_bytes` measures that reservation, not the process heap.

Only fixed labels, public plan values, local counts, and validated boolean
assertions enter the summary. Tokens, fingerprints, account metadata, prompts,
response text, tool identifiers, arguments, results, upstream IDs, and raw errors
are never printed or saved. Runtime evidence stays in memory until reduced to the
allowed summary.

## Live review

The live entry point requires `liveprobe` plus all four explicit launch controls.
It verifies the exact clean commit, tracked probe inputs, plan digest, saved
mapping, Go environment, and client baselines. Then it may access the selected
account and run once. Runtime checks detect drift; they do not establish consent.

The [spec](../../docs/specs/0003-first-claude-code-loop/index.md) requires review of
the concrete plan and clean code commit before one explicit launch. A new launch,
changed plan, code, baseline, model, or destination set needs a new review. No live
run has been performed during implementation. Live requests may consume account
credits, and local cancellation cannot establish that remote computation stopped.

The first approved launch stopped at `baseline_changed` before account access or
inference: installed Claude Code was `2.1.286`, while the reviewed baseline was
`2.1.285`. All six cases remain unrun. The replacement plan pins `2.1.286` and
requires a new review of its exact digest and code commit. Kiro CLI remains
`2.8.0`; synthetic requests and limits are unchanged. See the spec verification
record for the original launch identity and result.

The approved replacement passed its baseline checks, then stopped with
`configuration_invalid`. The existing read only `config check` command confirmed
that no saved gateway configuration exists. It made zero inference attempts and
did not read Kiro credentials. Local initialization, linking, and the exact saved
model mapping must be established before a newly approved launch. The harness
does not perform those setup mutations itself.

Local setup was then approved and completed. The following approved launch stopped
with `source_unavailable` before any inference request. Metadata diagnosis found
that Kiro declares `state.value` as BLOB while storing the selected profile as text.
The prepared reader correction accepts that declaration for the profile value
column only. It still requires bounded text storage and all existing source checks.
Synthetic harness databases now exercise that declaration; actual BLOB values and
changes to the token table remain rejected. The plan bytes are unchanged, but the
new code commit needs review before another launch.

The next approved run passed combined source validation and selected the reviewed
US endpoint. It consumed one dispatch attempt, returned `needs_evidence`, and
produced no decoded response events. Five dependent cases remained unrun. The
original summary cannot distinguish transport failure from HTTP rejection.
The next prepared plan adds fixed failure stage, transport, and HTTP status
categories. It does not read error bodies or emit raw errors, addresses, headers,
or account data. Inference requests and limits remain unchanged. Its updated
code and diagnostic policy need a new exact review before another live launch.

The approved diagnostic run stopped at `failure_stage: source` with
`credential_expired`, before selecting a destination or dispatching any request.
Renew the IAM Identity Center session through the normal Kiro CLI flow before
further live work, then explicitly relink and restore the exact model mapping.
The harness does not renew or relink automatically. A new launch still requires
its own exact review. This expiry does not explain the earlier dispatch failure.

After renewed sign in and explicit gateway relinking, the approved diagnostic run
received HTTP 403 Forbidden from the reviewed US runtime path on its first request.
Local source validation passed, but remote authorization and model acceptance
remain unproven. Five dependent cases were unrun. No raw error body or headers
were retained, and no retry occurred. The current disposition is `needs_evidence`;
further authorization evidence should precede another inference proposal.

The authorization investigation found that normal saved token mode omits
`TokenType`, while the CLI adds user agent and opt out metadata. No evidence
establishes those differences as the cause of the 403, and the inspected connector
does not rewrite the operation URI. The next prepared diagnostic uses the named
AWS error discriminator: one bounded `X-Amzn-Errortype` header, or, if absent,
`code` or `__type` from at most 16 KiB of JSON. It may read one additional byte to
detect an oversized body. It emits only allowlisted classes and fixed fallback
labels. No error message, account value, raw header, or body is saved or printed.
This replaces the former policy of leaving every error body unread only after a
new exact plan review. No live request was made during research or local tests.

The approved bounded classifier run received HTTP 403 with `service_error:
access_denied` from its first request. It inspected 119 JSON error response bytes
in memory and retained only fixed labels. No raw message or account value was
printed or saved. Five dependent cases remained unrun, and no retry occurred.
The exact access rule is still unknown. Further authorization evidence or a
separately reviewed native client comparison is needed before another candidate;
the official client's account access and possible refresh are outside this run.

The operator confirmed that the successful native test used Opus 5.5. Only the
provided session's metadata was inspected; its conversation was not opened. The
prepared comparison candidate now pins `claude-opus-5.5` exactly. No actual mapping
or live request has changed yet. Approval must cover explicit relinking of the current fixed snapshot, adding
that mapping, and the new exact run identity. Changed token bytes clear old
mappings through the existing link rule; unchanged bytes preserve them. The previous Sonnet results remain historical evidence. The same
requested output and thinking controls are hypotheses for Opus 5.5, not a claim
that its service accepts them. No fallback, renewal, or implicit model change is
added to the runner.

The operator approved the Opus mapping and one comparison run. Explicit relinking
and local validation succeeded, but the first Opus request also returned HTTP 403
with `access_denied`. Five dependent cases remained unrun. Matching the native
model did not resolve the denial. No raw response was retained and no retry
occurred. The next investigation should trace the native authentication and
request path; another live operation needs its own review.

The operator then authorized testing the bundled agent target. At clean commit
`5c6e8c6fc2221e9194c7945f6caf1309556c24db`, plan digest
`eecbd193c8aec5ab93ce6ed3c77d33a0160a7f6af7980070ade6c2f66b6b621d`,
one request with `KiroRuntimeService.GenerateAssistantResponse` again received
HTTP 403 with `access_denied`. Five cases were unrun. The run stopped without
retry or account changes. The target change alone was insufficient; native token
and profile selection remain unresolved. Full deterministic checks passed.

The diagnostic at `8dbeeae` again observed the first four cases and identified
`reasoningContentEvent` before the cancellation text trigger. The prepared
correction discards bounded reasoning objects only in the two independent
failure checks. It records an event count, retains no values or history, and
keeps cancellation tied to nonempty text. Reasoning remains unsupported in
normal conversation and tool cases. The byte cutoff and all other limits remain.
