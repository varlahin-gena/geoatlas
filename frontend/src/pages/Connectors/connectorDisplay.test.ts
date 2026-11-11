import { describe, expect, it } from 'vitest';
import {
  CONNECTOR_LIVE_WINDOW_MS,
  connectorLiveKind,
  connectorLiveLabel,
  connectorProtocolsLabel,
} from './connectorDisplay';

describe('connectorDisplay', () => {
  it('maps live kind from enabled + last event', () => {
    const now = 1_700_000_000_000;
    expect(connectorLiveKind(false, new Date(now).toISOString(), now)).toBe('off');
    expect(connectorLiveKind(true, undefined, now)).toBe('idle');
    expect(connectorLiveKind(true, new Date(now - 60_000).toISOString(), now)).toBe('on');
    expect(
      connectorLiveKind(true, new Date(now - CONNECTOR_LIVE_WINDOW_MS - 1).toISOString(), now),
    ).toBe('idle');
  });

  it('labels live state and protocols without jargon codes', () => {
    expect(connectorLiveLabel('on')).toBe('Жив');
    expect(connectorLiveLabel('idle')).toBe('Тишина');
    expect(connectorLiveLabel('off')).toBe('Выкл');
    expect(connectorProtocolsLabel('both')).toBe('UDP+TCP');
    expect(connectorProtocolsLabel('udp')).toBe('UDP');
    expect(connectorProtocolsLabel('tcp')).toBe('TCP');
  });
});
