import { apiFetch } from './client';

export interface ReputationFeed {
  name: string;
  url?: string;
  category?: string;
  format?: string;
  enabled?: boolean;
  last_refresh?: string;
}

export interface ReputationList {
  name: string;
  category?: string;
  count?: number;
  source?: string;
  updated_at?: string;
  last_error?: string;
}

export interface ReputationRefreshResult {
  updated?: string[];
  skipped?: string[];
  failed?: string[];
  errors?: Record<string, string>;
  counts?: Record<string, number>;
}

export function listReputationFeeds(): Promise<{ feeds?: ReputationFeed[] }> {
  return apiFetch<{ feeds?: ReputationFeed[] }>('/api/reputation/feeds');
}

export function listReputationLists(): Promise<{ lists?: ReputationList[] }> {
  return apiFetch<{ lists?: ReputationList[] }>('/api/reputation/lists');
}

export function listReputationCatalog(): Promise<{ feeds?: ReputationFeed[] }> {
  return apiFetch<{ feeds?: ReputationFeed[] }>('/api/reputation/catalog');
}

export function createReputationFeed(body: {
  name: string;
  url: string;
  category: string;
  format: string;
  enabled?: boolean;
}): Promise<unknown> {
  return apiFetch('/api/reputation/feeds', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export function refreshReputation(force = true): Promise<ReputationRefreshResult> {
  const q = force ? '?force=1' : '';
  return apiFetch<ReputationRefreshResult>(`/api/reputation/refresh${q}`, {
    method: 'POST',
    body: '{}',
  });
}

export function deleteReputationList(name: string): Promise<unknown> {
  return apiFetch(`/api/reputation/lists/${encodeURIComponent(name)}`, { method: 'DELETE' });
}
