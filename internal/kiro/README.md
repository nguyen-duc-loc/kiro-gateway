# Limited feasibility harness

This directory contains test code only. Nothing here is linked into the gateway.
The comparison plan selects `claude-opus-5.5` as both the saved mapping
and requested model. It is prepared for review, not approved for a live run.

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

The current binary trace connects the streaming operation to these candidates:

| Profile region | Destination |
|---|---|
| `us-east-1` | `https://runtime.us-east-1.kiro.dev:443/` |
| `eu-central-1` | `https://runtime.eu-central-1.kiro.dev:443/` |

Each request is POST with `application/x-amz-json-1.0` and
`X-Amz-Target: KiroRuntimeService.GenerateAssistantResponse`.
Bearer authentication and `profileArn` come from the same fresh combined snapshot.
The request carries `max_tokens: 1024` and `thinking.type: disabled` through
`additionalModelRequestFields`. Optional output usage is validated and reduced to
a boolean comparison with the requested limit. Missing usage stays unknown.
The bundled ACP agent supplies this operation target. Earlier runs using
`AmazonCodeWhispererStreamingService.GenerateAssistantResponse` were denied.
The controlled comparison with the new target also received HTTP 403 and
`access_denied`; inference compatibility remains unproven.

| Attempt | Assertion or trigger |
|---|---|
| 1 | Assemble `PROBE_MARKER` from at least two nonempty text events. |
| 2 | Receive one `probe_lookup` call with stable ID and complete `{"key":"alpha"}` arguments. |
| 3 | Return the fixed result to that exact tool ID and receive `probe-value-alpha`. |
| 4 | Retain the exchange and receive `probe-followup`. |
| 5 | Cancel the attempt after its first nonempty text event, then check cleanup. |
| 6 | Cut input at 256 bytes and report incomplete output. An unreached cutoff is inconclusive. |

No model supplied command is executed. Cases stop at the first unexpected failure
and never trigger corrective requests. Unknown events or fields, stream errors,
malformed frames, duplicate JSON members, incomplete tools, and wrong identifiers
cannot become tentative completion. A request for disabled thinking that produces
a reasoning event stops as inconclusive. The old synthetic fixture decoder remains
separate; its invented `probeFixtureComplete` event is rejected by the wire decoder.

## Local checks

You can run the synthetic harness with:

```sh
rtk proxy env GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=1 GOFLAGS= go test -mod=readonly -race ./internal/kiro ./internal/credentials ./internal/configstore
```

It uses temporary homes, synthetic SQLite records, and local TLS servers.
`probe_wire_checks_test.go` exercises the concrete request and response path,
including the complete tool exchange, tentative completion, error precedence,
plan drift, profile drift, DNS restrictions, output filtering, and budgets.
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
