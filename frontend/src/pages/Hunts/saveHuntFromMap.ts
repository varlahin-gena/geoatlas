import { createHunt, type HuntMapState } from '@/api/hunts';
import {
  extractHuntPlace,
  huntFilterLabel,
  huntGroupLabel,
  huntPeriodLabel,
  huntQueryLabel,
} from './huntDisplay';

const DEFAULT_SCHEDULE = {
  enabled: false,
  interval_min: 60,
  edge_threshold: 0,
  edge_ratio: 3,
};

/** One-line default name from the current map snapshot. */
export function suggestHuntName(map: HuntMapState): string {
  const place = extractHuntPlace(map);
  const parts: string[] = [];
  if (place) {
    parts.push(place);
  } else {
    parts.push(huntPeriodLabel(map.period || '1d'));
    if (map.query?.trim()) parts.push('поиск');
  }
  if (map.filter && map.filter !== 'all') {
    parts.push(huntFilterLabel(map.filter));
  }
  return parts.join(' · ');
}

/** Short facts shown under the name field. */
export function huntSnapshotLines(map: HuntMapState): string[] {
  const lines: string[] = [];
  if (map.period === 'custom' && (map.period_from || map.period_to)) {
    lines.push(`Период: ${map.period_from || '…'} — ${map.period_to || '…'}`);
  } else {
    lines.push(`Период: ${huntPeriodLabel(map.period || '1d')}`);
  }
  lines.push(`Группировка: ${huntGroupLabel(map.group_by)}`);
  if (map.filter && map.filter !== 'all') {
    lines.push(`Фильтр: ${huntFilterLabel(map.filter)}`);
  }
  if (map.query?.trim()) lines.push(`Поиск: ${huntQueryLabel(map)}`);
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
