import { apiFetch, apiGet, apiGetQuery, apiPath } from './client';
import type { components } from './openapi';

export type AnomalyEvent = components['schemas']['AnomalyEvent'];
export type AnomalySummary = components['schemas']['AnomalySummary'];
export type AnomalyList = components['schemas']['AnomalyList'];
export type AnomalyMapLink = components['schemas']['AnomalyMapLink'];
export type AnomalyAckResponse = components['schemas']['AnomalyAckResponse'];
export type AnomalyAssignResponse = components['schemas']['AnomalyAssignResponse'];

export function fetchAnomalySummary(init?: RequestInit): Promise<AnomalySummary> {
  return apiGet('/api/anomalies/summary', { cache: 'no-store', ...init });
}

export function fetchAnomalies(
  query?: { since?: string; severity?: string; code?: string; include_acked?: string; limit?: number },
  init?: RequestInit,
): Promise<AnomalyList> {
  return apiGetQuery('/api/anomalies', query, init);
}

export function ackAnomaly(fingerprint: string): Promise<AnomalyAckResponse> {
  return apiFetch<AnomalyAckResponse>(apiPath('/api/anomalies/{fingerprint}/ack', { fingerprint }), {
    method: 'POST',
    body: '{}',
  });
}

export function assignAnomaly(
  fingerprint: string,
  assigned_to: string,
): Promise<AnomalyAssignResponse> {
  return apiFetch<AnomalyAssignResponse>(
    apiPath('/api/anomalies/{fingerprint}/assign', { fingerprint }),
    {
      method: 'POST',
      body: JSON.stringify({ assigned_to }),
    },
  );
}
