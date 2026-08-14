import { apiFetch } from './client';
import type { AuthUser } from './types';

export function fetchMe(): Promise<AuthUser | null> {
  return fetch('/api/auth/me', { credentials: 'same-origin' }).then(async (res) => {
    if (res.status === 401) return null;
    if (!res.ok) throw new Error(`auth me: HTTP ${res.status}`);
    return res.json() as Promise<AuthUser>;
  });
}

export function login(username: string, password: string): Promise<AuthUser> {
  return apiFetch<AuthUser>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function logout(): Promise<void> {
  return apiFetch('/api/auth/logout', { method: 'POST' }).then(() => undefined);
}

/** Revoke всех сессий пользователя (session_version bump) + clear cookie. */
export function logoutAll(): Promise<void> {
  return apiFetch('/api/auth/logout-all', { method: 'POST' }).then(() => undefined);
}

export function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  return apiFetch('/api/auth/change-password', {
    method: 'POST',
    body: JSON.stringify({
      old_password: oldPassword,
      new_password: newPassword,
    }),
  }).then(() => undefined);
}

export function setGeoWizardDismissed(dismissed: boolean): Promise<AuthUser> {
  return apiFetch<AuthUser>('/api/auth/geo-wizard-dismiss', {
    method: 'POST',
    body: JSON.stringify({ dismissed }),
  });
}
