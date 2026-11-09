import { apiFetch } from '@/api/client';

export type ConnectorProtocols = 'udp' | 'tcp' | 'both';

export interface Connector {
  id: string;
  name: string;
  port: number;
  protocols: ConnectorProtocols;
  enabled: boolean;
  builtin?: boolean;
}

export interface ConnectorsListResponse {
  ok: boolean;
  connectors: Connector[];
  updated_at?: string;
  max_connectors: number;
  extra_ports: { min: number; max: number };
  default_port: number;
}

export interface ConnectorApplyResult {
  ok: boolean;
  conf_path?: string;
  reloaded: boolean;
  reload_error?: string;
  connectors: number;
  enabled: number;
}

export function listConnectors() {
  return apiFetch<ConnectorsListResponse>('/api/connectors');
}

export function createConnector(body: {
  id: string;
  name: string;
  port: number;
  protocols: ConnectorProtocols;
  enabled?: boolean;
}) {
  return apiFetch<{ ok: boolean; connector: Connector }>('/api/connectors', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

export function updateConnector(
  id: string,
  body: Partial<{ name: string; port: number; protocols: ConnectorProtocols; enabled: boolean }>,
) {
  return apiFetch<{ ok: boolean; connector: Connector }>(`/api/connectors/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

export function deleteConnector(id: string) {
  return apiFetch<{ ok: boolean; deleted: string }>(`/api/connectors/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export function applyConnectors() {
  return apiFetch<{ ok: boolean; apply: ConnectorApplyResult }>('/api/connectors/apply', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
  });
}
