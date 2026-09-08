import { describe, expect, it } from 'vitest';
import {
  MAP_REFRESH_DEFAULT_SEC,
  mapRefreshSecToMs,
  parseMapRefreshSec,
} from './mapRefreshInterval';

describe('parseMapRefreshSec', () => {
  it('accepts presets 30 / 60 / 300', () => {
    expect(parseMapRefreshSec(30)).toBe(30);
    expect(parseMapRefreshSec('60')).toBe(60);
    expect(parseMapRefreshSec(300)).toBe(300);
  });

  it('falls back to 5 minutes for invalid values', () => {
    expect(MAP_REFRESH_DEFAULT_SEC).toBe(300);
    expect(parseMapRefreshSec(null)).toBe(300);
    expect(parseMapRefreshSec('')).toBe(300);
    expect(parseMapRefreshSec(15)).toBe(300);
    expect(parseMapRefreshSec('nope')).toBe(300);
  });
});

describe('mapRefreshSecToMs', () => {
  it('converts seconds to ms', () => {
    expect(mapRefreshSecToMs(30)).toBe(30_000);
    expect(mapRefreshSecToMs(60)).toBe(60_000);
    expect(mapRefreshSecToMs(300)).toBe(300_000);
  });
});
