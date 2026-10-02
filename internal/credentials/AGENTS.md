# Credential snapshots

You can use [spec 0002](../../docs/specs/0002-local-configuration-credentials/index.md) for the token capture contract and [spec 0003](../../docs/specs/0003-first-claude-code-loop/index.md) for its selected profile extension.

`capture.go` owns the fixed source path, schema checks, bounded reads, token validation, and fingerprint comparison. `profile.go` adds the combined profile snapshot. The CLI owns linking and saving settings.

The source is `~/Library/Application Support/kiro-cli/data.sqlite3`. Token reads select only `main.auth_kv["kirocli:odic:token"]`. Combined reads also select `main.state["api.codewhisperer.profile"]`, after checking the token fingerprint. There is no source discovery, fallback, repair, or renewal.

Each operation owns one connection and one read transaction. The database opens with `mode=ro`, query only operation, a 1000 ms busy budget, and a five second context deadline shortened by the caller. Schema, value type, and byte length are checked before loading either record. Values must be text containing 1 through 65536 bytes. Only the profile value column may be declared `BLOB`; actual BLOB storage remains invalid.

Exact JSON member names and duplicate rejection matter. Struct decoding can accept case variants that overwrite a validated field. Fingerprints cover exact source bytes with separate token and profile digest prefixes. A fingerprint mismatch returns no usable snapshot.

The selected profile accepts exactly one string member named `profileName` or `profile_name`; its value is discarded. Routing uses the ARN region, currently limited to `us-east-1` and `eu-central-1`. The caller owns the profile pin between attempts. Snapshot accessors expose sensitive values for adapter use only, never logs, fixtures, or saved settings.

You can run `rtk proxy go test -race ./internal/credentials` from the repository root. The adjacent capture, snapshot, and profile tests use temporary homes and synthetic SQLite fixtures. Ordinary verification never opens your real credential store.

_Drafted by /sync from the introducing change, worth a quick human pass._
