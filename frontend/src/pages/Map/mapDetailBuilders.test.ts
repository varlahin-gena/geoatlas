import { describe, expect, it, vi } from 'vitest';

vi.mock('@deck.gl/layers', () => ({
  ArcLayer: class ArcLayer {},
  GeoJsonLayer: class GeoJsonLayer {},
  ScatterplotLayer: class ScatterplotLayer {},
  TextLayer: class TextLayer {},
}));

import { buildLineDetail } from './mapDetailBuilders';
import type { MapLine, MapPoint } from './mapTypes';

describe('buildLineDetail', () => {
  it('shows city from map points on endpoints', () => {
    const line: MapLine = {
      src: '173.0.106.19',
      dst: '10.93.0.49',
      src_lat: 37,
      src_lon: -122,
      dst_lat: 56.0184,
      dst_lon: 92.8672,
      count: 37,
      status: 'blocked',
      src_country: 'США',
      dst_country: 'Россия',
      src_zone: 'outside',
      dst_zone: 'inside',
    };
    const points: Record<string, MapPoint> = {
      '173.0.106.19': { lat: 37, lon: -122, country: 'США', city: '', count: 1 },
      '10.93.0.49': { lat: 56.0184, lon: 92.8672, country: 'Россия', city: 'Красноярск', count: 1 },
    };
    const detail = buildLineDetail(line, 'ip', [], points);
    const dst = detail.sections.find((s) => s.title === 'Назначение');
    expect(dst?.rows.some((r) => r.key === 'City' && r.value === 'Красноярск')).toBe(true);
    expect(dst?.rows.some((r) => r.key === 'Country' && r.value === 'Россия')).toBe(true);
  });
});
