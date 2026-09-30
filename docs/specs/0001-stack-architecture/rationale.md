# Reasoning for the Go gateway architecture

## Context

> Premise note: A working Kiro CLI login does not establish a usable model inference interface. The product depends on Claude Code receiving and executing model tool requests itself. That property needs a real compatibility proof before broader development.

You want dependable daily coding through the Claude models available to your Kiro account. Your existing access is Kiro CLI with IAM Identity Center. The initial platform is local macOS with one account. Claude Code must retain conversation, prompt, permission, and tool ownership. The scope prioritizes streaming, tool use, cancellation, long conversations, and visible failures over a dashboard.

This folder contained the scope and workflow skills, but no application source, root context file, existing spec, or Git repository at preflight. There was no stack to preserve. Team capacity, deadline, budget, and specific past gateway failures remain unknown. No compliance certification or organizational authorization was inferred from the account type.

The architecture decision is linked to scope feature 1, which includes a runnable foundation. The credential model, concrete protocol bridge, and renewal are separate planned features. This record settles their boundaries without pretending to replace their detailed designs.

## Options considered

### Option 1: Go executable with internal adapter

A foreground Go process serves the local client and translates requests through a replaceable internal Kiro adapter. The first slice reuses your CLI session. (basis: your selected runtime and process model, Go HTTP documentation)

**Pros**: Small distribution surface, explicit process lifecycle, standard HTTP transport, and no second application runtime.

**Cons**: Protocol compatibility still requires custom implementation and evidence. The adapter shares the process failure boundary. An undocumented upstream creates ongoing maintenance work.

### Option 2: TypeScript and Node gateway

A foreground Node process implements the same ownership and protocol boundaries, with TypeScript for the application. (basis: Node HTTP documentation, locally installed Node)

**Pros**: A large JavaScript contributor pool, familiar asynchronous streaming, and a straightforward path to a React based terminal interface.

**Cons**: Distribution needs a Node runtime or executable packaging work. Adding UI libraries does not resolve upstream compatibility. This was the runtime runner up, but you chose Go.

### Option 3: Rust executable with internal adapter

A native executable uses Rust for the local server, stream state, and adapter. (basis: native executable distribution and explicit ownership practices)

**Pros**: Strong type and ownership checks with a native distribution model.

**Cons**: The toolchain was absent locally, and implementation would add setup and language complexity without evidence that it solves the gateway's dominant protocol risk. No current Rust library selection was researched or made.

### Option 4: Client bridge over Kiro ACP

An outer client communicates with Kiro's existing agent through ACP, its Agent Client Protocol interface. (basis: Kiro ACP and architecture documentation)

**Pros**: Uses a documented Kiro integration boundary and existing agent session behavior.

**Cons**: The reviewed documentation describes Kiro's harness controlling conversation and tools. It does not establish caller supplied arbitrary model tool schemas and tool result continuation under Claude Code's agent loop. Selecting it as the inference backend would be an unsupported assumption about the required behavior.

## Rationale

Go matches your explicit preference and the foreground, one executable operating model. The standard HTTP library provides the transport primitives this project needs. A separate adapter process was considered, but you chose an internal interface because another process would introduce an extra lifecycle and protocol before the first working request. (basis: design conversation, Go HTTP documentation, explicit resource ownership)

The most important risk is semantic compatibility: whether instructions, tools, tool results, stream completion, and cancellation retain their meaning across the connection. Language choice cannot eliminate that risk. The architecture therefore makes the adapter replaceable and the real tool loop a delivery gate. You explicitly accepted investigation of an undocumented interface and its compatibility maintenance cost. (basis: project scope, Anthropic streaming and tool use documentation)

You chose Kiro CLI ownership of sign in for the first slice and asked to improve it later. A credential provider separates the temporary acquisition mechanism from the request transport. This choice avoids inventing an IAM Identity Center login implementation before account and protocol access are proven. It does not promise that the CLI exposes an appropriate reusable credential. (basis: design conversation, local Kiro CLI help)

The local listener, separate gateway credential, bounded resources, and restricted logging follow the installed Go security skill. Database, hosted error collection, and background worker infrastructure have no demonstrated role in this foundation. (basis: installed Go security skill, project scope, least privilege)

You suggested Ink if the CLI needs UI. The existing scope defers a visual interface, and the first slice only needs readable process output. Ink renders React in a Node.js process, so adopting it now would expand the chosen runtime and packaging model without advancing the compatibility proof. (basis: project scope, Ink documentation)

## Evidence and limits

| Observation | What it establishes | What it does not establish |
|---|---|---|
| Local `kiro-cli --version`: 2.8.0 | Installed CLI version | Supported private service contract |
| Local login help lists Identity Center license, provider URL, region, and device flow | The installed CLI has an IAM Identity Center login path | The account's region, scopes, tokens, or expiry |
| Local ACP help and official ACP page | An agent protocol interface exists | A raw inference API compatible with Claude Code's tool loop |
| Kiro model documentation | Opus and Sonnet model offerings and inference routing information | Availability of every model to this account or a public direct inference contract |
| Local Go 1.27.1 on darwin/arm64 and Node v24.20.0 | Both runtimes are available for development | Support for every platform or contributor toolchain |
| `rustc` absent from PATH | Additional Rust setup would be needed locally | Anything about Rust's current ecosystem quality |
| Security skill installed and reference files present | Project has the selected development guidance | An application security audit has occurred |
| gopls v0.23.0 starts over stdio and lists eight tools | The local server works and the project configuration parses | This already running conversation has reloaded that MCP connection |

The public pages reviewed did not reveal a documented Kiro direct inference and authentication contract for this account scenario. This is a documentation limit, not proof that no suitable internal interface exists. Anthropic's warning about gateways to non Claude models does not establish a prohibition for this Claude Opus and Sonnet scenario. (basis: Kiro models and Claude Code gateway documentation)

No Kiro credentials were inspected, no sign in was initiated, and no model inference was invoked during design. No application code or live compatibility test has been written. The scaffold and first loop remain future implementation work.

### Development tool installation

You explicitly selected discovery and then installation of the skill and MCP tool. The security skill was downloaded from `samber/cc-skills-golang` at revision `19a0626ae8565d27a7b7bdf59d8d99d94d7e284c` using the skill installer. Its installed location is `.agents/skills/golang-security`, with 14 files including 12 reference documents. No additional security scanner was installed by that operation.

The existing gopls binary at `/Users/nguyenducloc/go/bin/gopls` is registered in project `.codex/config.toml` with `args = ["mcp"]` and this workspace as its working directory. Codex reads it as enabled. A direct MCP initialization and tools listing succeeded, returning `go_diagnostics`, `go_file_context`, `go_package_api`, `go_rename_symbol`, `go_search`, `go_symbol_references`, `go_vulncheck`, and `go_workspace`. The process was stopped after the check. The configuration uses local absolute paths and needs adjustment if this checkout moves. (basis: local installation checks, official gopls MCP and Codex MCP documentation)

gopls MCP is experimental and optional. It runs as a development tool alongside the coding assistant. Its presence does not change the one process gateway runtime. GitHub tools were already connected. No additional Kiro or IAM MCP was selected.

### Independent review

At your request, a different model (`gpt-6-sol`) reviewed both spec files and the scope without editing files or fetching reference links. It found no blocking architecture decision gaps. The review confirmed that the scaffold is sufficiently specified and that credential and protocol discovery are appropriately gated to scope features 3 and 4. Two wording clarifications were applied: the health response names its `version` field, and tooling and release packaging ownership now explicitly points to scope features 2 and 9. This review does not establish live Kiro compatibility or constitute your acceptance of the spec.

## References

**Project sources**

1. [Project scope](../../scope/scope.md), including the Tracer Bullet approach and tool ownership requirement.
2. [Installed Go security skill](../../../.agents/skills/golang-security/SKILL.md), network and credential boundaries.
3. [.codex/config.toml](../../../.codex/config.toml), project MCP configuration.
4. Your recorded choices in this design conversation and the local version, help, installation, and MCP checks summarized above.

**Practices and standards**

1. Explicit resource ownership and cancellation.
2. Least privilege at local and upstream trust boundaries.
3. Protocol compatibility demonstrated with a real tool result loop.

**Verified links**

1. [Go HTTP package](https://pkg.go.dev/net/http).
2. [Node HTTP documentation](https://nodejs.org/api/http.html).
3. [Kiro ACP documentation](https://kiro.dev/docs/cli/acp/).
4. [How Kiro works](https://kiro.dev/docs/how-kiro-works/).
5. [Kiro model documentation](https://kiro.dev/docs/models/).
6. [Claude Code gateway documentation](https://code.claude.com/docs/en/llm-gateway).
7. [Anthropic message streaming](https://docs.anthropic.com/en/api/messages-streaming).
8. [Anthropic tool use](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/implement-tool-use).
9. [Ink repository and documentation](https://github.com/vadimdemedes/ink).
10. [Official gopls MCP documentation](https://go.dev/gopls/features/mcp).
11. [Codex MCP configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli).
