# Model schema probe

This development probe implements spec 0004 AC-16. You can use it to prepare a later catalogue investigation. It is excluded from the product binary. Offline preparation does not authorize account access or a management request. G1 through G4 remain open.

You can run the synthetic checks with:

```sh
rtk proxy env GOTOOLCHAIN=local GOWORK=off CGO_ENABLED=1 GOPROXY=off go test -mod=readonly -race -tags=schemaprobe ./internal/kiro -run '^TestSchema' -count=1 -args -schema-probe-launch=false
```

`scripts/check` includes the same tagged suite with launch forced off. The checks use temporary homes, invented SQLite records, and local TLS. `TestSchemaProbe` skips by default, even if an environment variable requests a launch. No native agent or SDK is executed.

The runner uses one combined snapshot, the exclusive existing settings lock, and one HTTP exchange. Its separate test policy accepts only the two specified management hosts. The product transport still accepts only its runtime hosts. Work stops at 25 seconds from launch acceptance. Connection cleanup has the remaining five seconds. Failures do not retry, follow pagination, refresh credentials, or modify settings.

The decoder bounds the response before reading it, rejects duplicate JSON keys throughout the page, and inspects only the eight fixed property paths. A reference or an uninspectable properties map yields an unknown path. Explicit absence requires an inspectable properties map without that component. A returned schema is a structural observation, never a service guarantee.

`schema_extract_test.go` defines the report and decoder. `schema_runner_test.go` owns settings, source access, request construction, and cleanup. `schema_launch_test.go` owns the disabled launch gate and artifact checks. The two check files cover the complete synthetic path and failures.

## Later live review

No concrete live plan is prepared or approved by this offline build. The governing spec places that step after independent code review. You can use `/check verify first real Claude Code coding loop: model schema investigation, offline only` for behavior verification, then the scope's test and independent review steps.

After review, the concrete plan can use the `schemaPlan` format and the exact `schemaContract()` constants. It binds the clean code commit, both destinations, request and extraction rules, native binary and bundled source SHA256 values, synthetic check results, independent review reference, and proposed command. The native files are hashed, never executed. The `live_approval` slot stays null while the operator reviews the final artifact.

The later proposed command must match the token list checked by `schemaLivePreflight`. The plan uses `<PLAN_SHA256>` in that list to avoid hashing itself. The actual command supplies the digest of the final reviewed bytes. A plan outside the checkout avoids changing the clean code commit. Empty provenance, failed checks, changed plan bytes, dirty code, missing review, and an empty approval slot fail before account access. The final launch also needs the explicit `-schema-probe-launch` flag. Earlier bridge approvals do not apply.

The only retained runtime output is a JSON report bounded to 16 KiB. Its outcome is `schema_observed` only after response validation and cleanup succeed. Missing observations remain null. A cleanup failure preserves an earlier failure category and prevents success. No raw catalogue, schema, account values, or upstream errors are printed or written.
