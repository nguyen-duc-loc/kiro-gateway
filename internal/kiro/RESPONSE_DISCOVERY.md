# CreateResponse discovery

This development probe implements the accepted [AC-17 contract](../../docs/specs/0004-claude-code-bridge/0004-createresponse-discovery.md). It tests one fixed AWS JSON request hypothesis. A structural observation does not establish inference success, supported controls, instruction priority, or any GA guarantee.

The implementation lives only in test files under the `responsediscovery` build tag. The product executable has no discovery entry point. `TestResponseDiscoveryProbe` skips before preflight, home lookup, settings, credentials, or networking unless its explicit launch flag is true. Environment variables cannot enable it.

## Offline checks

You can run the complete repository checks with:

```sh
rtk proxy ./scripts/check
```

You can run the discovery checks alone with:

```sh
rtk proxy env GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=1 GOFLAGS= go vet -mod=readonly -tags=responsediscovery ./internal/kiro
rtk proxy env GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=1 GOFLAGS= go test -mod=readonly -race -tags=responsediscovery ./internal/kiro -run '^TestResponseDiscovery' -args -response-discovery-launch=false
```

These tests use temporary settings, synthetic SQLite records, local TLS servers, inert software files, and temporary Git repositories. The tests for permanently blocked cleanup and output use disposable subprocesses. They never put a permanently blocked close in the ordinary test process.

The suite covers artifact preflight, exact request bytes and headers, one snapshot and dispatch, JSON, SSE, EventStream, finite projection, cancellation, failure precedence, limits, cleanup, and the disabled entry point. `scripts/check` clears inherited `GOFLAGS` and explicitly forces both development probe launch flags off in their tagged suites.

## Ownership and retained evidence

The runner holds the settings lock through transport cleanup. One worker owns connection close and then body close. The runner waits for that worker before releasing the lock and publishing an ordinary report. Lock release has a separate worker joined within the same cleanup deadline. A failed close or expired cleanup allowance clears the body summary and preserves the first failure category. The dedicated live test process then allows at most one second for the failure report and exits with status 1. A blocked or failed output may leave no complete report.

The body is bounded at 256 KiB plus one detection byte. Decoders validate discarded fields too. The report keeps only fixed labels, literal path kinds, local counts, and approved provenance. It saves no returned text, arbitrary keys, model names, account metadata, token counts, or upstream identifiers. It reads through the body end even after a candidate completion marker. A non 200 response may retain a fully validated structural summary, while keeping `http_status` as its failure category.

The generic JSON validator and bounded software hashing helper are shared with the catalogue probe in `discovery_shared_test.go`. The product transport and semantic decoder are unchanged.

## Later launch preparation

No live launch is authorized by these files. Independent implementation review and a concrete plan are still separate steps under AC-17. The scope milestone for the final plan remains open until that review is complete.

The version 1 plan has `version`, `code_commit`, `contract`, `software`, `synthetic_checks`, `independent_review`, `command`, and `live_approval`. `responseContract` supplies the exact compiled contract for one selected region. The plan must include the fixed request template, headers, paths, vocabularies, budgets, and policies. Its software entries are `native_binary` and `bundled_source`, with absolute paths and SHA256 digests. Hashing never executes either file. The two required check labels are `scripts/check:pass` and `response_discovery_race:pass`.

The final plan binds a clean code commit and its exact command tokens. The command selects only `^TestResponseDiscoveryProbe$`, uses count 1, and supplies `-response-discovery-plan`, `-response-discovery-plan-sha256`, and `-response-discovery-code-commit`. The recorded command uses `<PLAN_SHA256>` to avoid self hashing. The actual command supplies the digest of the final plan bytes.

During preparation, `live_approval` stays null. Only a separately authorized invocation may use `approved_for_one_response_discovery_run`. That approval is consumed even when preflight or source access stops before dispatch. No approved plan is included here. Every resulting observation returns to architecture, with G1 through G4 still open.
