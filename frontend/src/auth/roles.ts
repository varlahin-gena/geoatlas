import { ROLE_ADMIN, ROLE_DASHBOARD, ROLE_OPERATOR, type AuthUser, type UserRole } from '@/api/types';

/** Canonical UI roles (matches OpenAPI AuthUser / AuthUserPublic). */
export const USER_ROLES: readonly UserRole[] = [ROLE_OPERATOR, ROLE_DASHBOARD, ROLE_ADMIN];

export const USER_ROLE_OPTIONS: ReadonlyArray<{ value: UserRole; label: string }> = [
  { value: ROLE_OPERATOR, label: 'Оператор' },
  { value: ROLE_DASHBOARD, label: 'Дэшборд' },
  { value: ROLE_ADMIN, label: 'Администратор' },
];

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
