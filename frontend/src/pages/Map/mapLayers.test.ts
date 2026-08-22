import { describe, expect, it, vi } from 'vitest';

// Pure helpers live in mapLayers.ts next to deck layers; stub deck so vitest
// does not load WebGPU / wgsl_reflect.
vi.mock('@deck.gl/layers', () => ({
  ArcLayer: class ArcLayer {},
  GeoJsonLayer: class GeoJsonLayer {},
  ScatterplotLayer: class ScatterplotLayer {},
  TextLayer: class TextLayer {},
}));

import { hasCoords, statusRGB, topByCount, normalizeLonLat, resolveNodeLonLat, buildLineCoordFallback, buildDisplayCoordMap } from './mapLayers';
import type { MapLine } from './mapTypes';

describe('mapLayers helpers', () => {
  it('statusRGB maps blocked/unknown/allowed', () => {
    expect(statusRGB('blocked')).toEqual([248, 81, 73]);
    expect(statusRGB('unknown')).toEqual([110, 118, 129]);
    expect(statusRGB('allowed')).toEqual([63, 185, 80]);
    expect(statusRGB(undefined)).toEqual([63, 185, 80]);
  });

  it('hasCoords rejects missing or zero endpoints', () => {
    expect(
      hasCoords({
        src: 'a',
        dst: 'b',
        src_lat: 55,
        src_lon: 37,
        dst_lat: 52,
        dst_lon: 13,
      }),
    ).toBe(true);
    expect(hasCoords({ src: 'a', dst: 'b' })).toBe(false);
    expect(
      hasCoords({
        src: 'a',
        dst: 'b',
        src_lat: 0,
        src_lon: 0,
        dst_lat: 1,
        dst_lon: 2,
      }),
    ).toBe(false);
  });

  it('topByCount keeps highest count lines', () => {
    const lines: MapLine[] = [
      { src: 'a', dst: 'b', count: 1 },
      { src: 'c', dst: 'd', count: 9 },
      { src: 'e', dst: 'f', count: 3 },
    ];
    expect(topByCount(lines, 2).map((l) => l.count)).toEqual([9, 3]);
    expect(topByCount(lines, 10)).toHaveLength(3);
  });

  it('normalizeLonLat swaps out-of-range latitude', () => {
    expect(normalizeLonLat(92.8672, 56.0184)).toEqual([92.8672, 56.0184]);
    expect(normalizeLonLat(56.0184, 92.8672)).toEqual([92.8672, 56.0184]);
  });

  it('resolveNodeLonLat prefers line fallback when points map diverges', () => {
    const lines: MapLine[] = [
      {
        src: '1.2.3.4',
        dst: '10.93.0.49',
        src_lat: 40.7,
        src_lon: -74.0,
        dst_lat: 56.0184,
        dst_lon: 92.8672,
        count: 5,
      },
    ];
    const fallback = buildLineCoordFallback(lines);
    const points = {
      '1.2.3.4': { lat: 89, lon: 10, count: 5 },
      '10.93.0.49': { lat: 56.0184, lon: 92.8672, count: 5 },
    };
    expect(resolveNodeLonLat('1.2.3.4', undefined, undefined, points, fallback)).toEqual([
      -74.0, 40.7,
    ]);
    expect(resolveNodeLonLat('10.93.0.49', undefined, undefined, points, fallback)).toEqual([
      92.8672, 56.0184,
    ]);
  });

  it('buildDisplayCoordMap spreads co-located nodes', () => {
    const points = [
      { key: '10.72.0.1', lon: 38.8, lat: 45.1, count: 1 },
      { key: '10.72.0.2', lon: 38.8004, lat: 45.0996, count: 2 },
      { key: '8.8.8.8', lon: -122.1, lat: 37.4, count: 1 },
    ];
    const map = buildDisplayCoordMap(points);
    const a = map.get('10.72.0.1');
    const b = map.get('10.72.0.2');
    expect(a).toBeTruthy();
    expect(b).toBeTruthy();
    expect(a![0]).not.toBeCloseTo(b![0], 3);
    expect(map.get('8.8.8.8')).toEqual([-122.1, 37.4]);
  });
});
