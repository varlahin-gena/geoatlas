import { describe, expect, it, vi } from 'vitest';

// Pure helpers live in mapLayers.ts next to deck layers; stub deck so vitest
// does not load WebGPU / wgsl_reflect.
vi.mock('@deck.gl/layers', () => ({
  ArcLayer: class ArcLayer {},
  GeoJsonLayer: class GeoJsonLayer {},
  ScatterplotLayer: class ScatterplotLayer {},
  TextLayer: class TextLayer {},
}));

import { hasCoords, statusRGB, topByCount } from './mapLayers';
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
});
