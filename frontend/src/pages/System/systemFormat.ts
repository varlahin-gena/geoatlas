import { fmtBytes as fmtBytesShared, fmtNumber } from '@/lib/format';
import type { EdgesAgg } from './systemTypes';

export function num(v: unknown): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

/** @deprecated Prefer `@/lib/format` — kept for system pages. */
export function fmtBytes(bytes: unknown): string {
  return fmtBytesShared(bytes);
}

/** Axis ticks share one unit (from max split) so labels stay ordered and readable. */
export function fmtBytesAxisTicks(splits: number[]): string[] {
  const finite = splits.filter((v) => v != null && Number.isFinite(v));
  const maxAbs = finite.reduce((m, v) => Math.max(m, Math.abs(v)), 0);
  let div = 1;
  let suffix = 'Б';
  let digits = 0;
  if (maxAbs >= 1024 * 1024 * 1024) {
    div = 1024 * 1024 * 1024;
    suffix = 'ГБ';
    digits = 2;
  } else if (maxAbs >= 1024 * 1024) {
    div = 1024 * 1024;
    suffix = 'МБ';
    digits = 1;
  } else if (maxAbs >= 1024) {
    div = 1024;
    suffix = 'КБ';
    digits = 1;
  }
  return splits.map((v) => {
    if (v == null || !Number.isFinite(v)) return '';
    if (div === 1) return `${fmtNumber(v)} ${suffix}`;
    return `${(v / div).toFixed(digits)} ${suffix}`;
  });
}

export function fmtLag(sec: unknown): string {
  if (sec == null || sec === '') return '—';
  const s = num(sec);
  if (s < 1) return '<1 с';
  if (s < 60) return `${Math.round(s)} с`;
  if (s < 3600) return `${Math.round(s / 60)} мин`;
  return `${(s / 3600).toFixed(1)} ч`;
}

const HEALTH_STATE_LABEL: Record<string, string> = {
  up: 'работает',
  down: 'недоступен',
  unknown: 'неизвестно',
  running: 'работает',
  idle: 'простой',
  stopped: 'остановлен',
  ok: 'в норме',
  warn: 'внимание',
  error: 'ошибка',
  degraded: 'ограничен',
};

const CONTAINER_LABEL: Record<string, string> = {
  backend: 'Сервер',
  clickhouse: 'ClickHouse',
  'syslog-ng': 'Syslog',
  frontend: 'Интерфейс',
};

/** Health badge: up/down/running → Russian, unknown tokens pass through. */
export function healthStateLabel(raw?: string | null): string {
  if (raw == null || raw === '') return 'неизвестно';
  const key = String(raw).trim().toLowerCase();
  return HEALTH_STATE_LABEL[key] || String(raw);
}

export function containerLabel(name: string): string {
  return CONTAINER_LABEL[name] || name;
}

export function capacityHint(capPct: number): string {
  if (capPct > 125) {
    return 'Нагрузка выше лимита профиля — увеличьте профиль или снизьте входящий поток.';
  }
  return 'Нагрузка близка к лимиту профиля.';
}

export function fmtUptime(sec: unknown): string {
  const s = num(sec);
  if (s < 60) return `${Math.round(s)} с`;
  if (s < 3600) return `${Math.round(s / 60)} мин`;
  return `${(s / 3600).toFixed(1)} ч`;
}

export function toneClass(kind: 'ok' | 'warn' | 'bad' | ''): string {
  return kind || '';
}

export function queueTone(depth: number, capacity: number): 'ok' | 'warn' | 'bad' {
  if (!capacity) return 'ok';
  const r = depth / capacity;
  if (r >= 0.9) return 'bad';
  if (r >= 0.75) return 'warn';
  return 'ok';
}

export function fmtPercent(ratio: number): string {
  return `${(ratio * 100).toFixed(1)}%`;
}

export function pipelineIngestStatus(
  rate: Record<string, number>,
  ingest: Record<string, number>,
  queuePct: number,
): 'ok' | 'warn' | 'bad' {
  const drops = num(rate.drops_per_sec);
  const bufDrops = num(rate.buffer_drops_per_sec);
  const dropped = num(ingest.dropped_total);
  const bufDropped = num(ingest.buffer_drops_total);
  const buffered = num(ingest.buffered_lines);
  if (drops >= 100 || bufDrops >= 100) return 'bad';
  if (queuePct >= 0.9) return 'bad';
  if (queuePct >= 0.75) return 'warn';
  if (drops > 0 || bufDrops > 0 || dropped > 0 || bufDropped > 0) return 'warn';
  if (buffered > 100000) return 'bad';
  if (buffered > 10000) return 'warn';
  if (num(ingest.circuit_open) >= 1) return 'warn';
  return 'ok';
}

export function pipelineSyslogStatus(
  syslogng: Record<string, number>,
  up: number | undefined,
  fifoSize: number,
  dropsPerSec: number,
): 'ok' | 'warn' | 'bad' {
  if (up === 0) return 'bad';
  if (dropsPerSec >= 100) return 'bad';
  if (dropsPerSec > 0) return 'warn';
  if (num(syslogng.dropped_total) > 0) return 'warn';
  const cap = fifoSize * 2;
  if (cap > 0) {
    const ratio = num(syslogng.queued) / cap;
    if (ratio >= 0.9) return 'bad';
    if (ratio >= 0.75) return 'warn';
  }
  return 'ok';
}

export function dropsTone(admPerSec: number, bufPerSec: number): 'ok' | 'warn' | 'bad' {
  const total = admPerSec + bufPerSec;
  if (total >= 100) return 'bad';
  if (total > 0) return 'warn';
  return 'ok';
}

export function fmtDropAt(iso: unknown): string {
  if (iso == null || iso === '') return '—';
  const s = String(iso);
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  return d.toLocaleString();
}

const EDGES_STATE_LABEL: Record<string, string> = {
  idle: 'ожидание',
  pending: 'ожидает заполнения',
  running: 'в работе',
  ready: 'готово',
  error: 'ошибка',
};

const EDGES_PHASE_LABEL: Record<string, string> = {
  schema: 'схема',
  backfill: 'заполнение',
};

const EDGES_MESSAGE_HINT: Record<string, string> = {
  'up to date': 'Сводки актуальны — карта читает суточные данные.',
  'not started': 'Подготовка сводок ещё не запускалась.',
  'traffic_logs empty': 'Сырых логов пока нет — сводки появятся после первых событий.',
};

/** Badge for Overview: human status, not raw state/phase tokens. */
export function edgesAggBadge(edges?: EdgesAgg): string {
  if (!edges) return '—';
  const state = edges.state || 'idle';
  const stateLabel = EDGES_STATE_LABEL[state] || state;
  const phase = edges.phase || '';
  if (!phase || state === 'ready' || state === 'idle') return stateLabel;
  const phaseLabel = EDGES_PHASE_LABEL[phase] || phase;
  return `${stateLabel} · ${phaseLabel}`;
}

export function edgesMapSourceLabel(source?: string): string {
  if (!source) return '—';
  if (source === 'edges_daily') return 'суточные сводки';
  if (source === 'traffic_logs') return 'подробные логи';
  return source;
}

export function edgesAggMessageLabel(message?: string): string {
  if (!message) return '—';
  return EDGES_MESSAGE_HINT[message] || message;
}

export function edgesAggHint(edges?: EdgesAgg): string {
  if (!edges) return '';
  const msg = (edges.message || '').trim();
  if (msg && EDGES_MESSAGE_HINT[msg]) return EDGES_MESSAGE_HINT[msg];

  const state = edges.state || 'idle';
  const phase = edges.phase || '';
  if (state === 'running' && phase === 'schema') {
    return 'Готовится схема сводок — карта временно на подробных логах.';
  }
  if (state === 'running') {
    return 'Идёт заполнение сводок — карта пока на подробных логах.';
  }
  if (state === 'pending') {
    return 'Схема готова, заполнение истории ещё не запускалось.';
  }
  if (state === 'ready') {
    return 'Сводки готовы — карта читает суточные данные.';
  }
  if (state === 'error') {
    return msg
      ? `Ошибка подготовки сводок: ${msg}`
      : 'Ошибка подготовки сводок — перезапустите сервис или смотрите логи.';
  }
  if (msg) return msg;
  return '';
}

export function lagTone(sec: number): 'ok' | 'warn' | 'bad' | '' {
  if (!sec) return '';
  if (sec >= 60) return 'bad';
  if (sec >= 10) return 'warn';
  return 'ok';
}

export function bufferTone(n: number): 'ok' | 'warn' | 'bad' {
  if (n >= 100000) return 'bad';
  if (n >= 10000) return 'warn';
  return 'ok';
}

export function capacityTone(pct: number): 'ok' | 'warn' | 'bad' {
  if (pct >= 125) return 'bad';
  if (pct >= 90) return 'warn';
  return 'ok';
}
