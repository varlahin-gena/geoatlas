import { apiGetQuery } from './client';
import type { EventsPayload, SeriesPayload } from './eventsTypes';

export type { EventsPayload, SeriesPayload } from './eventsTypes';

export async function fetchMapEvents(opts: {
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
  const params = new URLSearchParams();
  params.set('group_by', opts.groupBy);
  params.set('limit', String(opts.limit));
  params.set('source', opts.source);
  // periodQuery is "&hours=6" / "&from=...&to=..." from mapConstants.
  const period = opts.periodQuery.replace(/^\?/, '').replace(/^&/, '');
  if (period) {
    new URLSearchParams(period).forEach((v, k) => params.set(k, v));
  }
  if (opts.filter && opts.filter !== 'all') params.set('filter', opts.filter);
  if (opts.country) params.set('country', opts.country);
  if (opts.q) params.set('q', opts.q);
  if (opts.repCategories?.length) params.set('rep_cat', opts.repCategories.join(','));
  if (opts.repLists?.length) params.set('rep_list', opts.repLists.join(','));
  if (opts.repSide && opts.repSide !== 'any') params.set('rep_side', opts.repSide);

  const data = await apiGetQuery('/api/events', params, {
    signal: opts.signal,
    cache: 'no-store',
  });
  return {
    ...data,
    lines: data.lines,
    points: data.points,
  };
}

export function fetchCountrySeries(
  country: string,
  periodQuery: string,
  source: 'live' | 'backup' = 'live',
  signal?: AbortSignal,
): Promise<SeriesPayload> {
  const params = new URLSearchParams();
  params.set('country', country);
  params.set('source', source);
  const period = periodQuery.replace(/^\?/, '').replace(/^&/, '');
  if (period) {
    new URLSearchParams(period).forEach((v, k) => params.set(k, v));
  }
  return apiGetQuery('/api/events/series', params, {
    signal,
    cache: 'no-store',
  });
}
