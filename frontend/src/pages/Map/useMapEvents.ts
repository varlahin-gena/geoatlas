import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { apiFetch, isAbortError } from '@/api/client';
import type { ToastKind } from '@/components/Toast';
import { buildPeriodQuery } from './mapConstants';
import type { EventsPayload, MapLine, MapPoint } from './mapTypes';

export type MapDataSource = 'live' | 'backup';

export function useMapEvents(toast: (msg: string, kind?: ToastKind) => void) {
  const [period, setPeriod] = useState('1d');
  const [periodFrom, setPeriodFrom] = useState('');
  const [periodTo, setPeriodTo] = useState('');
  const [groupBy, setGroupBy] = useState('city');
  const [points, setPoints] = useState<Record<string, MapPoint>>({});
  const [lines, setLines] = useState<MapLine[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
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
      const apiLimit = groupBy === 'ip' || groupBy === 'subnet' ? 50000 : 10000;
      const url =
        `/api/events?group_by=${encodeURIComponent(groupBy)}&limit=${apiLimit}` +
        `${periodQuery}&source=${encodeURIComponent(dataSource)}`;
      const data = await apiFetch<EventsPayload>(url, {
        signal: controller.signal,
        cache: 'no-store',
      });
      if (controller.signal.aborted) return;
      setPoints(data.points || {});
      setLines(data.lines || []);
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
  }, [groupBy, periodQuery, dataSource, toast]);

  useEffect(() => {
    void fetchData();
    return () => {
      abortRef.current?.abort();
    };
  }, [fetchData]);

  useEffect(() => {
    if (!autoRefresh || dataSource !== 'live') return;
    const id = window.setInterval(() => void fetchData(), 30000);
    return () => window.clearInterval(id);
  }, [autoRefresh, dataSource, fetchData]);

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
    autoRefresh,
    setAutoRefresh,
    dataSource,
    selectDataSource,
    backupAttached,
    periodQuery,
    fetchData,
  };
}
