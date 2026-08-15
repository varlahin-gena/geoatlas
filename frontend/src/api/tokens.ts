import { apiDelete, apiGet, apiPath, apiPost } from './client';

export interface TokenRow {
  id: string;
  name: string;
  scope: string;
  created_at?: string;
}

export function listTokens(): Promise<{ tokens: TokenRow[] }> {
  return apiGet('/api/tokens') as Promise<{ tokens: TokenRow[] }>;
}

export function createToken(body: { name: string; scope: string }): Promise<{ secret?: string }> {
  return apiPost('/api/tokens', body) as Promise<{ secret?: string }>;
}

export function deleteToken(id: string): Promise<unknown> {
  return apiDelete(apiPath('/api/tokens/{id}', { id }));
}
