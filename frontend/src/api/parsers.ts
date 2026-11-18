import { apiFetch } from '@/api/client';

export type ParserKind = 'cef' | 'kv';

export interface ParserFieldMap {
  src?: string;
  dst?: string;
  spt?: string;
  dpt?: string;
  act?: string;
  proto?: string;
  rule?: string;
  src_zone?: string;
  dst_zone?: string;
  src_country?: string;
  dst_country?: string;
  bytes_sent?: string;
  bytes_recv?: string;
  packets_sent?: string;
  packets_recv?: string;
  time?: string;
}

export interface ParserTransforms {
  proto_map_number?: boolean;
  time_unit?: string;
  skip_undefined_zones?: boolean;
  src_zone_fallback?: string[];
  dst_zone_fallback?: string[];
  rule_join?: { primary?: string; policy?: string };
}

export interface ParserSpec {
  kind: ParserKind;
  match: { cef_vendor?: string; required_keys?: string[] };
  fields: ParserFieldMap;
  transforms?: ParserTransforms;
  samples?: string[];
}

export interface ParserItem {
  id: string;
  name: string;
  vendor: string;
  enabled: boolean;
  priority: number;
  seed?: boolean;
  source: 'db' | 'builtin';
  editable: boolean;
  kind?: ParserKind;
  spec?: ParserSpec;
}

export interface ParseDryRunRow {
  n: number;
  line: string;
  parsed: boolean;
  skipped: boolean;
  vendor?: string;
  reason?: string;
  src_ip?: string;
  dst_ip?: string;
  action?: string;
  proto?: string;
}

export interface ParseDryRunResult {
  received: number;
  parsed: number;
  skipped: number;
  errors: number;
  results: ParseDryRunRow[];
}

export function listParsers() {
  return apiFetch<{ ok: boolean; parsers: ParserItem[] }>('/api/parsers');
}

export function getParser(id: string) {
  return apiFetch<{ ok: boolean; parser: ParserItem }>(`/api/parsers/${encodeURIComponent(id)}`);
}

export function createParser(body: {
  id: string;
  name: string;
  vendor: string;
  enabled?: boolean;
  priority?: number;
  spec: ParserSpec;
}) {
  return apiFetch<{ ok: boolean; parser: ParserItem }>('/api/parsers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

export function updateParser(
  id: string,
  body: Partial<{
    name: string;
    vendor: string;
    enabled: boolean;
    priority: number;
    spec: ParserSpec;
  }>,
) {
  return apiFetch<{ ok: boolean; parser: ParserItem }>(`/api/parsers/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

export function deleteParser(id: string) {
  return apiFetch<{ ok: boolean }>(`/api/parsers/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export function reorderParsers(order: { id: string; priority: number }[]) {
  return apiFetch<{ ok: boolean }>('/api/parsers/order', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ order }),
  });
}

export function dryRunParser(body: {
  text: string;
  rule_id?: string;
  spec?: ParserSpec;
  vendor?: string;
  name?: string;
}) {
  return apiFetch<ParseDryRunResult>('/api/parsers/dry-run', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}
