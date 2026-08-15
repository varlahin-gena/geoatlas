import { apiFetchRaw, apiGetQuery, apiPost, apiPut } from './client';

export interface GeoRange {
  network?: string;
  country?: string;
  region?: string;
  city?: string;
  lat?: number;
  lon?: number;
}

export interface GeoRangesLimits {
  upload_max_bytes?: number;
  upload_max_ranges?: number;
}

export interface GeoRangesResponse {
  ranges?: GeoRange[];
  count?: number;
  ip_hit?: boolean;
  index_ready?: boolean;
  limits?: GeoRangesLimits;
}

export interface GeoMissingRow {
  ip: string;
  kind?: string;
  count?: number;
  as_src?: number;
  as_dst?: number;
  sample_peer?: string;
  log_country?: string;
  log_city?: string;
  action_hint?: string;
  last_seen?: string;
}

export interface GeoMissingSummary {
  total?: number;
  unique_ips?: number;
  events?: number;
  public_focus?: number;
  public_unknown?: number;
  private?: number;
  by_kind?: Record<string, number>;
  [key: string]: unknown;
}

export interface GeoMissingResponse {
  items?: GeoMissingRow[];
  summary?: GeoMissingSummary;
}

export function fetchGeoRanges(
  params?: { ip?: string; limit?: number },
  init?: RequestInit,
): Promise<GeoRangesResponse> {
  return apiGetQuery(
    '/api/geo-ranges',
    { ip: params?.ip, limit: params?.limit },
    init,
  ) as Promise<GeoRangesResponse>;
}

export function updateGeoRange(body: Record<string, unknown>): Promise<{ updated?: string }> {
  return apiPut('/api/geo-ranges', body) as Promise<{ updated?: string }>;
}

export function clearGeoRanges(): Promise<{ index_before?: number }> {
  return apiPost('/api/geo-ranges/clear', {}) as Promise<{ index_before?: number }>;
}

export function exportGeoRanges(): Promise<Response> {
  return apiFetchRaw('/api/geo-ranges/export');
}

export function fetchGeoMissing(query: string): Promise<GeoMissingResponse> {
  // query already includes period params + limit (caller builds SoT query string).
  return apiGetQuery('/api/geo-missing', query) as Promise<GeoMissingResponse>;
}

export function createGeoRange(body: Record<string, unknown>): Promise<{
  ranges?: number;
  added?: string;
  entry?: { network?: string };
}> {
  return apiPost('/api/geo-ranges', body) as Promise<{
    ranges?: number;
    added?: string;
    entry?: { network?: string };
  }>;
}
