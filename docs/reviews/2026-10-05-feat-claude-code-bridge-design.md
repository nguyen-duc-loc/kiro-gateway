# Review, feat/claude-code-bridge-design, 2026-10-05

**Reviewed by**: GPT-6.1 Sol (author on GPT-6 Astra)
**Scope**: 10 changed files, branch versus main at `816d310b7f33af26f1f7e061ab627edf965bbf52`, focused on the model schema investigation
**Verdict**: Changes requested

## Summary

The probe makes one bounded catalogue request through the existing combined snapshot and transport boundaries, then retains a finite structural projection. Request construction, extraction, redaction, cancellation, and cleanup agree with AC-16 in the inspected paths. The remaining finding is a gap in deterministic coverage of the launch preflight, rather than a demonstrated authorization bypass or runtime defect.

The full branch inventory contains 71 changed files. This review covers the five schema Go files, `SCHEMA_PROBE.md`, `scripts/check`, and the scope, governing spec, and verification updates, including the current uncommitted tests. I also traced the shared Git preflight, configuration locking, credential snapshot, and transport implementations. Earlier bridge and repair work retains its October 2 and October 4 reviews; this assessment does not repeat that entire review or reassess the native source rationale.

## Major

### 🟠 Exercise the complete launch artifact gate offline, `internal/kiro/schema_launch_test.go:175`

**Problem**: No deterministic test invokes `schemaLivePreflight`. Every synthetic runner replaces it with a stub, and `TestSchemaProbe` skips before calling it during ordinary checks. The exact proposed command comparison at line 175 and the comparison of actual software bytes with their reviewed digests at line 180 consequently have no coverage. `TestSchemaPlanGate` accepts its valid fixture with `Command` unset because `readSchemaPlan` does not enforce that field, while the file hash tests exercise hashing without the manifest comparison.

**Why it matters**: AC-16 requires the concrete reviewed plan and clean code to match before account access. The verification checklist specifically requires agreement between the proposed command and native file hashes. Removing either of these comparisons would leave the current suite green. This is an uncovered security relevant branch under the configured review rubric, not evidence that the current comparisons are incorrect. A later manual review of one concrete plan does not exercise rejection of a changed command or changed software bytes.

**Suggested fix**: Make the local preflight inputs testable with temporary plan and software files plus controlled Git and environment inputs. Exercise a complete matching artifact and failures for a changed proposed command, mismatched software bytes, stale plan digest, changed or dirty code, and cancellation. Feed the resulting preflight through the synthetic runner and assert that each rejection returns `preflight` before home lookup, settings opening, snapshot access, or dispatch. Preserve the disabled real launch entry point throughout these tests.

## Strengths

* The management host policy stays confined to the tagged test binary. Synthetic TLS checks prove the exact request, absence of proxy and redirect behavior, one snapshot, one dispatch, and the settings lock held through response cleanup.
* The extractor distinguishes unknown paths from explicit absence, discards partial fields after an interpretation failure, and retains only fixed paths and permitted values. Tests exercise exact and exceeded limits, duplicate keys, references, pagination, and credential sentinels.
* The cleanup tests use a controlled clock to establish the 25 second work cutoff and 30 second total cutoff, preserve the first failure, and require one connection close.

## Test coverage

I used the check review rubric and the Go testing and Go security skill guidance. The first `scripts/check` attempt passed formatting, vet, and build, then failed because the sandbox denied ephemeral loopback listeners. The approved rerun, `rtk proxy env GOPROXY=off ./scripts/check`, passed formatting, vet, build, all ordinary race tests, and the tagged schema race suite with launch forced off.

The separate tagged race suite also passed with `-count=1` and `-schema-probe-launch=false`. Running only `TestSchemaProbe` with inherited schema and live launch environment variables, but no explicit launch flag, skipped before preflight. These checks used synthetic homes, SQLite records, and local TLS. I made no live request, accessed no operator credential store, and did not execute an installed client or native agent. The concrete live plan remains pending; no GA gate is established by this review.
