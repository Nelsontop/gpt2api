# 二次编辑：直接选中已生成图片进行修改

**日期**: 2026-06-25
**状态**: 设计评审

## 问题

当前图片生成流程中，用户想要对已生成的图片进行修改时，必须：
1. 手动下载生成的图片
2. 在创建页面重新上传该图片
3. 输入修改后的提示词
4. 再次提交生成请求

这个"下载再上传"的操作繁琐且不必要，因为生成的图片已经在服务器端持久化存储。

## 目标

在 CreateStudioPage 的 "My Works" 区域的 WorkCard 上增加"二次编辑"按钮，用户点击后自动将图片和原始提示词回填到创建区域，跳过手动下载和上传步骤。

## 方案

**方案 A：URL 直传**（选定）

将服务器端的缓存 URL（如 `/api/v1/gen/cached/...`）直接作为 `ref_assets` 传给创建请求。后端 `normalizeInputRefs()` 已支持缓存 URL 格式，无需后端改动。

替代方案（未选定）：
- 方案 B（前端 fetch 转 base64）：需要下载图片数据再上传，有网络延迟
- 方案 C（新增后端 API）：过度工程，违反 YAGNI

## 设计细节

### 1. 附件类型扩展

当前 `attachments` 类型：
```ts
Array<{ id: string; name: string; dataUrl: string }>
```

扩展为联合类型：
```ts
type AttachmentItem =
  | { type: 'data'; id: string; name: string; dataUrl: string }   // 原有的 base64（来自文件上传）
  | { type: 'url'; id: string; name: string; url: string; thumbUrl?: string }  // 新增的缓存 URL（来自二次编辑）
```

### 2. 提交逻辑调整

`createImage` 和 `createVideo` 的 `ref_assets` 字段需要根据 attachment type 输出不同值：
- `type: 'data'` → `item.dataUrl`（data: URL，和现在一样）
- `type: 'url'` → `item.url`（缓存 URL，后端 normalizeInputRefs 会直接使用）

### 3. 附件显示区域调整

- `type: 'data'` → 用 `dataUrl` 作为 `<img src>`（和现在一样）
- `type: 'url'` → 用 `thumbUrl ?? url` 作为 `<img src>`
  - `/api/v1/gen/cached/...` 不需要认证，直接可访问
  - `/api/v1/gen/assets/...` 也不需要认证（公开路由）
  - 外部 CDN URL 也公开可访问

### 4. WorkCard 改动

在 WorkCard 展开区域增加"二次编辑"按钮：
- **显示条件**：`item.kind === 'image'` && `item.status === 2`（已完成）&& `results?.[0]?.url` 存在
- **点击行为**：触发 `onReEdit` 回调，传入 `GenerationTask` 对象
- **位置**：在现有 "复制" 和 "使用此提示词" 按钮旁边

### 5. CreateStudioPage 回调处理

```ts
const handleReEdit = (item: GenerationTask) => {
  const result = item.results?.[0];
  if (!result?.url) return;

  if (mode !== 'image') {
    // 用户在 video/text 模式，需要先切换到 image 模式
    reEditQueueRef.current = item;
    navigate('/create/image');
    return;
  }
  applyReEdit(item);
};

const applyReEdit = (item: GenerationTask) => {
  const result = item.results?.[0];
  if (!result?.url) return;
  setPrompt(item.prompt || '');
  setAttachments([
    { type: 'url', id: `reedit-${item.task_id}`, name: '二次编辑', url: result.url, thumbUrl: result.thumb_url }
  ]);
  promptRef.current?.focus();
};
```

### 6. Mode 切换冲突处理

当前代码在 mode 切换时会清空 `attachments`（line 218-221）。使用 `reEditQueueRef` 保存待填入数据，mode 切换完成后填入：

```ts
const reEditQueueRef = useRef<GenerationTask | null>(null);

useEffect(() => {
  if (reEditQueueRef.current && mode === 'image') {
    applyReEdit(reEditQueueRef.current);
    reEditQueueRef.current = null;
  }
}, [mode]);
```

### 7. 多图结果

当前 WorkCard 只展示 `results[0]`（第一张图）。"二次编辑"按钮也只针对第一张图。如果用户想编辑其他图，需要先通过 PreviewLightbox 查看，后续迭代中可扩展。

## 改动范围

### 前端（仅 CreateStudioPage.tsx）

1. 扩展 `attachments` 类型为联合类型 `AttachmentItem`
2. 添加 `handleReEdit` 和 `applyReEdit` 函数
3. 修改 `WorkCard`：增加 `onReEdit` 回调和"二次编辑"按钮
4. 修改提交逻辑：`ref_assets` 根据 attachment type 输出 dataUrl 或 url
5. 修改附件显示：`type: 'url'` 用 url/thumbUrl 显示
6. 添加 `reEditQueueRef` 处理 mode 切换冲突

### 后端

**无改动**。`normalizeInputRefs()` 已支持 `/api/v1/gen/cached/...` URL。

### 数据库

**无改动**。

## 不做的事（YAGNI）

- 不在 HistoryPage 添加"二次编辑"按钮（用户明确只改 CreateStudioPage）
- 不在 PreviewLightbox 添加"二次编辑"按钮
- 不支持多图批量二次编辑
- 不新增后端 API
- 不修改后端 URL 格式或认证逻辑
