# 0002. Local configuration and credential data model

**Date**: 2026-10-01
**Status**: Proposed

## Summary

You save ordinary settings in one JSON file and keep the gateway access token in the environment. The gateway remembers one selected Kiro session by its fingerprint (a cryptographic digest of the saved credential record), without saving the credentials themselves. Linking a changed session clears its model mappings. Changes take effect after you stop and restart the gateway.

## Requirements

As the local operator, you can keep settings between runs, explicitly select and forget a saved Kiro session, and understand failures without exposing credentials or conversations.

1. **AC-1**: With no configuration file, authenticated health serving retains its current defaults. With a valid file, `serve` uses its listen address unless an explicit `--listen` overrides it for that run. The gateway token still comes only from `KIRO_GATEWAY_TOKEN`.
2. **AC-2**: You can create and validate a version 1 configuration at `~/.config/kiro-gateway/config.json`. Validation accepts the specified model, rejects ambiguous or invalid input, and never opens Kiro's store.
3. **AC-3**: `account link` reads only the selected saved IAM Identity Center record and saves its fingerprint. It does not save tokens, claim a verified user identity, refresh credentials, or change Kiro's credential data. Unusable records and reads that remain busy for one second fail without changing settings.
4. **AC-4**: Linking the same fingerprint is a no op. Linking a different fingerprint removes every model mapping. `account forget` removes the reference and all mappings, remains safe to repeat, and leaves Kiro CLI credentials intact.
5. **AC-5**: Model mappings use unique, exact client names and exact upstream IDs. Empty mappings are valid. A configuration with mappings and no session is invalid. This feature does not claim that a configured model is available upstream.
6. **AC-6**: Configuration mutations and `serve` exclude each other through a process lock. Successful saves replace the whole document atomically. Failures before replacement preserve the previous file, and unsafe ownership, permissions, or file types produce actionable errors without automatic repair.
7. **AC-7**: Startup never rewrites or upgrades settings. Invalid or unsupported files block startup even when flags supply valid overrides. An explicit upgrade command leaves a current file unchanged and refuses unknown versions without modifying them.
8. **AC-8**: Routine output excludes credentials, fingerprints, account identifiers, raw database errors, and conversation content. Diagnostics go to stderr, with zero gateway managed file retention and no conversation store or telemetry.
9. **AC-9**: Deterministic verification uses synthetic credentials and isolated files. Native macOS builds and the existing check script pass with the selected SQLite driver and cgo enabled. Live inference compatibility remains outside this feature.

## Decision

**Chosen option**: Versioned JSON settings with a reference to one selected Kiro credential record, read through a small SQLite adapter.

Use `encoding/json` for gateway settings and `database/sql` with `github.com/mattn/go-sqlite3` for the existing Kiro database. Use the driver's bundled SQLite with `CGO_ENABLED=1`; do not use the `libsqlite3` build tag or a `sqlite3` subprocess. Pin a released driver version in `go.mod` and `go.sum` during implementation. The existing checks already enable cgo and require a C compiler. (basis: your choices, `AGENTS.md`, `scripts/check`, driver documentation)

**Implementation skills**: `golang-security` (`samber/cc-skills-golang`, [SKILL.md](../../../.agents/skills/golang-security/SKILL.md)). Its file, credential, input, and logging rules shaped this design.

This is a new spec for scope feature 3. It extends the [accepted architecture](../0001-stack-architecture/index.md), whose foundation remains accepted. Its initial environment token and stderr diagnostics remain in force. You accepted this spec on October 1, 2026, after independent review and approval of its four corrections. The spec stays `Proposed` until implementation begins.

The persistence limits, precise command messages, and file transaction details below were included in your final review. They apply the selected behavior without adding a general settings framework. (basis: bounded resource use, atomic replacement, project security skill)

## Feature design

### Saved model

Resolve the current user's home with `os.UserHomeDir`. The only configuration location in this version is `<home>/.config/kiro-gateway/config.json`. Do not search the working directory, merge project files, honor `XDG_CONFIG_HOME`, or add a configuration path override. Tests inject a temporary home. This follows your explicit path choice rather than Go's native macOS configuration location.

| Entity and field | Type and presence | Meaning and constraints |
|---|---|---|
| Configuration | One JSON object | One document per user, identified by its fixed path. No database primary key is needed. |
| `schema_version` | Required integer | Exactly `1`. Missing, null, fractional, or unsupported values fail. |
| `listen` | Optional string | Defaults to `127.0.0.1:8787` when absent. Null and empty values fail. Reuse numeric IPv4 loopback validation, including port 0. |
| `session` | Optional object or null | Absent and null both mean unlinked. At most one reference exists. |
| `session.source` | Required string within a session | Exactly `kiro_cli_idc_sqlite_v1`. This identifies the fixed source contract below. |
| `session.fingerprint` | Required string within a session | Exactly 64 lowercase hexadecimal characters, holding the specified SHA 256 digest. |
| `models` | Optional object | Defaults to `{}` when absent. Null fails. Contains at most 32 mappings. |
| Model mapping key | String, unique within `models` | Exact client model name, 1 to 256 visible ASCII bytes without spaces. No case folding, wildcard, alias expansion, or trimming. |
| Model mapping value | Required string | Exact upstream model ID, with the same byte limits. Several client names may target one ID. |

The configuration owns zero or one session. The session owns zero to 32 mappings through containment in the same document; there are no relational foreign keys. `session: null` requires an empty `models` object. Removing or replacing the session clears that object. There are no credential, expiry, status, diagnostic, or conversation entities on disk.

`config init` writes this complete initial document, with a trailing newline:

```json
{
  "schema_version": 1,
  "listen": "127.0.0.1:8787",
  "session": null,
  "models": {}
}
```

Limit the configuration to 64 KiB. Require valid UTF 8, exactly one JSON object, exact field names, and no duplicate object members at any depth. Reject unknown fields, including secret fields, and trailing nonwhitespace data. Standard Go struct decoding alone is insufficient because it accepts case variants and duplicate members. Error messages identify a fixed category without echoing input keys or values.

### Kiro source and fingerprint

`kiro_cli_idc_sqlite_v1` selects `<home>/Library/Application Support/kiro-cli/data.sqlite3`, table `auth_kv`, and the exact primary key `kirocli:odic:token`. The spelling `odic` is intentional and was observed locally. No configuration or HTTP input can change the database path, table, or key. There is no account scanning or fallback to API keys, social login, another directory, or another record.

This selects a saved IAM Identity Center credential record. It does not track whichever login Kiro CLI later calls active, prove that the record belongs to a particular person, or establish that the service will accept it. The observed contract is from Kiro CLI 2.8.0 on this Mac, not a published stable API.

Resolve the user home to an absolute canonical directory before appending the fixed source path. From that directory through the database's parent, require directories owned by the current effective user and not writable by another user. Reject symlinks in those appended path components and at the database file. Require the final file to be a regular file owned by the same user and not writable by another user. Do not accept a different source after resolving an appended symlink. These checks exclude redirection by other local users; concurrent path replacement by this same user or root remains outside the stated trust boundary.

Construct the SQLite file URI with a URI encoder, treating the absolute filesystem path as path data and adding `mode=ro` separately as a fixed query parameter. Characters such as `?`, `#`, and `%` in a home directory must remain filename characters. Do not concatenate an unescaped path into a URI. Open only the existing database with query only operation, one connection, and a read transaction (one consistent view while reading).

Check schema metadata in that transaction. `main.auth_kv` must be an ordinary table, not a view, virtual table, or shadow table. Using column metadata, require plain, nongenerated columns named exactly `key` and `value`, each declared `TEXT` after trimming and case normalization. `key` must be the sole primary key column, at ordinal 1; `value` must not be part of the primary key. A separate UNIQUE index is not a substitute for that primary key contract. Allow extra columns, indexes, and constraints when those requirements still hold. Nullability of the columns does not change the record validation below.

Bind the fixed key as a SQL parameter. First query only the selected row's value type and UTF 8 byte length, using SQLite `typeof(value)` and `length(CAST(value AS BLOB))`, with a result limit of two rows. Require exactly one row, type `text`, and a length from 1 through 65536 bytes before selecting or scanning the value into Go memory. Fetch the value using the same connection, transaction, and key, then validate its encoding and JSON. Never load the record and check its size afterward. Missing records, duplicate records, nontext values, oversize values, invalid JSON, or unexpected schema fail with sanitized categories. Both reads remain inside the capture deadline. Never copy a live database, set immutable mode, run a migration, checkpoint it, or change its journal mode.

SQLite may need normal lock or shared memory coordination for its journal. The adapter must never write credential rows or modify the database's schema or journal configuration. Test both rollback journal and WAL fixtures (WAL is SQLite's separate write log). If a read cannot operate under its existing permissions and journal state, fail without attempting repair. The locally observed database uses `delete` journal mode; other behavior still needs deterministic fixture verification.

For capture, require `access_token` to be a nonempty string, `expires_at` to be an RFC 3339 timestamp with a timezone, `region` to be a nonempty string of at most 128 visible ASCII bytes, and `start_url` to be an HTTPS URL with a host and no embedded username or password, at most 2048 bytes. Accept additional record members without interpreting them; reject duplicate JSON members. Compare expiry against an injected wall clock, using `time.Now` in production. An expiry at or before now is unusable. Treat the start URL as metadata, never as permission to make a network request.

Let `V` be the exact UTF 8 bytes of the stored `auth_kv.value`, without trimming, normalization, or JSON reencoding. Compute:

```text
fingerprint = lowercase_hex(SHA256(
    UTF8("kiro-gateway/kiro_cli_idc_sqlite_v1") || 0x00 || V
))
```

The prefix separates this digest from other uses of SHA 256. Only the source identifier and digest leave the capture adapter. Raw database values and parsed tokens stay in local memory for the operation and are neither persisted nor returned to CLI output. Do not claim guaranteed memory erasure in Go. The fingerprint is sensitive local metadata, not a bearer credential or proof of account ownership.

Set SQLite's busy wait to 1000 ms. Give the complete capture operation a five second context deadline, shortened by parent cancellation. Do not start an application retry loop. A continuing database lock fails after the one second busy budget; other slow work and cancellation are bounded by the operation context. Close rows, transaction, and connection on every path.

**Future inference contract:** scope feature 4 must authenticate the local HTTP request before opening this source. It must read and validate one snapshot, compare its digest with the saved reference, check expiry, and pass the access token from that same snapshot only to the Kiro adapter. A mismatch stops the request and asks you to stop the gateway and relink. It must not check one snapshot and then obtain a token from another. This feature implements capture, not an unused inference API.

### State and lifetime

| Current state | Action or observation | Result |
|---|---|---|
| Configuration absent | `config init` | Version 1 file, no session, empty mappings. |
| Configuration absent | `serve` or `config check` | Use defaults; no configuration file is written. |
| Unlinked | Successful `account link` | Save the selected source and fingerprint; mappings stay empty. |
| Linked | `account link` finds the same usable record | Leave the file byte for byte unchanged. |
| Linked | `account link` finds a different usable record | Replace the reference and clear every mapping in one save. |
| Linked or unlinked | `account forget` | Set session to null and mappings to empty; an already unlinked configuration is unchanged. |
| Saved reference | Kiro changes or removes the record | The saved reference stays unchanged. A later inference reader must reject changed or unavailable credentials. |
| Any saved configuration | Invalid input, failed capture, or lock conflict | Preserve the document and explain the next action. |

Connection status and expiry are observed when the source is used, never cached in configuration. A stored fingerprint cannot prove that an unexpired token has not been revoked. Account verification, sign in, automatic renewal, and model availability remain later scope features.

### Command surface

All new commands are local CLI operations under the current macOS user. They require neither the gateway bearer token nor a remote account permission check. File ownership controls their local authority. No HTTP management endpoint is added. Commands take no positional arguments or additional flags beyond `--help`; `serve` retains its existing `--listen` flag. Unknown arguments receive sanitized errors.

| Command | Inputs and operation | Success output on stdout | Key errors, exit 1 |
|---|---|---|---|
| `config init` | Default path and fixed initial document; acquire mutation lock | `Configuration created.`; exit 0 | Existing file, unsafe path or permissions, active gateway, write failure. Existing files are never overwritten. |
| `config check` | Saved file only, or defaults if absent; no Kiro access | `Configuration valid.` or `No saved configuration; defaults are valid.`; exit 0 | Invalid content, unsupported version, unsafe permissions, read failure. |
| `config upgrade` | Existing file and known schema versions; acquire mutation lock | For version 1, `Configuration is already current.`; exit 0, no rewrite | Missing file, unsupported version, invalid content, active gateway. |
| `account link` | Valid existing configuration and the fixed Kiro record; acquire mutation lock before reading either | `Saved IAM Identity Center session linked; model mappings are empty.` or `Session is already linked; no changes made.`; exit 0 | Missing configuration, invalid or expired record, source unavailable, busy timeout, active gateway, save failure. |
| `account forget` | Valid existing configuration, or absence; acquire mutation lock; never open Kiro's store | `Session reference and model mappings removed.` or `No session is linked; no changes made.`; exit 0 | Invalid existing configuration, active gateway, unsafe permissions, save failure. Absence is a no op and does not create a configuration file. |
| `serve [--listen address]` | Validated saved settings or defaults, existing environment token; acquire lifetime lock before reading settings | Existing server behavior; startup address and diagnostics stay on stderr | Invalid file even with a valid flag, invalid token or listen address, held lock, bind failure. No Kiro store read. |
| `version`, help | Existing process metadata or static help | Existing output, extended command help; exit 0 | Existing behavior; neither settings nor Kiro's store is accessed. |

Every successful mutating command is safe to repeat, subject to a deliberately changed source. For `config init`, repeating against an existing file reports that it already exists rather than overwriting it. `account link` is the explicit authorization to adopt the selected record and discard mappings if it differs; no second interactive prompt is required. If capture or save fails, neither the old reference nor its mappings change.

Ordinary settings and model mappings are edited in the JSON file. Stop the gateway before editing. A running process uses the validated settings snapshot it loaded at startup; a manual editor does not participate in the command lock, and edits take effect only after restart. Help explains this limitation. There is no file watcher, automatic reload, settings setter API, or model discovery command in this feature.

### File safety and upgrades

Create the gateway directory with mode `0700`, and its configuration and stable `.lock` file with mode `0600`, owned by the current effective user. Reject existing gateway files or directories with a different owner, wider access, symlinks, or unexpected file types. Validate opened descriptors, not only a pathname before opening. Do not follow a configuration or lock symlink or automatically chmod existing paths. Parent directories need not be private but must not permit another user to replace the gateway directory. Use scoped file operations anchored to validated directory descriptors. (basis: installed file security guidance)

Use a nonblocking exclusive macOS `flock` on the stable `.lock` file. `serve` holds it through shutdown and connection cleanup. Every command that could write holds it from before reading the configuration until all persistence work ends. On conflict, fail immediately with `Stop the running gateway or wait for the other configuration command, then retry.` Never unlink or replace the lock file as routine cleanup; its empty file may remain after exit, while the operating system releases its lock on process termination. No PID or diagnostic history is stored in it.

`serve` and mutating commands may create the private directory and lock file even when the configuration is absent. `config check`, help, and `version` do not create them. A readonly validation of an atomically replaced file sees one complete document and does not need the process lock.

Serialize a validated complete document to an unpredictable temporary file in the same private directory, using `os.CreateTemp` and mode `0600`. Check every write, sync and close the file before installing it. For `config init`, atomically create `config.json` as a hard link to the temporary file, then unlink the temporary name. The link operation must fail if the destination already exists, including a file created after an earlier existence check. Do not implement initialization as an existence check followed by a replacing rename. For mutations of an existing configuration, atomically rename the temporary file over `config.json`. Use operations scoped to the validated directory, and sync the directory after installation and temporary name cleanup.

Clean up the temporary file on ordinary failures. An interrupted write may leave a private temporary file containing only configuration metadata; a later successful mutator removes only recognizable gateway temporary files after validating ownership and type. Do not keep automatic backups, previous session references, or a settings history. User created editor backups remain outside gateway management.

A failure before the destination link or rename leaves the prior configuration intact. A failure after either installation operation may mean the new document is already visible; report an uncertain save outcome, advise `config check`, and do not claim rollback or blindly restore old data. This includes failure to remove the temporary name after a successful initialization link. Crash recovery accepts either complete version, or absence for an initialization not yet installed, and never a partial JSON file. Cooperative commands obey the lock; a same user editor can still race a replacing mutation, so documentation asks you to finish manual edits before running mutations. Initialization's refusal to overwrite does not depend on that cooperation.

There is no existing persisted schema to migrate. Version 1 is the first schema, and the current upgrade command is an explicit no op for that version. Missing or unknown versions, including version 0 and newer versions, fail unchanged. Any future schema change must define and test its migration and recovery behavior before that version is supported. Startup never guesses a migration or silently drops fields.

### Value sourcing

| Action | Value produced or used | Source |
|---|---|---|
| Locate settings and source | Current home | `os.UserHomeDir`, injected for tests; never a request field. |
| Create configuration | Version and defaults | Constants and initial document in this spec. |
| Serve | Effective listen address | Explicit `--listen`, else saved `listen`, else the default; validate the saved document before applying overrides. |
| Authenticate HTTP | Local gateway credential | Existing `KIRO_GATEWAY_TOKEN` rules from spec 0001. |
| Link session | Source identifier | Fixed provider constant `kiro_cli_idc_sqlite_v1`. |
| Link session | Fingerprint | Exact selected `auth_kv.value` bytes and the digest formula above. |
| Inspect selected record | Region, sign in URL, expiry | `region`, `start_url`, and `expires_at` in that same record; not `whoami`, guessed defaults, or saved duplicates. |
| Reject expiry | Current time | Injected wall clock, `time.Now` in production. |
| Save mapping | Client name and target ID | Your explicit JSON entries. Runtime resolution and upstream availability proof belong to feature 4. |
| Link or forget | Whether mappings are cleared | Comparison with the prior saved source and fingerprint, or explicit forget. |
| Emit command result | Outcome and error category | Validated command state and bounded filesystem or provider result; fixed English messages. |
| Health and request logs | Version, request ID, elapsed time, status | Existing foundation implementation; this feature adds no account readiness assertion. |

### Security, diagnostics, and failure recovery

The gateway token keeps its existing minimum length and visible ASCII validation. It is never copied into the JSON document, accepted as a command argument, or printed. Kiro's real raw credential record is neither copied into a gateway file nor included in fixtures; tests use synthetic records. The source reader follows the path and ownership rules above and does not change Kiro's permissions. A user who can act as this same macOS user is within the local operating authority; this design does not promise protection from that user or root.

Log fixed event, outcome, elapsed time, and sanitized error category fields through `slog` on stderr. Preserve the foundation's request fields. CLI mutations emit a local completion event without values from the saved record. Never log input documents, environment values, SQL error strings, Kiro output, fingerprints, sign in URLs, region, file contents, or identifiers from the account. There is no file log, log retention setting, telemetry, or saved conversation. Shell redirection chosen by you is outside gateway retention. (basis: accepted architecture, your retention choice, installed logging and secrets guidance)

| Failure | Behavior and recovery |
|---|---|
| Missing configuration | Serving and checking use defaults. Linking or upgrading asks you to run `config init`. Forget is a no op. |
| Invalid configuration or unsupported schema | Refuse operation, preserve bytes, identify the fixed failure category, and suggest editing the file or using a compatible gateway version. Never bypass it with flags. |
| Unsafe gateway permissions | Refuse use; help shows the fixed directory and file modes to apply yourself. Do not print a shell command containing untrusted input. |
| Missing, malformed, or unsupported Kiro record | Link fails unchanged. Explain that the saved IAM Identity Center source is unavailable or unsupported. No fallback account selection. |
| Expired Kiro credential | Link fails unchanged. Ask you to sign in through Kiro CLI, then link again. No refresh command is invoked. |
| Busy source, deadline, or cancellation | Close the operation and preserve settings. Distinguish busy, timeout, and cancellation categories; no automatic retry. |
| Disk full or permission failure during save | Preserve the old document before installation; apply the uncertain outcome rule after a successful destination link or rename. Never report a successful mutation prematurely. |
| Record changes after linking | Do not rewrite the reference automatically. Future inference must fail comparison and require explicit relinking. |

### Code ownership and verification

`internal/cli` owns commands and dependency wiring. Add `internal/config` for the typed document, parsing, validation, and pure state changes. Add `internal/configstore` for file access, replacement, and process locking. Add `internal/credentials` when the capture command first needs its SQLite reader. These are working packages, not empty placeholders. Adapters depend on the inner configuration types; inner logic imports neither `database/sql` nor HTTP. Keep small storage and capture interfaces at the CLI consumer. The capture interface returns only a typed session reference and an error. `internal/gateway` remains unaware of configuration files and Kiro credentials.

Use standard `testing` and `httptest`, with synthetic SQLite databases and temporary homes. Do not open the operator's real store in unit, integration, CI, or ordinary verification tests. The source shape observations in the rationale are design evidence, not executable fixtures.

| Critical scenario | Contract verified |
|---|---|
| Initialize, edit the listen address, run `serve`, authenticate `/healthz`, restart, and observe saved settings; exercise flag precedence and absent file | AC-1, AC-2 |
| Reject unknown or duplicate members, case variants, invalid UTF 8, trailing JSON, unsupported version, oversize input, invalid loopback, and invalid model ownership | AC-2, AC-5, AC-7 |
| Capture a synthetic unexpired record; independently compute the digest from its exact bytes; mutate only whitespace, expiry, region, or a token and observe a changed digest | AC-3 |
| Reject a record above 64 KiB before scanning its value; prove metadata and value reads use one transaction, and test accepted additive columns against rejected key or column changes | AC-3, AC-9 |
| Read from a temporary home containing URI metacharacters; reject source symlinks and unsafe parents without changing or selecting a different store | AC-3, AC-9 |
| Relink unchanged and changed fixtures; forget twice; prove mapping cleanup is atomic and the synthetic source's rows and schema remain unchanged | AC-3, AC-4, AC-5 |
| Hold a source lock, cancel a read, use rollback and WAL fixtures, omit the record, corrupt its JSON, and supply expired credentials; bound completion and preserve settings | AC-3, AC-9 |
| Run separate gateway and mutator processes against one temporary home; prove lock exclusion and release after process termination | AC-6 |
| Exercise symlinks, broad permissions, temporary write failure, failures before installation and after link or rename, and a competing file creation during init; verify complete documents, no overwrite by init, and truthful outcomes | AC-6, AC-7 |
| Check version 1 without rewriting, reject unknown versions, and prove configuration validation never invokes a credential source | AC-2, AC-7 |
| Use distinct sentinel secrets, account metadata, and conversation strings; assert they are absent from stdout, stderr, and all gateway managed files | AC-8 |
| Run the repository check script and native build with bundled SQLite and no runtime `sqlite3` command | AC-9 |

## Build plan

Follow the project's Tracer Bullet approach, proving one usable path before adding breadth.

1. **Saved settings through the real health server.** Add the version 1 document, validation, private file store, atomic initialization, and process lock. Wire `config init`, `config check`, and `serve` so a changed listen address reaches a real authenticated health request and survives restart. Carry the existing token and diagnostic rules through this thread. Satisfies **AC-1**, **AC-2**, **AC-5**, **AC-6**, **AC-8**.
2. **Explicit session capture and removal.** Add the pinned SQLite dependency and capture adapter using synthetic fixtures first. Wire `account link` and `account forget`, exact record fingerprinting, expiry validation, one second busy handling, and atomic mapping cleanup. Add no inference or renewal layer. Satisfies **AC-3**, **AC-4**, **AC-5**, **AC-6**, **AC-8**, **AC-9**.
3. **Upgrade and recovery contract.** Add `config upgrade`, complete schema and permission failure handling, and verify interruption, lock contention, byte preserving no ops, and uncertain save outcomes. Document manual edit and restart behavior. Satisfies **AC-2**, **AC-6**, **AC-7**, **AC-8**.
4. **Verification and handoff.** Complete the critical scenarios, run `rtk proxy ./scripts/check`, and document setup, removal semantics, cgo builds, zero retained diagnostics, and the limits of session pinning. Apply the scope's GA verification, separate model review, and documentation workflow. Record dependency version and native build evidence. Satisfies **AC-1** through **AC-9**.

The only initial schema creation is the version 1 configuration document in the first thread. This plan makes no migration or write to Kiro's database.

## Consequences

**Positive**: Saved settings remain inspectable, changes are explicit, and the gateway does not own a copy of Kiro's secrets. Exact snapshot pinning prevents silently substituting a different credential record.

**Tradeoffs**: Any changed record requires relinking and rebuilding model mappings, including normal refreshes or harmless JSON formatting changes. The private SQLite schema can change between Kiro releases. cgo adds a build requirement, and command locking means you stop serving before making managed changes. No saved diagnostic history is available after terminal output is lost.

**Limits**: A fingerprint identifies a snapshot, not a person, account permission, revocation state, or usable inference interface. Forgetting the reference neither logs out Kiro CLI nor securely erases backups made by other software. Configuration completion does not establish live Claude Code compatibility.

## Follow-up

1. Scope feature 4 owns live inference, destination restrictions, exact model availability, and the authenticated snapshot comparison contract above. Prove those before claiming account or protocol readiness.
2. Scope feature 5 owns stable identity investigation, renewal, and any replacement for strict snapshot pinning. A stable identity source must be demonstrated before reducing the relink requirement.
3. Independent review of this design is complete. The scope's GA workflow still calls for verification, tests, review, and documentation of the implementation.
4. At implementation completion, durable context needs the new packages, cgo dependency, configuration location, and removal semantics. `/sync` owns those edits; this spec does not change `AGENTS.md`.
5. Record your tooling declines in durable context when it is next maintained: `eduardo-sl/go-agent-skills` skills `go-database` and `go-context`, and `liliang-cn/mcp-sqlite-server`. No tools were installed or connected; do not offer these again without a new need or your request.

## Rationale

Reasoning, alternatives, source observations, and verified references: see [rationale.md](rationale.md).
