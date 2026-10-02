# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

* Saved version 1 settings at `~/.config/kiro-gateway/config.json`, with a loopback listener, one session reference, and up to 32 exact client name to upstream model ID mappings (see [spec 0002](docs/specs/0002-local-configuration-credentials/index.md)).
* `config init`, `config check`, and `config upgrade` let you create and validate settings without a gateway token. Initialization preserves existing files. Upgrade leaves version 1 unchanged and rejects unknown versions without modification. Validation rejects oversized files, unknown or duplicate members, invalid UTF 8, and invalid field values without echoing file contents.
* `account link` reads your existing unexpired IAM Identity Center record from Kiro CLI's local SQLite store and saves only its source identifier and SHA 256 fingerprint. Linking identical bytes preserves settings exactly. Linking changed bytes, including token renewal or JSON formatting changes, clears all model mappings. Linking does not refresh credentials or establish live inference compatibility.
* `account forget` removes the session reference and all model mappings. You can repeat it safely. Kiro CLI credentials remain intact.
* Private configuration storage with ownership, permission, and symlink checks, plus atomic saves that install a complete file. Failures before installation preserve the previous file. Errors after installation report an uncertain save outcome so you can check settings before retrying.
* Local mutation diagnostics report outcome, elapsed time, and sanitized error categories on stderr. The gateway retains no diagnostic files, credential copies, or conversation content.
* A test only protocol feasibility harness covers streamed text, a tool request, a matching synthetic tool result, a followup turn, cancellation, and interruption at 256 bytes. You can exercise it with synthetic credentials and local TLS servers. Live runs require separate review and explicit launch controls (see [spec 0003](docs/specs/0003-first-claude-code-loop/index.md)).
* Combined token and selected profile snapshots let the harness read both records in one SQLite transaction, route by the profile ARN region, and stop if the linked token or pinned profile bytes change during a run. Credentials and profile pins stay in memory.
* Feasibility runs have a six attempt ceiling, time and byte limits, fixed regional destinations, and sanitized summaries. The harness holds the configuration lock and makes no automatic retry, credential renewal, or model fallback.
* Recorded all six live feasibility cases as observed on October 1, 2026, at commit `4fd0b70`, requesting `claude-opus-5.5`, with verdict `limited_candidate_observed` (see the [verification record](docs/specs/0003-first-claude-code-loop/verify.md#approved-six-case-proof-passed-october-1-2026)). Instructions remain in user content, stream end permits only tentative completion, and resolved model identity, token usage, and output token limits remain unverified. The gateway inference endpoint and a real Claude Code coding task remain pending.

### Changed

* `serve` uses the saved listener unless you supply `--listen`. Invalid saved settings block startup even with an override. Serving and settings mutations hold an exclusive process lock, so you can stop the server before changing settings and restart it to apply edits.
* Builds now require `CGO_ENABLED=1` and a C compiler for `github.com/mattn/go-sqlite3 v1.14.50`, which bundles SQLite. No external SQLite executable is required.

### Fixed

* The feasibility harness now applies its inactivity deadline to DNS resolution and connection setup. Preflight commands also bound pipe cleanup and reject expired contexts, so stalled connections or version checks cannot silently bypass those limits.
