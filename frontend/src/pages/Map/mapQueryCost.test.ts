import { describe, expect, it } from 'vitest';
import {
  assessMapQueryCost,
  effectiveMapLimit,
  mapQueryWarning,
} from './mapQueryCost';

describe('assessMapQueryCost', () => {
  it('marks ip+6h as heavy', () => {
    const cost = assessMapQueryCost({
      period: '6h',
      periodFrom: '',
      periodTo: '',
      groupBy: 'ip',
      search: '',
      focusedCountry: null,
      repActive: false,
    });
    expect(cost.tier).toBe('heavy');
    expect(cost.limitCap).toBe(3000);
  });

  it('raises cap when filter present on heavy query', () => {
    const cost = assessMapQueryCost({
      period: '7d',
      periodFrom: '',
      periodTo: '',
      groupBy: 'ip',
      search: 'src_ip:10.0.0.1',
      focusedCountry: null,
      repActive: false,
    });
    expect(cost.tier).toBe('heavy');
    expect(cost.limitCap).toBe(8000);
  });

  it('city+1d stays light', () => {
    const cost = assessMapQueryCost({
      period: '1d',
      periodFrom: '',
      periodTo: '',
      groupBy: 'city',
      search: '',
      focusedCountry: null,
      repActive: false,
    });
    expect(cost.tier).toBe('light');
  });
});

describe('effectiveMapLimit', () => {
  it('caps heavy requests', () => {
    const cost = assessMapQueryCost({
      period: '1h',
      periodFrom: '',
      periodTo: '',
      groupBy: 'ip',
      search: '',
      focusedCountry: null,
      repActive: false,
    });
    const { applied, capped } = effectiveMapLimit(20000, cost);
    expect(capped).toBe(true);
    expect(applied).toBe(3000);
  });
});

describe('mapQueryWarning', () => {
  it('returns message for heavy capped query', () => {
    const cost = assessMapQueryCost({
      period: '6h',
      periodFrom: '',
      periodTo: '',
      groupBy: 'ip',
      search: '',
      focusedCountry: null,
      repActive: false,
    });
    const msg = mapQueryWarning({
      cost,
      requestedLimit: 5000,
      effectiveLimit: 3000,
      limitCapped: true,
      source: 'ip_live_ip',
    });
    expect(msg).toMatch(/Тяжёлый запрос/);
    expect(msg).toMatch(/3.?000/);
  });

  it('returns null for light city day', () => {
    const cost = assessMapQueryCost({
      period: '1d',
      periodFrom: '',
      periodTo: '',
      groupBy: 'city',
      search: '',
      focusedCountry: null,
      repActive: false,
    });
    expect(
      mapQueryWarning({
        cost,
        requestedLimit: 5000,
        effectiveLimit: 5000,
        limitCapped: false,
        source: 'geo_city',
      }),
    ).toBeNull();
  });
});
