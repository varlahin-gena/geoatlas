export type {
  AuditEvent,
  BackupCatalog,
  BackupEntry,
  BackupSchedule,
  DREvent,
  EdgesAgg,
  FailedLogin,
  HistoryPayload,
  HistoryPoint,
  Retention,
  SystemStats,
} from '@/api/systemTypes';

export type Tab = 'overview' | 'pipeline' | 'backup' | 'security' | 'audit' | 'charts';

export const CONTAINERS = ['backend', 'clickhouse', 'syslog-ng', 'frontend'] as const;
export const PERIODS = [
  ['1h', '1ч'],
  ['6h', '6ч'],
  ['24h', '24ч'],
  ['7d', '7д'],
] as const;
