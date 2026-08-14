import { apiFetch } from './client';
import type { EventsPayload, SeriesPayload } from '@/pages/Map/mapTypes';

export function fetchMapEvents(opts: {
  groupBy: string;
  limit: number;
  periodQuery: string;
  source: 'live' | 'backup';
  signal?: AbortSignal;
}): Promise<EventsPayload> {
  const url =
    `/api/events?group_by=${encodeURIComponent(opts.groupBy)}&limit=${opts.limit}` +
    `${opts.periodQuery}&source=${encodeURIComponent(opts.source)}`;
  return apiFetch<EventsPayload>(url, {
    signal: opts.signal,
    cache: 'no-store',
  });
}

export function fetchCountrySeries(
  country: string,
  periodQuery: string,
  source: 'live' | 'backup' = 'live',
  signal?: AbortSignal,
): Promise<SeriesPayload> {
  const url =
    `/api/events/series?country=${encodeURIComponent(country)}` +
    `${periodQuery}&source=${encodeURIComponent(source)}`;
  return apiFetch<SeriesPayload>(url, { signal, cache: 'no-store' });
}
