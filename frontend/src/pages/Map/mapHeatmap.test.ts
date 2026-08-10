import { describe, expect, it } from 'vitest';
import {
  buildCountryStats,
  countryAliases,
  lineMatchesFocusedCountry,
} from './mapHeatmap';
import type { MapLine, MapPointEntry } from './mapTypes';

describe('mapHeatmap helpers', () => {
  it('countryAliases includes EN and RU for Russia', () => {
    const a = countryAliases('Russia');
    expect(a.has('russia')).toBe(true);
    expect(a.has('россия')).toBe(true);
  });

  it('lineMatchesFocusedCountry matches src/dst country and point country', () => {
    const points: Record<string, { lat: number; lon: number; country?: string }> = {
      a: { lat: 55, lon: 37, country: 'Russia' },
      b: { lat: 52, lon: 13, country: 'Germany' },
    };
    const l: MapLine = {
      src: 'a',
      dst: 'b',
      src_country: 'Russia',
      dst_country: 'Germany',
    };
    expect(lineMatchesFocusedCountry(l, null, points)).toBe(true);
    expect(lineMatchesFocusedCountry(l, 'Russia', points)).toBe(true);
    expect(lineMatchesFocusedCountry(l, 'Germany', points)).toBe(true);
    expect(lineMatchesFocusedCountry(l, 'France', points)).toBe(false);
    // Focused EN name matches RU label on the line via COUNTRY_NAMES_RU reverse lookup
    const ruLine: MapLine = {
      src: 'a',
      dst: 'b',
      src_country: 'Россия',
      dst_country: 'Germany',
    };
    expect(lineMatchesFocusedCountry(ruLine, 'Russia', points)).toBe(true);
  });

  it('buildCountryStats sums counts by country', () => {
    const points: MapPointEntry[] = [
      { key: '1', lat: 1, lon: 1, country: 'Russia', count: 3 },
      { key: '2', lat: 2, lon: 2, country: 'Russia', count: 2 },
      { key: '3', lat: 3, lon: 3, country: 'Germany', count: 1 },
    ];
    const stats = buildCountryStats(points);
    expect(stats.Russia).toBe(5);
    expect(stats.Germany).toBe(1);
  });
});
