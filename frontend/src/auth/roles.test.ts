import { describe, expect, it } from 'vitest';
import { ROLE_ADMIN, ROLE_OPERATOR, type AuthUser } from '@/api/types';
import { deriveIsAdmin, deriveReputationEnabled, deriveUiAuthEnabled } from './roles';

function user(partial: Partial<AuthUser> & Pick<AuthUser, 'username' | 'role'>): AuthUser {
  return partial;
}

describe('deriveIsAdmin', () => {
  it('is false when there is no user', () => {
    expect(deriveIsAdmin(null)).toBe(false);
    expect(deriveIsAdmin(undefined)).toBe(false);
  });

  it('is true for administrator', () => {
    expect(deriveIsAdmin(user({ username: 'a', role: ROLE_ADMIN }))).toBe(true);
  });

  it('is false for operator', () => {
    expect(deriveIsAdmin(user({ username: 'o', role: ROLE_OPERATOR }))).toBe(false);
  });

  it('is true when auth is disabled (appliance mode)', () => {
    expect(
      deriveIsAdmin(user({ username: 'local', role: ROLE_OPERATOR, authDisabled: true })),
    ).toBe(true);
  });
});

describe('deriveReputationEnabled / deriveUiAuthEnabled', () => {
  it('are false without a user', () => {
    expect(deriveReputationEnabled(null)).toBe(false);
    expect(deriveUiAuthEnabled(null)).toBe(false);
  });

  it('respect flags on the user', () => {
    expect(
      deriveReputationEnabled(user({ username: 'a', role: ROLE_ADMIN, reputationEnabled: false })),
    ).toBe(false);
    expect(
      deriveUiAuthEnabled(user({ username: 'a', role: ROLE_ADMIN, authDisabled: true })),
    ).toBe(false);
    expect(deriveUiAuthEnabled(user({ username: 'a', role: ROLE_ADMIN }))).toBe(true);
  });
});
