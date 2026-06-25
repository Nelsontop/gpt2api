# WorkCard UX 优化：展开交互 + 按钮层级 + i2i 标识

**日期**: 2026-06-25
**状态**: 设计评审

## 问题

1. **展开交互不明显**：当前提示词区域 hover 只有极淡底色变化，用户很难发现可以点击展开看完整提示词和操作按钮
2. **按钮层级不清晰**：复制、使用此提示词、二次编辑三个按钮视觉完全相同，无法区分主次
3. **i2i 图片没有标识**：通过二次编辑（image-to-image）生成的图片，在 WorkCard 上没有视觉标签区分"基于原图编辑的"和"全新生成的"

## 设计

### 1. 展开交互：右侧箭头始终可见

当前：提示词整行是可点击的 button，hover 有 `bg-neutral-50`。ChevronDown 仅在提示词超过 28 字时显示。

改为：
- 提示词行右侧始终显示 `ChevronRight` 箭头图标（无论提示词长度）
- 点击箭头或整行 → 展开，箭头变为 `ChevronDown`（向下旋转）
- 提示词行 hover 加 `bg-neutral-50` 底色
- 展开后整行有 `bg-neutral-100` 底色标识

不可展开的情况（极短提示词且无操作按钮）：仍然显示箭头，点击展开后只显示完整提示词和收起按钮（没有复制/使用提示词/二次编辑按钮）。实际上所有卡片都应该可以展开查看完整提示词 — 现在的 `canExpand` 只基于提示词 > 28 字的限制不合理。

**改动**：去掉 `canExpand` 条件，所有卡片都可以展开。箭头图标始终可见。

### 2. 按钮层级：同一底色，深浅文字区分

三个操作按钮统一用 `bg-neutral-100` 底色，通过文字颜色和图标区分层级：

| 按钮 | 底色 | 文字色 | 图标 | 角色 |
|------|------|--------|------|------|
| 复制 | `bg-neutral-100` | `text-neutral-400` | `Copy` | 辅助 |
| 使用此提示词 | `bg-neutral-100` | `text-neutral-800` | `ArrowUpLeft` | 操作 |
| 二次编辑 | `bg-neutral-100` | `text-neutral-800` | `Sparkles` | 操作 |
| 收起 | 无底色 | `text-neutral-400` | 无 | 辅助（右侧） |

hover 效果：所有按钮 `hover:bg-neutral-200`。

### 3. i2i 标识：图片左上角"编辑"标签

在 WorkCard 图片区域左上角，现有 `图片` 标签旁边，如果是 i2i 模式生成的任务，追加一个 `编辑` 标签：

- `t2i` 任务：只显示 `图片` 标签
- `i2i` 任务：显示 `图片` + `编辑` 两个标签并排

标签样式：
- `图片`：现有样式 `bg-black/55 text-white`
- `编辑`：`bg-sky-500/80 text-white`（天蓝色，与二次编辑按钮的语义关联）

视频任务不变（只显示 `视频` 标签）。

### 4. 后端改动：GenerationTaskResp 增加 mode 字段

当前 `GenerationTaskResp` DTO 没有 `mode` 字段，但 DB 表 `generation_task` 有 `mode` 列（`t2i`/`i2i`/`t2v`/`i2v`）。

改动：
- **DTO**：`GenerationTaskResp` 增加 `Mode string json:"mode,omitempty"` 字段
- **Handler**：`taskToResp()` 映射 `t.Mode → r.Mode`
- **前端类型**：`GenerationTask` 增加 `mode?: 't2i' | 'i2i' | 't2v' | 'i2v'` 字段

## 改动范围

### 后端
- `backend/internal/dto/generation.go` — GenerationTaskResp 增加 Mode 字段
- `backend/internal/handler/generation_handler.go` — taskToResp 映射 Mode

### 前端
- `frontend/apps/user/src/lib/types.ts` — GenerationTask 增加 mode 字段
- `frontend/apps/user/src/pages/create/CreateStudioPage.tsx` — WorkCard 展开交互、按钮样式、i2i 标签

## 不做的事（YAGNI）

- 不改动 HistoryPage（用户之前确认保持原样）
- 不改动 PreviewLightbox
- 不给视频任务加 i2i/i2v 标识标签
- 不在 GenerationResult 层级加 mode（只加在 task 层级）
