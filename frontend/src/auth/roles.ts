import { ROLE_ADMIN, type AuthUser } from '@/api/types';

/** True only for an authenticated admin (or auth-disabled appliance mode). */
export function deriveIsAdmin(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  return !!user.authDisabled || user.role === ROLE_ADMIN;
}

export function deriveReputationEnabled(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  return user.reputationEnabled !== false;
}

export function deriveUiAuthEnabled(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  return !user.authDisabled;
}
