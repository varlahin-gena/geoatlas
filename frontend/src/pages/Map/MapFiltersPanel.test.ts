import { describe, expect, it } from 'vitest';
import { countActiveMapFilters } from './MapFiltersPanel';

describe('countActiveMapFilters', () => {
  it('returns 0 for defaults', () => {
    expect(
      countActiveMapFilters({
        groupBy: 'ip',
        filter: 'all',
        repFilterCount: 0,
        repColorArcs: false,
        hideIntraCountry: false,
      }),
    ).toBe(0);
  });

  it('counts each active dimension', () => {
    expect(
      countActiveMapFilters({
        groupBy: 'country',
        filter: 'blocked',
        repFilterCount: 2,
        repColorArcs: true,
        hideIntraCountry: false,
      }),
    ).toBe(4);
  });
});
