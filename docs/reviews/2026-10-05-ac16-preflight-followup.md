# Review, feat/claude-code-bridge-design, 2026-10-05

**Reviewed by**: GPT-6 Sol (author on GPT-6 Astra)
**Scope**: 7 files, focused AC-16 schema implementation versus main at `816d310b7f33af26f1f7e061ab627edf965bbf52`, including uncommitted and untracked preflight tests
**Verdict**: Approve

## Summary

The new deterministic tests close the October 5 review's Major preflight coverage finding. A matching synthetic plan passes the actual `schemaLivePreflight` and reaches one TLS catalogue exchange; isolated mutations to the proposed command, both pinned software files, plan digest, commit, working tree, context, and build environment all fail before home lookup, settings, snapshot, or dispatch. The local software hash ceiling now accommodates the pinned installed binary while retaining a finite limit and full byte digest.

This focused followup reviewed the six schema test files and `scripts/check`, and traced the shared Git preflight and transport used by the probe. It does not reopen the earlier full bridge reviews or claim live catalogue evidence.

## Strengths

- The positive preflight test exercises the real artifact gate through a clean temporary Git repository and inert files, then continues through synthetic settings, SQLite, and local TLS. The negative tests keep the other inputs valid, so deleting the command or software digest comparison would make the respective test fail.
- Rejections assert the `preflight` category and zero home, store, reader, and dial calls. The real launch flag stays disabled in ordinary checks, including the tagged race suite.
- The 2 GiB software bound is confined to provenance hashing in the tagged probe. A sparse synthetic file above the old 1 GiB ceiling verifies a complete digest including a trailing nonzero byte; a 2 GiB plus one byte file is rejected. Response and report limits remain unchanged.

## Test coverage

I applied the check review rubric and Go testing and Go security guidance. My first focused race run could not bind the synthetic loopback TLS server under the filesystem sandbox. The same focused command passed with ephemeral loopback access: `go test -mod=readonly -race -tags=schemaprobe ./internal/kiro -run '^TestSchema(LivePreflight|HashFile)' -count=1 -args -schema-probe-launch=false`. The main thread also reported a passing uncached full tagged schema race suite and `scripts/check` after the hash fix. These checks used synthetic data only; I did not access an operator credential store, execute installed software, or make a live request.

The preflight comparisons are now covered at the actual integration boundary. A later live run still requires a separately reviewed exact plan and explicit authorization under AC-16; this approval concerns offline code readiness only.
