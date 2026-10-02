# Experimental bridge

Spec 0004 defines the implemented experiment. `serve --experimental-bridge` adds authenticated Messages, streaming Messages, and local token estimates for the exact `claude-opus-5.5` mapping. Plain `serve` keeps its existing health behavior.

The parser retains one string system entry immediately after each user message. The adapter appends that string to its paired user content and prepends top level system text only to the first user. Text, tool schemas, arguments, IDs, and results retain their specified values. Unsupported fields still fail before account access.

Top level and history instructions become user context. System priority and instruction replacement are not guaranteed. Completion is inferred from validated clean EOF, reasoning is discarded, usage is estimated, and `max_tokens` is advisory. Effort is ignored, beta features are unsupported, and serving model identity is unverified. A whole missing frame at a clean boundary may escape detection. GA compatibility remains pending.

The installed Claude Code `2.1.287` offline exercises pass with manual permission mode and an isolated host that approves only the exact invented fixture operations. They cover Read, Edit, Bash, matching results, a second user turn, and one failed attempt for each synthetic 429, 502, and interrupted stream with `CLAUDE_CODE_MAX_RETRIES=0`. The full wire exercise also uses the production Kiro adapter with synthetic SQLite and a local TLS service. This establishes offline behavior, not a live account coding loop or the live human permission procedure.

You can reproduce the explicit client exercises with:

```sh
rtk proxy go test -race -tags clientbridge ./internal/gateway -run TestInstalledClient -count=1 -v -args -client-bridge
rtk proxy go test -race -tags clientbridge ./internal/kiro -run TestInstalledClientWireLoopOffline -count=1 -v -args -client-bridge
```

Ordinary repository checks do not launch Claude Code or contact Kiro. The original mismatch capture remains historical evidence in `internal/gateway/testdata/claude-code-2.1.287-bridge.json`. The new outcomes are in `internal/gateway/testdata/claude-code-2.1.287-offline-loop.json`.

The bounded live runner and its concrete plan are described in `internal/kiro/BRIDGE.md`. That run still needs the separate review required by spec 0004. No live coding verdict or GA completion is claimed.
