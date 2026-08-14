import { apiFetch, apiFetchRaw } from './client';

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
  const qs = new URLSearchParams();
  if (params?.ip) qs.set('ip', params.ip);
  if (params?.limit != null) qs.set('limit', String(params.limit));
  const q = qs.toString();
  return apiFetch<GeoRangesResponse>(`/api/geo-ranges${q ? `?${q}` : ''}`, init);
}

export function updateGeoRange(body: Record<string, unknown>): Promise<{ updated?: string }> {
  return apiFetch<{ updated?: string }>('/api/geo-ranges', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export function clearGeoRanges(): Promise<{ index_before?: number }> {
  return apiFetch<{ index_before?: number }>('/api/geo-ranges/clear', {
    method: 'POST',
    body: '{}',
  });
}

export function exportGeoRanges(): Promise<Response> {
  return apiFetchRaw('/api/geo-ranges/export');
}

export function fetchGeoMissing(query: string): Promise<GeoMissingResponse> {
  // query already includes period params + limit (caller builds SoT query string).
  return apiFetch<GeoMissingResponse>(`/api/geo-missing?${query}`);
}

export function createGeoRange(body: Record<string, unknown>): Promise<{
  ranges?: number;
  added?: string;
  entry?: { network?: string };
}> {
  return apiFetch<{ ranges?: number; added?: string; entry?: { network?: string } }>(
    '/api/geo-ranges',
    {
      method: 'POST',
      body: JSON.stringify(body),
    },
  );
}
