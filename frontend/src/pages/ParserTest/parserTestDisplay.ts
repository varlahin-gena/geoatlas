export type ParseRowStatus = 'parsed' | 'skipped' | 'error';

export function parseRowStatus(row: {
  parsed?: boolean;
  skipped?: boolean;
}): ParseRowStatus {
  if (row.parsed) return 'parsed';
  if (row.skipped) return 'skipped';
  return 'error';
}

export function parseRowStatusLabel(status: ParseRowStatus): string {
  if (status === 'parsed') return 'Разобрано';
  if (status === 'skipped') return 'Пропущено';
  return 'Ошибка';
}

export function shortParseLine(raw?: string, max = 80): string {
  const text = (raw || '').replace(/\s+/g, ' ').trim();
  if (!text) return '—';
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1)}…`;
}

export function parseFieldsLabel(row: {
  src_ip?: string;
  dst_ip?: string;
  action?: string;
  proto?: string;
  reason?: string;
  parsed?: boolean;
}): string {
  if (row.parsed) {
    return `${row.src_ip || '?'} → ${row.dst_ip || '?'} · ${row.action || '—'} · ${row.proto || '—'}`;
  }
  return (row.reason || '').trim() || '—';
}
