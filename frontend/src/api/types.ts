export const ROLE_ADMIN = 'administrator';
export const ROLE_OPERATOR = 'operator';

export type UserRole = typeof ROLE_ADMIN | typeof ROLE_OPERATOR;

export interface AuthUser {
  username: string;
  full_name?: string;
  role: UserRole;
  must_reset_password?: boolean;
  geo_wizard_dismissed?: boolean;
  authDisabled?: boolean;
  reputationEnabled?: boolean;
}

export interface SystemVersion {
  display?: string;
  ref?: string;
  version?: string;
  source?: string;
  commit?: string;
}

export interface ReputationHit {
  list: string;
  category: string;
  network?: string;
}
