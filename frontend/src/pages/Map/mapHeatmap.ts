import { COUNTRY_LABEL_MAX_RANK, COUNTRY_NAMES_RU, cssRgb, mapRuCountry } from './mapConstants';
import type { CountryCentroid, MapLine, MapPoint, MapPointEntry } from './mapTypes';

export type GeoFeature = {
  type?: string;
  properties?: Record<string, unknown>;
  geometry?: {
    type: string;
    coordinates: unknown;
  };
};

export type GeoFeatureCollection = {
  type: 'FeatureCollection';
  features: GeoFeature[];
};

export function heatmapColorRGB(
  value: number,
  max: number,
  mapTilesFailed: boolean,
): [number, number, number, number?] {
  if (!max || value <= 0) {
    const land = cssRgb('--map-land-rgb', mapTilesFailed ? 255 : 0);
    return [land[0], land[1], land[2]];
  }
  const t = Math.pow(value / max, 0.7);
  const stops = [
    { p: 0.0, c: [40, 60, 90] },
    { p: 0.5, c: [70, 130, 180] },
    { p: 0.8, c: [214, 158, 46] },
    { p: 1.0, c: [220, 60, 50] },
  ];
  for (let i = 1; i < stops.length; i++) {
    if (t <= stops[i].p) {
      const a = stops[i - 1];
      const b = stops[i];
      const f = (t - a.p) / (b.p - a.p);
      const c = a.c.map((v, j) => Math.round(v + (b.c[j] - v) * f));
      return [c[0], c[1], c[2], 210];
    }
  }
  return [220, 60, 50, 210];
}

function featureCountryName(feature: GeoFeature): string {
  const p = feature.properties || {};
  return String(
    p.name_long || p.NAME_LONG || p.admin || p.ADMIN || p.name || p.NAME || '',
  );
}

export function matchCountryFeature(feature: GeoFeature, country: string): boolean {
  if (!feature || !country) return false;
  const p = feature.properties || {};
  const candidates = [
    p.name,
    p.name_long,
    p.admin,
    p.sovereignt,
    p.ADMIN,
    p.NAME,
    p.NAME_LONG,
    p.ISO_A2,
    p.ISO_A3,
    p.iso_a2,
    p.iso_a3,
  ]
    .filter(Boolean)
    .map((x) => String(x).toLowerCase());
  const target = String(country).toLowerCase();
  if (candidates.includes(target)) return true;
  const ruToEn = Object.entries(COUNTRY_NAMES_RU)
    .filter(([, ru]) => ru.toLowerCase() === target)
    .map(([en]) => en.toLowerCase());
  return ruToEn.some((en) => candidates.includes(en));
}

export function resolveCountryKeyFromFeature(
  feature: GeoFeature,
  stats: Record<string, number>,
): string {
  if (!feature) return '';
  for (const country of Object.keys(stats)) {
    if (matchCountryFeature(feature, country)) return country;
  }
  return featureCountryName(feature);
}

export function buildCountryStats(points: MapPointEntry[]): Record<string, number> {
  const stats: Record<string, number> = {};
  for (const p of points) {
    if (!p || !p.country || p.country === 'Неизвестно') continue;
    stats[p.country] = (stats[p.country] || 0) + (p.count || 0);
  }
  return stats;
}

function precomputeFeatureHeat(
  features: GeoFeature[],
  stats: Record<string, number>,
): Map<GeoFeature, number> {
  const byName = new Map<string, number>();
  for (const [country, value] of Object.entries(stats)) {
    const target = String(country).toLowerCase();
    byName.set(target, value);
    for (const [en, ru] of Object.entries(COUNTRY_NAMES_RU)) {
      if (String(ru).toLowerCase() === target) byName.set(String(en).toLowerCase(), value);
    }
  }
  const heat = new Map<GeoFeature, number>();
  for (const f of features) {
    const p = f.properties || {};
    const candidates = [
      p.name,
      p.name_long,
      p.admin,
      p.sovereignt,
      p.ADMIN,
      p.NAME,
      p.NAME_LONG,
      p.ISO_A2,
      p.ISO_A3,
      p.iso_a2,
      p.iso_a3,
    ].filter(Boolean);
    let v = 0;
    for (let i = 0; i < candidates.length; i++) {
      const hit = byName.get(String(candidates[i]).toLowerCase());
      if (hit != null) {
        v = hit;
        break;
      }
    }
    heat.set(f, v);
  }
  return heat;
}

export interface CountryStatsCache {
  stats: Record<string, number>;
  max: number;
  heat: Map<GeoFeature, number>;
}

export function getCountryStatsCache(
  points: MapPointEntry[],
  countriesGeoJSON: GeoFeatureCollection | null,
): CountryStatsCache {
  const stats = buildCountryStats(points);
  const max = Math.max(0, ...Object.values(stats), 0);
  const features = countriesGeoJSON?.features || [];
  const heat = precomputeFeatureHeat(features, stats);
  return { stats, max, heat };
}

function polygonBBoxCentroid(coords: number[][]): [number, number, number] | null {
  const ring = coords[0] as unknown as number[][];
  if (!ring || !ring.length) return null;
  let minX = 180;
  let maxX = -180;
  let minY = 90;
  let maxY = -90;
  for (const p of ring) {
    if (p[0] < minX) minX = p[0];
    if (p[0] > maxX) maxX = p[0];
    if (p[1] < minY) minY = p[1];
    if (p[1] > maxY) maxY = p[1];
  }
  return [(minX + maxX) / 2, (minY + maxY) / 2, (maxX - minX) * (maxY - minY)];
}

function featureCentroid(feature: GeoFeature): { lon: number; lat: number; area: number } | null {
  const g = feature.geometry;
  if (!g) return null;
  if (g.type === 'Polygon') {
    const c = polygonBBoxCentroid(g.coordinates as number[][]);
    return c ? { lon: c[0], lat: c[1], area: c[2] } : null;
  }
  if (g.type === 'MultiPolygon') {
    let best: [number, number, number] | null = null;
    for (const poly of g.coordinates as number[][][]) {
      const c = polygonBBoxCentroid(poly as unknown as number[][]);
      if (!c) continue;
      if (!best || c[2] > best[2]) best = c;
    }
    return best ? { lon: best[0], lat: best[1], area: best[2] } : null;
  }
  return null;
}

export function buildCountryCentroids(
  countriesGeoJSON: GeoFeatureCollection | null,
): CountryCentroid[] {
  if (!countriesGeoJSON?.features) return [];
  const result: CountryCentroid[] = [];
  for (const f of countriesGeoJSON.features) {
    const name = featureCountryName(f);
    if (!name) continue;
    const c = featureCentroid(f);
    if (!c) continue;
    const p = f.properties || {};
    const labelRank = (p.LABELRANK ?? p.labelrank ?? p.label_rank) as number | undefined;
    let importance: number;
    if (typeof labelRank === 'number') {
      importance = labelRank;
    } else {
      const area = c.area || 0;
      importance = area > 1000 ? 2 : area > 200 ? 4 : area > 50 ? 6 : 9;
    }
    if (importance > COUNTRY_LABEL_MAX_RANK) continue;
    result.push({
      name,
      lat: c.lat,
      lon: c.lon,
      label: mapRuCountry(name),
      rank: importance,
    });
  }
  return result;
}

export function lineMatchesFocusedCountry(
  line: MapLine,
  focusedCountry: string | null,
  allPoints: Record<string, MapPoint>,
): boolean {
  if (!focusedCountry) return true;
  const target = String(focusedCountry).toLowerCase();
  const src = String(line.src_country || '').toLowerCase();
  const dst = String(line.dst_country || '').toLowerCase();
  if (src === target || dst === target) return true;
  const ru = Object.entries(COUNTRY_NAMES_RU)
    .filter(([en]) => en.toLowerCase() === target)
    .map(([, ruName]) => ruName.toLowerCase());
  if (ru.some((r) => src === r || dst === r)) return true;
  const sp = allPoints[line.src];
  const dp = allPoints[line.dst];
  if (sp && String(sp.country || '').toLowerCase() === target) return true;
  if (dp && String(dp.country || '').toLowerCase() === target) return true;
  return false;
}

export function countryAliases(country: string): Set<string> {
  const target = String(country || '').toLowerCase();
  const aliases = new Set([target]);
  Object.entries(COUNTRY_NAMES_RU).forEach(([en, ru]) => {
    if (en.toLowerCase() === target || String(ru).toLowerCase() === target) {
      aliases.add(en.toLowerCase());
      aliases.add(String(ru).toLowerCase());
    }
  });
  return aliases;
}

export async function loadCountriesGeoJSON(): Promise<GeoFeatureCollection | null> {
  try {
    const res = await fetch('/data/countries.geojson');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return (await res.json()) as GeoFeatureCollection;
  } catch (e) {
    console.warn('countries.geojson not loaded:', e);
    return null;
  }
}
