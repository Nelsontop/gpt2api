import { useEffect, useState } from 'react';
import { systemApi } from '../lib/services';

type CronMode = 'hourly' | 'daily' | 'weekly' | 'custom';

const WEEKDAYS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];

function pad2(n: number) {
  return n < 10 ? `0${n}` : String(n);
}

// 把 cron 解析成结构化模式；匹配不上的归 custom。
function parseCron(expr: string): { mode: CronMode; hourlyInterval: number; dailyHour: number; dailyMinute: number; weeklyDay: number } {
  const parts = expr.trim().split(/\s+/);
  const fallback = { mode: 'custom' as CronMode, hourlyInterval: 1, dailyHour: 9, dailyMinute: 0, weeklyDay: 1 };
  if (parts.length !== 5) return { ...fallback, mode: 'custom' };
  const m = parts[0] ?? '';
  const h = parts[1] ?? '';
  const dom = parts[2] ?? '';
  const mon = parts[3] ?? '';
  const dow = parts[4] ?? '';
  if (dom === '*' && mon === '*' && dow === '*' && h.startsWith('*/')) {
    const n = parseInt(h.slice(2), 10);
    if (!Number.isNaN(n) && m === '0') return { mode: 'hourly', hourlyInterval: n || 1, dailyHour: 9, dailyMinute: 0, weeklyDay: 1 };
  }
  if (dom === '*' && mon === '*' && dow === '*') {
    const hh = parseInt(h, 10);
    const mm = parseInt(m, 10);
    if (!Number.isNaN(hh) && !Number.isNaN(mm) && hh >= 0 && hh <= 23 && mm >= 0 && mm <= 59) {
      return { mode: 'daily', hourlyInterval: 1, dailyHour: hh, dailyMinute: mm, weeklyDay: 1 };
    }
  }
  if (dom === '*' && mon === '*' && dow !== '*') {
    const hh = parseInt(h, 10);
    const mm = parseInt(m, 10);
    const dd = parseInt(dow, 10);
    if (!Number.isNaN(hh) && !Number.isNaN(mm) && !Number.isNaN(dd) && hh >= 0 && hh <= 23 && mm >= 0 && mm <= 59 && dd >= 0 && dd <= 6) {
      return { mode: 'weekly', hourlyInterval: 1, dailyHour: hh, dailyMinute: mm, weeklyDay: dd };
    }
  }
  return { ...fallback, mode: 'custom' };
}

function buildCron(mode: CronMode, hourlyInterval: number, dailyHour: number, dailyMinute: number, weeklyDay: number): string {
  switch (mode) {
    case 'hourly':
      return `0 */${Math.max(1, hourlyInterval)} * * *`;
    case 'daily':
      return `${pad2(dailyMinute)} ${pad2(dailyHour)} * * *`;
    case 'weekly':
      return `${pad2(dailyMinute)} ${pad2(dailyHour)} * * ${weeklyDay}`;
    case 'custom':
    default:
      return '';
  }
}

export default function CronEditor({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
}) {
  const parsed = parseCron(value);
  const [mode, setMode] = useState<CronMode>(parsed.mode);
  const [hourlyInterval, setHourlyInterval] = useState(parsed.hourlyInterval);
  const [dailyHour, setDailyHour] = useState(parsed.dailyHour);
  const [dailyMinute, setDailyMinute] = useState(parsed.dailyMinute);
  const [weeklyDay, setWeeklyDay] = useState(parsed.weeklyDay);
  const [customExpr, setCustomExpr] = useState(value);

  // 当外部 value 变化（如加载完成或切换预设），重新解析模式。
  useEffect(() => {
    const p = parseCron(value);
    setMode(p.mode);
    setHourlyInterval(p.hourlyInterval);
    setDailyHour(p.dailyHour);
    setDailyMinute(p.dailyMinute);
    setWeeklyDay(p.weeklyDay);
    setCustomExpr(value);
  }, [value]);

  // 模式或字段变化时，生成新 cron 并回调。
  useEffect(() => {
    if (mode === 'custom') {
      if (customExpr && customExpr !== value) onChange(customExpr);
      return;
    }
    const next = buildCron(mode, hourlyInterval, dailyHour, dailyMinute, weeklyDay);
    if (next && next !== value) onChange(next);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, hourlyInterval, dailyHour, dailyMinute, weeklyDay, customExpr]);

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <select
          className="select"
          value={mode}
          onChange={(e) => setMode(e.target.value as CronMode)}
          disabled={disabled}
        >
          <option value="hourly">每小时</option>
          <option value="daily">每天</option>
          <option value="weekly">每周</option>
          <option value="custom">自定义</option>
        </select>

        {mode === 'hourly' && (
          <label className="flex items-center gap-1 text-small text-text-secondary">
            每
            <input
              type="number"
              className="input w-20"
              min={1}
              max={23}
              value={hourlyInterval}
              onChange={(e) => setHourlyInterval(Math.max(1, Math.min(23, Number(e.target.value) || 1)))}
              disabled={disabled}
            />
            小时
          </label>
        )}

        {mode === 'daily' && (
          <label className="flex items-center gap-1 text-small text-text-secondary">
            时间
            <input
              type="time"
              className="input w-28"
              value={`${pad2(dailyHour)}:${pad2(dailyMinute)}`}
              onChange={(e) => {
                const [h, m] = e.target.value.split(':').map(Number);
                setDailyHour(h || 0);
                setDailyMinute(m || 0);
              }}
              disabled={disabled}
            />
          </label>
        )}

        {mode === 'weekly' && (
          <>
            <label className="flex items-center gap-1 text-small text-text-secondary">
              星期
              <select
                className="select w-24"
                value={weeklyDay}
                onChange={(e) => setWeeklyDay(Number(e.target.value))}
                disabled={disabled}
              >
                {WEEKDAYS.map((label, idx) => (
                  <option key={idx} value={idx}>{label}</option>
                ))}
              </select>
            </label>
            <label className="flex items-center gap-1 text-small text-text-secondary">
              时间
              <input
                type="time"
                className="input w-28"
                value={`${pad2(dailyHour)}:${pad2(dailyMinute)}`}
                onChange={(e) => {
                  const [h, m] = e.target.value.split(':').map(Number);
                  setDailyHour(h || 0);
                  setDailyMinute(m || 0);
                }}
                disabled={disabled}
              />
            </label>
          </>
        )}

        {mode === 'custom' && (
          <input
            className="input font-mono flex-1 min-w-[200px]"
            type="text"
            value={customExpr}
            onChange={(e) => setCustomExpr(e.target.value)}
            disabled={disabled}
            placeholder="0 */1 * * *"
          />
        )}
      </div>

      {mode !== 'custom' && (
        <div className="text-tiny text-text-tertiary">
          当前 Cron: <code className="font-mono">{buildCron(mode, hourlyInterval, dailyHour, dailyMinute, weeklyDay)}</code>
        </div>
      )}

      <CronPreview expr={value} disabled={disabled} />
    </div>
  );
}

function CronPreview({ expr, disabled }: { expr: string; disabled?: boolean }) {
  const [times, setTimes] = useState<string[]>([]);
  const [error, setError] = useState<string>('');

  useEffect(() => {
    if (disabled || !expr.trim()) {
      setTimes([]);
      setError('');
      return;
    }
    const handle = window.setTimeout(async () => {
      try {
        const res = await systemApi.cronPreview(expr, 3);
        setTimes(res.times ?? []);
        setError('');
      } catch (e: any) {
        setTimes([]);
        setError(e?.message || '表达式无效');
      }
    }, 300);
    return () => window.clearTimeout(handle);
  }, [expr, disabled]);

  if (disabled) return null;
  if (error) return <div className="text-tiny text-danger">表达式无效：{error}</div>;
  if (times.length === 0) return null;
  return (
    <div className="text-tiny text-text-tertiary">
      下次执行时间：
      <ul className="mt-1 space-y-0.5">
        {times.map((t) => (
          <li key={t} className="font-mono">• {t}</li>
        ))}
      </ul>
    </div>
  );
}
