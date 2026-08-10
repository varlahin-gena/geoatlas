export function escapeHTML(v: unknown): string {
  return String(v ?? '').replace(/[&<>"']/g, (ch) => {
    const map: Record<string, string> = {
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;',
    };
    return map[ch] ?? ch;
  });
}

export function fmtNumber(n: unknown): string {
  return (Number(n) || 0).toLocaleString('ru-RU');
}

export function fmtDate(iso: unknown): string {
  if (!iso) return '—';
  try {
    return new Date(String(iso)).toLocaleString('ru-RU');
  } catch {
    return String(iso);
  }
}

export function safeNext(next: string | null | undefined, fallback = '/'): string {
  if (!next || typeof next !== 'string') return fallback;
  return next.startsWith('/') && !next.startsWith('//') ? next : fallback;
}
