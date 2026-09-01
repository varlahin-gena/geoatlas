import { apiFetch, apiGet, apiGetQuery, apiPath, apiPut } from './client';
import type { components } from './openapi';

export type AnomalyEvent = components['schemas']['AnomalyEvent'];
export type AnomalySummary = components['schemas']['AnomalySummary'];
export type AnomalyList = components['schemas']['AnomalyList'];
export type AnomalyMapLink = components['schemas']['AnomalyMapLink'];
export type AnomalyAckResponse = components['schemas']['AnomalyAckResponse'];
export type AnomalyAssignResponse = components['schemas']['AnomalyAssignResponse'];

export type AnomalyEngineSettings = {
  enabled: boolean;
  scan_interval_min: number;
  learning_days: number;
  suppress_hours: number;
  include_private: boolean;
  new_country_min_share: number;
  updated_at?: string;
};

export type AnomalyScanStatus = {
  enabled?: boolean;
  learning?: boolean;
  last_ok?: string;
  last_error?: string;
  last_duration?: string;
  last_inserted?: number;
  last_skip?: string;
  enterprise_nets?: number;
};

export type AnomalyThresholds = {
  port_scan_ports?: number;
  port_scan_events?: number;
  horizontal_hosts?: number;
  horizontal_events?: number;
  surge_ratio?: number;
  surge_abs_min?: number;
  surge_floor?: number;
  new_country_min?: number;
  new_country_baseline?: number;
  new_country_min_share?: number;
  rep_min_events?: number;
  byte_surge_ratio?: number;
  byte_surge_abs_min?: number;
  byte_surge_floor?: number;
  beacon_min_hours?: number;
  beacon_max_avg_bytes?: number;
  beacon_min_regularity?: number;
  lateral_hosts?: number;
  lateral_events?: number;
};

export type AnomalyEngineSettingsView = {
  ok?: boolean;
  settings?: AnomalyEngineSettings;
  install_profile?: string;
  thresholds?: AnomalyThresholds;
  status?: AnomalyScanStatus;
};

export type AnomalyEpisode = {
  episode_id: string;
  anchor_ip?: string;
  started_at?: string;
  updated_at?: string;
  alert_count?: number;
  high_count?: number;
  warn_count?: number;
  max_severity?: string;
  codes?: string[];
  fingerprints?: string[];
};

export function fetchAnomalySummary(init?: RequestInit): Promise<AnomalySummary> {
  return apiGet('/api/anomalies/summary', { cache: 'no-store', ...init });
}

export function fetchAnomalies(
  query?: { since?: string; severity?: string; code?: string; include_acked?: string; limit?: number },
  init?: RequestInit,
): Promise<AnomalyList> {
  return apiGetQuery('/api/anomalies', query, init);
}

export function fetchAnomalyEpisodes(
  query?: { since?: string; include_acked?: string },
  init?: RequestInit,
): Promise<{ ok?: boolean; episodes?: AnomalyEpisode[] }> {
  return apiGetQuery('/api/anomalies/episodes', query, init);
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

export function fetchAnomalyEngineSettings(init?: RequestInit): Promise<AnomalyEngineSettingsView> {
  return apiGet('/api/anomalies/settings', { cache: 'no-store', ...init });
}

export function putAnomalyEngineSettings(body: AnomalyEngineSettings): Promise<AnomalyEngineSettingsView> {
  return apiPut('/api/anomalies/settings', body);
}
