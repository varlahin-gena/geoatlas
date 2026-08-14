import { describe, expect, it } from 'vitest';
import {
  mapFetchLimit,
  mapServerScope,
  parseMapViewSearch,
  serializeMapViewSearch,
} from './mapQuery';

describe('mapFetchLimit', () => {
  it('clamps to slider range', () => {
    expect(mapFetchLimit(5000)).toBe(5000);
    expect(mapFetchLimit(50)).toBe(100);
    expect(mapFetchLimit(99999)).toBe(20000);
  });
});

describe('mapServerScope', () => {
  it('sends focused country and raw q (including advanced)', () => {
    expect(mapServerScope('Moscow', 'Russia')).toEqual({ country: 'Russia', q: 'Moscow' });
    expect(mapServerScope('country:Germany AND rule:block', null)).toEqual({
      country: '',
      q: 'country:Germany AND rule:block',
    });
  });
});

describe('map view query string', () => {
  it('omits defaults', () => {
    const sp = serializeMapViewSearch({
      period: '1d',
      periodFrom: '',
      periodTo: '',
      groupBy: 'city',
      filter: 'all',
      search: '',
      focusedCountry: null,
    });
    expect(sp.toString()).toBe('');
  });

  it('round-trips period, group, filter, q, country', () => {
    const sp = serializeMapViewSearch({
      period: '6h',
      periodFrom: '',
      periodTo: '',
      groupBy: 'ip',
      filter: 'blocked',
      search: 'country:Germany',
      focusedCountry: 'Russia',
    });
    expect(parseMapViewSearch(sp)).toEqual({
      period: '6h',
      periodFrom: '',
      periodTo: '',
      groupBy: 'ip',
      filter: 'blocked',
      search: 'country:Germany',
      focusedCountry: 'Russia',
    });
  });

  it('keeps custom from/to only for custom period', () => {
    const sp = serializeMapViewSearch({
      period: 'custom',
      periodFrom: '2026-08-01T00:00',
      periodTo: '2026-08-02T00:00',
      groupBy: 'city',
      filter: 'all',
      search: '',
      focusedCountry: null,
    });
    expect(parseMapViewSearch(sp)).toMatchObject({
      period: 'custom',
      periodFrom: '2026-08-01T00:00',
      periodTo: '2026-08-02T00:00',
    });
  });
});
