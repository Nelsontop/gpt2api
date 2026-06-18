# 开发进度 / Changelog

> 仅记录对外可见的能力变更与里程碑；项目内部规范文档见 `docs/`。

---

## v2.1.0 — 2026-06-18

### 账号池并发控制

- 新增 `PickConcurrent` / `ReleaseConcurrent`：按权重限制单账号并发数，避免单账号过载
- 新增 `RecordLatency`：记录每次请求响应时间，供调度策略参考
- 新增 `MarkSuspect`：任务失败时降低账号权重，配合熔断机制
- 图片生成与文字对话链路均已接入并发控制

### Cloudflare WAF 绕过

- 新增 `pkg/curltransport`：基于 `libcurl` + OpenSSL 3.5.6 的 HTTP transport，支持 JA3/JA4 指纹伪装
- 新增 `chatgpt_cf_refresh_service`：自动维护 ChatGPT Web 路线的 cf_clearance cookie
- 配合 FlareSolverr 容器实现两层 CF 绕过
- GPT Web 图片路线可通过 CF 绕过直接访问 `chatgpt.com`

### 账号健康检查

- 新增 `account_health_check_service`：定期探测账号可用性
- 新增 `account_pool_recovery_service`：自动恢复短暂熔断的账号
- 管理后台系统配置页集成健康检查触发与状态展示

### GPT Web 路线优化

- 按图片数量动态计算超时：单图超时可配置（默认 10 分钟），总超时不超过 30 分钟
- 选号失败诊断信息增强：输出账号池状态、可用账号数、冷却账号数等
- Web 路线与 Codex 路线自动切换逻辑优化

### 校验器框架

- 新增 `pkg/validator`：统一参数校验翻译，支持中文错误信息
- 新增 `dto.RegisterValidatorTypes()`：DTO 级别校验规则注册
- 所有 Gin 路由接入校验翻译中间件

### 前端

- CDK 管理页重写：批量生成（按数量/前缀/面额）、批量删除、统计概览卡片
- Admin 配置页新增全局代理模式 UI（Fixed / Random）
- 用户前台创作页布局优化、计费页充值记录增强

### 其他

- 新增 `public_handler`：无需鉴权的公开接口
- `pricing_test.go`、`generation_service_test.go`、`account_health_check_service_test.go` 测试补充
- `generation_gpt_web_retry_test.go`：GPT Web 路线重试测试

---

## v2.0.1 — 2026-05-04

### 视频生成

- 修复 默认分辨率仍按 `480p` 下发的问题：常量 `defaultVideoResolution` 改为 `1080p`，`defaultVideoSize` 改为 `1920x1080`
- 新增 `quality` 入参，`standard / draft → 720p`、`hd → 1080p`，并按 `aspect_ratio`（`16:9 / 9:16 / 1:1`）正确推导宽高
- 兜底宽高 由硬编码 `1280x720` 改为 `videoConfig` 推导出来的默认宽高，保留后续接入 4K 等更高分辨率的扩展位

### 代理管理

- 新增 `POST /admin/api/v1/proxies/import` 批量导入：按行解析 `scheme://user:pass@host:port#name`，密码 AES-256-GCM 落盘
- 新增 `POST /admin/api/v1/proxies/batch-delete` 批量软删除
- 新增 `POST /admin/api/v1/proxies/batch-test` 批量测试，信号量并发 4，返回 `tested / ok / failed`
- 前端 `ProxiesPage` 重写：批量导入 / 批量删除 / 批量测试 / 多选交互

### Token（账号）管理

- 列表新增 `plan_type` 过滤项（`basic / super / heavy`），通过 `oauth_meta` JSON 字段查询
- 导入后自动并发探测（信号量并发 4）GROK Cookie 账号，识别 plan 类型后回填，导入结果新增 `detected / pending / failed` 字段
- 新增 `POST /admin/api/v1/accounts/batch-assign-proxy` 批量代理分配：
  - `mode = single`：所有选中账号绑定到同一个 `proxy_id`
  - `mode = cycle`：按 `idx % len(proxy_ids)` 轮询绑定到 `proxy_ids` 列表
- 前端 `TokenAccountsPage` 重写：账号类型列、按类型过滤、批量代理分配弹窗、导入结果回显

### 系统配置

- 新增 `proxy.selection_mode` 全局代理选择模式：`fixed`（固定代理） / `random`（随机代理）
- `random` 模式下，每次任务通过 `crypto/rand` 从启用代理列表中随机挑一个
- 账号级 `proxy_id` 仍始终优先于全局策略
- 前端 `ConfigPage` 新增「全局代理模式」下拉与说明文案

### 其他

- 清理界面与文档中暴露给最终用户的源码入口与品牌点
- 修复 `resolveProxyURL` 在 `account_test_service` 与 `generation_service` 中的重复实现，统一走 `proxySvc`

---

## v2.0.0 — 2026-04-27

- 统一文字、图片、视频三条生成链路
- 统一账号池、代理、刷新、熔断、轮换、用量检测
- 统一 OpenAI 兼容 API（`/v1/chat/completions`、`/v1/images/*`、`/v1/video/*`、`/v1/models`）
- 统一管理后台：用户、账单、CDK、优惠码、模型价格、请求日志、上游日志
- 统一部署：Docker Compose 一键拉起，可平滑迁移到 K8s

---

## v1.0.x

历史稳定基线，仅作为对照保留，新需求不再回灌。

---

## 当前已具备模块

| 模块 | 状态 | 备注 |
|------|------|------|
| 后端 API / Admin / OpenAI / Worker | ✅ | 4 个 cmd 二进制 + healthz / readyz |
| GPT / GROK 账号池 | ✅ | 批量导入 · 自动探测 · 熔断 · 轮换 |
| 代理池 | ✅ | 批量导入 · 批量测试 · 固定 / 随机回落 |
| 系统配置中心 | ✅ | OAuth 刷新窗口 · 代理策略 · 数据保留 |
| 用户前台 | ✅ | 文 / 图 / 视频 · 历史 · 密钥 · 账户 |
| 管理后台 | ✅ | Token / 代理 / 用户 / 计费 / CDK / 日志 |
| OpenAI 兼容层 | ✅ | chat / images / video / models |
| 计费体系 | ✅ | 积分 · 充值 · CDK · 模型价格 |
| 部署体系 | ✅ | Docker Compose 单机 · 反代 · SSL |

## 后续路线

- 前端图片 / 视频任务面板暴露 `quality` 选项与 4K 选项预留
- 上游日志按账号 / 代理维度的聚合视图
- 限流策略表单化（当前部分仍在配置文件）
- K8s Helm Chart 预研
