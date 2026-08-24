import { ROLE_ADMIN, ROLE_DASHBOARD, ROLE_OPERATOR, type AuthUser, type UserRole } from '@/api/types';

/** True only for an authenticated admin (or auth-disabled appliance mode). */
export function deriveIsAdmin(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  return !!user.authDisabled || user.role === ROLE_ADMIN;
}

/** Human-readable role label for UI. */
export function roleLabelRu(role: UserRole): string {
  switch (role) {
    case ROLE_ADMIN:
      return 'Администратор';
    case ROLE_DASHBOARD:
      return 'Дэшборд';
    default:
      return 'Оператор';
  }
}

export function deriveReputationEnabled(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  return user.reputationEnabled !== false;
}

export function deriveUiAuthEnabled(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  return !user.authDisabled;
}
