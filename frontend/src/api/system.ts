import { apiDelete, apiFetch, apiGet, apiGetQuery, apiPath, apiPost, apiPut } from './client';
import type { SystemVersion } from './types';
import type { components } from './openapi';
import type {
  AuditEvent,
  BackupCatalog,
  BackupSchedule,
  DREvent,
  HistoryPayload,
  Retention,
  SystemStats,
} from './systemTypes';

export type {
  AuditEvent,
  BackupCatalog,
  BackupSchedule,
  DREvent,
  HistoryPayload,
  Retention,
  SystemStats,
} from './systemTypes';
export type { SystemVersion };

export type SystemStatus = components['schemas']['SystemStatus'];
export type TlsStatus = components['schemas']['TlsStatus'];
export type TlsMutationResponse = components['schemas']['TlsMutationResponse'];

export function fetchSystemVersion(init?: RequestInit): Promise<SystemVersion> {
  return apiGet('/api/system/version', init);
}

export function fetchSystemStatus(init?: RequestInit): Promise<SystemStatus> {
  return apiGet('/api/system/status', { cache: 'no-store', ...init });
}

export function fetchSystemStats(init?: RequestInit): Promise<SystemStats> {
  // OpenAPI SystemStats is a loose blob; UI type narrows nested metric maps.
  return apiGet('/api/system/stats', init) as Promise<SystemStats>;
}

export function fetchRetention(): Promise<{ ok?: boolean; retention?: Retention }> {
  return apiGet('/api/system/retention');
}

export function putRetention(body: Retention): Promise<{ ok?: boolean; retention?: Retention }> {
  return apiPut('/api/system/retention', body);
}

export function fetchSystemHistory(period: string): Promise<HistoryPayload> {
  return apiGetQuery('/api/system/history', { period });
}

export function fetchBackups(): Promise<BackupCatalog> {
  return apiGet('/api/system/backups');
}

export function fetchDRHistory(query?: {
  since?: string;
  limit?: number;
  action?: string;
  status?: string;
  actor?: string;
}): Promise<{ items?: DREvent[] }> {
  return apiGetQuery('/api/dr/history', query || {});
}

export function fetchAuditLog(query?: {
  since?: string;
  limit?: number;
  action?: string;
  result?: string;
  actor?: string;
}): Promise<{ items?: AuditEvent[] }> {
  return apiGetQuery('/api/audit', query || {});
}

export function createBackup(): Promise<unknown> {
  return apiPost('/api/system/backups', {});
}

export function putBackupSchedule(schedule: BackupSchedule): Promise<{
  ok?: boolean;
  schedule?: BackupSchedule;
}> {
  return apiPut('/api/system/backup-schedule', schedule);
}

export function attachBackup(name: string): Promise<unknown> {
  return apiFetch(apiPath('/api/system/backups/{name}/attach', { name }), {
    method: 'POST',
    body: '{}',
  });
}

export function detachBackup(name: string): Promise<unknown> {
  return apiFetch(apiPath('/api/system/backups/{name}/detach', { name }), {
    method: 'POST',
    body: '{}',
  });
}

export function deleteBackup(name: string): Promise<unknown> {
  return apiDelete(apiPath('/api/system/backups/{name}', { name }));
}

export function fetchTlsStatus(): Promise<components['schemas']['TlsStatusResponse']> {
  return apiGet('/api/system/tls');
}

export function putTls(body: {
  cert_pem: string;
  key_pem: string;
  current_password: string;
}): Promise<TlsMutationResponse> {
  return apiPut('/api/system/tls', body);
}

export function postTlsReload(): Promise<TlsMutationResponse> {
  return apiPost('/api/system/tls/reload', {});
}
