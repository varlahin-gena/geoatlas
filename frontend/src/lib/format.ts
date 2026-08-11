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

/** ISO → локаль ru-RU. timeZone — IANA (напр. Europe/Moscow); иначе пояс браузера. */
export function fmtDate(iso: unknown, timeZone?: string): string {
  if (!iso) return '—';
  try {
    const d = new Date(String(iso));
    if (Number.isNaN(d.getTime())) return String(iso);
    const opts: Intl.DateTimeFormatOptions = {};
    const tz = typeof timeZone === 'string' ? timeZone.trim() : '';
    if (tz) {
      opts.timeZone = tz;
      opts.timeZoneName = 'short';
    }
    return d.toLocaleString('ru-RU', opts);
  } catch {
    return String(iso);
  }
}

export function safeNext(next: string | null | undefined, fallback = '/'): string {
  if (!next || typeof next !== 'string') return fallback;
  return next.startsWith('/') && !next.startsWith('//') ? next : fallback;
}
