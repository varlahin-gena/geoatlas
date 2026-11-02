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
    const clearFocusedCountry = vi.fn();
    const chips = describeActiveMapFilters({
      groupBy: 'city',
      filter: 'blocked',
      repFilterCount: 2,
      repColorArcs: true,
      hideIntraCountry: true,
      search: 'src_ip=1.2.3.4',
      focusedCountry: 'Russia',
      setGroupBy,
      setFilter,
      setSearch,
      clearFocusedCountry,
    });
    expect(chips.map((c) => c.id)).toEqual([
      'focus',
      'groupBy',
      'filter-blocked',
      'rep',
      'repColor',
      'hideIntra',
      'search',
    ]);
    expect(chips[0]?.label).toMatch(/^Фокус:/);
    chips.find((c) => c.id === 'focus')?.clear();
    chips.find((c) => c.id === 'groupBy')?.clear();
    chips.find((c) => c.id === 'filter-blocked')?.clear();
    chips.find((c) => c.id === 'search')?.clear();
    expect(clearFocusedCountry).toHaveBeenCalled();
    expect(setGroupBy).toHaveBeenCalledWith('ip');
    expect(setFilter).toHaveBeenCalledWith('all');
    expect(setSearch).toHaveBeenCalledWith('');
  });

  it('shows hideIntra chip for city_country', () => {
    const chips = describeActiveMapFilters({
      groupBy: 'city_country',
      filter: 'all',
      repFilterCount: 0,
      repColorArcs: false,
      hideIntraCountry: true,
    });
    expect(chips.map((c) => c.id)).toEqual(['groupBy', 'hideIntra']);
    expect(chips[0]?.label).toMatch(/город — страна/i);
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
    expect(
      countActiveMapFilters({
        groupBy: 'ip',
        filter: 'all',
        repFilterCount: 0,
        repColorArcs: false,
        hideIntraCountry: false,
        focusedCountry: 'Russia',
      }),
    ).toBe(1);
  });
});
