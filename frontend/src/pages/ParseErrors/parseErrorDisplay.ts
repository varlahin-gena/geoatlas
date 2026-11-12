/** Short human label for parse-error reason; full text stays available via title. */
export function parseErrorReasonLabel(raw?: string): string {
  const text = (raw || '').trim();
  if (!text) return '—';

  const lower = text.toLowerCase();
  const parsers = /парсером\(ами\)\s*\[([^\]]+)\]/i.exec(text);
  const parserList = parsers?.[1]?.trim();

  if (lower.includes('не удалось разобрать') || lower.includes('не разобр')) {
    return parserList ? `Не разобрано · ${parserList}` : 'Не разобрано';
  }
  if (lower.includes('не распознан') || lower.includes('unknown format') || lower.includes('no parser')) {
    return parserList ? `Формат не распознан · ${parserList}` : 'Формат не распознан';
  }
  if (lower.includes('skip') || lower.includes('пропущ')) {
    return 'Пропущено';
  }
  if (text.length <= 56) return text;
  return `${text.slice(0, 53)}…`;
}

/** One-line preview of a raw syslog line for table cells. */
export function shortParseErrorRaw(raw?: string, max = 72): string {
  const text = (raw || '').replace(/\s+/g, ' ').trim();
  if (!text) return '—';
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1)}…`;
}

export function parseErrorReasonCounts(rows: { reason?: string }[]): { label: string; count: number }[] {
  const map = new Map<string, number>();
  for (const row of rows) {
    const label = parseErrorReasonLabel(row.reason);
    map.set(label, (map.get(label) || 0) + 1);
  }
  return [...map.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label, 'ru'));
}
