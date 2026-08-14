import { apiFetch } from './client';
import type { UserRole } from './types';

export interface UserRow {
  username: string;
  full_name?: string;
  role: string;
  must_reset_password?: boolean;
  created_at?: string;
}

export function listUsers(): Promise<{ users: UserRow[] }> {
  return apiFetch<{ users: UserRow[] }>('/api/users');
}

export function createUser(body: {
  username: string;
  password: string;
  role: string;
  full_name?: string;
  must_reset_password?: boolean;
}): Promise<unknown> {
  return apiFetch('/api/users', { method: 'POST', body: JSON.stringify(body) });
}

export function updateUserFullName(username: string, full_name: string): Promise<unknown> {
  return apiFetch(`/api/users/${encodeURIComponent(username)}/full-name`, {
    method: 'POST',
    body: JSON.stringify({ full_name }),
  });
}

export function updateUserRole(username: string, role: UserRole | string): Promise<unknown> {
  return apiFetch(`/api/users/${encodeURIComponent(username)}/role`, {
    method: 'POST',
    body: JSON.stringify({ role }),
  });
}

export function deleteUser(username: string): Promise<unknown> {
  return apiFetch(`/api/users/${encodeURIComponent(username)}`, { method: 'DELETE' });
}

export function resetUserPassword(
  username: string,
  body: { password: string; must_reset_password?: boolean },
): Promise<unknown> {
  return apiFetch(`/api/users/${encodeURIComponent(username)}/reset-password`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}
