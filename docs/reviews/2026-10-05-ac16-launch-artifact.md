# Independent AC-16 launch artifact review

**Reviewed by:** GPT-6 Sol (implementation author: GPT-6 Astra)  
**Date:** October 5, 2026  
**Verdict:** Approve for presentation to the operator. This is not authorization to launch.

## Exact artifact

I reviewed `/private/tmp/kiro-ac16-review-n1g0w3ue/plan.json` and `REVIEW.md` without changing either. The plan is a 3,328 byte, owner readable regular file with SHA256 `041c4f6f9cc3221d573261fb464074aa2e997c7caa05c161e1e2326f8e03bdd4`. It has no duplicate JSON keys and has `live_approval: null`. The checkout is clean at the plan's `code_commit`, `7d11dc1253038dcbb0656e85697c7bc0ff462e55`; no Go code or check script changed after the independently approved fix commit `defb5f6b5f5122fa7da848ee8f14a29098d88ac0`. The referenced independent review is present in that commit.

The plan contract exactly equals the tagged runner's `schemaContract()` JSON. Its command array equals the shell command in `REVIEW.md` after replacing the plan's `<PLAN_SHA256>` placeholder with the approved form digest. The command is also the exact token sequence checked by `schemaLivePreflight`: it selects only `TestSchemaProbe`, supplies the explicit launch flag, names this plan, and pins this code commit. The two named management endpoints, request target and body, selected model, eight schema paths, finite retained values, response and report bounds, one snapshot and dispatch limits, deadlines, and no retry, renewal, fallback, inference, or settings mutation agree with AC-16 and the runner.

I independently hashed both installed software files without executing them. The native binary is 1,141,385,408 bytes and hashes to `2118bd89d96830a4f0e0884e4f4071fb94e4b6036c757126afe9e3f80c9d90c6`; the bundled source is 19,338,772 bytes and hashes to `233e4dec77cd538e35b691d1fd0e12ca64a7c90d720c4dbf6979f21b4c45482f`. Both match the plan and the source baseline recorded in spec 0004 rationale. The 2 GiB software hash ceiling admits the complete pinned native file and leaves response and report limits unchanged.

Changing only `live_approval` to `approved_for_one_catalogue_run`, preserving the plan's two-space JSON indentation and final newline, produces SHA256 `8c945a6fb7d477e23f4b5535a1e92c5a0f715a433172c19b41eefabd47295ac3` in memory. I did not write that approved form. Any other byte change would require another review and digest.

## Evidence and boundary

The [offline code review](/Users/nguyenducloc/kiro-gateway/docs/reviews/2026-10-05-ac16-preflight-followup.md) approved the preflight fix with zero findings. I independently passed the focused synthetic preflight and software hash race tests with the real launch flag forced off. The main verification record reports passing `scripts/check`, an uncached tagged schema race suite, and a disabled real entry point when launch environment variables were set without the flag. I did not read account settings or credentials, run installed native software, make a management request, or execute the proposed launch command.

This review establishes an exact, reviewable artifact for one separate operator decision. The current null approval intentionally fails preflight. Any later run remains subject to the operator's explicit authorization and the runtime's account, source, network, and cleanup checks; it cannot be treated as GA evidence.
