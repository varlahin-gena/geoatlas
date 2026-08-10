export type Tab = 'overview' | 'pipeline' | 'security' | 'charts';

export const CONTAINERS = ['backend', 'clickhouse', 'syslog-ng', 'frontend'] as const;
export const PERIODS = [
  ['1h', '1ч'],
  ['6h', '6ч'],
  ['24h', '24ч'],
  ['7d', '7д'],
] as const;

export interface Alert {
  level?: string;
  code?: string;
  target?: string;
  message?: string;
}

export interface FailedLogin {
  username?: string;
  ip?: string;
  count?: number;
  first_at?: string;
  last_at?: string;
  locked?: boolean;
  locked_until?: string;
}

export interface EdgesAgg {
  state?: string;
  phase?: string;
  message?: string;
  raw_rows?: number;
  agg_rows?: number;
  days_total?: number;
  days_done?: number;
  map_source?: string;
  prefer_agg?: boolean;
  geo_prefer_agg?: boolean;
  started_at?: string;
  updated_at?: string;
}

export interface SystemStats {
  alerts?: Alert[];
  containers?: Record<string, { cpu_pct?: number; mem_bytes?: number }>;
  health?: Record<string, Record<string, unknown>>;
  pipeline?: Record<string, Record<string, number>>;
  storage?: Record<string, Record<string, number>>;
  backend_info?: { num_goroutine?: number; heap_alloc_mb?: number; go_version?: string };
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
