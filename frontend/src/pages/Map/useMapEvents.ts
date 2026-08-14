import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { isAbortError } from '@/api/client';
import { fetchMapEvents } from '@/api/events';
import type { ToastKind } from '@/components/Toast';
import { usePolling } from '@/lib/usePolling';
import { compileSearchQuery } from '@/lib/search';
import { buildPeriodQuery } from './mapConstants';
import { mapFetchLimit, mapServerScope, type MapActionFilter } from './mapQuery';
import type { MapLine, MapPoint } from './mapTypes';

export type MapDataSource = 'live' | 'backup';

export function useMapEvents(
  toast: (msg: string, kind?: ToastKind) => void,
  opts: {
    filter: MapActionFilter;
    maxArcs: number;
    focusedCountry: string | null;
    search: string;
  },
) {
  const { filter, maxArcs, focusedCountry, search } = opts;
  const [period, setPeriod] = useState('1d');
  const [periodFrom, setPeriodFrom] = useState('');
  const [periodTo, setPeriodTo] = useState('');
  const [groupBy, setGroupBy] = useState('city');
  const [points, setPoints] = useState<Record<string, MapPoint>>({});
  const [lines, setLines] = useState<MapLine[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [eventStats, setEventStats] = useState({ rawPairs: 0, skippedNoGeo: 0 });
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [dataSource, setDataSource] = useState<MapDataSource>('live');
  const [backupAttached, setBackupAttached] = useState('');
  const abortRef = useRef<AbortController | null>(null);

  const periodQuery = useMemo(
    () => buildPeriodQuery(period, periodFrom, periodTo),
    [period, periodFrom, periodTo],
  );

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
      const searchMode = compileSearchQuery(search).mode;
      const limit = mapFetchLimit(maxArcs, groupBy, searchMode);
      const { country, q } = mapServerScope(search, focusedCountry);
      const data = await fetchMapEvents({
        groupBy,
        limit,
        filter,
        country,
        q,
        periodQuery,
        source: dataSource,
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;
      setPoints(data.points || {});
      setLines(data.lines || []);
      setEventStats({
        rawPairs: Number(data.stats?.raw_pairs) || 0,
        skippedNoGeo: Number(data.stats?.skipped_no_geo) || 0,
      });
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
  }, [groupBy, periodQuery, dataSource, toast, filter, maxArcs, focusedCountry, search]);

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
    period,
    setPeriod,
    periodFrom,
    setPeriodFrom,
    periodTo,
    setPeriodTo,
    groupBy,
    setGroupBy,
    points,
    lines,
    loading,
    fetchError,
    eventStats,
    autoRefresh,
    setAutoRefresh,
    dataSource,
    selectDataSource,
    backupAttached,
    periodQuery,
    fetchData,
  };
}
