# Kiro Gateway

The foundation is a foreground Go executable for macOS. It provides authenticated process health. Kiro inference and Claude Code compatibility are pending the first live coding loop.

You can build with the Go 1.27.1 toolchain and the Xcode Command Line Tools C compiler:

```sh
CGO_ENABLED=1 go build -o bin/kiro-gateway ./cmd/kiro-gateway
./bin/kiro-gateway version
```

You can start the server with a randomly generated local credential:

```sh
export KIRO_GATEWAY_TOKEN="$(openssl rand -hex 32)"
./bin/kiro-gateway serve
```

`KIRO_GATEWAY_TOKEN` is required and must contain at least 32 visible ASCII characters without spaces. It is a local gateway credential, separate from your Kiro account. The server accepts it only from the environment. It does not read or change Kiro credentials.

`serve --listen 127.0.0.1:8787` overrides the saved listener for one run. Without a flag, the saved `listen` setting applies, or `127.0.0.1:8787` if it is absent. Only numeric IPv4 addresses in `127.0.0.0/8` are accepted. Port `0` selects an available port, and startup output reports the actual address. An invalid saved file blocks startup even with a valid flag.

From a shell with the same credential, you can check process health:

```sh
printf 'Authorization: Bearer %s\n' "$KIRO_GATEWAY_TOKEN" |
  curl --silent --show-error --header @- http://127.0.0.1:8787/healthz
```

The response is `{"status":"running","version":"dev"}`. Missing or invalid credentials return HTTP 401. This endpoint reports process health only, not upstream readiness.

Ctrl+C or a termination signal stops the listener, cancels active work, and allows up to five seconds before closing remaining connections. Diagnostics go to stderr and exclude request headers, bodies, and credentials.

You can inject a release version when building:

```sh
go build -ldflags '-X main.version=0.1.0' -o bin/kiro-gateway ./cmd/kiro-gateway
```

The code is split between `cmd/kiro-gateway` for process entry, `internal/cli` for commands, and `internal/gateway` for local HTTP and lifecycle. `internal/config` holds settings and session transitions, `internal/configstore` owns file storage and locking, and `internal/credentials` captures the selected Kiro record. The bridge and inference adapter remain future work. The module uses the local name `kiro-gateway` until a public repository is chosen.

The [architecture spec](docs/specs/0001-stack-architecture/index.md) records the boundaries and the required live tool loop evidence. The [scope](docs/scope/scope.md) tracks verification and the later slices.

## Saved settings and session references

You can create and check your settings with:

```sh
./bin/kiro-gateway config init
./bin/kiro-gateway config check
```

The only settings file is `~/.config/kiro-gateway/config.json`. Its initial contents are:

```json
{
  "schema_version": 1,
  "listen": "127.0.0.1:8787",
  "session": null,
  "models": {}
}
```

Stop the gateway before editing this JSON file or running commands that change settings. Finish any manual edits before running a mutation. Each command uses a process lock, and a running server holds that lock until shutdown. Manual editors do not participate in it. Edits take effect after restart. Neither the working directory nor `XDG_CONFIG_HOME` changes the saved location.

You can select the existing IAM Identity Center record after signing in through Kiro CLI:

```sh
./bin/kiro-gateway account link
```

Link reads only `~/Library/Application Support/kiro-cli/data.sqlite3`, table `auth_kv`, key `kirocli:odic:token` (the spelling `odic` is intentional). It reads an existing unexpired record without refreshing or changing it, then saves only its source identifier and SHA 256 fingerprint. This private source contract was observed in Kiro CLI 2.8.0 and can change. A saved reference identifies exact bytes, not a person, verified account, available model, or working inference connection.

When linked, you may add up to 32 exact client name to upstream ID entries in `models`. Names and IDs must contain 1 to 256 visible ASCII bytes without spaces. Several client names may point to one ID. Empty mappings are valid. Mappings require a session reference and do not establish upstream availability.

Repeating link with the same record preserves the file exactly. Any change to the record, including token renewal or JSON whitespace, produces a different fingerprint. Linking that changed record clears every model mapping. There is no automatic account switch or credential renewal.

You can remove the reference and all mappings with:

```sh
./bin/kiro-gateway account forget
```

Forget is safe to repeat and leaves Kiro CLI credentials intact. It does not log you out of Kiro or remove editor backups. The gateway keeps no settings history, credential copies, conversation files, telemetry, or diagnostic files. Routine diagnostics go to stderr with zero gateway managed retention. Output redirected by your shell is outside that retention policy.

`config upgrade` explicitly checks an existing file. Version 1 is already current and remains byte for byte unchanged. Unknown versions fail without modification. Startup never upgrades or repairs settings. Files larger than 64 KiB, unknown or duplicate members, invalid UTF 8, and invalid field values are rejected without echoing their contents.

The gateway creates its directory with mode `0700`, and `config.json` and the stable `.lock` file with mode `0600`. Existing paths must belong to your user, use those permissions or narrower access, and contain no appended symlinks. Parent directories must prevent other users from replacing these paths. Unsafe paths fail without automatic permission repair. The source directories and database must belong to your user and must not be writable by other users. The home directory itself is resolved to its canonical location.

Initialization never overwrites an existing file. Mutations install a complete file atomically. Failures before installation preserve the old file. If an error says the save outcome is uncertain, you can run `config check` and inspect the saved settings before retrying. A terminated process releases the lock automatically; the empty `.lock` file may remain. Later successful mutations remove recognizable private temporary files left by interrupted writes.

The [configuration spec](docs/specs/0002-local-configuration-credentials/index.md) records the complete model and safety contract. Tests use synthetic databases and isolated homes, including real process locking and health requests. They do not read your Kiro credentials or prove live inference compatibility.

## Experimental Claude Code bridge

You can enable the Messages API with `serve --experimental-bridge` after linking a session and saving the exact mapping `"claude-opus-5.5": "claude-opus-5.5"`. Stop the gateway before editing settings, run `config check`, then restart. Startup and health do not read Kiro credentials.

```sh
rtk proxy go run ./cmd/kiro-gateway serve --experimental-bridge
```

The flag enables authenticated `POST /v1/messages` with SSE or ordinary JSON and `POST /v1/messages/count_tokens`. Both use the existing bearer token and API version `2023-06-01`. Only one inference can run at a time. The count endpoint estimates tokens locally. Plain `serve` keeps inference routes disabled.

This is experimental. Top level and history instructions become user context, without guaranteed system priority or replacement. Completion is inferred, reasoning is discarded, usage is estimated, and `max_tokens` is advisory. Effort is ignored, beta features are unsupported, and serving model identity is unverified. Startup output and response headers disclose these limits.

The pinned Claude Code `2.1.287` completed the offline Read, Edit, Bash, tool result, and followup exercise through a synthetic TLS service. One separately approved real account coding proof also passed with 8 dispatches, including a followup test, cancellation, and interrupted stream handling. This is an observed experimental loop; broader compatibility and GA acceptance remain pending. The [bridge guide](internal/bridge/README.md) describes the offline checks. The [live proof guide](internal/kiro/BRIDGE.md) links the concrete plan, its separate run review, and the manual permission procedure.

## Development

You need Git, the exact Go version in `go.mod` (currently 1.27.1), and a C compiler for the SQLite adapter and race detector. On macOS, the Xcode Command Line Tools provide the compiler. The module pins `github.com/mattn/go-sqlite3` at `v1.14.50` and uses its bundled SQLite with cgo enabled. Do not use the `libsqlite3` build tag. No external SQLite executable or live credentials are required. The first build downloads the pinned Go module.

From a clean checkout, you can run every check with:

```sh
./scripts/check
```

The command checks formatting without changing files, runs `go vet`, builds every package, and runs all tests with the race detector. It stops at the first failure. It uses the installed Go toolchain and rejects a different version instead of downloading one. Module operations use `-mod=readonly`, and checks ignore any surrounding Go workspace.

You can fix formatting with `gofmt -w cmd internal`, then run the checks again. If you add Go source outside `cmd` or `internal`, include its directory in the formatting step in `scripts/check`.

You can enable the same checks before each commit in this clone:

```sh
git config --local core.hooksPath .githooks
```

This replaces the clone's hook directory setting. If you already use custom hooks, you can call `./scripts/check` from your existing hook instead. The hook checks the working tree, so you should stage the changes you intend to commit after any fixes. Cloning does not enable hooks automatically. You can remove this setting with `git config --local --unset core.hooksPath`.

When using the project's agent shell, prefix these commands with `rtk proxy`, for example `rtk proxy ./scripts/check`. RTK is optional for contributors and CI.

The [GitHub Actions workflow](.github/workflows/check.yml) runs the same command on pushes and pull requests using `macos-15`. It reads the Go version from `go.mod`. Actions are pinned to commit hashes, repository access is limited to reading, and dependency caching is disabled. You can update the action hashes and their version comments together when upgrading. The runner label fixes the macOS generation, but GitHub updates the image over time.
