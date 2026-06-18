# KleinAI — Deployment & Conventions

## Deployment

**Local runtime:** `docker compose -f deploy/docker-compose.local-runtime.yml up -d --build`
- Pre-build Go binaries locally: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` → `bin/docker/`
- Dockerfile copies pre-built binaries (doesn't build in Docker)
- Base image: `curlimages/curl` (provides OpenSSL 3.5.6 for Cloudflare bypass)
- External MySQL/Redis (1Panel-managed containers)
- FlareSolverr container for CF challenge solving
- Env secrets from `deploy/.env.local-runtime`

**Ports:** api=17180, admin=17188, openai=17200, user-web=5173, admin-web=5174

## Conventions

- **Error codes:** HTTP_STATUS + 3-digit sub-code (e.g., 400101=InvalidParam). `errcode.Error` struct with Wrap/WithMsg.
- **Response format:** `{"code":0,"msg":"ok","data":...}` or `{"code":400101,"msg":"参数错误","detail":"..."}` (detail only in dev).
- **Credential storage:** All tokens/keys encrypted with AES-256-GCM. Decrypted at service layer, never passed to providers as encrypted.
- **Config priority:** env vars > config.{env}.yaml > config.yaml. Prefix `KLEIN_`.
- **Provider selection:** `KLEIN_PROVIDER_GPT=real|mock`, `KLEIN_PROVIDER_GROK=real|mock`.
- **Account status:** 1=enabled, 0=disabled, 2=broken(circuit), -1=banned.
- **Gen task status:** 0=pending, 1=running, 2=succeeded, 3=failed, 4=refunded.
- **Billing:** Points system. 1 point = 100 units. Frozen on task create, settled on success, refunded on failure.
- **Routing:** User APIs use JWT auth (middleware.AuthJWT), OpenAI APIs use API Key auth (middleware.AuthAPIKey), Admin APIs use JWT with admin subject.
- **Docker rebuild:** Must rebuild Go binaries locally first, then `docker compose up -d --build`.