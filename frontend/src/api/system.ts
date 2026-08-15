import { apiFetch, apiGet } from './client';
import type { SystemVersion } from './types';
import type { components } from './openapi';
import type {
  BackupCatalog,
  BackupSchedule,
  HistoryPayload,
  Retention,
  SystemStats,
} from './systemTypes';

export type {
  BackupCatalog,
  BackupSchedule,
  HistoryPayload,
  Retention,
  SystemStats,
};
export type { SystemVersion };

export type SystemStatus = components['schemas']['SystemStatus'];

export function fetchSystemVersion(init?: RequestInit): Promise<SystemVersion> {
  return apiGet('/api/system/version', init);
}

export function fetchSystemStatus(init?: RequestInit): Promise<SystemStatus> {
  return apiGet('/api/system/status', { cache: 'no-store', ...init });
}

export function fetchSystemStats(init?: RequestInit): Promise<SystemStats> {
  return apiGet('/api/system/stats', init) as Promise<SystemStats>;
}

export function fetchRetention(): Promise<{ retention?: Retention } & Retention> {
  return apiFetch<{ retention?: Retention } & Retention>('/api/system/retention');
}

export function putRetention(body: Retention): Promise<{ retention?: Retention }> {
  return apiFetch<{ retention?: Retention }>('/api/system/retention', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export function fetchSystemHistory(period: string): Promise<HistoryPayload> {
  return apiFetch<HistoryPayload>(`/api/system/history?period=${encodeURIComponent(period)}`);
}

export function fetchBackups(): Promise<BackupCatalog> {
  return apiFetch<BackupCatalog>('/api/system/backups');
}

export function createBackup(): Promise<unknown> {
  return apiFetch('/api/system/backups', { method: 'POST', body: '{}' });
}

export function putBackupSchedule(schedule: BackupSchedule): Promise<{
  ok?: boolean;
  schedule?: BackupSchedule;
}> {
  return apiFetch<{ ok?: boolean; schedule?: BackupSchedule }>('/api/system/backup-schedule', {
    method: 'PUT',
    body: JSON.stringify(schedule),
  });
}

export function attachBackup(name: string): Promise<unknown> {
  return apiFetch(`/api/system/backups/${encodeURIComponent(name)}/attach`, {
    method: 'POST',
    body: '{}',
  });
}

export function detachBackup(name: string): Promise<unknown> {
  return apiFetch(`/api/system/backups/${encodeURIComponent(name)}/detach`, {
    method: 'POST',
    body: '{}',
  });
}

export function deleteBackup(name: string): Promise<unknown> {
  return apiFetch(`/api/system/backups/${encodeURIComponent(name)}`, { method: 'DELETE' });
}
