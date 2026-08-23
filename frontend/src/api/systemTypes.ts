import type { components } from './openapi';

type Alert = components['schemas']['SystemAlert'];
export type FailedLogin = components['schemas']['SystemFailedLogin'];
export type EdgesAgg = components['schemas']['SystemEdgesAgg'];

/**
 * UI view of GET /api/system/stats.
 * Wire schema (`SystemStats` in openapi) is intentionally loose (additionalProperties);
 * nestings below are the fields the System page actually reads.
 */
export interface SystemStats {
  alerts?: Alert[];
  containers?: Record<string, components['schemas']['SystemContainerStats']>;
  health?: Record<string, Record<string, unknown>>;
  pipeline?: Record<string, Record<string, number>>;
  storage?: Record<string, Record<string, number>>;
  backend_info?: components['schemas']['SystemBackendInfo'];
  install_profile?: {
    profile?: string;
    profile_label?: string;
    host?: Record<string, unknown>;
    limits?: Record<string, unknown>;
    capacity?: { expected_eps_min?: number; expected_eps_max?: number };
  };
  edges_agg?: EdgesAgg;
  failed_logins?: FailedLogin[];
  uptime_sec?: number;
  timestamp?: string;
}

export type Retention = components['schemas']['RetentionSettings'];
export type BackupCatalog = components['schemas']['BackupCatalog'];
export type BackupSchedule = components['schemas']['BackupSchedule'];
export type BackupEntry = components['schemas']['BackupEntry'];
export type DREvent = components['schemas']['DREvent'];
export type AuditEvent = components['schemas']['AuditEvent'];
export type HistoryPayload = components['schemas']['SystemHistoryResponse'];
export type HistoryPoint = {
  t: string;
  v: number;
};
