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

  it('accepts city_country group', () => {
    const sp = serializeMapViewSearch({
      period: '1d',
      periodFrom: '',
      periodTo: '',
      groupBy: 'city_country',
      filter: 'all',
      search: '',
      focusedCountry: null,
    });
    expect(sp.get('group')).toBe('city_country');
    expect(parseMapViewSearch(sp).groupBy).toBe('city_country');
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

  it('uses user map_group_by when URL has no group', () => {
    const sp = new URLSearchParams();
    expect(parseMapViewSearch(sp, { groupBy: 'country' }).groupBy).toBe('country');
    expect(parseMapViewSearch(sp, { groupBy: 'city_country' }).groupBy).toBe('city_country');
    expect(parseMapViewSearch(sp).groupBy).toBe('city');
  });

  it('URL group wins over user default', () => {
    const sp = new URLSearchParams('group=ip');
    expect(parseMapViewSearch(sp, { groupBy: 'country' }).groupBy).toBe('ip');
  });

  it('omits group when equal to user default', () => {
    const sp = serializeMapViewSearch(
      {
        period: '1d',
        periodFrom: '',
        periodTo: '',
        groupBy: 'country',
        filter: 'all',
        search: '',
        focusedCountry: null,
      },
      { groupBy: 'country' },
    );
    expect(sp.get('group')).toBeNull();
    expect(sp.toString()).toBe('');
  });
});

