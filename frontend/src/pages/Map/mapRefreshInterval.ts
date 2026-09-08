/** localStorage key for map auto-refresh interval (seconds). */
const MAP_REFRESH_STORAGE_KEY = 'ga.map.refreshSec';

/** Allowed presets in seconds: 30s / 1m / 5m. */
export const MAP_REFRESH_PRESETS_SEC = [30, 60, 300] as const;

export type MapRefreshSec = (typeof MAP_REFRESH_PRESETS_SEC)[number];

/** Default refresh interval: 5 minutes. */
export const MAP_REFRESH_DEFAULT_SEC: MapRefreshSec = 300;

export function parseMapRefreshSec(raw: unknown): MapRefreshSec {
  const n = typeof raw === 'number' ? raw : Number(String(raw ?? '').trim());
  if (n === 30 || n === 60 || n === 300) return n;
  return MAP_REFRESH_DEFAULT_SEC;
}

export function loadMapRefreshSec(): MapRefreshSec {
  try {
    return parseMapRefreshSec(localStorage.getItem(MAP_REFRESH_STORAGE_KEY));
  } catch {
    return MAP_REFRESH_DEFAULT_SEC;
  }
}

export function saveMapRefreshSec(sec: MapRefreshSec): void {
  try {
    localStorage.setItem(MAP_REFRESH_STORAGE_KEY, String(sec));
  } catch {
    /* ignore quota / private mode */
  }
}

export function mapRefreshSecToMs(sec: MapRefreshSec): number {
  return sec * 1000;
}
