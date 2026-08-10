export interface ReputationHit {
  list: string;
  category: string;
  network?: string;
}

export interface MapPoint {
  lat: number;
  lon: number;
  country?: string;
  city?: string;
  region?: string;
  label?: string;
  count?: number;
  reputation?: ReputationHit[];
}

export interface MapLine {
  src: string;
  dst: string;
  src_label?: string;
  dst_label?: string;
  src_lat?: number;
  src_lon?: number;
  dst_lat?: number;
  dst_lon?: number;
  status?: string;
  blocked?: boolean;
  count?: number;
  allowed_count?: number;
  blocked_count?: number;
  bytes_sent?: number;
  bytes_recv?: number;
  rule?: string;
  proto?: string;
  src_port?: number;
  dst_port?: number;
  src_zone?: string;
  dst_zone?: string;
  src_country?: string;
  dst_country?: string;
  device?: string;
  last_action?: string;
  src_reputation?: ReputationHit[];
  dst_reputation?: ReputationHit[];
  _flowAlpha?: number;
  _flowTilt?: number;
  _flowRank?: number;
}

export interface MapPointEntry extends MapPoint {
  key: string;
}

export interface EventsPayload {
  points?: Record<string, MapPoint>;
  lines?: MapLine[];
  period?: string;
  source?: string;
  stats?: Record<string, unknown>;
}

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
  sparklineHtml?: string;
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
