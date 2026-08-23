import { apiDelete, apiFetchRaw, apiGetQuery, apiPost, apiPut } from './client';
import type { components } from './openapi';

export type GeoRange = components['schemas']['GeoRange'];
export type GeoRangesResponse = components['schemas']['GeoRangesResponse'];
export type GeoMissingRow = components['schemas']['GeoMissingRow'];
export type GeoMissingResponse = components['schemas']['GeoMissingResponse'];
export type EnterpriseNet = components['schemas']['EnterpriseNet'];

export type GeoMissingSummary = NonNullable<GeoMissingResponse['summary']> & {
  total?: number;
  unique_ips?: number;
  events?: number;
  public_focus?: number;
  public_unknown?: number;
  private?: number;
  by_kind?: Record<string, number>;
  [key: string]: unknown;
};

export function fetchGeoRanges(
  params?: { ip?: string; limit?: number; q?: string },
  init?: RequestInit,
): Promise<GeoRangesResponse> {
  return apiGetQuery(
    '/api/geo-ranges',
    { ip: params?.ip, limit: params?.limit, q: params?.q },
    init,
  );
}

export function updateGeoRange(body: Record<string, unknown>): Promise<{ updated?: string }> {
  return apiPut('/api/geo-ranges', body);
}

export function clearGeoRanges(): Promise<{ index_before?: number }> {
  return apiPost('/api/geo-ranges/clear', {});
}

export function exportGeoRanges(): Promise<Response> {
  return apiFetchRaw('/api/geo-ranges/export');
}

export function fetchGeoMissing(query: string): Promise<GeoMissingResponse> {
  // query already includes period params + limit (caller builds SoT query string).
  return apiGetQuery('/api/geo-missing', query);
}

export function createGeoRange(body: Record<string, unknown>): Promise<
  components['schemas']['GeoRangeCreateResponse']
> {
  return apiPost('/api/geo-ranges', body);
}

export function fetchEnterpriseNets(init?: RequestInit): Promise<
  components['schemas']['EnterpriseNetsResponse']
> {
  return apiGetQuery('/api/enterprise-nets', undefined, init);
}

export function addEnterpriseNet(body: {
  network: string;
  label?: string;
  country?: string;
  region?: string;
  city?: string;
}): Promise<{ item?: EnterpriseNet }> {
  return apiPost('/api/enterprise-nets', body);
}

export function deleteEnterpriseNet(startIP: number, endIP: number): Promise<unknown> {
  return apiDelete(`/api/enterprise-nets/${startIP}/${endIP}`);
}
