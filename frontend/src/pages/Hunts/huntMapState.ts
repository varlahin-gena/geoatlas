import type { MapViewState } from '@/pages/Map/mapQuery';
import type { HuntMapState } from '@/api/hunts';

export function mapViewToHuntState(view: {
  period: string;
  periodFrom: string;
  periodTo: string;
  groupBy: string;
  filter: string;
  search: string;
  focusedCountry: string | null;
  maxArcs?: number;
}): HuntMapState {
  return {
    period: view.period || '1d',
    period_from: view.periodFrom || undefined,
    period_to: view.periodTo || undefined,
    group_by: view.groupBy || 'city',
    filter: view.filter || 'all',
    query: view.search?.trim() || undefined,
    country: view.focusedCountry?.trim() || undefined,
    limit: view.maxArcs || 5000,
  };
}

export function huntStateToMapView(state: HuntMapState): Partial<MapViewState> {
  return {
    period: state.period || '1d',
    periodFrom: state.period_from || '',
    periodTo: state.period_to || '',
    groupBy: state.group_by || 'city',
    filter: (state.filter as MapViewState['filter']) || 'all',
    search: state.query || '',
    focusedCountry: state.country || null,
  };
}
