import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { apiFetch, isAbortError } from '@/api/client';
import type { ToastKind } from '@/components/Toast';
import { buildPeriodQuery } from './mapConstants';
import type { EventsPayload, MapLine, MapPoint } from './mapTypes';

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
  const abortRef = useRef<AbortController | null>(null);

  const periodQuery = useMemo(
    () => buildPeriodQuery(period, periodFrom, periodTo),
    [period, periodFrom, periodTo],
  );

  const fetchData = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    try {
      const apiLimit = groupBy === 'ip' || groupBy === 'subnet' ? 50000 : 10000;
      const url = `/api/events?group_by=${encodeURIComponent(groupBy)}&limit=${apiLimit}${periodQuery}`;
      const data = await apiFetch<EventsPayload>(url, {
        signal: controller.signal,
        cache: 'no-store',
      });
      if (controller.signal.aborted) return;
      setPoints(data.points || {});
      setLines(data.lines || []);
      setFetchError(null);
    } catch (e) {
      if (isAbortError(e) || controller.signal.aborted) return;
      const msg = e instanceof Error ? e.message : 'Ошибка загрузки';
      setFetchError(msg);
      toast(msg, 'error');
    } finally {
      if (abortRef.current === controller) {
        setLoading(false);
      }
    }
  }, [groupBy, periodQuery, toast]);

  useEffect(() => {
    void fetchData();
    return () => {
      abortRef.current?.abort();
    };
  }, [fetchData]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => void fetchData(), 30000);
    return () => window.clearInterval(id);
  }, [autoRefresh, fetchData]);

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
    periodQuery,
    fetchData,
  };
}
