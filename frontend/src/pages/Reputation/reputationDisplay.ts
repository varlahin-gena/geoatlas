const CATEGORY_LABELS: Record<string, string> = {
  attacks: 'Атаки',
  block: 'Блок',
  c2: 'C2',
  drop: 'Drop',
  malware: 'Вредоносное',
  botnet: 'Ботнет',
  spam: 'Спам',
  unknown: 'Неизвестно',
};

const FORMAT_LABELS: Record<string, string> = {
  netset: 'Список IP (netset)',
  plain: 'Список IP (plain)',
  spamhaus_json: 'Spamhaus JSON',
  csv_ip: 'CSV с IP',
};

export const REPUTATION_CATEGORY_OPTIONS = [
  { value: 'attacks', label: CATEGORY_LABELS.attacks },
  { value: 'block', label: CATEGORY_LABELS.block },
  { value: 'c2', label: CATEGORY_LABELS.c2 },
  { value: 'drop', label: CATEGORY_LABELS.drop },
  { value: 'malware', label: CATEGORY_LABELS.malware },
  { value: 'botnet', label: CATEGORY_LABELS.botnet },
  { value: 'spam', label: CATEGORY_LABELS.spam },
  { value: 'unknown', label: CATEGORY_LABELS.unknown },
] as const;

export const REPUTATION_FORMAT_OPTIONS = [
  { value: 'netset', label: FORMAT_LABELS.netset },
  { value: 'spamhaus_json', label: FORMAT_LABELS.spamhaus_json },
  { value: 'csv_ip', label: FORMAT_LABELS.csv_ip },
] as const;

export function reputationCategoryLabel(raw?: string): string {
  const key = (raw || '').trim().toLowerCase();
  if (!key) return '—';
  return CATEGORY_LABELS[key] || raw!.trim();
}

export function reputationFormatLabel(raw?: string): string {
  const key = (raw || '').trim().toLowerCase();
  if (!key) return '—';
  return FORMAT_LABELS[key] || raw!.trim();
}

export function reputationSourceLabel(raw?: string): string {
  const key = (raw || '').trim().toLowerCase();
  if (!key) return '—';
  if (key === 'url') return 'URL';
  if (key === 'upload' || key === 'file' || key === 'csv') return 'Файл';
  return raw!.trim();
}

/** Short label for table cells: host · filename, full URL via title. */
export function shortReputationUrl(raw?: string): string {
  const url = (raw || '').trim();
  if (!url) return '—';
  try {
    const u = new URL(url);
    const parts = u.pathname.split('/').filter(Boolean);
    const file = parts[parts.length - 1] || '';
    const host = u.host.replace(/^www\./, '');
    if (file && parts.length > 1) return `${host} · ${decodeURIComponent(file)}`;
    if (file) return `${host}/${decodeURIComponent(file)}`;
    return host || url;
  } catch {
    if (url.length <= 48) return url;
    return `${url.slice(0, 22)}…${url.slice(-18)}`;
  }
}
