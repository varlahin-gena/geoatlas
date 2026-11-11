export type ConnectorLiveKind = 'on' | 'idle' | 'off';

/** Fresh last_event_at window for «жив» status. */
export const CONNECTOR_LIVE_WINDOW_MS = 5 * 60 * 1000;

export function connectorLiveKind(
  enabled: boolean,
  lastEventAt: string | undefined,
  now = Date.now(),
): ConnectorLiveKind {
  if (!enabled) return 'off';
  if (!lastEventAt) return 'idle';
  const t = Date.parse(lastEventAt);
  if (!Number.isFinite(t)) return 'idle';
  return now - t <= CONNECTOR_LIVE_WINDOW_MS ? 'on' : 'idle';
}

export function connectorLiveLabel(kind: ConnectorLiveKind): string {
  if (kind === 'on') return 'Жив';
  if (kind === 'idle') return 'Тишина';
  return 'Выкл';
}

export function connectorLiveClass(kind: ConnectorLiveKind): string {
  if (kind === 'on') return 'status-on';
  if (kind === 'idle') return 'status-idle';
  return 'status-off';
}

export function connectorLiveTitle(kind: ConnectorLiveKind): string {
  if (kind === 'on') {
    return 'Есть события за последние 5 мин. Клик — выключить.';
  }
  if (kind === 'idle') {
    return 'Включён, но свежего трафика нет (5 мин). Клик — выключить.';
  }
  return 'Выключен. Клик — включить.';
}

export function connectorProtocolsLabel(raw?: string): string {
  const key = (raw || '').trim().toLowerCase();
  if (key === 'both') return 'UDP+TCP';
  if (key === 'udp') return 'UDP';
  if (key === 'tcp') return 'TCP';
  return raw?.trim() || '—';
}
