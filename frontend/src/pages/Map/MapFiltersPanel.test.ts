import { describe, expect, it, vi } from 'vitest';
import { countActiveMapFilters, describeActiveMapFilters } from './MapFiltersPanel';

describe('describeActiveMapFilters', () => {
  it('returns empty for defaults', () => {
    expect(
      describeActiveMapFilters({
        groupBy: 'ip',
        filter: 'all',
        repFilterCount: 0,
        repColorArcs: false,
        hideIntraCountry: false,
      }),
    ).toEqual([]);
  });

  it('describes non-defaults and clears via callbacks', () => {
    const setGroupBy = vi.fn();
    const setFilter = vi.fn();
    const setSearch = vi.fn();
    const chips = describeActiveMapFilters({
      groupBy: 'city',
      filter: 'blocked',
      repFilterCount: 2,
      repColorArcs: true,
      hideIntraCountry: true,
      search: 'src_ip=1.2.3.4',
      setGroupBy,
      setFilter,
      setSearch,
    });
    expect(chips.map((c) => c.id)).toEqual([
      'groupBy',
      'filter-blocked',
      'rep',
      'repColor',
      'hideIntra',
      'search',
    ]);
    chips.find((c) => c.id === 'groupBy')?.clear();
    chips.find((c) => c.id === 'filter-blocked')?.clear();
    chips.find((c) => c.id === 'search')?.clear();
    expect(setGroupBy).toHaveBeenCalledWith('ip');
    expect(setFilter).toHaveBeenCalledWith('all');
    expect(setSearch).toHaveBeenCalledWith('');
  });

  it('countActiveMapFilters ignores search', () => {
    expect(
      countActiveMapFilters({
        groupBy: 'ip',
        filter: 'all',
        repFilterCount: 0,
        repColorArcs: false,
        hideIntraCountry: false,
      }),
    ).toBe(0);
    expect(
      countActiveMapFilters({
        groupBy: 'city',
        filter: 'blocked',
        repFilterCount: 0,
        repColorArcs: false,
        hideIntraCountry: false,
      }),
    ).toBe(2);
  });
});
