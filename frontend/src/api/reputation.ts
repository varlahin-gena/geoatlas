import { apiDelete, apiFetch, apiGet, apiPath, apiPost } from './client';
import type { components } from './openapi';

export type ReputationFeed = components['schemas']['ReputationFeed'];
export type ReputationList = components['schemas']['ReputationListMeta'];
export type ReputationRefreshResult = components['schemas']['ReputationRefreshResult'];

export function listReputationFeeds(): Promise<{ feeds?: ReputationFeed[] }> {
  return apiGet('/api/reputation/feeds');
}

export function listReputationLists(): Promise<{ lists?: ReputationList[] }> {
  return apiGet('/api/reputation/lists');
}

export function listReputationCatalog(): Promise<{ feeds?: ReputationFeed[] }> {
  return apiGet('/api/reputation/catalog');
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
  return apiFetch<ReputationRefreshResult>(path, { method: 'POST', body: '{}' });
}

export function deleteReputationList(name: string): Promise<unknown> {
  return apiDelete(apiPath('/api/reputation/lists/{name}', { name }));
}
