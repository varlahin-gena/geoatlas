import type { components } from '@/api/openapi';

export type Tab = 'overview' | 'pipeline' | 'backup' | 'security' | 'charts';

export const CONTAINERS = ['backend', 'clickhouse', 'syslog-ng', 'frontend'] as const;
export const PERIODS = [
  ['1h', '1ч'],
  ['6h', '6ч'],
  ['24h', '24ч'],
  ['7d', '7д'],
] as const;

export type Alert = components['schemas']['SystemAlert'];
export type FailedLogin = components['schemas']['SystemFailedLogin'];
export type EdgesAgg = components['schemas']['SystemEdgesAgg'];

/** Nested UI fields. Wire envelope: OpenAPI SystemStats (`src/api/openapi.d.ts`). */
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

export interface Retention {
  traffic_logs_days?: number;
  edges_days?: number;
  parse_errors_days?: number;
  system_metrics_days?: number;
  updated_at?: string;
}

export interface BackupStatus {
  state?: string;
  message?: string;
  name?: string;
  started_at?: string;
  updated_at?: string;
}

export interface BackupEntry {
  name: string;
  created_at?: string;
  size_bytes?: number;
  has_auth?: boolean;
  attached?: boolean;
  /** manual | schedule | отсутствует у старых бэкапов */
  source?: 'manual' | 'schedule' | string;
}

export interface BackupSchedule {
  enabled?: boolean;
  hour?: number;
  minute?: number;
  timezone?: string;
  keep?: number;
  include_edges?: boolean;
  include_auth?: boolean;
  updated_at?: string;
  last_run_at?: string;
  last_run_date?: string;
}

export interface BackupCatalog {
  ok?: boolean;
  enabled?: boolean;
  dir_ready?: boolean;
  keep?: number;
  include_edges?: boolean;
  include_auth?: boolean;
  attached?: string;
  schedule?: BackupSchedule;
  next_run_at?: string;
  backups?: BackupEntry[];
  status?: BackupStatus;
  hint?: string;
}

export interface HistoryPoint {
  t: string;
  v: number;
}

export interface HistoryPayload {
  period?: string;
  from?: string;
  to?: string;
  series?: Record<string, HistoryPoint[]>;
}
