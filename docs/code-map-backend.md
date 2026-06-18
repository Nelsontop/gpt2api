# KleinAI — Backend Code Map

## cmd/ (4 entrypoints)
```
cmd/api/main.go       → router.MountAPI
cmd/admin/main.go     → router.MountAdmin
cmd/openai/main.go    → router.MountOpenAI
cmd/worker/main.go    → asynq + GrokCFRefresh + ChatGPTCFRefresh + AccountHealthCheck
```

## internal/bootstrap/
`Deps` struct: Cfg, DB, Redis, JWT, Limiter, AES — injected into all routers/services.

## internal/router/
- `common.go` — New(Options) builds gin.Engine with base middleware + /healthz + /readyz
- `api.go` — MountAPI: auth, users, keys, billing, gen, public/at-import
- `admin.go` — MountAdmin: auth, dashboard, users, accounts, proxies, system, CDK, billing, promo, logs
- `openai.go` — MountOpenAI: /v1/models, /v1/chat/completions, /v1/images/*, /v1/video/*

## internal/handler/ (Gin handlers — thin HTTP→service bridge)
```
auth_handler.go          generation_handler.go   billing_handler.go
apikey_handler.go        openai_handler.go       public_handler.go
admin_auth_handler.go    admin_account_handler.go admin_user_handler.go
admin_cdk_handler.go     admin_proxy_handler.go  admin_promo_handler.go
admin_billing_handler.go admin_system_handler.go  admin_dashboard_handler.go
admin_log_handler.go
```

## internal/service/ (business logic)
```
generation_service.go    (1403 lines — core: create task, pick account, call provider, settle billing)
chat_service.go          (chat completions via GPT/Grok web)
account_pool.go          (548 lines — round-robin/weighted scheduling, cooldown, concurrency tracking)
account_test_service.go  (connectivity probe: curl subprocess + CF bypass for ChatGPT)
account_admin_service.go (CRUD + batch import/delete/assign-proxy)
account_health_check_service.go (scheduled health巡检, auto-disable broken accounts)
auth_service.go          admin_auth_service.go    admin_user_service.go
billing_service.go       cdk_service.go           apikey_service.go
proxy_service.go         system_config_service.go user_service.go
pricing.go               (model → points lookup)
openai_oauth_service.go  (OAuth token refresh for ChatGPT accounts)
grok_cf_refresh_service.go    (FlareSolverr → Grok cf_clearance)
chatgpt_cf_refresh_service.go (FlareSolverr → ChatGPT cf_clearance)
grok_token.go            (Grok web token extraction)
```

## internal/provider/ (provider abstraction)
```
provider.go          — Provider interface: Name() + Generate(ctx, Request) → Result
factory/factory.go   — Build() → map["gpt"/"grok"]Provider, env-var driven (mock|real)
gpt/gpt.go           (2469 lines — ChatGPT web scraping: webBootstrap, webUploadImage, webConversation SSE)
grok/grok.go         (318 lines — Grok video API + web WebClient)
grok/web_client.go   (Grok web cookie/CF handling)
mock/mock.go         (stub provider for dev)
```

## internal/dto/ (request/response structs with Gin validator tags)
```
auth.go  account.go  generation.go  billing.go  apikey.go  proxy.go
admin_user.go  admin_billing.go  admin_dashboard.go  admin_log.go  admin_promo.go
validator_register.go  (custom validator types)
```

## internal/model/ (GORM entities)
```
account.go    — Account (provider, auth_type, credential_enc, oauth, status, cooldown, test results)
user.go       — User (points, frozen_points, plan, invite system)
generation.go — GenerationTask + GenerationResult + GenerationUpstreamLog
billing.go    — WalletLog + ConsumeRecord + RefundRecord
apikey.go     — APIKey (prefix+hash+salt, never stores plaintext)
proxy.go      — Proxy (protocol, host, port, auth, health check)
promo.go      — PromoCode + RedeemCodeBatch + RedeemCode
system_config.go — SystemConfig (KV JSON store)
admin.go      — AdminUser
```

## internal/repo/ (GORM queries)
```
account_repo.go  user_repo.go  generation_repo.go  billing_repo.go
apikey_repo.go   proxy_repo.go  promo_repo.go  system_config_repo.go
admin_repo.go    dashboard_repo.go  wallet_repo.go
```

## internal/middleware/
```
auth.go      — AuthJWT (user/admin subject)
apikey.go    — AuthAPIKey (SHA256 hash lookup)
ratelimit.go — RateLimitIP (redis-based)
cors.go      requestid.go  recovery.go  access_log.go  security.go
```

## pkg/ (shared utilities)
```
config/config.go     — Viper: env > config.{env}.yaml > config.yaml
errcode/errcode.go   — Business error codes (400xxx-502xxx), Error struct with Wrap/WithMsg
response/response.go — OK/Fail/Page helpers, detail only in dev mode
logger/logger.go     — Zap + lumberjack
jwtx/jwt.go          — JWT manager (access/refresh)
crypto/crypto.go     — AES-256-GCM encrypt/decrypt
database/mysql.go    redis.go — GORM/connectors
outbound/client.go   — uTLS HTTP client (Chrome fingerprint) with HTTP/SOCKS5 proxy
curltransport/       — curl subprocess RoundTripper (OpenSSL 3.5.6 for Cloudflare bypass)
proxyx/proxyx.go     — Proxy URL parsing + standard client builder
snowflake/           ratelimit/  httpc/  jwtpayload/  validator/  version/
```

## migrations/ (goose)
18 sequential SQL migrations from init schema through upstream log, CF settings.

## configs/
```
config.yaml          — defaults (no secrets)
config.dev.yaml      — dev overrides
config.prod.yaml     — prod overrides (secrets from env vars)
```