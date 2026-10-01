# Offline feasibility harness

This directory contains test code only. Nothing here is linked into the gateway.
The current candidate plan selects `claude-sonnet-5` as both the client mapping and
target. It records `needs_evidence` and cannot dispatch a live request.

The operator reaffirmed `preserve_distinct_system_role` on October 1, 2026. The
plan validator rejects an absent policy or a downgrade to user context. Static
inspection of the installed arm64 binary now records exact runtime endpoint
selector outputs and serializer key writer calls, tied to its SHA 256 digest.
The current serializer has an opaque `additionalModelRequestFields` document;
this is not evidence that it accepts system instructions or output bounds.
The inspected fields do not establish a distinct system role. That finding is
an evidence gap, not proof that the remote service cannot support one.

A further contract inspection on October 1, 2026 confirmed the same binary digest.
The installed message metadata decoder recognizes `conversationId` and
`utteranceId`, then skips other keys. That decoder does not establish successful
turn completion. The [official region documentation](https://kiro.dev/docs/enterprise/supported-regions/)
also distinguishes the Kiro profile region used for inference from the Identity
Center region. A token alone cannot establish that profile region. The accepted
profile amendment now reads the fixed selected profile alongside the token.
Agent prompt and headless output documentation describe CLI behavior,
without supplying the missing runtime wire contract for the 2.8.0 baseline.
The candidate plan records these sources and their limits.

Preparation remains `needs_evidence`. Continuing requires evidence of the exact
destination and authentication path, distinct system instruction placement,
generation controls, and positive turn completion. Reading sources beyond the
accepted token and selected profile, or changing instruction roles, requires
`/architect` to extend the decision first.
The live dispatcher and live case bodies remain unimplemented because their
required inputs are unresolved.

You can run the offline checks with:

```sh
rtk proxy env GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=1 GOFLAGS= go test -mod=readonly -race ./internal/kiro ./internal/credentials ./internal/configstore
```

The tests create temporary settings, synthetic SQLite credentials, and local TLS
servers. They exercise one shared request path. `probe-offline-cases.json` contains
invented response fixtures. The request envelope in `fixtureRequest`, including
its separate instructions field, is a local test contract. Neither that field nor
`probeFixtureComplete` is proposed as part of the Kiro protocol.

| Case | Local evidence |
|---|---|
| Text | Two text events assemble the instruction marker, followed by an explicit fixture completion event. |
| Tool | Argument fragments assemble a bounded JSON object for `probe_lookup`; the observed name and ID are checked. |
| Result | A fixed lookup result is returned to that exact ID, with the assistant tool request retained in history. No supplied command is executed. |
| Follow up | The original instructions and completed tool exchange remain in the request. |
| Cancel | The first text event cancels only the attempt context. Local cleanup completes and the parent permits the next case. |
| Interrupt | A fixed 256 byte cutoff produces an incomplete frame. Completing before that cutoff is inconclusive. |

The fixture runner exercises `observed`, `contradicted`, `inconclusive`, and
`unrun` cases, plus the spec's verdict priority. A decoded contradiction survives
later cancellation. Missing completion, authentication failure, exhausted budget,
or an unreached injection trigger cannot become a supported candidate. An offline
`candidate_supported` result proves the synthetic assertions only, not Kiro model
access or Claude Code compatibility. Optional model identity and usage metadata
remain unknown when absent and are reduced to booleans when present.

The request path holds an existing configuration lock without creating missing
settings. It freezes model selection and the session reference, then reads one
fresh combined token and selected profile snapshot per attempt. Both fixed rows
share one SQLite connection, transaction, and five second deadline. Manual
settings edits cannot adopt a changed credential for the active run. The token
is selected from the same bytes as the checked fingerprint.

The profile reader validates the exact ARN and one string valued `profileName`
or `profile_name`, then discards the name. The ARN region selects a destination
from a frozen synthetic map. Every map entry is restricted to local TLS. The
first valid profile digest is pinned for the run; even whitespace or ignored
field changes stop a later attempt before dispatch. A new run can select a new
profile without changing the saved token reference. Profile values and digests
have no output or settings field. Ordinary `Capture` and `ReadSnapshot` still
read only the token record.

Transport is restricted to numeric IPv4 loopback TLS with explicit fixture trust.
It ignores environment proxies, rejects redirects, and disables connection reuse,
HTTP/2, and request body replay. An attempted connection consumes a slot even if
it fails. The sequence has at most six attempts, a ten minute parent deadline,
a two minute request deadline, and a 30 second received byte idle limit. Tests
shorten these clocks. Operator cancellation prevents subsequent dispatch.

Requests are capped at 64 KiB and responses at 8 MiB. Both HTTP headers and local
frame headers are capped at 16 KiB. The synthetic decoder additionally caps each
event payload at 64 KiB. It checks both EventStream CRC32 values and lengths before
allocation. A 16 MiB reservation budget covers retained history, argument copies,
encoded requests, observations, and conservative decoder scratch allowances.
It reserves an additional 2 MiB for both bounded account records and their
temporary byte and string copies.
`peak_reserved_bytes` reports that reservation, not the process heap size. Raw
payloads, credentials, tool IDs, arguments, unknown spellings, and error text have
no summary output field.

`TestProtocolProbe` exists only with the `liveprobe` tag. It skips before access
unless its explicit gate is set. With the gate set, preflight checks the exact
clean commit, tracked input inventory (including files Git might ignore), plan
digest, Go environment, and client version baselines. It then rejects the missing
candidate contract before configuration, credential access, or network state.
Changing a plan status or environment value cannot enable a live dispatcher.
Ordinary tests exercise preflight with synthetic command results, not installed
client commands or operator state.

The following work remains before a live review:

1. Establish the current IAM Identity Center destination and token scheme
   without adding an unapproved credential source. The profile region source
   is implemented, but the live destination and authentication trace is pending.
2. Establish instruction placement, requested model controls, and positive model
   turn completion from current evidence. The fixture fields cannot fill these
   gaps. The evidence and missing fields are in `testdata/probe-plan.json`.
3. Implement the concrete wire requests and response decoder, then exercise them
   through these controls. Add the exact six live cases and reviewed observation
   labels. The current framing and fixture semantics cannot substitute for them.
4. Commit that complete candidate and its offline evidence, then present its plan
   digest and clean commit for the operator's one run review under spec 0003.
   This offline checkpoint is not a live approval or live result.

Local builder checks on October 1, 2026 passed `scripts/check`, including the race
detector, with fake live controls inherited by the ordinary suite. A separate
build with `liveprobe` selected and the live gate unset compiled and skipped
`TestProtocolProbe`. No real configuration or credential store was read, and no
live inference ran. The scope records the completed offline checkpoint separately
from the incomplete live preparation. Its independent verification, tests, review,
and documentation steps remain pending; the existing spec verification checklist
remains the acceptance checklist.
