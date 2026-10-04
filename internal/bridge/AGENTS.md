# Experimental client protocol

You can use [spec 0004](../../docs/specs/0004-claude-code-bridge/index.md) for the experimental contract and [README.md](README.md) for explicit offline client exercises.

`bridge.go` owns normalized request, event, failure, and generator types plus canonical content and token estimates. `request.go` validates the complete client request before account access. `response.go` owns response accumulation. `observation.go` carries fixed development lifecycle labels. HTTP serving belongs in `internal/gateway`; upstream transport and credentials belong outside this package.

Requests admit only the exact `claude-opus-5.5` model. Preserve tool schemas, IDs, result order, and error flags. Unsupported fields fail validation; accepted compatibility hints are discarded. One string system entry may immediately follow a user message, but normalization must not imply system priority or instruction replacement.

Each response owns its accumulator. Text precedes tool calls, with at most 2 MiB of text, 16 tool calls, and 256 KiB per tool input. Check remaining capacity before appending. Keep text in a bounded builder and materialize it in `Complete` before using final content or usage. Never copy a used builder or share response state across requests.

`Generator.Generate` is synchronous and emits serially. Callback errors stop generation. Complete tools may be emitted only after the adapter validates the whole response and finishes cleanup. `inferred_clean_eof` is an experimental completion basis, not authoritative upstream success. Token counts remain estimates.

Development observations contain only a local request ID and fixed labels. `ObserveTerminal` belongs after successful final response writes and reports only `end_turn` or `tool_use`. A tool handoff alone does not establish a completed task turn. Ordinary serving installs no observer.

You can run `rtk proxy go test -race ./internal/bridge` from the repository root. The fragmented allocation check exercises the production accumulator with 1024 prebuilt 64 byte fragments and a 1 MiB cumulative heap budget per operation. You can run it without parallel package activity using `rtk proxy go test -p 1 ./internal/bridge ./internal/kiro -run '^TestFragmentedAccumulationAllocation$' -count=1`. Installed client exercises require their explicit build tags and launch flags; ordinary checks remain synthetic.

_Drafted by /sync from the introducing change, worth a quick human pass._
