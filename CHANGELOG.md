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

### Changed

* `serve` uses the saved listener unless you supply `--listen`. Invalid saved settings block startup even with an override. Serving and settings mutations hold an exclusive process lock, so you can stop the server before changing settings and restart it to apply edits.
* Builds now require `CGO_ENABLED=1` and a C compiler for `github.com/mattn/go-sqlite3 v1.14.50`, which bundles SQLite. No external SQLite executable is required.
