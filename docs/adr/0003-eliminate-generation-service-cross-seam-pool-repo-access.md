# ADR-0003: Eliminate GenerationService Cross-Seam Access to pool.repo

Status: implemented (2026-05-22)

## Context

GenerationService accessed `s.pool.repo.Update()` directly in three places:
- `disableProviderAccount` (OAuth 401 → permanently disable account)
- `markProviderQuotaLimited` (usage limit → Broken status + cooldown)
- `gptOAuthAccessToken` (OAuth token refresh → update encrypted tokens)

This bypasses AccountPool's interface seam. If AccountPool's backing store changes
(e.g., MySQL → Redis), GenerationService must also change. The `s.pool.repo == nil`
nil checks scattered through GenerationService indicate it's reaching into internals
it shouldn't know about.

## Decision

Add three new methods to AccountPool that encapsulate the account state mutations:

1. **`DisableAccount(ctx, accountID, reason)`** — permanently disables an account
   (sets status=Disabled, last_test_status=Fail, increments error_count, invalidates bucket).
   Replaces `disableProviderAccount` which previously did `s.pool.repo.Update` + `s.pool.Reload`.

2. **`MarkQuotaLimited(ctx, accountID, reason, until)`** — marks quota exhaustion
   (sets status=Broken, cooldown_until, increments error_count, invalidates bucket).
   Replaces `markProviderQuotaLimited`.

3. **`UpdateAccountFields(ctx, accountID, fields)`** — general field update without
   bucket invalidation (for OAuth token refresh where scheduling state doesn't change).
   Replaces `s.pool.repo.Update` in `gptOAuthAccessToken`.

GenerationService now calls these pool methods instead of reaching into `pool.repo`.
All `s.pool.repo == nil` nil checks removed — pool always has repo in production.

## Consequences

- **Positive**: AccountPool seam is clean. GenerationService no longer knows whether
  accounts are stored in MySQL, Redis, or anything else. All account state mutations
  go through AccountPool's interface, which can add side effects (cache invalidation,
  event publishing, etc.) without callers knowing.
- **Positive**: DisableAccount and MarkQuotaLimited automatically invalidate the
  bucket, so callers don't need to remember to call `s.pool.Reload` after updates.
- **Negative**: UpdateAccountFields is a pass-through method (thin wrapper over repo.Update).
  It adds depth only marginally — the value is in keeping the seam clean, not in the
  method itself. If this method grows with side effects later, it earns its depth.