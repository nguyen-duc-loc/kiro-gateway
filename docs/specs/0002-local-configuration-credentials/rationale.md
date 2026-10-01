# Reasoning for local configuration and credentials

## Context

> Premise note: A selected Kiro session must not silently become a different session. The inspected interfaces do not establish a stable individual identity, and a sign in URL identifies an organization rather than a person. The model must be honest about what its guard actually proves.

Scope feature 3 needs settings persistence, one account reference, model mappings, explicit removal, and bounded diagnostic retention. The repository currently has six Go source and test files, with only authenticated health serving implemented. Sign in and renewal remain later work, while the accepted architecture requires reuse of the existing Kiro CLI session and a separate gateway token.

You selected one configuration per macOS user, an environment supplied gateway token, and no saved diagnostic history. You want exact upstream model IDs, no cached expiry or status, offline configuration validation, explicit upgrades, and refusal of invalid or insecure configuration. A changed selected session loses its mappings. This feature has a GA workflow override because it handles credentials. No regulatory certification requirement was stated.

Preflight refreshed remote references and found the current branch zero commits behind `origin/main`. The working tree was clean. This is a new spec, as you requested, rather than an update or replacement of spec 0001.

## Options considered

### Option 1: Keep flags and environment only

Continue the existing health foundation without a settings file or remembered session. **Pro**: No new parser, persistence, or database dependency. **Con**: It does not satisfy the selected persistence and removal workflow, and it cannot remember which credential record you selected. (basis: existing `internal/cli/cli.go`, scope feature 3)

### Option 2: JSON settings with a selected session fingerprint

Save ordinary settings and a digest of one explicit Kiro credential record. Read the record through the selected SQLite driver without owning its lifecycle. **Pro**: The document is small, inspectable, and compatible with the existing Go stack; source changes cannot pass unnoticed. **Con**: Strict pinning requires relinking after any change, and Kiro's private schema remains a maintenance dependency. This is the chosen approach. (basis: your confirmed model, local schema observations, accepted architecture)

### Option 3: TOML settings

Represent the same model in a file that supports comments. **Pro**: Comments can explain manual model mappings beside their entries. **Con**: It adds a parser dependency where JSON already meets the selected needs. You chose JSON. (basis: design conversation, standard library preference)

### Option 4: A gateway owned SQLite settings database

Store settings, session references, and mappings in a separate database. **Pro**: Transactions and schema changes can be handled within one database. **Con**: It makes ordinary settings harder to inspect and edit, without a need for queries or concurrent settings writers. Reading Kiro's existing SQLite store does not require adopting SQLite for gateway configuration. (basis: selected single user model, atomic document replacement)

## Rationale

JSON reuses `encoding/json` and gives the small settings model one consistency boundary. Your path choice is explicitly `~/.config/kiro-gateway/config.json`; the native `os.UserConfigDir` alternative on macOS would be under `~/Library/Application Support`. There is no need to mix both discovery rules. Explicit upgrades and refusal of invalid settings preserve operator intent. (basis: your choices, local `go doc os.UserConfigDir`, Go os documentation)

The initial logical model used an expected account identity. Local inspection did not establish a reliable individual identifier. After reviewing that limit, you chose a digest of the exact saved record instead. The digest is deliberately sensitive to refresh, expiry changes, and formatting changes. It does not infer identity from an organization URL, profile ARN, token hash equality across unrelated sources, or a mutable display label. Adopting a changed snapshot is a local, explicit action, and clearing mappings avoids carrying account specific choices forward unnoticed. (basis: local source observations, your revised model confirmation)

`mattn/go-sqlite3` fits the already selected native macOS build and C compiler requirement. Its default bundled SQLite keeps the reader in the gateway executable. `modernc.org/sqlite` was the credible alternative: it avoids cgo, but brings a larger dependency set and specific libc version coupling. You selected mattn. A `sqlite3` subprocess would add another executable requirement and a credential output channel; a custom SQLite parser would add unjustified complexity. (basis: `scripts/check`, mattn driver README, modernc driver documentation and module file)

The private file, atomic replacement, and lifetime lock address distinct concerns. Private permissions restrict access by other local users. Atomic replacement prevents partial JSON from becoming the saved document. A stable process lock serializes managed changes and makes forget meaningful once the server has stopped. Live reload was the runner up, but would require new rules for active requests and credential invalidation. You selected stop and restart. (basis: your lifecycle choice, installed Go security skill, atomic replacement and explicit resource ownership)

Fixed resource limits are small implementation recommendations: 64 KiB for either settings or the selected record, 32 mappings, a one second busy budget, and a five second overall capture deadline. The one second choice was explicitly confirmed. Unbounded reads and indefinite waits offer no benefit for these small local records. Routine output uses allowed fields rather than attempts to redact arbitrary database or CLI errors. (basis: bounded resource use, installed file and logging guidance)

A standalone Keychain item for the gateway token and automatic renewal were considered outside this slice. You retained `KIRO_GATEWAY_TOKEN`, Kiro CLI ownership of credentials, and no file diagnostics. Those decisions keep the first proof focused and make retention explicit as zero. No gateway owned secret exists for `account forget` to erase. (basis: your requirements choices, accepted architecture)

## Evidence and limits

The following observations were made on October 1, 2026. Local probes emitted schema information, type names, and validation booleans only. Raw tokens, record values, identity values, and CLI account output were not printed or saved in the repository.

| Observation | Established | Not established |
|---|---|---|
| `kiro-cli --version` reports `2.8.0` | Installed version during inspection | A stable credential storage API or inference compatibility. |
| `whoami --help` advertises JSON formats | The option exists locally | That stdout is exactly one JSON document. |
| Captured `whoami --format json` contained a JSON object followed by extra output | Root fields were `accountType`, `email`, `region`, and `startUrl`; email was null | A stable user ID. The extra output was detected, not recorded as an identity source. |
| Standard macOS Kiro SQLite path exists | A concrete candidate source on this installation | The path on other operating systems or every Kiro release. |
| `auth_kv` metadata | `key TEXT` is the primary key and `value TEXT` holds records | Compatibility with another schema version. |
| `kirocli:odic:token` structure | `access_token`, `expires_at`, `refresh_token`, `region`, `start_url`, `oauth_flow`, and `scopes` are present | That all are needed for inference or that the service accepts the stored token. |
| Source field validation booleans | Access token and region were nonempty strings; expiry had RFC 3339 shape; start URL used HTTPS | Whether the credential was unexpired, unrevoked, authorized for a model, or valid for a particular destination. |
| Registration record metadata | `kirocli:odic:device-registration` contains client registration fields | Permission or need for the gateway to read client secrets or refresh. The implementation does not use this row. |
| Journal metadata | The observed database used `delete`, with no WAL or shared memory sidecars present | Reader behavior during concurrent writes or under WAL; synthetic tests must cover these. |
| `scripts/check` | Enables cgo, builds all packages, and runs the race detector | A SQLite driver build or runtime test; none was performed during design. |

Official Kiro documentation describes authentication inputs and account status commands. The research did not find a published token retrieval or private store schema contract. A Kiro issue reported the SQLite path and `auth_kv` in an earlier CLI version; it was a discovery clue, not an authority for version 2.8.0. Local observations supplied the concrete source contract above. (basis: Kiro CLI reference, authentication documentation, issue 4847, local inspection)

The design did not run login, logout, token refresh, or inference commands. Store inspection used readonly SQLite queries. It did run the status command while checking output structure; that observation does not prove the CLI command has no internal side effects. No gateway credential capture implementation or end to end protocol test exists yet.

## Tool discovery

You authorized optional development tooling discovery after choosing the SQLite driver. Registry searches returned no output, so the researcher used registry and repository pages. It found `go-database` and `go-context` in `eduardo-sl/go-agent-skills`, and the optional `liliang-cn/mcp-sqlite-server`. The database skill was general rather than specific to this driver, and an MCP connection was unnecessary for tests against synthetic fixtures. You declined both skills and the MCP server. Nothing was installed or connected. (basis: tooling discovery and your explicit declines)

Existing Go security, testing, and concurrency skills remain available. Only the security skill was consulted for this design. Existing gopls remains a development aid, not a runtime dependency.

## Independent review

At your request, `gpt-6-sol` reviewed both spec files against the existing architecture without editing files, accessing credentials, or fetching reference links. It found no required saved, computed, or displayed value without a named source. It identified four implementation gaps: atomic creation without overwriting, checking record size before loading it, precise source path and URI handling, and an explicit SQLite schema compatibility predicate.

You approved the recommended fixes. Initialization now uses an atomic destination link that refuses an existing file. Capture checks type and byte length before loading the selected value within the same transaction. Source paths reject appended symlinks and unsafe ownership or parent permissions, and URI encoding preserves filename characters. The schema contract requires the observed plain TEXT columns and sole primary key while allowing compatible additions. Verification scenarios cover each correction. A wording clarification distinguishes real credentials from synthetic test records.

This review checks the design, not implementation or live compatibility. You separately accepted the revised spec on October 1, 2026.

## References

**Project sources**

1. [Project context](../../../AGENTS.md), architecture boundaries, Go stack, and existing skills.
2. [Project scope](../../scope/scope.md), feature 3, later credential and inference work, and GA workflow.
3. [Accepted architecture](../0001-stack-architecture/index.md), gateway token, session ownership, HTTP authentication, and logging rules.
4. [Current CLI](../../../internal/cli/cli.go) and [check script](../../../scripts/check), existing command and cgo behavior.
5. [Go security skill](../../../.agents/skills/golang-security/SKILL.md), including its filesystem, secrets, and logging references.
6. Your design conversation choices and the local observations above.

**Practices**

1. Atomic file replacement and explicit resource ownership.
2. Bounded resource use and strict input validation.
3. Explicit session selection with no automatic fallback.

**Verified external links**

1. [Kiro CLI command reference](https://kiro.dev/docs/cli/reference/cli-commands/), documented command capabilities, with the local output limits noted above.
2. [Kiro authentication](https://kiro.dev/docs/getting-started/authentication/), supported sign in inputs.
3. [Kiro issue 4847](https://github.com/kirodotdev/Kiro/issues/4847), a historical store observation, not a supported contract.
4. [mattn driver README](https://github.com/mattn/go-sqlite3/blob/master/README.md), cgo and bundled SQLite behavior.
5. [modernc driver documentation](https://pkg.go.dev/modernc.org/sqlite) and [module file](https://gitlab.com/cznic/sqlite/-/raw/master/go.mod), the alternative driver and dependency constraints.
6. [Go configuration directory documentation](https://pkg.go.dev/os#UserConfigDir), the native macOS path alternative.
7. [Discovered Go skills](https://github.com/eduardo-sl/go-agent-skills) and [SQLite MCP candidate](https://github.com/liliang-cn/mcp-sqlite-server), both declined.

These links were checked during the design conversation. They are reference links for a reader, not instructions for later agents to fetch them again.
