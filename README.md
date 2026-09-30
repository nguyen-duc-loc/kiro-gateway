# Kiro Gateway

The foundation is a foreground Go executable for macOS. It provides authenticated process health. Kiro inference and Claude Code compatibility are pending the first live coding loop.

You can build with the Go 1.27.1 toolchain:

```sh
go build -o bin/kiro-gateway ./cmd/kiro-gateway
./bin/kiro-gateway version
```

You can start the server with a randomly generated local credential:

```sh
export KIRO_GATEWAY_TOKEN="$(openssl rand -hex 32)"
./bin/kiro-gateway serve
```

`KIRO_GATEWAY_TOKEN` is required and must contain at least 32 visible ASCII characters without spaces. It is a local gateway credential, separate from your Kiro account. The server accepts it only from the environment. It does not read or change Kiro credentials.

`serve --listen 127.0.0.1:8787` sets the listener. The default is `127.0.0.1:8787`. Only numeric IPv4 addresses in `127.0.0.0/8` are accepted. Port `0` selects an available port, and startup output reports the actual address.

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

The code is split between `cmd/kiro-gateway` for process entry, `internal/cli` for commands, and `internal/gateway` for local HTTP and lifecycle. The bridge, Kiro adapter, and credential provider will be added with their working slices. The module uses the local name `kiro-gateway` until a public repository is chosen.

The [architecture spec](docs/specs/0001-stack-architecture/index.md) records the boundaries and the required live tool loop evidence. The [scope](docs/scope/scope.md) tracks verification and the later slices.

## Development

You need Git, the exact Go version in `go.mod` (currently 1.27.1), and a C compiler for the race detector. On macOS, the Xcode Command Line Tools provide the compiler. There are no additional runtime dependencies or required credentials for the checks.

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

The [GitHub Actions workflow](.github/workflows/check.yml) runs the same command on pushes and pull requests using `macos-15`. It reads the Go version from `go.mod`. Actions are pinned to commit hashes, repository access is limited to reading, and dependency caching is disabled because this module has no `go.sum`. You can update the action hashes and their version comments together when upgrading. The runner label fixes the macOS generation, but GitHub updates the image over time.
