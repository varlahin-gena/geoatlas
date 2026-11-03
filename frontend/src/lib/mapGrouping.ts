/** Shared map arc grouping keys / RU labels (filters panel, profile, query). */

export const MAP_GROUP_BY_KEYS = [
  'ip',
  'subnet',
  'city',
  'country',
  'continent',
  'city_country',
] as const;

export type MapGroupBy = (typeof MAP_GROUP_BY_KEYS)[number];

export const MAP_GROUP_BY_DEFAULT: MapGroupBy = 'city';

export const MAP_GROUP_BY_LABELS: Record<MapGroupBy, string> = {
  ip: 'IP',
  subnet: '/24',
  city: 'Город',
  country: 'Страна',
  continent: 'Континент',
  city_country: 'Город — страна',
};

const GROUP_SET = new Set<string>(MAP_GROUP_BY_KEYS);

export function isMapGroupBy(v: string | null | undefined): v is MapGroupBy {
  return !!v && GROUP_SET.has(v);
}

export function normalizeMapGroupBy(
  raw: string | null | undefined,
  fallback: MapGroupBy = MAP_GROUP_BY_DEFAULT,
): MapGroupBy {
  const v = (raw || '').trim().toLowerCase();
  return isMapGroupBy(v) ? v : fallback;
}
