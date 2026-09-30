# Kiro Gateway

## Stack

Go 1.27.1, one `kiro-gateway` module and foreground macOS executable. Prefer the standard library: `net/http`, `flag`, `encoding/json`, `context`, `os/signal`, and `log/slog`. Go modules manage dependencies; there are no external runtime packages. The accepted architecture calls for a replaceable internal Kiro adapter using the existing Kiro CLI session. Only the authenticated health foundation is implemented.

## Build approach

Tracer Bullet (prove one real coding loop through every layer, then strengthen it in working slices).
Source: [scope](docs/scope/scope.md). Claude Code owns conversation and tool execution. Live compatibility needs the streaming, tool result, continuation, and cancellation evidence in the architecture spec.

## Commands

You can build with `rtk proxy go build -o bin/kiro-gateway ./cmd/kiro-gateway`, inspect the version with `rtk proxy go run ./cmd/kiro-gateway version`, and serve with `rtk proxy go run ./cmd/kiro-gateway serve` after setting `KIRO_GATEWAY_TOKEN` in the environment.
You can format with `rtk proxy gofmt -w cmd internal`, check with `rtk proxy go vet ./...`, compile all packages with `rtk proxy go build ./...`, and test with `rtk proxy go test -race ./...`.
You can run all checks with `rtk proxy ./scripts/check` and enable the commit hook in each clone with `rtk proxy git config --local core.hooksPath .githooks`.

## Rules

* Prefix every shell command with `rtk`; use `rtk proxy` for commands without a dedicated wrapper. See your [RTK guidance](/Users/nguyenducloc/.codex/RTK.md).
* Follow Clean Architecture within the accepted layout: process entry in `cmd/kiro-gateway`, wiring in `internal/cli`, and local HTTP in `internal/gateway`. Add `internal/bridge`, `internal/kiro`, and `internal/credentials` only with their working slices.
* Keep dependencies directed toward inner logic. Put small interfaces at the consuming boundary and implement I/O in adapters. Keep protocol transformations independent of HTTP transport and credential storage, using explicit request and event types. Avoid empty layers or a general provider framework.
* Document exported APIs, use idiomatic Go names, return explicit errors, and validate settings before serving. Keep user facing errors clear and sanitized.
* Carry cancellation through request contexts; keep stream state per request. Bind only to numeric IPv4 loopback and authenticate before account access. Keep gateway and Kiro credentials separate, and exclude secrets and conversation content from logs. The first live slice has no automatic replay.
* Use Go `testing` and `httptest`, with `*_test.go` beside source. Unit tests exercise inner logic without live dependencies; integration tests cover adapters and process behavior. Use synthetic credentials, isolated environments, and ephemeral loopback ports. Keep live compatibility evidence separate from deterministic tests.

## Tooling

Selected: `scripts/check` checks formatting without changing files, runs `go vet`, compiles all packages, and runs unit and integration tests with the race detector. It requires the exact Go version in `go.mod` and a C compiler, disables automatic toolchain downloads and surrounding Go workspaces, and uses `-mod=readonly`. The optional `.githooks/pre-commit` runs the same checks on the working tree. GitHub Actions runs them on pushes and pull requests using `macos-15` and actions pinned to commit hashes in `.github/workflows/check.yml`. Include any Go source directories added outside `cmd` and `internal` in the formatting step. See `README.md` for setup.

## Git

- integration: on
- branch prefix: feat/
- commit: per-milestone

Use conventional commit messages. Pushes and pull requests require explicit user confirmation.

## Specs

Specs live in `docs/specs/<number>-<name>/index.md`, with supporting rationale alongside. The [accepted architecture](docs/specs/0001-stack-architecture/index.md) is the stack source of truth. Credential persistence and live inference need their own decisions before implementation.

## Agent skills

* [architect](.agents/skills/architect/): `JavaScript-Mastery-Pro/skills`, architecture decisions and specs.
* [audit](.agents/skills/audit/): `JavaScript-Mastery-Pro/skills`, initial project context.
* [check](.agents/skills/check/): `JavaScript-Mastery-Pro/skills`, behavior verification and independent review.
* [debug](.agents/skills/debug/): `JavaScript-Mastery-Pro/skills`, bug reproduction and focused fixes.
* [develop](.agents/skills/develop/): `JavaScript-Mastery-Pro/skills`, implementation from accepted decisions.
* [document](.agents/skills/document/): `JavaScript-Mastery-Pro/skills`, prose about shipped changes.
* [scope](.agents/skills/scope/): `JavaScript-Mastery-Pro/skills`, product scope and working slices.
* [sync](.agents/skills/sync/): `JavaScript-Mastery-Pro/skills`, durable context maintenance after changes.
* [test](.agents/skills/test/): `JavaScript-Mastery-Pro/skills`, tests for changed behavior.
* [golang-security](.agents/skills/golang-security/): `samber/cc-skills-golang`, security guidance for Go network, credential, input, and logging changes.
* [go-testing](.agents/skills/go-testing/): `cxuu/golang-skills`, Go test structure, subtests, and helpers.
* [go-concurrency](.agents/skills/go-concurrency/): `cxuu/golang-skills`, goroutine lifecycle and synchronization.

MCP servers: gopls (configured in `.codex/config.toml`; local paths need adjustment if the checkout moves).

_Drafted by /audit from the repo, worth a quick human pass. Edit freely: once a line stops matching this draft, later runs treat it as curated and will flag rather than overwrite it._
