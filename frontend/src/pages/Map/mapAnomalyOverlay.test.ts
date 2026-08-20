import { describe, expect, it } from 'vitest';
import { highlightFromAnomaly } from './mapAnomalyOverlay';
import type { AnomalyEvent } from '@/api/anomalies';
import type { MapLine, MapPoint } from './mapTypes';

const points: Record<string, MapPoint> = {
  '203.0.113.5': { lat: 1, lon: 2, country: 'CN', city: 'Beijing', count: 10 },
  '198.51.100.9': { lat: 3, lon: 4, country: 'US', city: 'Ashburn', count: 4 },
};

const lines: MapLine[] = [
  {
    src: '203.0.113.5',
    dst: '198.51.100.9',
    src_lat: 1,
    src_lon: 2,
    dst_lat: 3,
    dst_lon: 4,
    count: 5,
    status: 'blocked',
  },
];

describe('highlightFromAnomaly', () => {
  it('matches IP nodes and edge in ip grouping', () => {
    const item: AnomalyEvent = {
      fingerprint: 'abc',
      code: 'rep_new_peer',
      src_ip: '203.0.113.5',
      dst_ip: '198.51.100.9',
    };
    const h = highlightFromAnomaly(item, points, lines, 'ip');
    expect(h.nodeKeys).toEqual(['203.0.113.5', '198.51.100.9']);
    expect(h.edgeKeys).toHaveLength(1);
  });

  it('matches country in city grouping', () => {
    const item: AnomalyEvent = {
      fingerprint: 'c',
      code: 'new_country_dst',
      dst_country: 'CN',
    };
    const h = highlightFromAnomaly(item, points, [], 'city');
    expect(h.nodeKeys).toContain('203.0.113.5');
  });

  it('matches city in city grouping', () => {
    const item: AnomalyEvent = {
      fingerprint: 'city',
      code: 'custom_city_alert',
      src_city: 'Ashburn',
    };
    const h = highlightFromAnomaly(item, points, [], 'city');
    expect(h.nodeKeys).toContain('198.51.100.9');
  });
});
