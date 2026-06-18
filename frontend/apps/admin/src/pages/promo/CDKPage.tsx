import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Ticket, AlertCircle, CheckCircle2, RefreshCw, Search, Eye, Plus, Copy, Trash2 } from 'lucide-react';
import { useState } from 'react';

import { ApiError } from '../../lib/api';
import { cdkApi } from '../../lib/services';
import type { CDKBatchItem, CDKCreateBatchBody, CDKCreateBatchResp } from '../../lib/types';
import { fmtNumber, fmtPoints, fmtTime } from '../../lib/format';
import { toast } from '../../stores/toast';

export default function CDKPage() {
  const qc = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState<'' | 0 | 1>('');
  const [page, setPage] = useState(1);
  const pageSize = 20;

  // 选中批次 → 弹窗查看码列表
  const [selectedBatch, setSelectedBatch] = useState<CDKBatchItem | null>(null);
  const [codeStatus, setCodeStatus] = useState<'' | 0 | 1 | 2>('');
  const [codePage, setCodePage] = useState(1);
  const codePageSize = 20;

  // 生成表单
  const [showForm, setShowForm] = useState(false);
  const [body, setBody] = useState<CDKCreateBatchBody>({
    batch_no: '',
    name: '',
    points: 1000,
    qty: 100,
    per_user_limit: 1,
    expire_at: 0,
  });
  const [lastCreated, setLastCreated] = useState<CDKCreateBatchResp | null>(null);

  // === 批次列表 ===
  const batchQuery = useQuery({
    queryKey: ['admin', 'cdk', 'batches', keyword, statusFilter, page],
    queryFn: () =>
      cdkApi.listBatches({
        keyword: keyword.trim() || undefined,
        status: statusFilter === '' ? undefined : statusFilter,
        page,
        page_size: pageSize,
      }),
  });

  const batches = batchQuery.data?.list ?? [];
  const batchTotal = batchQuery.data?.total ?? 0;
  const batchPages = Math.max(1, Math.ceil(batchTotal / pageSize));

  // === 码列表 ===
  const codeQuery = useQuery({
    queryKey: ['admin', 'cdk', 'codes', selectedBatch?.id, codeStatus, codePage],
    queryFn: () =>
      cdkApi.listCodes({
        batch_id: selectedBatch!.id,
        status: codeStatus === '' ? undefined : codeStatus,
        page: codePage,
        page_size: codePageSize,
      }),
    enabled: selectedBatch !== null,
  });

  const codes = codeQuery.data?.list ?? [];
  const codeTotal = codeQuery.data?.total ?? 0;
  const codePages = Math.max(1, Math.ceil(codeTotal / codePageSize));

  // === 生成 ===
  const createMut = useMutation({
    mutationFn: (b: CDKCreateBatchBody) => cdkApi.createBatch(b),
    onSuccess: (r) => {
      toast.success(`已生成批次 ${r.batch_no}（共 ${r.total_qty} 张）`);
      setLastCreated(r);
      setShowForm(false);
      qc.invalidateQueries({ queryKey: ['admin', 'cdk', 'batches'] });
    },
    onError: (e: ApiError) => toast.error(e.message),
  });

  // === 删除批次 ===
  const deleteMut = useMutation({
    mutationFn: (id: number) => cdkApi.deleteBatch(id),
    onSuccess: () => {
      toast.success('批次已删除');
      qc.invalidateQueries({ queryKey: ['admin', 'cdk', 'batches'] });
    },
    onError: (e: ApiError) => toast.error(e.message),
  });

  // === 切换批次状态 ===
  const toggleMut = useMutation({
    mutationFn: (b: CDKBatchItem) => cdkApi.toggleBatchStatus(b.id, b.status === 1 ? 0 : 1),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'cdk', 'batches'] }),
    onError: (e: ApiError) => toast.error(e.message),
  });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!body.batch_no.trim() || !body.name.trim()) {
      toast.error('请填写批次号和名称');
      return;
    }
    if (body.points <= 0 || body.qty <= 0) {
      toast.error('点数和数量必须 > 0');
      return;
    }
    createMut.mutate({
      ...body,
      batch_no: body.batch_no.trim(),
      name: body.name.trim(),
      per_user_limit: body.per_user_limit || 0,
      expire_at: body.expire_at || undefined,
    });
  };

  const copyToClipboard = async (text: string): Promise<boolean> => {
    // Try modern clipboard API first
    if (navigator.clipboard && navigator.clipboard.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch (e) {
        // Fall through to fallback
      }
    }
    // Fallback: use textarea and execCommand
    try {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.left = '-999999px';
      textarea.style.top = '-999999px';
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();
      const success = document.execCommand('copy');
      document.body.removeChild(textarea);
      return success;
    } catch (e) {
      return false;
    }
  };

  const copyCode = (code: string) => {
    copyToClipboard(code).then((ok) => {
      if (ok) toast.success('已复制');
      else toast.error('复制失败，请手动复制');
    });
  };

  const copyAllCodes = async () => {
    if (!selectedBatch) return;
    if (codeTotal === 0) {
      toast.info('当前批次没有可复制的兑换码');
      return;
    }
    
    toast.info('正在加载全部兑换码...');
    
    try {
      // Fetch all codes by iterating through pages
      const allCodes: string[] = [];
      const pageSize = 100; // Use larger page size for efficiency
      const totalPages = Math.ceil(codeTotal / pageSize);
      
      for (let pageNum = 1; pageNum <= totalPages; pageNum++) {
        const res = await cdkApi.listCodes({
          batch_id: selectedBatch.id,
          status: codeStatus === '' ? undefined : codeStatus,
          page: pageNum,
          page_size: pageSize,
        });
        
        if (res.list) {
          allCodes.push(...res.list.map((c) => c.code));
        }
      }
      
      if (allCodes.length === 0) {
        toast.info('没有可复制的兑换码');
        return;
      }
      
      const text = allCodes.join('\n');
      const ok = await copyToClipboard(text);
      if (ok) toast.success(`已复制 ${allCodes.length} 个兑换码`);
      else toast.error('复制失败，请手动复制');
    } catch (e) {
      toast.error('复制失败：' + (e instanceof Error ? e.message : '未知错误'));
    }
  };

  const parseRewardPoints = (rv: string): number => {
    try { const obj = JSON.parse(rv); return obj.points ?? 0; } catch { return 0; }
  };

  const cdkStatusLabel = (s: number) => {
    if (s === 0) return { label: '未使用', tone: 'ok' };
    if (s === 1) return { label: '已使用', tone: 'mute' };
    if (s === 2) return { label: '无效', tone: 'err' };
    return { label: String(s), tone: 'mute' };
  };

  return (
    <div className="page page-wide space-y-4">
      <header className="page-header">
        <div>
          <h1 className="page-title flex items-center gap-2">
            <Ticket className="text-klein-500" size={26} />
            兑换码 CDK
          </h1>
          <p className="page-subtitle">
            按批次生成和管理 CDK；每张 CDK 只能被使用一次，使用后写入 wallet_log 并入账。
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button className="btn btn-outline btn-md" onClick={() => batchQuery.refetch()} disabled={batchQuery.isFetching}>
            <RefreshCw size={16} className={batchQuery.isFetching ? 'animate-spin' : ''} /> 刷新
          </button>
          <button className="btn btn-primary btn-md" onClick={() => setShowForm(true)}>
            <Plus size={16} /> 生成批次
          </button>
        </div>
      </header>

      {/* 批次筛选 */}
      <div className="card card-section flex flex-wrap items-center gap-2 !py-3">
        <div className="relative min-w-[220px] flex-1">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-text-tertiary" />
          <input className="input pl-9" value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} placeholder="搜索批次号、名称" />
        </div>
        <select className="select select-sm min-w-[110px]" value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value as typeof statusFilter); setPage(1); }}>
          <option value="">全部状态</option>
          <option value="1">启用</option>
          <option value="0">停用</option>
        </select>
      </div>

      {/* 批次列表 */}
      <div className="card table-wrap">
        <table className="data-table min-w-[900px]">
          <thead>
            <tr>
              <th>ID</th>
              <th>批次号</th>
              <th>名称</th>
              <th>单码点数</th>
              <th>数量</th>
              <th>已用</th>
              <th>每用户限领</th>
              <th>过期时间</th>
              <th>状态</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {batches.map((b) => (
              <tr key={b.id}>
                <td>{b.id}</td>
                <td><code className="kbd">{b.batch_no}</code></td>
                <td>{b.name}</td>
                <td className="font-semibold">{fmtPoints(parseRewardPoints(b.reward_value))} 点</td>
                <td>{fmtNumber(b.total_qty)}</td>
                <td>{fmtNumber(b.used_qty)}</td>
                <td>{b.per_user_limit > 0 ? `${b.per_user_limit} 次` : '不限'}</td>
                <td>{fmtTime(b.expire_at)}</td>
                <td>
                  <button className={b.status === 1 ? 'btn btn-outline btn-sm' : 'btn btn-ghost btn-sm'} onClick={() => toggleMut.mutate(b)} disabled={toggleMut.isPending}>
                    {b.status === 1 ? '启用' : '停用'}
                  </button>
                </td>
                <td className="whitespace-nowrap">{fmtTime(b.created_at)}</td>
                <td>
                  <div className="flex items-center gap-1">
                    <button className="btn btn-ghost btn-icon btn-sm" onClick={() => { setSelectedBatch(b); setCodePage(1); setCodeStatus(''); }} title="查看码列表">
                      <Eye size={14} />
                    </button>
                    <button className="btn btn-danger-ghost btn-icon btn-sm" onClick={() => { if (confirm(`删除批次 ${b.batch_no}（${b.name}）及其所有 ${b.total_qty} 张兑换码？此操作不可撤销。`)) deleteMut.mutate(b.id); }} title="删除批次" disabled={deleteMut.isPending}>
                      <Trash2 size={14} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {!batchQuery.isLoading && batches.length === 0 && (
              <tr><td colSpan={11} className="py-10 text-center text-text-tertiary">暂无批次</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* 批次分页 */}
      <div className="card card-section flex flex-wrap items-center justify-between gap-3 !py-2">
        <span className="text-small text-text-tertiary">第 {page} / {batchPages} 页，共 {fmtNumber(batchTotal)} 条</span>
        <div className="flex gap-2">
          <button className="btn btn-outline btn-sm" disabled={page <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>上一页</button>
          <button className="btn btn-outline btn-sm" disabled={page >= batchPages} onClick={() => setPage((p) => p + 1)}>下一页</button>
        </div>
      </div>

      {/* 弹窗：码列表 */}
      {selectedBatch && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-surface-overlay p-4">
          <div className="card card-section w-full max-w-4xl space-y-4">
            <header className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-h4 font-semibold text-text-primary">
                  批次 <code className="kbd">{selectedBatch.batch_no}</code> 的兑换码
                </h2>
                <p className="text-small text-text-tertiary">
                  {selectedBatch.name} · 单码 {fmtPoints(parseRewardPoints(selectedBatch.reward_value))} 点 · 共 {fmtNumber(selectedBatch.total_qty)} 张
                </p>
              </div>
              <button className="btn btn-ghost btn-sm" onClick={() => setSelectedBatch(null)}>关闭</button>
            </header>

            <div className="flex items-center gap-2">
              <button className="btn btn-outline btn-sm" onClick={copyAllCodes} disabled={codeTotal === 0}>
                <Copy size={14} className="mr-1" /> 复制全部 ({codeTotal})
              </button>
              <select className="select select-sm min-w-[120px]" value={codeStatus} onChange={(e) => { setCodeStatus(e.target.value as typeof codeStatus); setCodePage(1); }}>
                <option value="">全部状态</option>
                <option value="0">未使用</option>
                <option value="1">已使用</option>
                <option value="2">无效</option>
              </select>
            </div>

            <div className="table-wrap max-h-[60vh] overflow-auto">
              <table className="data-table min-w-[700px]">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>兑换码</th>
                    <th>状态</th>
                    <th>使用者</th>
                    <th>使用时间</th>
                    <th>创建时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {codes.map((co) => {
                    const sl = cdkStatusLabel(co.status);
                    return (
                      <tr key={co.id}>
                        <td>{co.id}</td>
                        <td><code className="kbd">{co.code}</code></td>
                        <td><span className={sl.tone === 'ok' ? 'text-success' : sl.tone === 'err' ? 'text-danger' : 'text-text-tertiary'}>{sl.label}</span></td>
                        <td>{co.used_by ?? '—'}</td>
                        <td className="whitespace-nowrap">{fmtTime(co.used_at)}</td>
                        <td className="whitespace-nowrap">{fmtTime(co.created_at)}</td>
                        <td>
                          <button className="btn btn-ghost btn-icon btn-sm" onClick={() => copyCode(co.code)} title="复制码">
                            <Copy size={14} />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                  {!codeQuery.isLoading && codes.length === 0 && (
                    <tr><td colSpan={7} className="py-10 text-center text-text-tertiary">暂无兑换码</td></tr>
                  )}
                </tbody>
              </table>
            </div>

            <div className="flex items-center justify-between gap-3">
              <span className="text-small text-text-tertiary">第 {codePage} / {codePages} 页，共 {fmtNumber(codeTotal)} 条</span>
              <div className="flex gap-2">
                <button className="btn btn-outline btn-sm" disabled={codePage <= 1} onClick={() => setCodePage((p) => Math.max(1, p - 1))}>上一页</button>
                <button className="btn btn-outline btn-sm" disabled={codePage >= codePages} onClick={() => setCodePage((p) => p + 1)}>下一页</button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 弹窗：生成批次表单 */}
      {showForm && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-surface-overlay p-4">
          <form onSubmit={submit} className="card card-section w-full max-w-3xl space-y-4">
            <header className="flex items-center justify-between gap-3">
              <h2 className="text-h4 font-semibold text-text-primary">生成 CDK 批次</h2>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowForm(false)}>关闭</button>
            </header>

            <div className="grid gap-3 md:grid-cols-2">
              <Field label="批次号" hint="同批次唯一，如 SPRING2026-A">
                <input className="input" value={body.batch_no} onChange={(e) => setBody((s) => ({ ...s, batch_no: e.target.value }))} placeholder="SPRING2026-A" />
              </Field>
              <Field label="批次名称" hint="展示给运营 / 客服的友好名称">
                <input className="input" value={body.name} onChange={(e) => setBody((s) => ({ ...s, name: e.target.value }))} placeholder="春节活动 100 点" />
              </Field>
              <Field label="单码点数（×100 储存）" hint={`输入 1000 = 实际 10.00 点；当前等价：${fmtPoints(body.points)} 点`}>
                <input type="number" min={1} className="input" value={body.points} onChange={(e) => setBody((s) => ({ ...s, points: Math.max(1, Number(e.target.value) || 0) }))} />
              </Field>
              <Field label="生成数量" hint="单批次最多 100,000 张">
                <input type="number" min={1} max={100_000} className="input" value={body.qty} onChange={(e) => setBody((s) => ({ ...s, qty: Math.max(1, Number(e.target.value) || 0) }))} />
              </Field>
              <Field label="每用户限领次数" hint="0 表示不限制；建议 1（防止羊毛党）">
                <input type="number" min={0} className="input" value={body.per_user_limit ?? 0} onChange={(e) => setBody((s) => ({ ...s, per_user_limit: Number(e.target.value) || 0 }))} />
              </Field>
              <Field label="过期时间（可选）" hint="留空表示永久有效">
                <input type="datetime-local" className="input" onChange={(e) => { const v = e.target.value; if (!v) { setBody((s) => ({ ...s, expire_at: 0 })); return; } const t = Math.floor(new Date(v).getTime() / 1000); setBody((s) => ({ ...s, expire_at: t })); }} />
              </Field>
            </div>

            <div className="flex flex-col items-stretch justify-between gap-3 rounded-md bg-klein-gradient-soft p-4 md:flex-row md:items-center">
              <div className="flex items-center gap-2 text-small text-text-secondary">
                <AlertCircle size={16} className="text-klein-500" />
                预计生成：
                <strong className="text-text-primary mx-1">{fmtNumber(body.qty)}</strong>张，单码价值
                <strong className="text-text-primary mx-1">{fmtPoints(body.points)} 点</strong>，合计
                <strong className="text-klein-500 mx-1">{fmtPoints(body.points * body.qty)} 点</strong>
              </div>
              <div className="flex justify-end gap-2">
                <button className="btn btn-outline btn-md" onClick={() => setShowForm(false)}>取消</button>
                <button type="submit" className="btn btn-primary btn-md" disabled={createMut.isPending}>{createMut.isPending ? '生成中…' : '生成批次'}</button>
              </div>
            </div>

            {lastCreated && (
              <div className="flex items-start gap-3 rounded-md border border-success/40 p-3">
                <CheckCircle2 className="text-success shrink-0 mt-0.5" size={20} />
                <div className="flex-1 space-y-1">
                  <p className="text-text-primary font-medium">最新生成成功</p>
                  <p className="text-small text-text-secondary">
                    批次 ID #{lastCreated.id} · 批次号<code className="kbd mx-1">{lastCreated.batch_no}</code> · 共 {fmtNumber(lastCreated.total_qty)} 张
                  </p>
                </div>
              </div>
            )}
          </form>
        </div>
      )}
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: React.ReactNode; children: React.ReactNode }) {
  return <label className="field"><span className="field-label">{label}</span>{children}{hint && <span className="field-hint">{hint}</span>}</label>;
}