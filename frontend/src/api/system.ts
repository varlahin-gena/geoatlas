import { apiDelete, apiFetch, apiGet, apiGetQuery, apiPath, apiPost, apiPut } from './client';
import type { SystemVersion } from './types';
import type { components } from './openapi';
import type {
  AuditEvent,
  BackupCatalog,
  BackupSchedule,
  DREvent,
  FailedLogin,
  HistoryPayload,
  Retention,
  SystemStats,
} from './systemTypes';

export type {
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

export type LoginPolicy = {
  max_fails: number;
  window_sec: number;
  lockout_sec: number;
};

export type AuthSessionRecord = {
  id: string;
  username: string;
  role: string;
  ip: string;
  user_agent?: string;
  created_at?: string;
  expires_at?: string;
  last_seen_at?: string;
};

type SecurityThreatWindow = {
  csrf_reject?: number;
  proxy_gate?: number;
  api_rate_limit?: number;
};

export type SystemSecuritySummary = {
  failed_logins?: FailedLogin[];
  login_policy?: LoginPolicy;
  sessions?: AuthSessionRecord[];
  auth_recent?: AuditEvent[];
  threat_counts?: {
    last_24h?: SecurityThreatWindow;
    last_7d?: SecurityThreatWindow;
  };
  posture?: {
    users?: {
      total?: number;
      by_role?: Record<string, number>;
      must_reset_password?: number;
    };
    tokens?: { total?: number; expiring_soon?: number };
    tls?: {
      configured?: boolean;
      days_left?: number;
      self_signed?: boolean;
      not_after?: string;
    };
  };
};

export function fetchSecuritySummary(init?: RequestInit): Promise<SystemSecuritySummary> {
  return apiGet('/api/system/security', init) as Promise<SystemSecuritySummary>;
}

export function putLoginPolicy(body: LoginPolicy): Promise<LoginPolicy> {
  return apiPut('/api/system/login-policy', body) as Promise<LoginPolicy>;
}

export function revokeSession(id: string): Promise<{ ok?: boolean }> {
  return apiDelete(apiPath('/api/auth/sessions/{id}', { id })) as Promise<{ ok?: boolean }>;
}

export function revokeUserSessions(username: string): Promise<{ ok?: boolean; revoked?: number }> {
  return apiPost('/api/auth/sessions/revoke-user', { username }) as Promise<{
    ok?: boolean;
    revoked?: number;
  }>;
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

export function ackSystemAlert(code: string, target?: string): Promise<{ ok?: boolean }> {
  return apiPost('/api/system/alerts/ack', { code, target: target || '' }) as Promise<{
    ok?: boolean;
  }>;
}
