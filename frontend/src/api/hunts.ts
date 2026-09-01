import { apiFetch, apiGet, apiPath } from './client';

export type HuntMapState = {
  period: string;
  period_from?: string;
  period_to?: string;
  group_by: string;
  filter: string;
  query?: string;
  country?: string;
  limit?: number;
  data_source?: string;
};

type HuntSchedule = {
  enabled: boolean;
  interval_min: number;
  edge_threshold: number;
  edge_ratio: number;
};

export type HuntRun = {
  at: string;
  edge_count: number;
  raw_pairs: number;
  query_cost?: string;
  skipped?: string;
  breach?: boolean;
  prev_edges?: number;
  ratio?: number;
};

export type SavedHunt = {
  id: string;
  name: string;
  notes?: string;
  map: HuntMapState;
  schedule: HuntSchedule;
  runs?: HuntRun[];
  updated_at?: string;
  last_run_at?: string;
};

export function listMyHunts(init?: RequestInit): Promise<{ ok?: boolean; hunts?: SavedHunt[] }> {
  return apiGet('/api/me/hunts', { cache: 'no-store', ...init });
}

export function createHunt(body: Partial<SavedHunt>): Promise<{ ok?: boolean; hunt?: SavedHunt }> {
  return apiFetch('/api/me/hunts', { method: 'POST', body: JSON.stringify(body) });
}

export function updateHunt(id: string, body: Partial<SavedHunt>): Promise<{ ok?: boolean; hunt?: SavedHunt }> {
  return apiFetch(apiPath('/api/me/hunts/{id}', { id }), {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export function deleteHunt(id: string): Promise<{ ok?: boolean }> {
  return apiFetch(apiPath('/api/me/hunts/{id}', { id }), { method: 'DELETE' });
}

export function runHunt(id: string): Promise<{ ok?: boolean; hunt?: SavedHunt; run?: HuntRun }> {
  return apiFetch(apiPath('/api/me/hunts/{id}/run', { id }), { method: 'POST', body: '{}' });
}
