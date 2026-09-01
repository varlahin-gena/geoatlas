import { apiFetch, apiGet, apiPath, apiPut } from './client';

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

export type HuntSchedule = {
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
  return apiPut(apiPath('/api/me/hunts/{id}', { id }), body);
}

export function deleteHunt(id: string): Promise<{ ok?: boolean }> {
  return apiFetch(apiPath('/api/me/hunts/{id}', { id }), { method: 'DELETE' });
}

export function runHunt(id: string): Promise<{ ok?: boolean; hunt?: SavedHunt; run?: HuntRun }> {
  return apiFetch(apiPath('/api/me/hunts/{id}/run', { id }), { method: 'POST', body: '{}' });
}

export function huntMapHref(hunt: SavedHunt): string {
  const sp = new URLSearchParams();
  const m = hunt.map;
  if (m.period && m.period !== '1d') sp.set('period', m.period);
  if (m.period === 'custom') {
    if (m.period_from) sp.set('from', m.period_from);
    if (m.period_to) sp.set('to', m.period_to);
  }
  if (m.group_by && m.group_by !== 'city') sp.set('group', m.group_by);
  if (m.filter && m.filter !== 'all') sp.set('filter', m.filter);
  if (m.query) sp.set('q', m.query);
  if (m.country) sp.set('country', m.country);
  const qs = sp.toString();
  return qs ? `/?${qs}` : '/';
}
