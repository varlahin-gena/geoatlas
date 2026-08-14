import { apiFetch } from './client';

export interface SearchTemplate {
  id: string;
  name: string;
  query: string;
  owner?: string;
  username?: string;
}

export function listSearchTemplates(
  scope?: 'all',
  init?: RequestInit,
): Promise<{ templates?: SearchTemplate[] }> {
  const q = scope === 'all' ? '?scope=all' : '';
  return apiFetch<{ templates?: SearchTemplate[] }>(`/api/me/search-templates${q}`, init);
}

export function createSearchTemplate(body: {
  name: string;
  query: string;
}): Promise<unknown> {
  return apiFetch('/api/me/search-templates', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export function deleteSearchTemplate(id: string): Promise<unknown> {
  return apiFetch(`/api/me/search-templates/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
