const KIND_LABELS: Record<string, string> = {
  public_unknown: 'Публичный',
  private: 'Приватный',
  loopback: 'Loopback',
  link_local: 'Link-local',
  multicast: 'Multicast',
  invalid: 'Некорректный',
  unknown: 'Неизвестно',
};

export function geoMissingKindLabel(raw?: string): string {
  const key = (raw || '').trim();
  if (!key) return '—';
  return KIND_LABELS[key] || key;
}

export function geoMissingLogPlace(city?: string, country?: string): string {
  const parts = [city, country].map((x) => (x || '').trim()).filter(Boolean);
  return parts.length ? parts.join(', ') : '—';
}
