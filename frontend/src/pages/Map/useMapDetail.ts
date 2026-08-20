import { useCallback, useRef, useState } from 'react';
import { isAbortError } from '@/api/client';
import type { ToastKind } from '@/components/Toast';
import { mapRuCountry } from './mapConstants';
import {
  buildCountryDetailBase,
  buildLineDetail,
  buildPointDetail,
  fetchCountrySeries,
  linesForCountry,
} from './mapDetailBuilders';
import { getCountryStatsCache, type GeoFeature, type GeoFeatureCollection } from './mapHeatmap';
import type {
  DetailState,
  MapLine,
  MapPoint,
  MapPointEntry,
} from './mapTypes';

async function copyToClipboard(text: string, toast: (m: string, t?: ToastKind) => void) {
  try {
    await navigator.clipboard.writeText(text);
    toast('Скопировано', 'success');
  } catch {
    toast('Не удалось скопировать', 'error');
  }
}

export function useMapDetail(opts: {
  groupBy: string;
  lines: MapLine[];
  points: Record<string, MapPoint>;
  visibleLines: MapLine[];
  countriesGeoJSON: GeoFeatureCollection | null;
  periodQuery: string;
  dataSource?: 'live' | 'backup';
  toast: (msg: string, kind?: ToastKind) => void;
  applySearchFilter: (value: string) => void;
  clearFocusedCountry: () => void;
  setFocusedCountry: (key: string | null) => void;
}) {
  const {
    groupBy,
    lines,
    points,
    visibleLines,
    countriesGeoJSON,
    periodQuery,
    dataSource = 'live',
    toast,
    applySearchFilter,
    clearFocusedCountry,
    setFocusedCountry,
  } = opts;

  const [detail, setDetail] = useState<DetailState | null>(null);
  const seriesAbortRef = useRef<AbortController | null>(null);

  const closeDetail = useCallback(() => {
    setDetail(null);
    if (seriesAbortRef.current) {
      try {
        seriesAbortRef.current.abort();
      } catch {
        /* ignore */
      }
      seriesAbortRef.current = null;
    }
  }, []);

  const openLineDetail = useCallback(
    (line: MapLine) => {
      setDetail(
        buildLineDetail(
          line,
          groupBy,
          [
            {
              label: 'Копировать src',
              onClick: () => void copyToClipboard(line.src || '', toast),
            },
            {
              label: 'Копировать dst',
              onClick: () => void copyToClipboard(line.dst || '', toast),
            },
            {
              label: 'Поиск src',
              onClick: () => applySearchFilter(line.src_label || line.src || ''),
            },
            {
              label: 'Поиск dst',
              onClick: () => applySearchFilter(line.dst_label || line.dst || ''),
            },
          ],
          points,
        ),
      );
    },
    [groupBy, toast, applySearchFilter, points],
  );

  const openPointDetail = useCallback(
    (point: MapPointEntry) => {
      const actions = [
        {
          label: 'Копировать ключ',
          onClick: () => void copyToClipboard(point.key || '', toast),
        },
        {
          label: 'Искать узел',
          onClick: () => applySearchFilter(point.label || point.key || ''),
        },
      ];
      if (point.country && point.country !== 'Неизвестно') {
        actions.push({
          label: 'Искать страну',
          onClick: () => applySearchFilter(mapRuCountry(point.country)),
        });
      }
      setDetail(buildPointDetail(point, lines, actions, openLineDetail));
    },
    [lines, toast, applySearchFilter, openLineDetail],
  );

  const openCountryDetail = useCallback(
    (countryKey: string, _feature?: GeoFeature) => {
      if (!countryKey) return;
      setFocusedCountry(countryKey);
      const pointsForStats: MapPointEntry[] = Object.entries(points)
        .filter(([, p]) => p && !(p.lat === 0 && p.lon === 0))
        .map(([key, p]) => ({ key, ...p }));
      const { stats: countryStats } = getCountryStatsCache(pointsForStats, countriesGeoJSON);
      const events = countryStats[countryKey] || 0;
      const topLines = linesForCountry(countryKey, visibleLines, points).slice(0, 20);
      const base = buildCountryDetailBase(
        countryKey,
        events,
        topLines,
        [
          {
            label: 'Сбросить фокус',
            onClick: () => {
              clearFocusedCountry();
              closeDetail();
            },
          },
          {
            label: 'Искать страну',
            onClick: () => applySearchFilter(mapRuCountry(countryKey)),
          },
        ],
        openLineDetail,
      );
      setDetail(base);

      if (seriesAbortRef.current) {
        try {
          seriesAbortRef.current.abort();
        } catch {
          /* ignore */
        }
      }
      const controller = new AbortController();
      seriesAbortRef.current = controller;
      void (async () => {
        try {
          const data = await fetchCountrySeries(countryKey, periodQuery, dataSource, controller.signal);
          setDetail((prev) => {
            if (!prev || prev.countryKey !== countryKey) return prev;
            return {
              ...prev,
              sparklineLoading: false,
              sparklinePoints: data.points || [],
              bucketSec: data.bucket_sec,
            };
          });
        } catch (e) {
          if (isAbortError(e) || (e as { name?: string })?.name === 'AbortError') return;
          setDetail((prev) => {
            if (!prev || prev.countryKey !== countryKey) return prev;
            return {
              ...prev,
              sparklineLoading: false,
              sparklineError: e instanceof Error ? e.message : String(e),
            };
          });
        }
      })();
    },
    [
      points,
      countriesGeoJSON,
      visibleLines,
      periodQuery,
      dataSource,
      clearFocusedCountry,
      closeDetail,
      applySearchFilter,
      openLineDetail,
      setFocusedCountry,
    ],
  );

  return {
    detail,
    closeDetail,
    openLineDetail,
    openPointDetail,
    openCountryDetail,
  };
}
