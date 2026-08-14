import { useCallback, useState } from 'react';
import { compileSearchQuery } from '@/lib/search';
import { useDebouncedValue } from '@/lib/useDebouncedValue';

export const MAP_LIMIT_MIN = 100;
export const MAP_LIMIT_MAX = 20000;
export const MAP_LIMIT_HARD_MAX = 50000;

export type MapActionFilter = 'all' | 'allowed' | 'blocked';

/** CH LIMIT: draw cap, bumped when advanced search still runs on the client. */
export function mapFetchLimit(
  maxArcs: number,
  groupBy: string,
  searchMode: 'empty' | 'simple' | 'advanced' | 'error',
): number {
  const draw = Math.min(MAP_LIMIT_MAX, Math.max(MAP_LIMIT_MIN, Math.round(Number(maxArcs) || 5000)));
  if (searchMode === 'advanced') {
    const floor = groupBy === 'ip' || groupBy === 'subnet' ? 10000 : 8000;
    return Math.min(MAP_LIMIT_HARD_MAX, Math.max(draw, floor));
  }
  return draw;
}

export function mapServerScope(
  search: string,
  focusedCountry: string | null,
): { country: string; q: string } {
  const country = (focusedCountry || '').trim();
  const compiled = compileSearchQuery(search);
  if (compiled.mode === 'simple' && compiled.ast?.type === 'TERM') {
    return { country, q: compiled.ast.value.trim() };
  }
  if (
    compiled.mode === 'advanced' &&
    compiled.ast?.type === 'TERM' &&
    compiled.ast.field === 'country' &&
    !country
  ) {
    return { country: compiled.ast.value.trim(), q: '' };
  }
  return { country, q: '' };
}

export function useMapViewQuery() {
  const [filter, setFilter] = useState<MapActionFilter>('all');
  const [search, setSearch] = useState('');
  const [builderOpen, setBuilderOpen] = useState(false);
  const [minCount, setMinCount] = useState(1);
  const [maxArcs, setMaxArcs] = useState(5000);
  const [focusedCountry, setFocusedCountry] = useState<string | null>(null);
  const debouncedSearch = useDebouncedValue(search, 300);
  const debouncedMaxArcs = useDebouncedValue(maxArcs, 300);

  const clearFocusedCountry = useCallback(() => {
    setFocusedCountry(null);
  }, []);

  const applySearchFilter = useCallback((value: string) => {
    setFocusedCountry(null);
    setSearch(value);
  }, []);

  return {
    filter,
    setFilter,
    search,
    setSearch,
    debouncedSearch,
    debouncedMaxArcs,
    builderOpen,
    setBuilderOpen,
    minCount,
    setMinCount,
    maxArcs,
    setMaxArcs,
    focusedCountry,
    setFocusedCountry,
    clearFocusedCountry,
    applySearchFilter,
  };
}
