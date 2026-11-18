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

/** Empty string / nullish / zero numeric (omitempty) → em dash. */
export function formatParseCell(value?: string | number | null): string {
  if (value === undefined || value === null) return '—';
  if (typeof value === 'number') {
    if (!Number.isFinite(value) || value === 0) return '—';
    return String(value);
  }
  const text = value.trim();
  return text || '—';
}
