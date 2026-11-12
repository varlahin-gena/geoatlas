import { describe, expect, it } from 'vitest';
import type { AnomalyEvent } from '@/api/anomalies';
import {
  anomalyCodeLabel,
  anomalyEndpointLabel,
  anomalyEventsHours,
  anomalyEventsQuery,
  anomalyMapHref,
  anomalyPeersScopeLabel,
  compareAnomalies,
  formatAnomalyDescription,
  formatAnomalyWindow,
  investigateHref,
  investigationTemplateName,
  matchesAnomalySearch,
  nextAnomalySort,
  peersLinesToCsv,
} from './anomalyDisplay';

describe('anomalyDisplay', () => {
  const item = {
    fingerprint: 'abc',
    title: 'Scan from 203.0.113.5',
    code: 'port_scan',
    src_ip: '203.0.113.5',
    map: { period: '15m', group: 'ip', filter: 'all', q: 'src:203.0.113.5' },
  } as AnomalyEvent;

  it('builds map href from anomaly map link', () => {
    const href = anomalyMapHref(item);
    expect(href).toContain('group=ip');
    expect(href).toContain('q=src%3A203.0.113.5');
    expect(href).toContain('alert=abc');
    expect(href).not.toContain('period=15m');
  });

  it('filters rows by search text', () => {
    expect(matchesAnomalySearch(item, '203.0.113')).toBe(true);
    expect(matchesAnomalySearch(item, 'missing')).toBe(false);
  });

  it('maps detector codes to Russian labels', () => {
    expect(anomalyCodeLabel('horizontal_scan')).toBe('Сканирование подсети');
    expect(anomalyCodeLabel('port_scan')).toBe('Сканирование портов');
    expect(anomalyCodeLabel('unknown_code')).toBe('unknown_code');
  });

  it('derives events hours and query for peers panel', () => {
    expect(anomalyEventsHours(item)).toBe(1);
    expect(anomalyEventsHours({ ...item, map: { ...item.map!, period: '1d' } })).toBe(24);
    expect(anomalyEventsHours({ ...item, map: { ...item.map!, period: '2h' } })).toBe(2);
    expect(anomalyEventsQuery(item)).toBe('src:203.0.113.5');
    expect(
      anomalyEventsQuery({ fingerprint: 'x', src_ip: '10.0.0.1' } as AnomalyEvent),
    ).toBe('src:10.0.0.1');
  });

  it('builds investigate href', () => {
    expect(investigateHref('fp/1')).toBe('/investigate?alert=fp%2F1');
    expect(investigateHref('')).toBe('/investigate');
  });

  it('serializes peers CSV', () => {
    const csv = peersLinesToCsv([
      {
        src: '10.0.0.1',
        dst: '1.2.3.4',
        dst_port: 443,
        last_action: 'allow',
        count: 12,
        bytes_sent: 100,
        bytes_recv: 200,
      },
      { src: 'a,b', dst: 'c', count: 1 },
    ]);
    expect(csv.startsWith('src,dst,dst_port,action,count,bytes_sent,bytes_recv\n')).toBe(true);
    expect(csv).toContain('10.0.0.1,1.2.3.4,443,allow,12,100,200');
    expect(csv).toContain('"a,b",c,,');
  });

  it('builds investigation template name', () => {
    const name = investigationTemplateName(item, new Date(Date.UTC(2026, 8, 1)));
    expect(name).toBe('IR port_scan 203.0.113.5 2026-09-01');
  });

  it('formats blocked surge description from detail', () => {
    const surge = {
      code: 'blocked_surge',
      code_label: 'Всплеск блокировок',
      title: 'Всплеск блокировок: 10.0.0.0/8 (Новосибирск) (680861 за 15 мин, было 103447)',
      detail: {
        network: '10.0.0.0/8',
        label: 'Новосибирск',
        blocked_current: 680861,
        blocked_previous: 103447,
        window_minutes: 15,
      },
    } as AnomalyEvent;
    expect(formatAnomalyDescription(surge)).toEqual({
      subject: 'Новосибирск',
      detail: '10.0.0.0/8',
      delta: '680\u00a0861 за 15 мин · было 103\u00a0447',
    });
  });

  it('strips code prefix from generic titles', () => {
    expect(
      formatAnomalyDescription({
        ...item,
        title: 'Сканирование портов с 203.0.113.5',
      }),
    ).toEqual({
      subject: 'с 203.0.113.5',
    });
    expect(formatAnomalyDescription(item)).toEqual({
      subject: 'Scan from 203.0.113.5',
    });
  });

  it('prefers IP then place for endpoint labels', () => {
    expect(anomalyEndpointLabel('10.0.0.1', 'Москва', 'RU')).toBe('10.0.0.1');
    expect(anomalyEndpointLabel('', 'Москва', 'RU')).toBe('Москва, RU');
    expect(anomalyEndpointLabel('', '', '', '10.0.0.0/8')).toBe('10.0.0.0/8');
    expect(anomalyEndpointLabel()).toBe('');
  });

  it('sorts anomalies and toggles direction', () => {
    const a = { fingerprint: 'a', detected_at: '2026-11-11T10:00:00Z', event_count: 10 } as AnomalyEvent;
    const b = { fingerprint: 'b', detected_at: '2026-11-11T12:00:00Z', event_count: 5 } as AnomalyEvent;
    expect(compareAnomalies(a, b, 'detected_at', 'desc')).toBeGreaterThan(0);
    expect(compareAnomalies(a, b, 'event_count', 'desc')).toBeLessThan(0);
    expect(nextAnomalySort('detected_at', 'desc', 'detected_at')).toEqual({
      key: 'detected_at',
      dir: 'asc',
    });
    expect(nextAnomalySort('detected_at', 'desc', 'event_count')).toEqual({
      key: 'event_count',
      dir: 'desc',
    });
  });

  it('formats same-day windows compactly', () => {
    const win = formatAnomalyWindow('2026-11-11T03:22:26.000Z', '2026-11-11T03:37:26.000Z');
    expect(win.title).toContain('→');
    expect(win.label).toMatch(/^\d{2}:\d{2}–\d{2}:\d{2}$/);
  });

  it('builds human peers scope labels', () => {
    expect(
      anomalyPeersScopeLabel(
        { map: { q: 'city:Новосибирск' } } as AnomalyEvent,
        1,
      ),
    ).toBe('Связи за 1 ч · город Новосибирск');
    expect(anomalyPeersScopeLabel({ src_ip: '10.0.0.1' } as AnomalyEvent, 2)).toBe(
      'Связи за 2 ч · источник 10.0.0.1',
    );
  });
});
