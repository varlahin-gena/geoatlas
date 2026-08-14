import { apiFetch } from './client';

export interface TokenRow {
  id: string;
  name: string;
  scope: string;
  created_at?: string;
}

export function listTokens(): Promise<{ tokens: TokenRow[] }> {
  return apiFetch<{ tokens: TokenRow[] }>('/api/tokens');
}

export function createToken(body: { name: string; scope: string }): Promise<{ secret?: string }> {
  return apiFetch<{ secret?: string }>('/api/tokens', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export function deleteToken(id: string): Promise<unknown> {
  return apiFetch(`/api/tokens/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
