export function fmtNumber(n: unknown): string {
  return (Number(n) || 0).toLocaleString('ru-RU');
}

/** Bytes → Б / КБ / МБ / ГБ (binary 1024). */
export function fmtBytes(bytes: unknown): string {
  const b = Number(bytes);
  const n = Number.isFinite(b) ? Math.max(0, b) : 0;
  if (n < 1024) return `${fmtNumber(n)} Б`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} КБ`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} МБ`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} ГБ`;
}

/** ISO → локаль ru-RU. timeZone — IANA (напр. Europe/Moscow); иначе пояс браузера. */
export function fmtDate(iso: unknown, timeZone?: string): string {
  if (!iso) return '—';
  try {
    const d = new Date(String(iso));
    if (Number.isNaN(d.getTime())) return String(iso);
    // Go zero time / unset JSON timestamps marshal as 0001-01-01…
    if (d.getUTCFullYear() < 1970) return '—';
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
