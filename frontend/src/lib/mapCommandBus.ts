/** Map-only actions from the global command palette. */
export const GA_MAP_COMMAND = 'ga-map-command';

export type MapCommandDetail =
  | { type: 'set-period'; period: string }
  | { type: 'reset-filters' }
  | { type: 'focus-search' };

export function dispatchMapCommand(detail: MapCommandDetail): void {
  document.dispatchEvent(new CustomEvent(GA_MAP_COMMAND, { detail }));
}

export function isMapCommandDetail(v: unknown): v is MapCommandDetail {
  if (!v || typeof v !== 'object') return false;
  const t = (v as { type?: string }).type;
  return t === 'set-period' || t === 'reset-filters' || t === 'focus-search';
}
