import { apiFetch } from './client';
import type { EventsPayload, SeriesPayload } from './eventsTypes';

export type { EventsPayload, SeriesPayload };

export function fetchMapEvents(opts: {
  groupBy: string;
  limit: number;
  filter?: string;
  country?: string;
  q?: string;
  periodQuery: string;
  source: 'live' | 'backup';
  repCategories?: string[];
  repLists?: string[];
  repSide?: string;
  signal?: AbortSignal;
}): Promise<EventsPayload> {
  let url =
    `/api/events?group_by=${encodeURIComponent(opts.groupBy)}&limit=${opts.limit}` +
    `${opts.periodQuery}&source=${encodeURIComponent(opts.source)}`;
  if (opts.filter && opts.filter !== 'all') {
    url += `&filter=${encodeURIComponent(opts.filter)}`;
  }
  if (opts.country) {
    url += `&country=${encodeURIComponent(opts.country)}`;
  }
  if (opts.q) {
    url += `&q=${encodeURIComponent(opts.q)}`;
  }
  if (opts.repCategories?.length) {
    url += `&rep_cat=${encodeURIComponent(opts.repCategories.join(','))}`;
  }
  if (opts.repLists?.length) {
    url += `&rep_list=${encodeURIComponent(opts.repLists.join(','))}`;
  }
  if (opts.repSide && opts.repSide !== 'any') {
    url += `&rep_side=${encodeURIComponent(opts.repSide)}`;
  }
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
