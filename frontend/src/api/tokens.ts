import { apiDelete, apiGet, apiPath, apiPost } from './client';
import type { components } from './openapi';

export type TokenRow = components['schemas']['ApiToken'];
export type TokenCreateResponse = components['schemas']['ApiTokenCreateResponse'];

export function listTokens(): Promise<{ tokens?: TokenRow[] }> {
  return apiGet('/api/tokens');
}

export function createToken(body: { name: string; scope: string }): Promise<TokenCreateResponse> {
  return apiPost('/api/tokens', body);
}

export function deleteToken(id: string): Promise<unknown> {
  return apiDelete(apiPath('/api/tokens/{id}', { id }));
}
