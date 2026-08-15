import { apiDelete, apiFetch, apiGet, apiPath, apiPost } from './client';
import type { UserRole } from './types';

export interface UserRow {
  username: string;
  full_name?: string;
  role: string;
  must_reset_password?: boolean;
  created_at?: string;
}

export function listUsers(): Promise<{ users: UserRow[] }> {
  return apiGet('/api/users') as Promise<{ users: UserRow[] }>;
}

export function createUser(body: {
  username: string;
  password: string;
  role: string;
  full_name?: string;
  must_reset_password?: boolean;
}): Promise<unknown> {
  return apiPost('/api/users', body);
}

export function updateUserFullName(username: string, full_name: string): Promise<unknown> {
  return apiFetch(apiPath('/api/users/{username}/full-name', { username }), {
    method: 'POST',
    body: JSON.stringify({ full_name }),
  });
}

export function updateUserRole(username: string, role: UserRole | string): Promise<unknown> {
  return apiFetch(apiPath('/api/users/{username}/role', { username }), {
    method: 'POST',
    body: JSON.stringify({ role }),
  });
}

export function deleteUser(username: string): Promise<unknown> {
  return apiDelete(apiPath('/api/users/{username}', { username }));
}

export function resetUserPassword(
  username: string,
  body: { password: string; must_reset_password?: boolean },
): Promise<unknown> {
  return apiFetch(apiPath('/api/users/{username}/reset-password', { username }), {
    method: 'POST',
    body: JSON.stringify(body),
  });
}
