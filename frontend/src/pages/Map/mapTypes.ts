import type { MapPoint, SeriesPoint } from '@/api/eventsTypes';
import type { ReputationHit } from '@/api/types';

export type {
  MapLine,
  MapPoint,
  SeriesPoint,
} from '@/api/eventsTypes';
export type { ReputationHit };

export interface MapPointEntry extends MapPoint {
  key: string;
}

type DetailKind = 'line' | 'point' | 'country';

export interface DetailRow {
  key: string;
  value: string;
  color?: string;
  hint?: string;
  onClick?: () => void;
}

export interface DetailSection {
  title?: string;
  rows: DetailRow[];
}

export interface DetailAction {
  label: string;
  onClick: () => void;
}

export interface DetailState {
  kind: DetailKind;
  title: string;
  sections: DetailSection[];
  actions: DetailAction[];
  countryKey?: string;
  sparklinePoints?: SeriesPoint[];
  sparklineLoading?: boolean;
  sparklineError?: string;
  bucketSec?: number;
}

export type RepFilterSide = 'any' | 'src' | 'dst' | 'both';

export interface ViewState {
  longitude: number;
  latitude: number;
  zoom: number;
  pitch: number;
  bearing: number;
}

export interface CountryCentroid {
  name: string;
  lat: number;
  lon: number;
  label: string;
  rank: number;
}
