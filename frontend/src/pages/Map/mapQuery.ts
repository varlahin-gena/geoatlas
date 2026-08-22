import { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useDebouncedValue } from '@/lib/useDebouncedValue';
import { PERIODS } from './mapPeriods';

const MAP_LIMIT_MIN = 100;
const MAP_LIMIT_MAX = 20000;

export type MapActionFilter = 'all' | 'allowed' | 'blocked';

export const MAP_VIEW_DEFAULTS = {
  period: '1d',
  periodFrom: '',
  periodTo: '',
  groupBy: 'city',
  filter: 'all' as MapActionFilter,
  search: '',
  focusedCountry: null as string | null,
};

export type MapViewState = typeof MAP_VIEW_DEFAULTS;

const PERIOD_KEYS = new Set<string>(PERIODS.map(([id]) => id));
const GROUP_KEYS = new Set(['ip', 'subnet', 'city', 'country']);
const FILTER_KEYS = new Set<MapActionFilter>(['all', 'allowed', 'blocked']);

/** CH LIMIT: draw cap. Search/reputation apply on the server before LIMIT. */
export function mapFetchLimit(maxArcs: number): number {
  return Math.min(MAP_LIMIT_MAX, Math.max(MAP_LIMIT_MIN, Math.round(Number(maxArcs) || 5000)));
}

export function mapServerScope(
  search: string,
  focusedCountry: string | null,
): { country: string; q: string } {
  return {
    country: (focusedCountry || '').trim(),
    q: search.trim(),
  };
}

export function parseMapViewSearch(sp: URLSearchParams): MapViewState {
  const periodRaw = (sp.get('period') || '').trim();
  const period = PERIOD_KEYS.has(periodRaw) ? periodRaw : MAP_VIEW_DEFAULTS.period;
  const groupRaw = (sp.get('group') || '').trim().toLowerCase();
  const groupBy = GROUP_KEYS.has(groupRaw) ? groupRaw : MAP_VIEW_DEFAULTS.groupBy;
  const filterRaw = (sp.get('filter') || '').trim().toLowerCase() as MapActionFilter;
  const filter = FILTER_KEYS.has(filterRaw) ? filterRaw : MAP_VIEW_DEFAULTS.filter;
  const country = (sp.get('country') || '').trim();
  return {
    period,
    periodFrom: period === 'custom' ? (sp.get('from') || '').trim() : '',
    periodTo: period === 'custom' ? (sp.get('to') || '').trim() : '',
    groupBy,
    filter,
    search: sp.get('q') || '',
    focusedCountry: country || null,
  };
}

export function serializeMapViewSearch(state: MapViewState): URLSearchParams {
  const sp = new URLSearchParams();
  if (state.period && state.period !== MAP_VIEW_DEFAULTS.period) {
    sp.set('period', state.period);
  }
  if (state.period === 'custom') {
    if (state.periodFrom) sp.set('from', state.periodFrom);
    if (state.periodTo) sp.set('to', state.periodTo);
  }
  if (state.groupBy && state.groupBy !== MAP_VIEW_DEFAULTS.groupBy) {
    sp.set('group', state.groupBy);
  }
  if (state.filter && state.filter !== MAP_VIEW_DEFAULTS.filter) {
    sp.set('filter', state.filter);
  }
  const q = state.search.trim();
  if (q) sp.set('q', q);
  const country = (state.focusedCountry || '').trim();
  if (country) sp.set('country', country);
  return sp;
}

function parseAlertFingerprint(sp: URLSearchParams): string | null {
  const v = (sp.get('alert') || '').trim();
  return v || null;
}

function sameView(a: MapViewState, b: MapViewState): boolean {
  return (
    a.period === b.period &&
    a.periodFrom === b.periodFrom &&
    a.periodTo === b.periodTo &&
    a.groupBy === b.groupBy &&
    a.filter === b.filter &&
    a.search === b.search &&
    a.focusedCountry === b.focusedCountry
  );
}

export function useMapViewQuery() {
  const [params, setParams] = useSearchParams();
  const parsed = parseMapViewSearch(params);
  const alertFingerprint = parseAlertFingerprint(params);
  const [search, setSearchState] = useState(parsed.search);
  const [builderOpen, setBuilderOpen] = useState(false);
  const [minCount, setMinCount] = useState(1);
  const [maxArcs, setMaxArcs] = useState(5000);
  const debouncedSearch = useDebouncedValue(search, 300);
  const debouncedMaxArcs = useDebouncedValue(maxArcs, 300);
  /** Desired alert fingerprint to keep across patchView; null after Live reset. */
  const alertKeepRef = useRef<string | null>(alertFingerprint);

  useEffect(() => {
    // Sync from URL unless we intentionally cleared (Live reset).
    if (alertFingerprint) alertKeepRef.current = alertFingerprint;
  }, [alertFingerprint]);

  useEffect(() => {
    setSearchState(parsed.search);
  }, [parsed.search]);

  const patchView = useCallback(
    (partial: Partial<MapViewState>, opts?: { clearAlert?: boolean }) => {
      setParams(
        (prev) => {
          const next = { ...parseMapViewSearch(prev), ...partial };
          if (partial.period && partial.period !== 'custom') {
            next.periodFrom = '';
            next.periodTo = '';
          }
          if (opts?.clearAlert) alertKeepRef.current = null;
          const alert = alertKeepRef.current;
          const nextSp = serializeMapViewSearch(next);
          if (alert) nextSp.set('alert', alert);
          const prevView = parseMapViewSearch(prev);
          const prevAlert = parseAlertFingerprint(prev);
          if (sameView(next, prevView) && alert === prevAlert) return prev;
          return nextSp;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  useEffect(() => {
    if (debouncedSearch !== search) return;
    if (debouncedSearch === parsed.search) return;
    patchView({ search: debouncedSearch });
  }, [debouncedSearch, parsed.search, search, patchView]);

  const setPeriod = useCallback(
    (v: string) => {
      patchView({ period: v });
    },
    [patchView],
  );
  const setPeriodFrom = useCallback(
    (v: string) => {
      patchView({ period: 'custom', periodFrom: v });
    },
    [patchView],
  );
  const setPeriodTo = useCallback(
    (v: string) => {
      patchView({ period: 'custom', periodTo: v });
    },
    [patchView],
  );
  const setGroupBy = useCallback(
    (v: string) => {
      patchView({ groupBy: v });
    },
    [patchView],
  );
  const setFilter = useCallback(
    (v: MapActionFilter) => {
      patchView({ filter: v });
    },
    [patchView],
  );
  const setSearch = useCallback((v: string) => {
    setSearchState(v);
  }, []);
  const setFocusedCountry = useCallback(
    (v: string | null) => {
      patchView({ focusedCountry: v });
    },
    [patchView],
  );
  const clearFocusedCountry = useCallback(() => {
    patchView({ focusedCountry: null });
  }, [patchView]);
  const applySearchFilter = useCallback(
    (value: string) => {
      setSearchState(value);
      patchView({ focusedCountry: null, search: value });
    },
    [patchView],
  );
  const applyView = useCallback(
    (partial: Partial<MapViewState>, opts?: { clearAlert?: boolean; alert?: string | null }) => {
      if (partial.search != null) setSearchState(partial.search);
      setParams(
        (prev) => {
          const next = { ...parseMapViewSearch(prev), ...partial };
          if (partial.period && partial.period !== 'custom') {
            next.periodFrom = '';
            next.periodTo = '';
          }
          const nextSp = serializeMapViewSearch(next);
          if (opts?.clearAlert || opts?.alert === null) {
            alertKeepRef.current = null;
          } else if (opts?.alert) {
            alertKeepRef.current = opts.alert;
          }
          const alert = alertKeepRef.current;
          if (alert) nextSp.set('alert', alert);
          return nextSp;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  const resetToLiveView = useCallback(() => {
    alertKeepRef.current = null;
    setSearchState(MAP_VIEW_DEFAULTS.search);
    try {
      sessionStorage.removeItem('geoatlas.mapAlert');
    } catch {
      /* ignore */
    }
    setParams(serializeMapViewSearch({ ...MAP_VIEW_DEFAULTS }), { replace: true });
  }, [setParams]);

  return {
    period: parsed.period,
    setPeriod,
    periodFrom: parsed.periodFrom,
    setPeriodFrom,
    periodTo: parsed.periodTo,
    setPeriodTo,
    groupBy: parsed.groupBy,
    setGroupBy,
    filter: parsed.filter,
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
    focusedCountry: parsed.focusedCountry,
    setFocusedCountry,
    clearFocusedCountry,
    applySearchFilter,
    applyView,
    alertFingerprint,
    resetToLiveView,
  };
}
