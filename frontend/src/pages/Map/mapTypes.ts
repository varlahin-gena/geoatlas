import type { components } from '@/api/openapi';
import type { ReputationHit } from '@/api/types';

export type { ReputationHit };

/** Keep in sync with openapi.yaml MapPoint / MapLine (+ backend model.Node / model.Line). */
export type MapPoint = components['schemas']['MapPoint'];
export type MapLine = components['schemas']['MapLine'] & {
  _flowAlpha?: number;
  _flowTilt?: number;
  _flowRank?: number;
};

export interface MapPointEntry extends MapPoint {
  key: string;
}

export type EventsPayload = Omit<components['schemas']['EventsResponse'], 'lines' | 'points'> & {
  points?: Record<string, MapPoint>;
  lines?: MapLine[];
};

export interface SeriesPoint {
  t?: string;
  allowed?: number;
  blocked?: number;
  total?: number;
}

export interface SeriesPayload {
  country?: string;
  bucket_sec?: number;
  period?: string;
  points?: SeriesPoint[];
}

export type DetailKind = 'line' | 'point' | 'country';

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
