# Experimental bridge

Spec 0004 defines the implemented experiment. `serve --experimental-bridge` adds authenticated Messages, streaming Messages, and local token estimates for the exact `claude-opus-5.5` mapping. Plain `serve` keeps its existing health behavior.

The parser retains one string system entry immediately after each user message. The adapter appends that string to its paired user content and prepends top level system text only to the first user. Text, tool schemas, arguments, IDs, and results retain their specified values. Unsupported fields still fail before account access.

Top level and history instructions become user context. System priority and instruction replacement are not guaranteed. Completion is inferred from validated clean EOF, reasoning is discarded, usage is estimated, and `max_tokens` is advisory. Effort is ignored, beta features are unsupported, and serving model identity is unverified. A whole missing frame at a clean boundary may escape detection. GA compatibility remains pending.

The installed Claude Code `2.1.289` offline exercises pass with manual permission mode and an isolated host that approves only the exact invented fixture operations. They cover Read, Edit, Bash, matching results, a second user turn, and one failed attempt for each synthetic 429, 502, and interrupted stream with `CLAUDE_CODE_MAX_RETRIES=0`. With the retry setting absent, 429 and 502 each reach a second request; interrupted SSE still produces one. The full wire exercise also uses the production Kiro adapter with synthetic SQLite and a local TLS service. The repair exercise proves successful test results and completed client turns after both edits, including the isolated boundary mutation check. These are offline observations for this version.

You can reproduce the explicit client exercises with:

```sh
rtk proxy go test -race -tags clientbridge ./internal/gateway -run TestInstalledClient -count=1 -v -args -client-bridge
rtk proxy go test -race -tags clientbridge ./internal/kiro -run TestInstalledClientWireLoopOffline -count=1 -v -args -client-bridge
rtk proxy go test -race -tags clientbridge ./internal/kiro -run '^TestRepairClient' -repair-client -v -count=1
```

Ordinary repository checks do not launch Claude Code or contact Kiro. The original mismatch capture and successful `2.1.287` records remain historical evidence. Current outcomes and the executable digest are in `internal/gateway/testdata/claude-code-2.1.289-offline-loop.json`. The consumed live plan and its runner still pin their original `2.1.287` artifacts.

The separately approved live proof passed on October 2, 2026 with `experimental_loop_observed`, 8 dispatches in 805.64 seconds, and cleanup within budget. Claude Code completed the coding and followup test loop under individual operator permissions. Cancellation and injected stream interruption also passed. The sanitized record is `internal/kiro/testdata/bridge-run-2026-10-02-04.json`. The runner and preserved plan are described in `internal/kiro/BRIDGE.md`. This launch approval is consumed. The separate verification workflow and GA acceptance remain pending.
