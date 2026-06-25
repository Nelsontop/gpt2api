# 二次编辑（Re-edit from History） Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "二次编辑" button to WorkCard that fills the creation area with the original image URL and prompt, skipping manual download/re-upload.

**Architecture:** Extend the `attachments` state to support URL-type items alongside existing data-URL items. WorkCard gets a new `onReEdit` callback. CreateStudioPage handles the callback by setting prompt + attachments and switching to image mode if needed. No backend changes.

**Tech Stack:** React (TypeScript), @tanstack/react-query, react-router-dom, lucide-react icons

## Global Constraints

- Single file modified: `frontend/apps/user/src/pages/create/CreateStudioPage.tsx`
- No backend changes — `normalizeInputRefs()` already handles cached URLs
- No database changes
- Attachment URLs (`/api/v1/gen/cached/...`, `/api/v1/gen/assets/...`, CDN) are all publicly accessible, no auth headers needed
- `GenerationResult.url` field is the canonical URL to use for ref_assets
- `GenerationResult.thumb_url` is optional; fall back to `url` for display
- Only `results[0]` is used for re-edit (single image, not multi-image batch)
- Button only appears for `kind === 'image'` && `status === 2` && `results?.[0]?.url` exists

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `frontend/apps/user/src/pages/create/CreateStudioPage.tsx` | Modify | All changes: type definition, state, callbacks, WorkCard component, attachment rendering, submit logic |

No new files. No file splits — the existing file is the established pattern for this page.

---

### Task 1: Define AttachmentItem type and update attachments state

**Files:**
- Modify: `frontend/apps/user/src/pages/create/CreateStudioPage.tsx:205`

**Interfaces:**
- Produces: `AttachmentItem` type used by all subsequent tasks; `attachments` state with new union type

- [ ] **Step 1: Add the AttachmentItem type definition**

Add after the `TEXT_MAX_ATTACHMENTS` / `VIDEO_MAX_ATTACHMENTS` constants (around line 81-82), before the `SUGGESTIONS` constant:

```ts
type AttachmentItem =
  | { type: 'data'; id: string; name: string; dataUrl: string }
  | { type: 'url'; id: string; name: string; url: string; thumbUrl?: string };
```

- [ ] **Step 2: Update the attachments state declaration**

Change line 205 from:

```ts
const [attachments, setAttachments] = useState<Array<{ id: string; name: string; dataUrl: string }>>([]);
```

To:

```ts
const [attachments, setAttachments] = useState<AttachmentItem[]>([]);
```

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/user/src/pages/create/CreateStudioPage.tsx
git commit -m "feat(reedit): define AttachmentItem type and update attachments state"
```

---

### Task 2: Update file upload handler to produce type:'data' items

**Files:**
- Modify: `frontend/apps/user/src/pages/create/CreateStudioPage.tsx:341-374`

**Interfaces:**
- Consumes: `AttachmentItem` type from Task 1
- Produces: `handleAttachFiles` now creates `type: 'data'` items (compatible with all downstream consumers)

- [ ] **Step 1: Update handleAttachFiles to produce type:'data' items**

Change the `handleAttachFiles` function (lines 348-374). The `readFileAsDataURL` helper stays the same. Only the mapping in `handleAttachFiles` changes:

Replace the data mapping (lines 362-365):

```ts
const data = await Promise.all(picked.map(async (file) => ({
  id: `${file.name}-${file.size}-${file.lastModified}`,
  name: file.name,
  dataUrl: await readFileAsDataURL(file),
})));
```

With:

```ts
const data = await Promise.all(picked.map(async (file) => ({
  type: 'data' as const,
  id: `${file.name}-${file.size}-${file.lastModified}`,
  name: file.name,
  dataUrl: await readFileAsDataURL(file),
})));
```

- [ ] **Step 2: Commit**

```bash
git add frontend/apps/user/src/pages/create/CreateStudioPage.tsx
git commit -m "feat(reedit): file upload produces type:data AttachmentItem"
```

---

### Task 3: Update submit logic to resolve ref_assets from both attachment types

**Files:**
- Modify: `frontend/apps/user/src/pages/create/CreateStudioPage.tsx:255-284`

**Interfaces:**
- Consumes: `AttachmentItem` type with `type: 'data'` (`.dataUrl`) and `type: 'url'` (`.url`)
- Produces: `ref_assets` as `string[]` for API calls — data URLs or cached URLs depending on type

- [ ] **Step 1: Add a refAssets helper function**

Add a helper function after the `submit` function (around line 340), before the `readFileAsDataURL` function:

```ts
const refAssets = () => attachments.map((a) => a.type === 'data' ? a.dataUrl : a.url);
```

- [ ] **Step 2: Update createImage mutation**

Change lines 255-267 from:

```ts
const createImage = useMutation({
  mutationFn: () => genApi.createImage({
    model: imageModel,
    prompt,
    count,
    ratio: imageRatio,
    ref_assets: attachments.map((item) => item.dataUrl),
    mode: attachments.length ? 'i2i' : 't2i',
    params: { resolution: imageResolution, quality: 'high' },
  }),
```

To:

```ts
const createImage = useMutation({
  mutationFn: () => genApi.createImage({
    model: imageModel,
    prompt,
    count,
    ratio: imageRatio,
    ref_assets: refAssets(),
    mode: attachments.length ? 'i2i' : 't2i',
    params: { resolution: imageResolution, quality: 'high' },
  }),
```

- [ ] **Step 3: Update createVideo mutation**

Change line 270 from:

```ts
mutationFn: () => genApi.createVideo({ model: videoModel, prompt, duration, ratio: videoRatio, quality: 'hd', ref_assets: attachments.map((item) => item.dataUrl), mode: attachments.length ? 'i2v' : 't2v' }),
```

To:

```ts
mutationFn: () => genApi.createVideo({ model: videoModel, prompt, duration, ratio: videoRatio, quality: 'hd', ref_assets: refAssets(), mode: attachments.length ? 'i2v' : 't2v' }),
```

- [ ] **Step 4: Update createText mutation**

Change line 276 from:

```ts
mutationFn: () => genApi.createText({ model: textModel, prompt, max_tokens: 1600, images: attachments.map((item) => item.dataUrl) }),
```

To:

```ts
mutationFn: () => genApi.createText({ model: textModel, prompt, max_tokens: 1600, images: refAssets() }),
```

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/user/src/pages/create/CreateStudioPage.tsx
git commit -m "feat(reedit): ref_assets resolves from both data and url attachment types"
```

---

### Task 4: Update attachment display to handle both types

**Files:**
- Modify: `frontend/apps/user/src/pages/create/CreateStudioPage.tsx:447-463`

**Interfaces:**
- Consumes: `AttachmentItem` — `type: 'data'` uses `.dataUrl` for img src; `type: 'url'` uses `.thumbUrl ?? .url`

- [ ] **Step 1: Update the attachment thumbnail rendering**

Change the attachments display block (lines 447-463) from:

```tsx
{attachments.length > 0 && (
  <div className="mt-3 flex flex-wrap gap-2">
    {attachments.map((item) => (
      <div key={item.id} className="group relative h-14 w-14 overflow-hidden rounded-[12px] bg-neutral-100">
        <img src={item.dataUrl} alt={item.name} className="h-full w-full object-cover" />
        <button
          type="button"
          onClick={() => setAttachments((prev) => prev.filter((x) => x.id !== item.id))}
          className="absolute right-1 top-1 grid h-5 w-5 place-items-center rounded-full bg-black/60 text-white opacity-0 transition group-hover:opacity-100"
          title="移除"
        >
          <X size={12} />
        </button>
      </div>
    ))}
  </div>
)}
```

To:

```tsx
{attachments.length > 0 && (
  <div className="mt-3 flex flex-wrap gap-2">
    {attachments.map((item) => (
      <div key={item.id} className="group relative h-14 w-14 overflow-hidden rounded-[12px] bg-neutral-100">
        <img src={item.type === 'data' ? item.dataUrl : (item.thumbUrl ?? item.url)} alt={item.name} className="h-full w-full object-cover" />
        <button
          type="button"
          onClick={() => setAttachments((prev) => prev.filter((x) => x.id !== item.id))}
          className="absolute right-1 top-1 grid h-5 w-5 place-items-center rounded-full bg-black/60 text-white opacity-0 transition group-hover:opacity-100"
          title="移除"
        >
          <X size={12} />
        </button>
      </div>
    ))}
  </div>
)}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/apps/user/src/pages/create/CreateStudioPage.tsx
git commit -m "feat(reedit): attachment display handles both data and url types"
```

---

### Task 5: Add handleReEdit callback and reEditQueueRef in CreateStudioPage

**Files:**
- Modify: `frontend/apps/user/src/pages/create/CreateStudioPage.tsx`

**Interfaces:**
- Consumes: `AttachmentItem` type, `GenerationTask` type, `navigate`, `mode`
- Produces: `handleReEdit(item: GenerationTask)` function used by WorkCard's `onReEdit` prop; `applyReEdit(item: GenerationTask)` internal helper; `reEditQueueRef` for mode-switch deferred fill

- [ ] **Step 1: Add reEditQueueRef**

After the `fileInputRef` declaration (line 212), add:

```ts
const reEditQueueRef = useRef<GenerationTask | null>(null);
```

- [ ] **Step 2: Add the applyReEdit function**

Add after the `submit` function (around line 340), before the `readFileAsDataURL` function. Place it right before the `refAssets` helper added in Task 3:

```ts
const applyReEdit = (item: GenerationTask) => {
  const result = item.results?.[0];
  if (!result?.url) return;
  setPrompt(item.prompt || '');
  setAttachments([
    { type: 'url', id: `reedit-${item.task_id}`, name: '二次编辑', url: result.url, thumbUrl: result.thumb_url },
  ]);
  promptRef.current?.focus();
};
```

- [ ] **Step 3: Add the handleReEdit function**

Add right after `applyReEdit`:

```ts
const handleReEdit = (item: GenerationTask) => {
  if (mode !== 'image') {
    reEditQueueRef.current = item;
    navigate('/create/image');
    return;
  }
  applyReEdit(item);
};
```

- [ ] **Step 4: Add useEffect to process reEdit queue after mode switch**

Add after the existing mode-switch useEffect (lines 218-222). This new effect watches for mode changes and fills in queued re-edit data:

```ts
useEffect(() => {
  if (reEditQueueRef.current && mode === 'image') {
    applyReEdit(reEditQueueRef.current);
    reEditQueueRef.current = null;
  }
}, [mode]);
```

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/user/src/pages/create/CreateStudioPage.tsx
git commit -m "feat(reedit): add handleReEdit callback and reEditQueueRef for mode-switch handling"
```

---

### Task 6: Add onReEdit callback prop to WorkCard and the "二次编辑" button

**Files:**
- Modify: `frontend/apps/user/src/pages/create/CreateStudioPage.tsx:712` (WorkCard signature), `816-834` (button area), `543-551` (WorkCard invocation)

**Interfaces:**
- Consumes: `GenerationTask` type
- Produces: `onReEdit?: (item: GenerationTask) => void` prop on WorkCard

- [ ] **Step 1: Update WorkCard props signature**

Change line 712 from:

```ts
function WorkCard({ item, onOpen, onUsePrompt }: { item: GenerationTask; onOpen: (preview: { url: string; type: 'image' | 'video'; title: string }) => void; onUsePrompt?: (prompt: string) => void }) {
```

To:

```ts
function WorkCard({ item, onOpen, onUsePrompt, onReEdit }: { item: GenerationTask; onOpen: (preview: { url: string; type: 'image' | 'video'; title: string }) => void; onUsePrompt?: (prompt: string) => void; onReEdit?: (item: GenerationTask) => void }) {
```

- [ ] **Step 2: Add the "二次编辑" button in the expanded section**

In the expanded buttons area (lines 816-834), add a new button after the "使用此提示词" button and before the "收起" button. Insert after line 834 (closing of onUsePrompt block):

```tsx
{onReEdit && item.kind === 'image' && item.status === 2 && item.results?.[0]?.url && (
  <button
    type="button"
    onClick={() => onReEdit(item)}
    className="inline-flex items-center gap-1 rounded bg-sky-500 px-2 py-1 text-xs text-white hover:bg-sky-600"
  >
    <Sparkles size={12} />
    二次编辑
  </button>
)}
```

Note: `Sparkles` is already imported at line 21.

- [ ] **Step 3: Pass onReEdit to WorkCard in the parent component**

Change the WorkCard invocation (lines 543-551) from:

```tsx
<WorkCard 
  key={item.task_id} 
  item={item} 
  onOpen={setPreview}
  onUsePrompt={(p) => {
    setPrompt(p);
    promptRef.current?.focus();
  }}
/>
```

To:

```tsx
<WorkCard 
  key={item.task_id} 
  item={item} 
  onOpen={setPreview}
  onUsePrompt={(p) => {
    setPrompt(p);
    promptRef.current?.focus();
  }}
  onReEdit={handleReEdit}
/>
```

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/user/src/pages/create/CreateStudioPage.tsx
git commit -m "feat(reedit): add onReEdit prop to WorkCard with 二次编辑 button"
```

---

### Task 7: Verify end-to-end flow manually

**Files:**
- No file changes — manual verification

- [ ] **Step 1: Start the frontend dev server**

```bash
cd /vol3/1000/workspace/gpt/gpt2api/frontend
npm run dev --workspace=apps/user
```

- [ ] **Step 2: Verify file upload still works**

1. Navigate to `/create/image`
2. Enter a prompt
3. Click the paperclip icon, select a local image file
4. Confirm the thumbnail appears in the attachment area
5. Click the X on the thumbnail to remove it — confirm it disappears
6. Submit a generation request with an attachment — confirm `ref_assets` contains a `data:` URL

- [ ] **Step 3: Verify the "二次编辑" button appears correctly**

1. Find a completed image task in "My Works"
2. Click the prompt text area to expand it
3. Confirm three buttons appear: "复制", "使用此提示词", "二次编辑"
4. Confirm "二次编辑" only appears for `kind === 'image'` tasks with `status === 2`
5. Confirm it does NOT appear for video tasks or failed/pending tasks

- [ ] **Step 4: Verify the re-edit flow works**

1. Click "二次编辑" on a completed image task
2. Confirm the prompt textarea fills with the original prompt
3. Confirm the attachment area shows the original image thumbnail (using the cached URL)
4. Confirm the X button on the thumbnail works to remove it
5. Modify the prompt and submit — confirm the API request `ref_assets` contains the cached URL (not a `data:` URL)

- [ ] **Step 5: Verify cross-mode re-edit**

1. Navigate to `/create/video` or `/create/text`
2. In "My Works", expand a completed image task
3. Click "二次编辑"
4. Confirm the page navigates to `/create/image`
5. Confirm the prompt and attachment are filled in after navigation
6. Confirm the textarea is focused

- [ ] **Step 6: Final commit (if any minor fixes needed)**

```bash
git add frontend/apps/user/src/pages/create/CreateStudioPage.tsx
git commit -m "feat(reedit): minor fixes from manual verification"
```

---

## Self-Review

**1. Spec coverage:**
- ✅ AttachmentItem type (spec section 1) → Task 1
- ✅ Submit logic with both types (spec section 2) → Task 3
- ✅ Attachment display (spec section 3) → Task 4
- ✅ WorkCard "二次编辑" button (spec section 4) → Task 6
- ✅ handleReEdit/applyReEdit callbacks (spec section 5) → Task 5
- ✅ Mode-switch reEditQueueRef (spec section 6) → Task 5
- ✅ Single-image only (spec section 7) → enforced by using `results[0]` only
- ✅ No backend changes → confirmed throughout
- ✅ No HistoryPage changes → not touched

**2. Placeholder scan:** No TBDs, TODOs, or vague steps. All code blocks are complete.

**3. Type consistency:**
- `AttachmentItem` type defined in Task 1, used consistently in Tasks 2-6
- `refAssets()` helper uses `a.type === 'data' ? a.dataUrl : a.url` — matches the union type fields
- `handleReEdit` takes `GenerationTask` — matches WorkCard's `onReEdit` prop type
- `applyReEdit` creates `{ type: 'url', id, name, url, thumbUrl }` — matches `AttachmentItem` union variant
- No naming conflicts across tasks
