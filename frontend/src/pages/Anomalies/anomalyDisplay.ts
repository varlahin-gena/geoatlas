import type { AnomalyEvent } from '@/api/anomalies';
import { anomalyMapToView } from '@/pages/Map/useMapAnomalies';
import { MAP_VIEW_DEFAULTS, serializeMapViewSearch } from '@/pages/Map/mapQuery';

export const ANOMALY_SEVERITY_OPTIONS = [
  { value: '', label: 'Любая важность' },
  { value: 'high', label: 'Критично' },
  { value: 'warn', label: 'Предупреждение' },
  { value: 'info', label: 'Информация' },
] as const;

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

function codeLabel(code?: string): string {
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
    default:
      return code || '';
  }
}

export function eventCodeLabel(item: AnomalyEvent): string {
  return item.code_label || codeLabel(item.code);
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
    item.device,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
  return hay.includes(q);
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
