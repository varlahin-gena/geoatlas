import { ApiError, apiFetch, apiGet, apiPatch, apiPost } from './client';
import type { components } from './openapi';
import type { AuthUser } from './types';

export type AuthPreferencesPatch = components['schemas']['AuthPreferencesRequest'];

export async function fetchMe(): Promise<AuthUser | null> {
  try {
    return await apiGet('/api/auth/me');
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}

export function login(username: string, password: string): Promise<AuthUser> {
  return apiPost('/api/auth/login', { username, password });
}

export function logout(): Promise<void> {
  return apiFetch('/api/auth/logout', { method: 'POST' }).then(() => undefined);
}

/** Revoke всех сессий пользователя (session_version bump) + clear cookie. */
export function logoutAll(currentPassword: string): Promise<void> {
  return apiFetch('/api/auth/logout-all', {
    method: 'POST',
    body: JSON.stringify({ current_password: currentPassword }),
  }).then(() => undefined);
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
  return apiPost('/api/auth/geo-wizard-dismiss', { dismissed });
}

export function updatePreferences(patch: AuthPreferencesPatch): Promise<AuthUser> {
  return apiPatch('/api/auth/me/preferences', patch);
}
