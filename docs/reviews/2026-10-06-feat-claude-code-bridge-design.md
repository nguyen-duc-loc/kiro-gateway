# Review, feat/claude-code-bridge-design, 2026-10-06

**Reviewed by**: gpt-6.1-sol (author on astra)
**Scope**: 21 files, branch vs main, limited to CreateResponse discovery and affected shared paths
**Verdict**: Approve

## Summary

The change adds a development probe for one fixed CreateResponse request hypothesis, with a bounded structural report and a separately gated live entry point. The implementation follows the accepted AC-17 request, source, framing, retention, cancellation, and cleanup rules. No blocker, major, minor, or nit was found in the reviewed slice. This verdict covers offline discovery preparation; it does not approve a live invocation, GA promotion, or unrelated earlier branch changes.

The branch base is `main`, with merge base `816d310b7f33af26f1f7e061ab627edf965bbf52`. Substantive review used the discovery changes after `6f30d76` through HEAD `a94c5a6`, including the current working tree changes in `response_checks_test.go`, `response_failure_checks_test.go`, and `docs/scope/scope.md`. Earlier bridge and catalogue work was consulted only where discovery consumes its interfaces or moves its helpers.

## Strengths

- The complete synthetic path uses actual plan preflight, temporary settings and SQLite, verified local TLS, and independent assertions of the fixed body and headers. It checks one combined snapshot, one dispatch, both permitted regions, unchanged settings bytes, and lock release.
- Observation retains only fixed labels and path kinds. JSON validation covers discarded fields, SSE continues after `[DONE]`, and EventStream validates lengths, every header, and both CRC checks without adopting production inference semantics.
- Cleanup ownership is explicit. One worker owns connection and body close; lock release begins only after transport cleanup succeeds. Disposable process tests cover permanently blocked connection close, lock release, and report output while preserving the first failure and withholding the summary.
- The real entry point defaults to disabled and requires a matching clean commit, plan digest, compiled contract, software hashes, dedicated test selection, and approval field before settings or account access. Repository checks clear inherited `GOFLAGS` and force both probe launch flags off.

## Test coverage

The changed tests cover the actual preflight path and its rejection branches, exact request construction, finite projection and ordering, fragmented streams, invalid and excessive framing, informational response rejection, body truncation, source and region failures, cancellation boundaries, work and source deadlines, failure precedence, cleanup failures, report overflow, and disabled launch. The shared JSON validator and software hashing helper were moved without behavior changes, and the existing catalogue suite still exercises them. No material uncovered branch was identified in the reviewed discovery logic.

The coordinating reviewer ran `rtk proxy ./scripts/check` against this working tree and reported exit 0 with all checks passing: formatting, ordinary vet and build, the complete race suite, tagged catalogue vet and race tests, and tagged discovery vet and race tests. The discovery suite completed in 9.355 seconds and the catalogue suite in 4.328 seconds. An initial sandbox restriction on local listeners was resolved for the synthetic checks. This review did not duplicate that complete run or introduce new tests.

The recorded offline verification also addresses the deliberately conservative unread body close behavior: a close error after unsupported media preserves the original `response_format` cause, reports failed cleanup, clears observations, and follows the dedicated process exit rule. That behavior agrees with the child contract.

No live probe, real credential store, native client, or external service was accessed during this review. Synthetic results establish local behavior only; actual service binding, response grammar, instruction semantics, serving identity, usage, and controls remain unverified. AC-14 and G1 through G4 remain open, and concrete launch preparation and explicit authorization remain separate steps.
