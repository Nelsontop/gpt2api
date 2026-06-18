# KleinAI — Database Tables

18 goose migrations covering:

| Table                  | Purpose                                   |
|------------------------|-------------------------------------------|
| user                   | Users (points, plan, invite)              |
| admin_user             | Admin accounts                            |
| account                | Third-party accounts (gpt/grok, 4 auth types) |
| api_key                | User API keys (prefix+hash, never plaintext) |
| generation_task        | Gen tasks (image/video/chat, status tracking) |
| generation_result      | Gen results (URLs, dimensions, metadata)  |
| generation_upstream_log| Provider request/response diagnostics     |
| wallet_log             | Points ledger (recharge/consume/refund)   |
| consume_record         | Consumption records (frozen→settled→refunded) |
| refund_record          | Refund logs                               |
| system_config          | KV JSON settings (CF, OAuth, pricing)     |
| proxy                  | Outbound proxies (HTTP/SOCKS5)            |
| promo_code             | Promo codes (amount/discount/gift)        |
| promo_code_use         | Promo usage records                       |
| redeem_code_batch      | CDK batches                               |
| redeem_code            | Individual CDK codes                      |

## Account statuses
1=enabled, 0=disabled, 2=broken(circuit breaker), -1=banned

## Generation task statuses
0=pending, 1=running, 2=succeeded, 3=failed, 4=refunded

## Auth types
api_key, cookie, oauth, at

## Account providers
gpt, grok