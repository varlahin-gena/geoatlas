export const PERIODS = [
  ['1h', '1 час'],
  ['3h', '3 часа'],
  ['6h', '6 часов'],
  ['12h', '12 часов'],
  ['1d', '1 день'],
  ['3d', '3 дня'],
  ['7d', '7 дней'],
  ['14d', '14 дней'],
  ['30d', '30 дней'],
  ['custom', 'Свой диапазон…'],
] as const;

/** Legacy minute presets → minimum map window is 1h. */
export function normalizeMapPeriod(period: string): string {
  const p = (period || '').trim();
  if (!p) return '';
  const m = p.match(/^(\d+)m$/i);
  if (m) return '1h';
  return p;
}
