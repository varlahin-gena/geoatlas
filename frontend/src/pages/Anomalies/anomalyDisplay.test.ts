import { describe, expect, it } from 'vitest';
import type { AnomalyEvent } from '@/api/anomalies';
import { anomalyMapHref, matchesAnomalySearch } from './anomalyDisplay';

describe('anomalyDisplay', () => {
  const item = {
    fingerprint: 'abc',
    title: 'Scan from 203.0.113.5',
    code: 'port_scan',
    src_ip: '203.0.113.5',
    map: { period: '15m', group: 'ip', filter: 'all', q: 'src:203.0.113.5' },
  } as AnomalyEvent;

  it('builds map href from anomaly map link', () => {
    expect(anomalyMapHref(item)).toContain('group=ip');
    expect(anomalyMapHref(item)).toContain('q=src%3A203.0.113.5');
  });

  it('filters rows by search text', () => {
    expect(matchesAnomalySearch(item, '203.0.113')).toBe(true);
    expect(matchesAnomalySearch(item, 'missing')).toBe(false);
  });
});
