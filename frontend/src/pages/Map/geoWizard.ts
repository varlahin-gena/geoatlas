/** Helpers for GeoIP first-run wizard (pure, unit-tested). */

export type GeoWizardStep = 'why' | 'upload' | 'done';

export const GEO_WIZARD_LS_KEY = 'nm.geoWizardDismissed';
export const GEO_UPLOAD_LARGE_BYTES = 32 * 1024 * 1024;

export interface GeoStatus {
  count: number;
  indexReady: boolean;
  uploadMaxBytes: number;
  uploadMaxRanges: number;
}

export interface DryRunPreview {
  ranges: number;
  sample: Array<{
    start_ip?: number;
    end_ip?: number;
    country?: string;
    region?: string;
    city?: string;
    lat?: number;
    lon?: number;
  }>;
}

export function shouldShowGeoWizard(opts: {
  isAdmin: boolean;
  dismissed: boolean;
  geoCount: number | null;
  forceOpen?: boolean;
}): boolean {
  if (!opts.isAdmin) return false;
  if (opts.forceOpen) return true;
  if (opts.dismissed) return false;
  if (opts.geoCount === null) return false;
  return opts.geoCount === 0;
}

/** Curl/install filled the table while the intro is open. Do not steal the upload step. */
export function shouldSkipToGeoWizardDone(
  step: GeoWizardStep,
  geo: Pick<GeoStatus, 'count' | 'indexReady'> | null,
): boolean {
  return step === 'why' && geo != null && geo.count > 0 && geo.indexReady;
}

export function readLocalDismissed(): boolean {
  try {
    return localStorage.getItem(GEO_WIZARD_LS_KEY) === '1';
  } catch {
    return false;
  }
}

export function writeLocalDismissed(dismissed: boolean): void {
  try {
    if (dismissed) localStorage.setItem(GEO_WIZARD_LS_KEY, '1');
    else localStorage.removeItem(GEO_WIZARD_LS_KEY);
  } catch {
    /* ignore */
  }
}

export function buildGeoCurlSnippet(origin: string, tokenPlaceholder = '$API_AUTH_TOKEN'): string {
  const base = origin.replace(/\/$/, '') || 'http://127.0.0.1';
  return [
    'cd /opt/network-monitor',
    'set -a; . ./.env; set +a',
    '',
    `curl -sS -w "\\nHTTP %{http_code}\\n" \\`,
    `  -H "Authorization: Bearer ${tokenPlaceholder}" \\`,
    `  -H "Content-Type: text/csv" \\`,
    `  --data-binary @/opt/network-monitor/geoip.csv \\`,
    `  "${base}/upload-geo"`,
  ].join('\n');
}

export function formatNetworkHint(startIp?: number, endIp?: number): string {
  if (startIp == null || endIp == null || !Number.isFinite(startIp) || !Number.isFinite(endIp)) {
    return '—';
  }
  const a = uint32ToIPv4(startIp);
  const b = uint32ToIPv4(endIp);
  return a === b ? a : `${a}-${b}`;
}

function uint32ToIPv4(n: number): string {
  const x = n >>> 0;
  return `${(x >>> 24) & 255}.${(x >>> 16) & 255}.${(x >>> 8) & 255}.${x & 255}`;
}

export type EmptyMapReason = 'loading' | 'error' | 'no_events' | 'no_geo' | 'filtered' | 'search_error' | null;

export function classifyEmptyMap(opts: {
  loading: boolean;
  fetchError: string | null;
  linesCount: number;
  visibleCount: number;
  rawPairs: number;
  skippedNoGeo: number;
  filterActive: boolean;
  searchError: string;
}): { reason: EmptyMapReason; title: string; text: string } | null {
  if (opts.loading) return null;
  if (opts.fetchError) {
    return { reason: 'error', title: 'Ошибка загрузки', text: opts.fetchError };
  }
  if (opts.searchError) {
    return { reason: 'search_error', title: 'Ошибка поиска', text: opts.searchError };
  }
  if (!opts.linesCount) {
    if (opts.rawPairs > 0 || opts.skippedNoGeo > 0) {
      return {
        reason: 'no_geo',
        title: 'Нет координат для карты',
        text:
          'События за период есть, но у узлов нет GeoIP-координат. Загрузите базу GeoIP (мастер на карте или страница «База GeoIP»).',
      };
    }
    return {
      reason: 'no_events',
      title: 'Нет событий за период',
      text: 'Попробуйте расширить период, уменьшить порог minCount или проверить ingest / загрузку логов.',
    };
  }
  if (!opts.visibleCount) {
    return {
      reason: 'filtered',
      title: 'Ничего не отображается',
      text: opts.filterActive
        ? 'Активные фильтры скрыли все связи.'
        : 'Все связи отфильтрованы текущими настройками.',
    };
  }
  return null;
}
