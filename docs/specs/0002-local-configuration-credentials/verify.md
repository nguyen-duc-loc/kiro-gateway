# Verify: Local configuration and credential data model · spec 0002 · updated 2026-10-01

_Steps derived from spec 0002 acceptance criteria and every row of its Value sourcing table. You can run `/check verify` with this checklist, then use `/test` to strengthen the durable checks._

## Commands and behavior

You can use temporary homes and synthetic records for every step. No step needs your real Kiro database. Leave these boxes open until the separate verification pass records its results.

1. [ ] Set an isolated home, including a directory name containing `?`, `#`, and `%`. Initialize and capture fixtures at the fixed paths. Set a different working directory and `XDG_CONFIG_HOME`; confirm they do not select another settings file or source. Reject symlinks in appended paths, including relative symlinks. Value source: current home. AC-2, AC-3, AC-6.
2. [ ] Run `config check` without a file and confirm defaults without directory creation. Run `config init` and inspect version 1, `127.0.0.1:8787`, null session, empty mappings, trailing newline, directory mode `0700`, and file modes `0600`. Repeat initialization and confirm no overwrite. Value source: specified constants. AC-2, AC-6.
3. [ ] Run the health server with absent settings, then save `127.0.0.1:0`, serve, authenticate, stop, and restart. Occupy the saved port and supply an explicit ephemeral listener. Confirm flag precedence. Supply invalid saved settings with a valid flag and confirm startup fails. Value source: flag, saved listen, or default in that order. AC-1, AC-7.
4. [ ] Use a synthetic `KIRO_GATEWAY_TOKEN` for health requests. Missing and incorrect tokens return 401. A token passed as a flag or saved JSON member fails. Value source: the environment only. AC-1, AC-2, AC-8.
5. [ ] Link a synthetic unexpired record under the exact key `kirocli:odic:token`. Confirm the saved source is `kiro_cli_idc_sqlite_v1`. A differently cased key or another record is not selected. Value source: fixed provider constant and selected key. AC-3.
6. [ ] Independently compute SHA 256 over `kiro-gateway/kiro_cli_idc_sqlite_v1`, one zero byte, and the exact stored value bytes. Compare the lowercase digest. Change whitespace, token, expiry, and region independently. Each changed snapshot changes the digest. Value source: exact selected record bytes. AC-3.
7. [ ] Vary `region`, `start_url`, and `expires_at` in one fixture. Reject missing or invalid fields, HTTP URLs, embedded URL credentials, duplicate JSON members, and invalid UTF 8. Confirm metadata does not come from another row. Value source: fields in the same selected snapshot. AC-3.
8. [ ] Set the injected clock before, at, and after expiry. Only the first capture succeeds. Confirm production uses the wall clock and does not cache expiry in settings. Value source: injected current time. AC-3.
9. [ ] Edit model names and IDs in the JSON. Preserve exact case and allow several client names to share one ID. Check empty mappings, 32 mappings, and 256 byte identifiers. Reject 33 mappings, spaces, nulls, unknown or duplicate fields, invalid loopback, unsupported schema, trailing data, oversize input, and mappings without a session. Value source: explicit JSON entries. AC-2, AC-5, AC-7.
10. [ ] Relink unchanged bytes and confirm the settings file is unchanged. Relink a different usable record and confirm all mappings clear in the same replacement. Forget twice and confirm only the reference and mappings disappear. Failed capture preserves the old settings. Confirm the fixture rows, schema, and journal mode remain unchanged. Value source: reference comparison or explicit forget. AC-3, AC-4, AC-5, AC-6.
11. [ ] Exercise missing source, expired credentials, a source lock, parent cancellation, timeout, unsupported schema, and unsafe file types, ownership, or permissions. Check the fixed error categories. Hold the gateway process lock and try each mutator; terminate the process and try again. Inject write, sync, install, and cleanup failures. Confirm preinstallation preservation, uncertain outcomes after installation, no overwrite during competing initialization, and cleanup limited to recognizable private temporary files. Value source: validated operation outcome. AC-3, AC-6, AC-7, AC-8.
12. [ ] Inspect real health output and request diagnostics for build version, request ID, elapsed time, and status. Search stdout, stderr, and gateway managed files for distinct sentinel tokens, fingerprints, account metadata, and conversation strings. Fingerprints may exist only in the active configuration reference and disappear after forget. Confirm no diagnostic or conversation store. Value source: existing process and request fields. AC-8.
13. [ ] Run `config upgrade` on a current file and compare exact bytes. Run it on missing, version 0, and newer version files and confirm refusal without modification. Run `config check` and `serve` with the Kiro source unavailable and confirm no source access is needed. AC-2, AC-7.
14. [ ] Run `rtk proxy ./scripts/check`, then `rtk proxy env CGO_ENABLED=1 go build -mod=readonly -o bin/kiro-gateway ./cmd/kiro-gateway` on native macOS. Check rollback and WAL fixtures, one transaction across metadata and value reads, size rejection before value loading, accepted additive columns, rejected changed key contracts, and resistance to a table shadowing metadata inspection. No external `sqlite3` command or live credentials are needed. AC-3, AC-9.

## Acceptance criteria coverage

| Criterion | Steps |
| --- | --- |
| AC-1 | 3, 4 |
| AC-2 | 1, 2, 4, 9, 13 |
| AC-3 | 1, 5, 6, 7, 8, 10, 11, 14 |
| AC-4 | 10 |
| AC-5 | 9, 10 |
| AC-6 | 1, 2, 10, 11 |
| AC-7 | 3, 9, 11, 13 |
| AC-8 | 4, 11, 12 |
| AC-9 | 14 |

## Build evidence

The implementation self check passed `scripts/check` on October 1, 2026, using Go 1.27.1, macOS arm64, cgo enabled, and `github.com/mattn/go-sqlite3 v1.14.50`. The script checked formatting, ran vet, compiled all packages, and ran unit and process integration tests with the race detector. Synthetic tests covered rollback and WAL, snapshot consistency, busy reads, URI escaping, absolute and relative symlink rejection, settings transactions, restart behavior, and process lock release. This records the builder's evidence; the separate verification and model review remain pending. Live inference compatibility is outside this feature.
