import type { components } from './openapi';

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

export type SeriesPayload = components['schemas']['EventsSeriesResponse'];
export type SeriesPoint = NonNullable<SeriesPayload['points']>[number];
