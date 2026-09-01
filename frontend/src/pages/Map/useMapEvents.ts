import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { isAbortError } from '@/api/client';
import { fetchMapEvents } from '@/api/events';
import type { ToastKind } from '@/components/Toast';
import { usePolling } from '@/lib/usePolling';
import { buildPeriodQuery } from './mapConstants';
import { mapFetchLimit, mapServerScope, type MapActionFilter } from './mapQuery';
import {
  assessMapQueryCost,
  effectiveMapLimit,
  type MapQueryCostTier,
} from './mapQueryCost';
import type { MapLine, MapPoint } from './mapTypes';

export type MapDataSource = 'live' | 'backup';

export function useMapEvents(
  toast: (msg: string, kind?: ToastKind) => void,
  opts: {
    period: string;
    periodFrom: string;
    periodTo: string;
    groupBy: string;
    filter: MapActionFilter;
    maxArcs: number;
    focusedCountry: string | null;
    search: string;
    repCategories: string[];
    repLists: string[];
    repSide: string;
    repActive: boolean;
  },
) {
  const {
    period,
    periodFrom,
    periodTo,
    groupBy,
    filter,
    maxArcs,
    focusedCountry,
    search,
    repCategories,
    repLists,
    repSide,
    repActive,
  } = opts;
  const [points, setPoints] = useState<Record<string, MapPoint>>({});
  const [lines, setLines] = useState<MapLine[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [eventStats, setEventStats] = useState({
    rawPairs: 0,
    skippedNoGeo: 0,
    source: '',
    limitRequested: 0,
    limitApplied: 0,
    limitCapped: false,
    queryCost: 'light' as MapQueryCostTier,
  });
  const [repFacets, setRepFacets] = useState<Record<string, string[]>>({});
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [dataSource, setDataSource] = useState<MapDataSource>('live');
  const [backupAttached, setBackupAttached] = useState('');
  const abortRef = useRef<AbortController | null>(null);

  const periodQuery = useMemo(
    () => buildPeriodQuery(period, periodFrom, periodTo),
    [period, periodFrom, periodTo],
  );

  const queryCost = useMemo(
    () =>
      assessMapQueryCost({
        period,
        periodFrom,
        periodTo,
        groupBy,
        search,
        focusedCountry,
        repActive,
      }),
    [period, periodFrom, periodTo, groupBy, search, focusedCountry, repActive],
  );

  const requestedLimit = useMemo(() => mapFetchLimit(maxArcs), [maxArcs]);
  const effectiveLimit = useMemo(() => {
    const { applied } = effectiveMapLimit(requestedLimit, queryCost);
    return applied;
  }, [requestedLimit, queryCost]);

  const selectDataSource = useCallback((next: MapDataSource) => {
    setDataSource(next);
    if (next === 'backup') {
      setAutoRefresh(false);
    }
  }, []);

  const fetchData = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    try {
      const { country, q } = mapServerScope(search, focusedCountry);
      const data = await fetchMapEvents({
        groupBy,
        limit: effectiveLimit,
        filter,
        country,
        q,
        periodQuery,
        source: dataSource,
        repCategories: repActive ? repCategories : undefined,
        repLists: repActive ? repLists : undefined,
        repSide: repActive ? repSide : undefined,
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;
      setPoints(data.points || {});
      setLines(data.lines || []);
      const stats = data.stats || {};
      setEventStats({
        rawPairs: Number(stats.raw_pairs) || 0,
        skippedNoGeo: Number(stats.skipped_no_geo) || 0,
        source: String(stats.source || ''),
        limitRequested: Number(stats.limit_requested) || requestedLimit,
        limitApplied: Number(stats.limit_applied) || effectiveLimit,
        limitCapped: Boolean(stats.limit_capped),
        queryCost: (stats.query_cost as MapQueryCostTier) || queryCost.tier,
      });
      setRepFacets(data.reputation_facets || {});
      const attached = (data.backup_attached || '').trim();
      setBackupAttached(attached);
      if (!attached && dataSource === 'backup') {
        setDataSource('live');
      }
      setFetchError(null);
    } catch (e) {
      if (isAbortError(e) || controller.signal.aborted) return;
      const msg = e instanceof Error ? e.message : 'Ошибка загрузки';
      setFetchError(msg);
      toast(msg, 'error');
      if (dataSource === 'backup') {
        setDataSource('live');
      }
    } finally {
      if (abortRef.current === controller) {
        setLoading(false);
      }
    }
  }, [
    groupBy,
    periodQuery,
    dataSource,
    toast,
    filter,
    maxArcs,
    focusedCountry,
    search,
    repActive,
    repCategories,
    repLists,
    repSide,
    queryCost,
    requestedLimit,
    effectiveLimit,
  ]);

  useEffect(() => {
    void fetchData();
    return () => {
      abortRef.current?.abort();
    };
  }, [fetchData]);

  usePolling(
    async () => {
      await fetchData();
    },
    30000,
    autoRefresh && dataSource === 'live',
    { runImmediately: false },
  );

  return {
    points,
    lines,
    loading,
    fetchError,
    eventStats,
    queryCost,
    requestedLimit,
    effectiveLimit,
    repFacets,
    autoRefresh,
    setAutoRefresh,
    dataSource,
    selectDataSource,
    backupAttached,
    periodQuery,
    fetchData,
  };
}
