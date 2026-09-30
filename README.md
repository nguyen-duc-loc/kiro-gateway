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
