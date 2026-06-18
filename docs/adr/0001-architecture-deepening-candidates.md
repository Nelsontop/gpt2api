# ADR-0001: Architecture Deepening Candidates

Status: proposed (2026-05-22)

## Context

Architecture review identified 6 friction points where modules are shallow,
cross-seam coupling leaks internals, or state is scattered across processes.
Each candidate is ranked by impact (locality/leverage loss + testability gap).

## Candidates

### C1. generation_service.go — 1400-line God Module

**Files**: `internal/service/generation_service.go`
**Problem**: GenerationService carries 6 unrelated responsibilities — task creation,
account picking, OAuth token refresh, proxy resolution, result caching (400 lines),
and error classification (200 lines). Its interface (9 injected deps) is nearly as
complex as its implementation. Deletion test: removing OAuth/cache/proxy sub-modules
would concentrate remaining core to ~200 lines (create → pick → call provider → settle).
**Benefits**: locality (OAuth bugs stay in OAuth module), leverage (smaller interface),
testability (OAuth/cache independently testable without full GenerationService construction).

### C2. gpt/gpt.go — 2500-line Web Scraper + API Route Monolith

**Files**: `internal/provider/gpt/gpt.go`
**Problem**: One Provider implements 3 routes (Codex API, Web bootstrap/requirements/
upload/poll/download). Web scraping is pure HTTP crawling (curltransport, SSE, CF bypass)
with zero shared state with Codex API. Implicit route selection via `_force_web_route`
and `shouldUseWebImage2` forces callers to understand internal branching.
**Benefits**: locality (CF/SSE changes don't affect Codex route), leverage (Codex callers
don't need to know web bootstrap), testability (Codex mock vs Web HTTP mock independently).

### C3. Per-Process AccountPool — 4 Independent Instances

**Files**: `router/api.go`, `admin.go`, `openai.go`, `cmd/worker/main.go`
**Problem**: 4 processes each create `NewAccountPool(accountRepo, 30s)` with fully
isolated in-memory state (suspect, active, latency). Suspect stuck across processes
was the critical symptom (fixed via TTL + fallback in ADR-0002). Active counts and
latency scores remain per-process — same account can be selected or skipped depending
on which process handles the request. No recovery service runs in api/openai processes.
**Current fix**: suspect TTL auto-evict (5 min) + PickConcurrent fallback bypasses
suspect when zero candidates. See ADR-0002 for details.
**Future optimization**: Redis-backed pool state for cross-process sharing of
suspect/active/latency. Requires Redis round-trips on every PickConcurrent call;
marginal benefit since per-process active tracking is correct behavior.

### C4. GenerationService Cross-Seam Access to pool.repo

**Files**: `generation_service.go:567,580,590,602,743,744`
**Problem**: `disableProviderAccount` and `markProviderQuotaLimited` directly access
`s.pool.repo.Update()`, bypassing AccountPool's interface seam. OAuth token refresh
(`gptOAuthAccessToken`) also writes through `s.pool.repo`. If AccountPool's repo
changes (e.g., to Redis), GenerationService must also change. Deletion test: moving
these operations into AccountPool methods concentrates account state changes in one module.
**Fix**: See ADR-0003.

### C5. Duplicated Service Wiring in 3 Router Files

**Files**: `router/api.go`, `admin.go`, `openai.go`
**Problem**: Each router manually wires AccountPool → BillingService → SystemConfigService
→ ProxyService → GenerationService → ChatService. Changing GenerationService constructor
requires editing 3 files.
**Impact**: Low — wiring changes are infrequent, each file only ~10 lines of construction.
Not a module-depth issue; a maintenance risk that's tolerable at current scale.

### C6. resolveProxyURL Duplicated Across Generation and Chat

**Files**: `generation_service.go:814`, `chat_service.go:576`
**Problem**: Same proxy resolution logic (account proxy → global proxy, random/fixed mode)
implemented twice with minor variations. GenerationService supports `PickEnabledRandom`
directly; ChatService resolves to a `pid` first.
**Fix**: Move to `ProxyService.ResolveURL(ctx, acc)` — proxy locality improves,
both callers simplify. Low urgency since the function is small and rarely changes.