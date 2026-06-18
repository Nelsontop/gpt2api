# KleinAI (gpt2api) — CLAUDE.md

Multi-provider AI generation platform (GPT image, Grok video, OpenAI-compatible chat/image/video). Go 1.24 backend, React frontend, MySQL+Redis, AES-256-GCM credential encryption.

## Documentation Index

- **Architecture & services** → [docs/architecture.md](docs/architecture.md)
- **Backend code map** → [docs/code-map-backend.md](docs/code-map-backend.md)
- **Frontend code map** → [docs/code-map-frontend.md](docs/code-map-frontend.md)
- **Data flow** → [docs/data-flow.md](docs/data-flow.md)
- **Database tables & statuses** → [docs/database.md](docs/database.md)
- **Deployment & conventions** → [docs/deployment-and-conventions.md](docs/deployment-and-conventions.md)

## Quick Reference

| Service | Port | Route Prefix | Auth |
|---------|------|-------------|------|
| api | 17180 | `/api/v1` | JWT |
| admin | 17188 | `/admin/api/v1` | JWT (admin) |
| openai | 17200 | `/v1` | API Key |
| worker | — | — | — |

- **Deploy:** `docker compose -f deploy/docker-compose.local-runtime.yml up -d --build`
- **Rebuild:** must build Go binaries locally first → `docker compose --build`
- **Config priority:** env vars > config.{env}.yaml > config.yaml (prefix `KLEIN_`)
- **Provider env:** `KLEIN_PROVIDER_GPT=real|mock`, `KLEIN_PROVIDER_GROK=real|mock`
- **CF bypass:** FlareSolverr container → cf_clearance → curltransport (OpenSSL 3.5.6)

## Agent skills

### Issue tracker
Issues live as GitHub issues (uses `gh` CLI). See `docs/agents/issue-tracker.md`.

### Triage labels
Five canonical triage roles mapped to default label strings. See `docs/agents/triage-labels.md`.

### Domain docs
Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.