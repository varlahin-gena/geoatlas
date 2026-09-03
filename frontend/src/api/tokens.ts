import { apiDelete, apiFetch, apiGet, apiPath, apiPost } from './client';import type { components } from './openapi';

export type TokenRow = components['schemas']['ApiToken'];
export type TokenCreateResponse = components['schemas']['ApiTokenCreateResponse'];
export type TokenCreateRequest = components['schemas']['ApiTokenCreateRequest'];
export type TokenScope = TokenCreateRequest['scope'];

export function listTokens(): Promise<{ tokens?: TokenRow[] }> {
  return apiGet('/api/tokens');
}

export function createToken(body: TokenCreateRequest): Promise<TokenCreateResponse> {
  return apiPost('/api/tokens', body);
}

export function rotateToken(id: string, currentPassword: string): Promise<TokenCreateResponse> {
  return apiFetch(apiPath('/api/tokens/{id}/rotate', { id }), {
    method: 'POST',
    body: JSON.stringify({ current_password: currentPassword }),
  });
}

export function deleteToken(id: string, currentPassword: string): Promise<unknown> {
  return apiFetch(apiPath('/api/tokens/{id}', { id }), {
    method: 'DELETE',
    body: JSON.stringify({ current_password: currentPassword }),
  });
}
