# ADR-0002: Suspect TTL + Fallback for Per-Process Pool Stuck

Status: implemented (2026-05-22)

## Context

AccountPool's suspect list is in-memory per-process. The AccountPoolRecoveryService
only runs in the worker process. When accounts fail in api/openai processes, they're
marked suspect locally but never recovered — all accounts become suspect, PickConcurrent
returns "暂无可用账号", and requests fail immediately even when accounts are healthy in DB.

## Decision

Two-part fix in `account_pool.go`:

1. **Suspect TTL auto-evict (5 minutes)**: New `suspectTTL` field on AccountPool.
   `evictExpiredSuspects()` runs before every `PickConcurrent`, removing suspect entries
   older than TTL. Accounts that are truly broken will be re-marked suspect on next failure.

2. **PickConcurrent fallback**: When zero candidates remain after suspect filtering,
   bypass suspect and try again with only predicate + canAcceptMore filters.宁可重试失败再标记,
   不因 suspect 卡死直接拒绝请求。

## Consequences

- **Positive**: Suspect stuck problem resolved without Redis or cross-process coordination.
  Each process self-heals via TTL; no external recovery service dependency.
- **Negative**: Suspect accounts might get one extra failed request during the brief window
  between TTL expiry and re-marking. Acceptable — one wasted request is better than all requests failing.
- **Future**: Redis-backed pool state (ADR-0001 C3) would eliminate per-process divergence
  entirely, but requires significant architecture change and adds latency to every PickConcurrent call.