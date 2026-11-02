import { createHunt, type HuntMapState } from '@/api/hunts';
import { PERIODS } from '@/pages/Map/mapPeriods';

const DEFAULT_SCHEDULE = {
  enabled: false,
  interval_min: 60,
  edge_threshold: 0,
  edge_ratio: 3,
};

const GROUP_LABELS: Record<string, string> = {
  city: 'город',
  country: 'страна',
  asn: 'ASN',
  org: 'организация',
  ip: 'IP',
};

const FILTER_LABELS: Record<string, string> = {
  all: 'все',
  allowed: 'разрешённые',
  blocked: 'заблокированные',
};

export function periodLabel(period: string): string {
  const hit = PERIODS.find(([id]) => id === period);
  return hit ? hit[1] : period;
}

/** One-line default name from the current map snapshot. */
export function suggestHuntName(map: HuntMapState): string {
  const parts = [periodLabel(map.period || '1d')];
  if (map.query?.trim()) parts.push('поиск');
  if (map.country?.trim()) parts.push(map.country.trim());
  if (map.filter && map.filter !== 'all') {
    parts.push(FILTER_LABELS[map.filter] || map.filter);
  }
  return parts.join(' · ');
}

/** Short facts shown under the name field. */
export function huntSnapshotLines(map: HuntMapState): string[] {
  const lines: string[] = [];
  if (map.period === 'custom' && (map.period_from || map.period_to)) {
    lines.push(`Период: ${map.period_from || '…'} — ${map.period_to || '…'}`);
  } else {
    lines.push(`Период: ${periodLabel(map.period || '1d')}`);
  }
  lines.push(`Группировка: ${GROUP_LABELS[map.group_by] || map.group_by || 'город'}`);
  if (map.filter && map.filter !== 'all') {
    lines.push(`Фильтр: ${FILTER_LABELS[map.filter] || map.filter}`);
  }
  if (map.query?.trim()) lines.push(`Поиск: ${map.query.trim()}`);
  if (map.country?.trim()) lines.push(`Страна: ${map.country.trim()}`);
  return lines;
}

export async function saveHuntFromMap(
  name: string,
  mapState: HuntMapState,
  toast: (msg: string, kind?: 'success' | 'error' | 'warn') => void,
): Promise<boolean> {
  const trimmed = name.trim();
  if (!trimmed) return false;
  try {
    await createHunt({
      name: trimmed,
      map: mapState,
      schedule: DEFAULT_SCHEDULE,
    });
    toast('Охота сохранена', 'success');
    return true;
  } catch (e) {
    toast(e instanceof Error ? e.message : 'Ошибка сохранения охоты', 'error');
    return false;
  }
}
