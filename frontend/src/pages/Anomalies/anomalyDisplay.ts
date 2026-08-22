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
