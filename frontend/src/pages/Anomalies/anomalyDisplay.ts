import type { AnomalyEvent } from '@/api/anomalies';
import { fmtDate, fmtNumber } from '@/lib/format';
import { anomalyMapToView } from '@/pages/Map/useMapAnomalies';
import { MAP_VIEW_DEFAULTS, serializeMapViewSearch } from '@/pages/Map/mapQuery';

export type AnomalySortKey = 'detected_at' | 'event_count';
export type AnomalySortDir = 'asc' | 'desc';

export type AnomalyDescriptionParts = {
  subject: string;
  detail?: string;
  delta?: string;
};

export const ANOMALY_CODE_OPTIONS = [
  { value: '', label: 'Все типы' },
  { value: 'port_scan', label: 'Сканирование портов' },
  { value: 'horizontal_scan', label: 'Сканирование подсети' },
  { value: 'blocked_surge', label: 'Всплеск блокировок' },
  { value: 'byte_surge', label: 'Всплеск объёма' },
  { value: 'beaconing', label: 'Периодическая связь' },
  { value: 'lateral_fanout', label: 'Веер по сети предприятия' },
  { value: 'new_country_dst', label: 'Новая страна назначения' },
  { value: 'rep_new_peer', label: 'Репутационная связь' },
  { value: 'hunt_threshold', label: 'Порог saved hunt' },
] as const;

export const ANOMALY_SINCE_OPTIONS = [
  { value: '24', label: '24 часа' },
  { value: '168', label: '7 суток' },
] as const;

export function sinceIsoHoursAgo(hours: number): string {
  return new Date(Date.now() - hours * 3_600_000).toISOString();
}

export function relTime(iso?: string): string {
  if (!iso) return '';
  const t = Date.parse(iso);
  if (!Number.isFinite(t)) return '';
  const sec = Math.max(0, Math.round((Date.now() - t) / 1000));
  if (sec < 60) return `${sec} с назад`;
  const min = Math.round(sec / 60);
  if (min < 60) return `${min} мин назад`;
  const h = Math.round(min / 60);
  if (h < 48) return `${h} ч назад`;
  return `${Math.round(h / 24)} дн назад`;
}

export function anomalyCodeLabel(code?: string): string {
  switch (code) {
    case 'port_scan':
      return 'Сканирование портов';
    case 'horizontal_scan':
      return 'Сканирование подсети';
    case 'blocked_surge':
      return 'Всплеск блокировок';
    case 'byte_surge':
      return 'Всплеск объёма';
    case 'beaconing':
      return 'Периодическая связь';
    case 'lateral_fanout':
      return 'Веер по сети предприятия';
    case 'new_country_dst':
      return 'Новая страна назначения';
    case 'rep_new_peer':
      return 'Репутационная связь';
    case 'hunt_threshold':
      return 'Порог saved hunt';
    default:
      return code || '';
  }
}

export function eventCodeLabel(item: AnomalyEvent): string {
  return item.code_label || anomalyCodeLabel(item.code);
}

export function severityLabel(severity?: string): string {
  switch (severity) {
    case 'high':
      return 'Критично';
    case 'warn':
      return 'Предупреждение';
    case 'info':
      return 'Информация';
    default:
      return severity || '—';
  }
}

export function anomalyMapHref(item: AnomalyEvent): string {
  // Keep the current map default period (1d) when opening from the list —
  // do not shrink to the alert's linked window.
  const partial = anomalyMapToView(item, MAP_VIEW_DEFAULTS.period);
  const sp = serializeMapViewSearch({ ...MAP_VIEW_DEFAULTS, ...partial });
  if (item.fingerprint) {
    sp.set('alert', item.fingerprint);
  }
  const qs = sp.toString();
  return qs ? `/?${qs}` : '/';
}

export function matchesAnomalySearch(item: AnomalyEvent, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const hay = [
    item.title,
    item.code,
    item.code_label,
    item.src_ip,
    item.dst_ip,
    item.src_country,
    item.dst_country,
    item.src_city,
    item.dst_city,
    item.device,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
  return hay.includes(q);
}

function detailRecord(item: AnomalyEvent): Record<string, unknown> | undefined {
  const d = item.detail;
  if (!d || typeof d !== 'object' || Array.isArray(d)) return undefined;
  return d as Record<string, unknown>;
}

function detailString(detail: Record<string, unknown> | undefined, key: string): string {
  const v = detail?.[key];
  return typeof v === 'string' ? v.trim() : '';
}

function detailNumber(detail: Record<string, unknown> | undefined, key: string): number | undefined {
  const v = detail?.[key];
  if (typeof v === 'number' && Number.isFinite(v)) return v;
  if (typeof v === 'string' && v.trim() !== '') {
    const n = Number(v);
    if (Number.isFinite(n)) return n;
  }
  return undefined;
}

function stripCodePrefix(title: string, codeLabel: string): string {
  const t = title.trim();
  const label = codeLabel.trim();
  if (!t || !label) return t;
  if (t.toLowerCase().startsWith(label.toLowerCase())) {
    return t.slice(label.length).replace(/^[\s.:—\-–]+/u, '').trim();
  }
  return t;
}

/** Structured description for the table: place / range / surge delta. */
export function formatAnomalyDescription(item: AnomalyEvent): AnomalyDescriptionParts {
  const detail = detailRecord(item);
  const code = (item.code || '').trim();
  const network = detailString(detail, 'network') || (item.device || '').trim();
  const label = detailString(detail, 'label');
  const curr =
    detailNumber(detail, 'blocked_current') ??
    detailNumber(detail, 'bytes_current') ??
    detailNumber(detail, 'current');
  const prev =
    detailNumber(detail, 'blocked_previous') ??
    detailNumber(detail, 'bytes_previous') ??
    detailNumber(detail, 'previous');
  const windowMin = detailNumber(detail, 'window_minutes');

  if (code === 'blocked_surge' || code === 'byte_surge') {
    const subject = label || network || stripCodePrefix(item.title || '', eventCodeLabel(item)) || '—';
    const detailLine = label && network && label !== network ? network : undefined;
    let delta: string | undefined;
    if (curr !== undefined && prev !== undefined) {
      const win = windowMin && windowMin > 0 ? ` за ${windowMin} мин` : '';
      delta = `${fmtNumber(curr)}${win} · было ${fmtNumber(prev)}`;
    }
    return { subject, detail: detailLine, delta };
  }

  const subject =
    stripCodePrefix(item.title || '', eventCodeLabel(item)) ||
    item.title?.trim() ||
    '—';
  return { subject };
}

/** Endpoint label: IP first, then city/country, then optional network fallback. */
export function anomalyEndpointLabel(
  ip?: string | null,
  city?: string | null,
  country?: string | null,
  fallback?: string | null,
): string {
  const addr = (ip || '').trim();
  if (addr) return addr;
  const place = [city, country].map((p) => (p || '').trim()).filter(Boolean).join(', ');
  if (place) return place;
  const fb = (fallback || '').trim();
  return fb || '';
}

/** Compact window label; full range stays in title. */
export function formatAnomalyWindow(
  start?: string | null,
  end?: string | null,
): { label: string; title: string } {
  const fullStart = start ? fmtDate(start) : '—';
  const fullEnd = end ? fmtDate(end) : '—';
  const title = `${fullStart} → ${fullEnd}`;
  if (!start && !end) return { label: '—', title: '' };

  const ds = start ? new Date(start) : null;
  const de = end ? new Date(end) : null;
  if (
    ds &&
    de &&
    !Number.isNaN(ds.getTime()) &&
    !Number.isNaN(de.getTime()) &&
    ds.getFullYear() === de.getFullYear() &&
    ds.getMonth() === de.getMonth() &&
    ds.getDate() === de.getDate()
  ) {
    const hm = (d: Date) =>
      d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
    return { label: `${hm(ds)}–${hm(de)}`, title };
  }
  return { label: title, title };
}

/** Human scope line for peers panel (API path stays in title attribute). */
export function anomalyPeersScopeLabel(item: AnomalyEvent, hours: number): string {
  const startMs = Date.parse(item.window_start || '');
  const endMs = Date.parse(item.window_end || item.detected_at || '');
  const parts =
    Number.isFinite(startMs) && Number.isFinite(endMs) && endMs >= startMs
      ? ['Связи за окно алерта']
      : [`Связи за ${hours} ч`];
  const q = anomalyEventsQuery(item).trim();
  if (!q) return parts[0];
  for (const token of q.split(/\s+/).filter(Boolean)) {
    const m = /^([a-z_]+):(.+)$/i.exec(token);
    if (!m) {
      parts.push(token);
      continue;
    }
    const key = m[1].toLowerCase();
    const val = m[2];
    if (key === 'city') parts.push(`город ${val}`);
    else if (key === 'src') parts.push(`источник ${val}`);
    else if (key === 'dst') parts.push(`назначение ${val}`);
    else if (key === 'country') parts.push(`страна ${val}`);
    else parts.push(`${key}:${val}`);
  }
  return parts.join(' · ');
}

export function anomalyNetworkFallback(item: AnomalyEvent): string {
  const detail = detailRecord(item);
  return detailString(detail, 'network') || (item.device || '').trim();
}

export function compareAnomalies(
  a: AnomalyEvent,
  b: AnomalyEvent,
  key: AnomalySortKey,
  dir: AnomalySortDir,
): number {
  let cmp = 0;
  if (key === 'event_count') {
    cmp = (a.event_count || 0) - (b.event_count || 0);
  } else {
    const ta = Date.parse(a.detected_at || '');
    const tb = Date.parse(b.detected_at || '');
    const na = Number.isFinite(ta) ? ta : 0;
    const nb = Number.isFinite(tb) ? tb : 0;
    cmp = na - nb;
  }
  if (cmp === 0) {
    cmp = (a.fingerprint || '').localeCompare(b.fingerprint || '');
  }
  return dir === 'asc' ? cmp : -cmp;
}

export function nextAnomalySort(
  currentKey: AnomalySortKey,
  currentDir: AnomalySortDir,
  nextKey: AnomalySortKey,
): { key: AnomalySortKey; dir: AnomalySortDir } {
  if (currentKey === nextKey) {
    return { key: currentKey, dir: currentDir === 'desc' ? 'asc' : 'desc' };
  }
  return { key: nextKey, dir: 'desc' };
}

/** Hours for GET /api/events from anomaly map.period or window. */
export function anomalyEventsHours(item: AnomalyEvent): number {
  const p = (item.map?.period || '').trim().toLowerCase();
  if (p.endsWith('d')) {
    const n = Number(p.slice(0, -1));
    if (Number.isFinite(n) && n > 0) return Math.min(24 * n, 168);
  }
  if (p.endsWith('h')) {
    const n = Number(p.slice(0, -1));
    if (Number.isFinite(n) && n > 0) return Math.min(n, 168);
  }
  if (p.endsWith('m')) {
    const n = Number(p.slice(0, -1));
    if (Number.isFinite(n) && n > 0) return Math.max(1, Math.ceil(n / 60));
  }
  return 1;
}

const PEERS_WINDOW_PAD_MS = 5 * 60_000;

/**
 * Prefer absolute anomaly window for peers so the panel still works when the
 * alert is older than map.period (15m → relative 1h from "now" goes empty).
 * Falls back to hours=N when window_* is missing.
 */
export function anomalyPeersPeriodQuery(item: AnomalyEvent): string {
  const startMs = Date.parse(item.window_start || '');
  const endMs = Date.parse(item.window_end || item.detected_at || '');
  if (Number.isFinite(startMs) && Number.isFinite(endMs) && endMs >= startMs) {
    const from = new Date(startMs - PEERS_WINDOW_PAD_MS).toISOString();
    const to = new Date(endMs + PEERS_WINDOW_PAD_MS).toISOString();
    return `from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`;
  }
  return `hours=${anomalyEventsHours(item)}`;
}

export function anomalyEventsQuery(item: AnomalyEvent): string {
  const q = (item.map?.q || '').trim();
  if (q) return q;
  if (item.src_ip) return `src:${item.src_ip}`;
  if (item.dst_ip) return `dst:${item.dst_ip}`;
  return '';
}

export function investigateHref(fingerprint: string): string {
  const fp = fingerprint.trim();
  if (!fp) return '/investigate';
  return `/investigate?alert=${encodeURIComponent(fp)}`;
}

export function absoluteAppHref(pathWithQuery: string): string {
  if (typeof window === 'undefined' || !window.location?.origin) {
    return pathWithQuery;
  }
  const path = pathWithQuery.startsWith('/') ? pathWithQuery : `/${pathWithQuery}`;
  return `${window.location.origin}${path}`;
}

const PEERS_CSV_HEADERS = ['src', 'dst', 'dst_port', 'action', 'count', 'bytes_sent', 'bytes_recv'] as const;

function csvEscape(value: string | number | undefined | null): string {
  const s = value === undefined || value === null ? '' : String(value);
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

/** Serialize aggregated map peers for client-side CSV download. */
export function peersLinesToCsv(
  lines: Array<{
    src?: string;
    dst?: string;
    dst_port?: number;
    last_action?: string;
    status?: string;
    count?: number;
    bytes_sent?: number;
    bytes_recv?: number;
  }>,
): string {
  const rows = [PEERS_CSV_HEADERS.join(',')];
  for (const line of lines) {
    rows.push(
      [
        csvEscape(line.src),
        csvEscape(line.dst),
        csvEscape(line.dst_port),
        csvEscape(line.last_action || line.status || ''),
        csvEscape(line.count ?? 0),
        csvEscape(line.bytes_sent ?? 0),
        csvEscape(line.bytes_recv ?? 0),
      ].join(','),
    );
  }
  return rows.join('\n') + '\n';
}

export function downloadTextFile(filename: string, contents: string, mime = 'text/csv;charset=utf-8'): void {
  const blob = new Blob([contents], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function investigationTemplateName(item: AnomalyEvent, now = new Date()): string {
  const code = (item.code || 'alert').trim() || 'alert';
  const src = (item.src_ip || item.dst_ip || 'any').trim();
  const y = now.getUTCFullYear();
  const m = String(now.getUTCMonth() + 1).padStart(2, '0');
  const d = String(now.getUTCDate()).padStart(2, '0');
  return `IR ${code} ${src} ${y}-${m}-${d}`;
}
