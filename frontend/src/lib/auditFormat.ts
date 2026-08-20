import type { AuditEvent, DREvent } from '@/api/systemTypes';

function pad2(n: number): string {
  return String(n).padStart(2, '0');
}

function formatBool(v: unknown, on = 'вкл', off = 'выкл'): string {
  if (v === true) return on;
  if (v === false) return off;
  return String(v ?? '—');
}

export function formatScheduleChangeDetails(d: Record<string, unknown>): string {
  const parts: string[] = [];
  const prev = d.prev_enabled;
  const next = d.enabled;
  if (typeof prev === 'boolean' && typeof next === 'boolean' && prev !== next) {
    parts.push(`авто: ${formatBool(prev)} → ${formatBool(next)}`);
  } else if (typeof next === 'boolean') {
    parts.push(`авто: ${formatBool(next)}`);
  }
  if (typeof d.hour === 'number' && typeof d.minute === 'number') {
    parts.push(`время: ${pad2(d.hour)}:${pad2(d.minute)}`);
  }
  if (typeof d.timezone === 'string' && d.timezone) {
    parts.push(`TZ: ${d.timezone}`);
  }
  if (typeof d.keep === 'number') {
    parts.push(`keep: ${d.keep}`);
  }
  if (typeof d.include_edges === 'boolean') {
    parts.push(`edges: ${formatBool(d.include_edges)}`);
  }
  if (typeof d.include_auth === 'boolean') {
    parts.push(`auth: ${formatBool(d.include_auth)}`);
  }
  return parts.length ? parts.join('; ') : '—';
}

export function formatAuditDetails(action?: string, details?: Record<string, unknown>): string {
  if (!details || !Object.keys(details).length) return '—';
  if (details.error) return String(details.error);

  if (action === 'backup.schedule.update') {
    return formatScheduleChangeDetails(details);
  }

  return Object.entries(details)
    .map(([k, v]) => `${k}: ${typeof v === 'boolean' ? formatBool(v) : String(v)}`)
    .join('; ');
}

export function formatDRMessage(item: DREvent): string {
  if (item.action === 'backup.schedule.update' && item.meta) {
    const details = formatScheduleChangeDetails(item.meta);
    if (details !== '—') return details;
  }
  return item.message || '—';
}

export function formatAuditRow(item: AuditEvent): string {
  return formatAuditDetails(item.action, item.details);
}
