# KleinAI — Architecture & Service Topology

Four Go binaries share the same `internal/` codebase, each with its own `cmd/` entrypoint:

```
┌─────────────────────────────────────────────────────┐
│                    Docker Compose                    │
│  flaresolverr ──── worker ──── api ──── admin ──── openai │
│  user-web ──── admin-web                            │
│  (MySQL + Redis are external, shared with 1Panel)   │
└─────────────────────────────────────────────────────┘
```

| Service   | Port  | Route Prefix     | Purpose                              |
|-----------|-------|------------------|---------------------------------------|
| api       | 17180 | `/api/v1`        | User-facing: auth, billing, gen, keys |
| admin     | 17188 | `/admin/api/v1`  | Admin dashboard: accounts, users, CDK |
| openai    | 17200 | `/v1`            | OpenAI-compatible API (key-authed)    |
| worker    | —     | —                | Async tasks (asynq), CF refresh, health check |

Frontend (pnpm monorepo):
- `frontend/apps/user/` — User SPA (React + Vite, port 5173 via nginx)
- `frontend/apps/admin/` — Admin SPA (React + Vite, port 5174 via nginx)
- `frontend/packages/theme/` — Shared Tailwind preset

**Language:** Go 1.24 backend, React + Vite + Tailwind frontend
**Module:** `github.com/kleinai/backend`
**DB:** MySQL (GORM) + Redis (asynq task queue)
**Auth:** JWT (access/refresh) + API Key (SHA256 hash)
**Encryption:** AES-256-GCM for all credential fields (account tokens, proxy passwords)