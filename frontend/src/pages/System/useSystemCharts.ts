import { useCallback, useEffect, useRef, type RefObject } from 'react';
import uPlot from 'uplot';
import { fetchSystemHistory } from '@/api/system';
import type { HistoryPayload } from '@/api/systemTypes';
import type { ToastKind } from '@/components/Toast';
import { usePolling } from '@/lib/usePolling';
import { alignSeries, makeChart, type ChartFormatOpts } from './systemCharts';
import { CONTAINERS, type Tab } from './systemTypes';

export function useSystemCharts(
  toast: (msg: string, kind?: ToastKind) => void,
  opts: {
    tab: Tab;
    period: string;
    autoRefresh: boolean;
    themeTick: number;
  },
) {
  const { tab, period, autoRefresh, themeTick } = opts;
  const chartEvents = useRef<HTMLDivElement>(null);
  const chartLag = useRef<HTMLDivElement>(null);
  const chartCpu = useRef<HTMLDivElement>(null);
  const chartMem = useRef<HTMLDivElement>(null);
  const chartBuffer = useRef<HTMLDivElement>(null);
  const chartStorage = useRef<HTMLDivElement>(null);
  const plotsRef = useRef<uPlot[]>([]);
  const historyFetching = useRef(false);

  const paintHistory = useCallback((data: HistoryPayload) => {
    plotsRef.current.forEach((p) => p.destroy());
    plotsRef.current = [];

    const mk = (
      ref: RefObject<HTMLDivElement | null>,
      title: string,
      labels: string[],
      keys: string[],
      chartOpts?: ChartFormatOpts,
    ) => {
      if (!ref.current) return;
      const series = data.series || {};
      const normalized: Record<string, { t: string; v: number }[]> = {};
      for (const [k, pts] of Object.entries(series)) {
        normalized[k] = (pts || []).map((p) => ({
          t: String(p.t ?? ''),
          v: Number(p.v) || 0,
        }));
      }
      const { xs, ys } = alignSeries(normalized, keys);
      if (!xs.length) {
        const host = ref.current;
        host.replaceChildren();
        const titleEl = document.createElement('div');
        titleEl.className = 'chart-title';
        titleEl.textContent = title;
        const empty = document.createElement('div');
        empty.className = 'empty';
        empty.style.padding = '24px';
        empty.textContent = 'Нет данных';
        host.append(titleEl, empty);
        return;
      }
      plotsRef.current.push(makeChart(ref.current, title, labels, xs, ys, chartOpts));
    };

    mk(chartEvents, 'События / сек', ['Ingest rate (live)', 'DB ingest (1m avg)'], [
      'pipeline.rate.events_per_sec',
      'pipeline.ingest.events_per_sec_db',
    ]);
    mk(chartLag, 'Лаг ingest (сек)', ['Lag (sec)'], ['pipeline.ingest.lag_sec']);
    mk(
      chartCpu,
      'CPU контейнеров (%)',
      [...CONTAINERS],
      CONTAINERS.map((c) => `container.${c}.cpu_pct`),
      { isPercent: true },
    );
    mk(
      chartMem,
      'Память контейнеров',
      [...CONTAINERS],
      CONTAINERS.map((c) => `container.${c}.mem_bytes`),
      { isBytes: true },
    );
    mk(
      chartBuffer,
      'Буферы',
      ['Ingest buffered', 'syslog-ng queued'],
      ['pipeline.ingest.buffered_lines', 'pipeline.syslogng.queued'],
      { isInt: true },
    );
    mk(
      chartStorage,
      'Размер хранилища',
      ['traffic_logs'],
      ['storage.traffic_logs.bytes_on_disk'],
      { isBytes: true },
    );
  }, []);

  const loadHistory = useCallback(async () => {
    if (historyFetching.current) return;
    historyFetching.current = true;
    try {
      const data = await fetchSystemHistory(period);
      paintHistory(data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка history', 'error');
    } finally {
      historyFetching.current = false;
    }
  }, [period, paintHistory, toast]);

  useEffect(() => {
    if (tab !== 'charts') {
      plotsRef.current.forEach((p) => p.destroy());
      plotsRef.current = [];
      return;
    }
    void loadHistory();
    return () => {
      plotsRef.current.forEach((p) => p.destroy());
      plotsRef.current = [];
    };
  }, [tab, period, themeTick, loadHistory]);

  usePolling(
    async () => {
      await loadHistory();
    },
    30000,
    autoRefresh && tab === 'charts',
    { runImmediately: false },
  );

  return {
    chartEvents,
    chartLag,
    chartCpu,
    chartMem,
    chartBuffer,
    chartStorage,
  };
}
