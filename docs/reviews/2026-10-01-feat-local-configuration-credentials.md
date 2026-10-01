# Review, feat/local-configuration-credentials, 2026-10-01

**Reviewed by**: GPT 6 Sol (author on GPT 6 Astra)
**Scope**: Configuration and credential code within 27 changed files, branch versus main
**Verdict**: Approve with nits

## Summary

The change adds a validated settings document, atomic local storage, explicit SQLite session capture, and CLI commands that keep credentials out of gateway files. The implementation follows the accepted data model and has strong tests for snapshot consistency, locking, recovery, and sanitized output. I found one minor error reporting issue in initialization. I did not find an unsafe file access or overwrite in that path.

## Minor

### 🟡 Initialization misclassifies an unsafe existing file, `internal/configstore/store.go:176`

**Problem**: If `config.json` already exists as a symlink or another unsafe file type, the nonreplacing link fails with `os.ErrExist` and `Save` returns `ErrExists` without validating the destination. A temporary home with a dangling `config.json` symlink reproduced the CLI message `configuration already exists; use config check` and the log category `local_command`. The symlink was left untouched.

**Why it matters**: The message directs the operator to check a file that cannot pass validation, instead of identifying the unsafe path and its required repair. This misses the spec's actionable error requirement for unsafe file types.

**Suggested fix**: After the failed initialization link, inspect the existing destination through the safe path validator. Return `ErrUnsafe` when it is unsafe, while preserving `ErrExists` for a regular private configuration. Add a test for initialization with an existing symlink or nonregular destination.

## Strengths

- The saved document rejects duplicate JSON members, unsupported versions, invalid model ownership, and unsafe listener addresses before serving.
- Capture checks the exact selected SQLite schema and record inside one transaction, and tests cover rollback and WAL snapshots, busy reads, expiry, and byte exact fingerprints.
- File replacement, process locking, and failure outcomes have focused tests, including real separate process behavior.

## Test coverage

The configuration, store, credential, CLI, and process tests cover the main success and failure paths with synthetic data and isolated homes. The existing unsafe destination case is not covered for `config init`. The repository check passed formatting, vet, build, and all race tests in the approved environment. A separate targeted CLI reproduction confirmed the minor finding.
