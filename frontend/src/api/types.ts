import type { components } from './openapi';

export const ROLE_ADMIN = 'administrator';
export const ROLE_OPERATOR = 'operator';
export const ROLE_DASHBOARD = 'dashboard';

export type AuthUser = components['schemas']['AuthUser'];
export type UserRole = AuthUser['role'];
export type SystemVersion = components['schemas']['SystemVersion'];
export type ReputationHit = components['schemas']['ReputationHit'];
