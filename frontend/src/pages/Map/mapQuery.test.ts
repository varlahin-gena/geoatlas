import { describe, expect, it } from 'vitest';
import { mapFetchLimit, mapServerScope } from './mapQuery';

describe('mapFetchLimit', () => {
  it('clamps to slider range for simple search', () => {
    expect(mapFetchLimit(5000, 'city', 'simple')).toBe(5000);
    expect(mapFetchLimit(50, 'city', 'empty')).toBe(100);
    expect(mapFetchLimit(99999, 'ip', 'empty')).toBe(20000);
  });

  it('raises floor for advanced client-side search', () => {
    expect(mapFetchLimit(500, 'city', 'advanced')).toBe(8000);
    expect(mapFetchLimit(500, 'ip', 'advanced')).toBe(10000);
    expect(mapFetchLimit(15000, 'city', 'advanced')).toBe(15000);
  });
});

describe('mapServerScope', () => {
  it('sends focused country and simple q', () => {
    expect(mapServerScope('Moscow', 'Russia')).toEqual({ country: 'Russia', q: 'Moscow' });
  });

  it('maps country: TERM to country param', () => {
    expect(mapServerScope('country:Germany', null)).toEqual({ country: 'Germany', q: '' });
  });
});
