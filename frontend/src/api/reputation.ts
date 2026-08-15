import { apiDelete, apiFetch, apiGet, apiPath, apiPost } from './client';

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
  return apiGet('/api/reputation/feeds') as Promise<{ feeds?: ReputationFeed[] }>;
}

export function listReputationLists(): Promise<{ lists?: ReputationList[] }> {
  return apiGet('/api/reputation/lists') as Promise<{ lists?: ReputationList[] }>;
}

export function listReputationCatalog(): Promise<{ feeds?: ReputationFeed[] }> {
  return apiGet('/api/reputation/catalog') as Promise<{ feeds?: ReputationFeed[] }>;
}

export function createReputationFeed(body: {
  name: string;
  url: string;
  category: string;
  format: string;
  enabled?: boolean;
}): Promise<unknown> {
  return apiPost('/api/reputation/feeds', body);
}

export function refreshReputation(force = true): Promise<ReputationRefreshResult> {
  const path = force ? '/api/reputation/refresh?force=1' : '/api/reputation/refresh';
  return apiFetch(path, { method: 'POST', body: '{}' }) as Promise<ReputationRefreshResult>;
}

export function deleteReputationList(name: string): Promise<unknown> {
  return apiDelete(apiPath('/api/reputation/lists/{name}', { name }));
}
