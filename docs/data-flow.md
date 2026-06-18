# KleinAI — Key Data Flow

## Image Generation (GPT Web)
```
User → POST /api/v1/gen/image
  → generation_service.CreateImage (freeze points, idem key, pick account)
  → gpt.Provider.Generate (webBootstrap → webRequirements → webUploadImage → webConversation SSE)
  → gpt.Provider uses curltransport (curl subprocess, OpenSSL 3.5.6) when cf_clearance available
  → settle billing (consume_record → wallet_log)
```

## Chat Completions (OpenAI-compatible)
```
Client → POST /v1/chat/completions (API Key auth)
  → chat_service.Chat (pick account, call provider, SSE stream back)
```

## Account Pool Scheduling
```
AccountPool.Pick(provider) → round-robin/weighted across available accounts
  → MarkUsed (update last_used_at, increment active count)
  → MarkFailed (increment error_count, cooldown if threshold reached)
```

## Cloudflare Bypass
```
FlareSolverr container → solves CF challenges → cf_clearance + cookies
  → stored in /app/storage/{grok|chatgpt}_cf.json + system_config DB table
  → account_test_service: curl subprocess probe with cf_clearance + same proxy IP
  → gpt.Provider: curltransport RoundTripper for all chatgpt.com HTTP calls
  → grok.WebClient: injects cf_clearance into headers
```