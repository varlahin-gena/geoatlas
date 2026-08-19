import { apiFetch, apiGet, apiGetQuery, apiPath } from './client';
import type { components } from './openapi';

export type AnomalyEvent = components['schemas']['AnomalyEvent'];
export type AnomalySummary = components['schemas']['AnomalySummary'];
export type AnomalyList = components['schemas']['AnomalyList'];
export type AnomalyMapLink = components['schemas']['AnomalyMapLink'];

export function fetchAnomalySummary(init?: RequestInit): Promise<AnomalySummary> {
  return apiGet('/api/anomalies/summary', { cache: 'no-store', ...init });
}

export function fetchAnomalies(
  query?: { since?: string; severity?: string; code?: string; include_acked?: string; limit?: number },
  init?: RequestInit,
): Promise<AnomalyList> {
  return apiGetQuery('/api/anomalies', query, init) as Promise<AnomalyList>;
}

export function ackAnomaly(fingerprint: string): Promise<{ ok?: boolean; fingerprint?: string }> {
  return apiFetch(apiPath('/api/anomalies/{fingerprint}/ack', { fingerprint }), {
    method: 'POST',
    body: '{}',
  }) as Promise<{ ok?: boolean; fingerprint?: string }>;
}
