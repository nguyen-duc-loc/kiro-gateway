# Experimental coding bridge

The production adapter implements spec 0004. It obtains one fresh combined snapshot per admitted inference, checks the saved token reference, pins the first profile digest in memory, and routes by that profile's region. Startup, health, counts, invalid input, and busy requests never read the source. There is no renewal, fallback, retry, or replay.

Each request opens one verified TLS connection using HTTP/1.1. DNS is resolved once, every returned address must be public IPv4, and the connection uses the first validated address with the fixed TLS server name. The transport ignores environment proxies, never redirects, never rewinds the request, and closes the connection before exposing completed tools. Byte and time limits follow the spec. The cancellation callback is joined before returning.

`request.go` constructs the evidenced wire request from normalized client input. `frames.go` validates framing and checksums. `events.go` streams validated text and holds tools until every frame, argument object, and stream end validates. `adapter.go` owns the source and profile pin. The old feasibility harness remains separate and its consumed approvals do not authorize this bridge.

## Prepared live proof

`testdata/bridge-plan.json` contains the exact synthetic Go repository, prompts, artifact digests, mapping, two allowed destinations, permission procedure, fault triggers, and limits. The implementation commit is pinned there. The reviewed plan digest must be supplied at launch. Version drift, changed source code, a dirty checkout, or missing setup stops preflight before account access.

The runner permits at most 20 dispatches during one 20 minute budget. The budget begins when the explicit launch flag is accepted, before preflight and setup. At 19 minutes 55 seconds it cancels work and reserves the remaining five seconds for cleanup. All permission waiting consumes that budget. A failed assertion, validation, source operation, dispatch, stream, client write, or cleanup closes the run to later inference. Expected cancellation and interruption stay closed until the matching request and cleanup assertions pass.

The coding task fixes `Clamp` in the disposable repository. After the first answer, the operator enters the exact followup prompt from the plan, asks for `TestClampUpperBoundary`, and exits after the second answer. Claude Code keeps manual permission mode. The operator reviews each requested file edit and shell command. No bypass or blanket approval is enabled. The gateway never executes model supplied tools.

Two subsequent local requests test cancellation after visible text and an injected incomplete frame. The interruption allows exactly 13 decoded body bytes after the first validated nonempty text event, then fails the read. This cuts the next frame after its prelude. If either trigger is not reached, the result is incomplete evidence, not success or permission for a retry.

The entry point is excluded from normal builds and tests. This command only verifies its disabled gate:

```sh
rtk proxy go test -tags livebridge ./internal/kiro -run TestLiveBridge -count=1 -v
```

After the concrete plan receives its separate run review, you can compile the runner and launch it from the repository root in a terminal. Replace `REVIEWED_PLAN_SHA256` with the reviewed digest. The explicit launch flag is not authorization by itself.

```sh
rtk proxy go test -c -tags livebridge -o bin/bridge-live.test ./internal/kiro
rtk proxy ./bin/bridge-live.test -test.run '^TestLiveBridge$' -test.v -live-bridge -bridge-plan internal/kiro/testdata/bridge-plan.json -bridge-plan-sha256 REVIEWED_PLAN_SHA256
```

Client output stays in the interactive terminal and is not copied into an evidence file. The runner reports fixed verdict, dispatch count, first failure category, and cleanup outcome. Temporary client configuration and the fixture are removed. It changes neither gateway settings nor Kiro's store.

`experimental_loop_observed` requires the coding and followup assertions, both expected faults, and cleanup within the budget. Any other outcome remains `needs_evidence`. Neither means GA acceptance. The separate behavior verification, test workflow, fresh model review, and change documentation remain pending.

## First live launch outcome

The approved launch on October 2, 2026 ended with `needs_evidence`, zero dispatches, and cleanup within the deadline. The child process was suspended because the runner gave it a separate process group without foreground terminal ownership. No coding or live failure assertion ran. The sanitized record is in `testdata/bridge-run-2026-10-02-01.json`; its original plan and commit remain in Git history.

The focused fix gives the child foreground ownership and restores the prior terminal group after exit or cancellation. It retains group cancellation and manual tool permissions. The synthetic terminal regression fails with the original launch and passes with the fix, including cancellation and foreground restoration. You can reproduce that local check without Claude or account access:

```sh
rtk proxy go test -c -race -tags livebridge -o bin/bridge-live.test ./internal/kiro
rtk proxy python3 internal/kiro/testdata/terminal-check.py bin/bridge-live.test
```

The first approval was consumed by that launch. The corrected code and updated plan need a separate run review. No automatic live retry is performed.

## Second live launch outcome

The corrected plan was separately approved and launched on October 2, 2026. Claude Code reached the production source read, which failed with `credential_expired`. The runner retained that first cause, stopped the client, and completed cleanup within the deadline. Dispatches remained zero. The coding task, tool permission exchange, followup, and live failure checks remain unrun. The sanitized record is in `testdata/bridge-run-2026-10-02-02.json`.

That approval is consumed. Before another run, the operator can renew the session through the normal Kiro CLI sign in flow. Explicit relinking and restoration of the exact model mapping require authorization; neither happened during this run. A new concrete plan and run review follow that recovery. That consumed plan remains available in Git history and is identified by the second run evidence record.

## Session recovery

After the operator confirmed renewed sign in, explicit `account link` succeeded. The exact `claude-opus-5.5` mapping was restored through the existing locked, validated configuration store, and `config check` passed. Both client artifacts still match the plan digests. No inference or profile inspection was performed during recovery, and no account values were retained in this record.

The current plan prepares a third launch under the same prompts, destinations, manual permission procedure, and 20 dispatch / 20 minute limits. It requires its own explicit run review. Earlier approvals remain consumed.

## Third live launch outcome and terminal streams

The separately approved recovered session run reached Kiro and consumed three dispatches. It ended with `coding_assertion_failed` and completed cleanup within the deadline. The client received model output, but its edit and shell permissions were not granted. No interactive permission prompt was available to the operator. The sanitized record is in `testdata/bridge-run-2026-10-02-03.json`.

The client inherited captured output streams in this execution channel and selected noninteractive behavior. The launcher now resolves the concrete terminal device behind stdin and connects all three client streams to it. It keeps separate process group cancellation and restores foreground ownership and terminal settings. The `/dev/tty` indirection was unsuitable for the pinned Bun runtime, so the concrete device is used instead. A synthetic regression reproduces captured stdout and stderr and checks all three client streams, input, cancellation, and foreground restoration.

The installed client also passed a fully interactive offline exercise through synthetic SQLite and TLS. It displayed individual Edit and Bash approval prompts, completed three matching tool results, and completed a followup turn across five synthetic dispatches. The optional LSP installation dialog was dismissed. No live Kiro dispatch occurred during these checks. You can run that offline exercise in a terminal:

```sh
rtk proxy go test -c -tags 'livebridge clientbridge' -o bin/bridge-live.test ./internal/kiro
rtk proxy ./bin/bridge-live.test -test.run '^TestInstalledClientInteractiveOffline$' -test.v -client-bridge -terminal-launch-check
```

For that disposable exercise, the initial task edits `value.go` and runs its tests. After the first response, enter `Confirm the completed fixture change on this subsequent turn.` After the second response, use `/exit`. Approve only the individual fixture operations, and dismiss optional plugin installation. The three minute offline deadline covers setup and interaction.

The third live approval is consumed. The corrected launcher must be pinned in a new plan before another reviewed launch. The live prompts, destinations, manual permission mode, and budget remain the same.
