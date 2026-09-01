import { describe, expect, it } from 'vitest';
import type { AnomalyEvent } from '@/api/anomalies';
import {
  anomalyEventsHours,
  anomalyEventsQuery,
  anomalyMapHref,
  matchesAnomalySearch,
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

  it('derives events hours and query for peers panel', () => {
    expect(anomalyEventsHours(item)).toBe(1);
    expect(anomalyEventsHours({ ...item, map: { ...item.map!, period: '1d' } })).toBe(24);
    expect(anomalyEventsHours({ ...item, map: { ...item.map!, period: '2h' } })).toBe(2);
    expect(anomalyEventsQuery(item)).toBe('src:203.0.113.5');
    expect(
      anomalyEventsQuery({ fingerprint: 'x', src_ip: '10.0.0.1' } as AnomalyEvent),
    ).toBe('src:10.0.0.1');
  });
});
