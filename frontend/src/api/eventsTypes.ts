import type { components } from './openapi';
import type { ReputationHit } from './types';

export type { ReputationHit };

/** Keep in sync with openapi.yaml MapPoint / MapLine (+ backend model.Node / model.Line). */
export type MapPoint = components['schemas']['MapPoint'];
export type MapLine = components['schemas']['MapLine'] & {
  _flowAlpha?: number;
  _flowTilt?: number;
  _flowRank?: number;
};

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
