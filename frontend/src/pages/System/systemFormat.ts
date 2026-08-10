import { fmtNumber } from '@/lib/format';
import type { EdgesAgg } from './systemTypes';

export function num(v: unknown): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

export function fmtBytes(bytes: unknown): string {
  const b = num(bytes);
  if (b < 1024) return `${fmtNumber(b)} Б`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} КБ`;
  if (b < 1024 * 1024 * 1024) return `${(b / (1024 * 1024)).toFixed(1)} МБ`;
  return `${(b / (1024 * 1024 * 1024)).toFixed(2)} ГБ`;
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

export function edgesAggHint(edges?: EdgesAgg): string {
  if (!edges) return '';
  if (edges.message) return edges.message;
  const state = edges.state || 'idle';
  const phase = edges.phase || '';
  if (state === 'running' && phase === 'schema') {
    return 'Идёт DROP/CREATE MV — карта временно на сырых traffic_logs.';
  }
  if (state === 'running') {
    return 'Backfill агрегатов — карта на traffic_logs до state=ready.';
  }
  if (state === 'ready') {
    return 'Агрегаты готовы — /api/events предпочитает edges_daily.';
  }
  if (state === 'error') {
    return 'Ошибка Ensure*/backfill — перезапустите backend или см. логи.';
  }
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
