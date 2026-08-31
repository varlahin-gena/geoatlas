import { ArcLayer, GeoJsonLayer, ScatterplotLayer, TextLayer } from '@deck.gl/layers';
import { LABEL_CHARSET, cssRgb } from './mapConstants';
import {
  buildCountryCentroids,
  getCountryStatsCache,
  heatmapColorRGB,
  matchCountryFeature,
  resolveCountryKeyFromFeature,
  type GeoFeature,
  type GeoFeatureCollection,
} from './mapHeatmap';
import { lineHasReputationHits } from './mapReputation';
import { edgeKey } from './mapAnomalyOverlay';
import type {
  CountryCentroid,
  MapLine,
  MapPoint,
  MapPointEntry,
  ViewState,
} from './mapTypes';

export function statusRGB(status: string | undefined): [number, number, number] {
  if (status === 'blocked') return [248, 81, 73];
  if (status === 'unknown') return [110, 118, 129];
  return [63, 185, 80];
}

function monoArcRGB(): [number, number, number] {
  return [88, 166, 255];
}

function arcRGB(
  status: string | undefined,
  line: MapLine,
  monoArcColor: boolean,
  repColorArcs: boolean,
): [number, number, number] {
  if (monoArcColor) return monoArcRGB();
  if (repColorArcs && lineHasReputationHits(line)) return [210, 153, 34];
  return statusRGB(status);
}

export function hasCoords(line: MapLine): boolean {
  return (
    typeof line.src_lat === 'number' &&
    typeof line.src_lon === 'number' &&
    typeof line.dst_lat === 'number' &&
    typeof line.dst_lon === 'number' &&
    !(line.src_lat === 0 && line.src_lon === 0) &&
    !(line.dst_lat === 0 && line.dst_lon === 0)
  );
}

/** Clamp/wrap lon/lat; swap when latitude is out of range (common bad GeoIP). */
export function normalizeLonLat(lon: number, lat: number): [number, number] {
  let lo = lon;
  let la = lat;
  if (!Number.isFinite(lo) || !Number.isFinite(la)) return [0, 0];
  if (Math.abs(la) > 90 && Math.abs(lo) <= 90) {
    [lo, la] = [la, lo];
  }
  la = Math.max(-90, Math.min(90, la));
  while (lo > 180) lo -= 360;
  while (lo < -180) lo += 360;
  return [lo, la];
}

function coordDistanceDeg(lon1: number, lat1: number, lon2: number, lat2: number): number {
  let dLon = Math.abs(lon1 - lon2);
  if (dLon > 180) dLon = 360 - dLon;
  return Math.hypot(dLon, Math.abs(lat1 - lat2));
}

/** True when the shorter great-circle path crosses the antimeridian (±180°). */
export function arcCrossesAntimeridian(sLon: number, tLon: number): boolean {
  return Math.abs(tLon - sLon) > 180;
}

/** Weighted lon/lat from visible line endpoints — matches arc anchor coords. */
export function buildLineCoordFallback(
  lines: MapLine[],
): Map<string, { lon: number; lat: number }> {
  const acc = new Map<string, { lonSum: number; latSum: number; w: number }>();
  for (const line of lines) {
    const w = line.count || 1;
    for (const [key, lon, lat] of [
      [line.src, line.src_lon, line.src_lat],
      [line.dst, line.dst_lon, line.dst_lat],
    ] as const) {
      if (!key || typeof lon !== 'number' || typeof lat !== 'number') continue;
      if (lon === 0 && lat === 0) continue;
      const [nLon, nLat] = normalizeLonLat(lon, lat);
      if (nLat === 0 && nLon === 0) continue;
      const cur = acc.get(key) ?? { lonSum: 0, latSum: 0, w: 0 };
      cur.lonSum += nLon * w;
      cur.latSum += nLat * w;
      cur.w += w;
      acc.set(key, cur);
    }
  }
  const out = new Map<string, { lon: number; lat: number }>();
  acc.forEach((v, k) => {
    if (v.w > 0) out.set(k, { lon: v.lonSum / v.w, lat: v.latSum / v.w });
  });
  return out;
}

export function resolveNodeLonLat(
  key: string,
  fallbackLon: number | undefined,
  fallbackLat: number | undefined,
  allPoints: Record<string, MapPoint>,
  lineFallback?: Map<string, { lon: number; lat: number }>,
): [number, number] {
  const fb = lineFallback?.get(key);
  const p = allPoints[key];
  const pOk = p && !(p.lat === 0 && p.lon === 0);
  const [pLon, pLat] = pOk ? normalizeLonLat(p.lon, p.lat) : [0, 0];
  const [fbLon, fbLat] = fb ? [fb.lon, fb.lat] : [NaN, NaN];

  // When points map diverges from line endpoints, trust lines (same source as ArcLayer).
  if (pOk && fb) {
    if (coordDistanceDeg(pLon, pLat, fbLon, fbLat) > 5) return [fbLon, fbLat];
    return [pLon, pLat];
  }
  if (fb) return [fbLon, fbLat];
  if (pOk) return [pLon, pLat];
  return normalizeLonLat(fallbackLon ?? 0, fallbackLat ?? 0);
}

function nodeLonLat(
  key: string,
  fallbackLon: number | undefined,
  fallbackLat: number | undefined,
  allPoints: Record<string, MapPoint>,
  lineFallback?: Map<string, { lon: number; lat: number }>,
): [number, number] {
  return resolveNodeLonLat(key, fallbackLon, fallbackLat, allPoints, lineFallback);
}

const GLOBE_LAYER_PARAMETERS = {
  cullMode: 'none' as const,
  depthTest: false,
  depthWrite: false,
};

function coordBucketKey(lon: number, lat: number, precision = 3): string {
  const m = 10 ** precision;
  return `${Math.round(lon * m) / m}:${Math.round(lat * m) / m}`;
}

/** Spread nodes that share the same geo (typical for LAN / geo_range subnets). */
export function buildDisplayCoordMap(points: MapPointEntry[]): Map<string, [number, number]> {
  const buckets = new Map<string, MapPointEntry[]>();
  for (const p of points) {
    const bk = coordBucketKey(p.lon, p.lat);
    const arr = buckets.get(bk) ?? [];
    arr.push(p);
    buckets.set(bk, arr);
  }
  const out = new Map<string, [number, number]>();
  buckets.forEach((group) => {
    if (group.length === 1) {
      out.set(group[0].key, [group[0].lon, group[0].lat]);
      return;
    }
    const baseLon = group[0].lon;
    const baseLat = group[0].lat;
    const n = group.length;
    const radius = Math.min(1.2, 0.12 + Math.sqrt(n) * 0.08);
    const latCos = Math.max(0.25, Math.cos((baseLat * Math.PI) / 180));
    group.forEach((p, i) => {
      const angle = (2 * Math.PI * i) / n;
      out.set(p.key, [
        baseLon + (radius * Math.cos(angle)) / latCos,
        baseLat + radius * Math.sin(angle) * 0.55,
      ]);
    });
  });
  return out;
}

function displayLonLat(
  key: string,
  fallbackLon: number | undefined,
  fallbackLat: number | undefined,
  allPoints: Record<string, MapPoint>,
  lineFallback: Map<string, { lon: number; lat: number }> | undefined,
  displayCoords: Map<string, [number, number]>,
): [number, number] {
  return displayCoords.get(key) ?? nodeLonLat(key, fallbackLon, fallbackLat, allPoints, lineFallback);
}

function arcHeightWithSpread(
  d: MapLine,
  allPoints: Record<string, MapPoint>,
  lineFallback: Map<string, { lon: number; lat: number }> | undefined,
  isGlobe: boolean,
): number {
  const base = isGlobe
    ? globeArcHeight(d, allPoints, lineFallback)
    : mapArcHeight(d, allPoints, lineFallback);
  const rankSpread = ((d._flowRank ?? 0) % 7) * 0.015;
  return base + rankSpread;
}

export function topByCount(arr: MapLine[], max: number): MapLine[] {
  if (!max || arr.length <= max) return arr;
  return [...arr].sort((a, b) => (b.count || 0) - (a.count || 0)).slice(0, max);
}

function decorateFlowLines(lines: MapLine[]): MapLine[] {
  if (!lines.length) return lines;
  const sorted = [...lines].sort((a, b) => (b.count || 0) - (a.count || 0));
  const n = sorted.length;
  const corridorTilt = new Map<string, number>();
  return sorted.map((line, idx) => {
    const rankT = n <= 1 ? 1 : 1 - idx / (n - 1);
    const alpha = Math.round(60 + rankT * 150);
    const edgeId = `${line.src}\0${line.dst}`;
    let tilt = corridorTilt.get(edgeId);
    if (tilt === undefined) {
      let h = 0;
      for (let i = 0; i < edgeId.length; i++) h = (h * 31 + edgeId.charCodeAt(i)) | 0;
      tilt = ((h % 11) - 5) * 1.5;
      corridorTilt.set(edgeId, tilt);
    }
    return { ...line, _flowAlpha: alpha, _flowTilt: tilt, _flowRank: idx };
  });
}

function arcTilt(d: MapLine): number {
  if (typeof d._flowTilt === 'number') return d._flowTilt;
  let h = 0;
  const s = (d.src || '') + '>' + (d.dst || '');
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return ((h % 7) - 3) * 2;
}

function mapArcHeight(
  d: MapLine,
  allPoints: Record<string, MapPoint>,
  lineFallback?: Map<string, { lon: number; lat: number }>,
): number {
  const [sLon, sLat] = nodeLonLat(d.src, d.src_lon, d.src_lat, allPoints, lineFallback);
  const [tLon, tLat] = nodeLonLat(d.dst, d.dst_lon, d.dst_lat, allPoints, lineFallback);
  let dLon = Math.abs(tLon - sLon);
  if (dLon > 180) dLon = 360 - dLon;
  const dist = Math.max(1, Math.hypot(dLon, Math.abs(tLat - sLat)));
  return Math.max(0.15, Math.min(0.35, 0.1 + dist / 160));
}

function globeArcHeight(
  d: MapLine,
  allPoints: Record<string, MapPoint>,
  lineFallback?: Map<string, { lon: number; lat: number }>,
): number {
  const [sLon, sLat] = nodeLonLat(d.src, d.src_lon, d.src_lat, allPoints, lineFallback);
  const [tLon, tLat] = nodeLonLat(d.dst, d.dst_lon, d.dst_lat, allPoints, lineFallback);
  let dLon = Math.abs(tLon - sLon);
  if (dLon > 180) dLon = 360 - dLon;
  const dist = Math.max(1, Math.hypot(dLon, Math.abs(tLat - sLat)));
  return Math.max(0.06, Math.min(0.18, 0.04 + dist / 380));
}

function isOnVisibleGlobeHemisphere(
  lon: number,
  lat: number,
  viewLon: number,
  viewLat: number,
): boolean {
  const toRad = Math.PI / 180;
  const φ1 = (viewLat || 0) * toRad;
  const λ1 = (viewLon || 0) * toRad;
  const φ2 = lat * toRad;
  const λ2 = lon * toRad;
  const cosC =
    Math.sin(φ1) * Math.sin(φ2) + Math.cos(φ1) * Math.cos(φ2) * Math.cos(λ2 - λ1);
  return cosC > 0.22;
}

function globeAngularDistanceRad(lon1: number, lat1: number, lon2: number, lat2: number): number {
  const toRad = Math.PI / 180;
  const φ1 = lat1 * toRad;
  const φ2 = lat2 * toRad;
  const dλ = (lon2 - lon1) * toRad;
  const cosC =
    Math.sin(φ1) * Math.sin(φ2) + Math.cos(φ1) * Math.cos(φ2) * Math.cos(dλ);
  return Math.acos(Math.max(-1, Math.min(1, cosC)));
}

function isArcVisibleOnGlobe(
  line: MapLine,
  viewLon: number,
  viewLat: number,
  allPoints: Record<string, MapPoint>,
  lineFallback?: Map<string, { lon: number; lat: number }>,
): boolean {
  const [sLon, sLat] = nodeLonLat(line.src, line.src_lon, line.src_lat, allPoints, lineFallback);
  const [tLon, tLat] = nodeLonLat(line.dst, line.dst_lon, line.dst_lat, allPoints, lineFallback);
  const srcVis = isOnVisibleGlobeHemisphere(sLon, sLat, viewLon, viewLat);
  const dstVis = isOnVisibleGlobeHemisphere(tLon, tLat, viewLon, viewLat);
  if (!srcVis || !dstVis) return false;
  if (globeAngularDistanceRad(sLon, sLat, tLon, tLat) > Math.PI * 0.55) return false;

  let dLon = tLon - sLon;
  if (dLon > 180) dLon -= 360;
  if (dLon < -180) dLon += 360;

  const samples = 10;
  for (let i = 1; i < samples; i++) {
    const t = i / samples;
    const lon = sLon + dLon * t;
    const lat = sLat + (tLat - sLat) * t;
    if (!isOnVisibleGlobeHemisphere(lon, lat, viewLon, viewLat)) return false;
  }
  return true;
}

function getVisiblePointsFromLines(
  lines: MapLine[],
  allPoints: Record<string, MapPoint>,
  lineFallback: Map<string, { lon: number; lat: number }>,
): MapPointEntry[] {
  const active = new Set<string>();
  lines.forEach((l) => {
    active.add(l.src);
    active.add(l.dst);
  });
  const result: MapPointEntry[] = [];
  active.forEach((key) => {
    const p = allPoints[key];
    const [lon, lat] = resolveNodeLonLat(key, undefined, undefined, allPoints, lineFallback);
    if (lat === 0 && lon === 0) return;
    result.push({
      key,
      ...(p || { lat: 0, lon: 0 }),
      lon,
      lat,
      count: p?.count ?? 1,
    });
  });
  return result;
}

export interface BuildLayersOpts {
  mode: 'map' | 'globe';
  lines: MapLine[];
  points: Record<string, MapPoint>;
  countriesGeoJSON: GeoFeatureCollection | null;
  showHeatmap: boolean;
  showCountryLabels: boolean;
  monoArcColor: boolean;
  repColorArcs: boolean;
  groupBy: string;
  focusedCountry: string | null;
  mapTilesFailed: boolean;
  heavyCountryLayersAllowed: boolean;
  theme: string;
  globeView: ViewState;
  maxArcs: number;
  onLineClick: (line: MapLine) => void;
  onPointClick: (point: MapPointEntry) => void;
  onCountryClick: (countryKey: string, feature: GeoFeature) => void;
  highlightNodeKeys?: string[];
  highlightEdgeKeys?: string[];
}

export interface BuildLayersResult {
  layers: unknown[];
  shown: number;
  total: number;
}

export function buildDeckLayers(opts: BuildLayersOpts): BuildLayersResult {
  const isGlobe = opts.mode === 'globe';
  const viewLon = isGlobe ? opts.globeView.longitude || 0 : 0;
  const viewLat = isGlobe ? opts.globeView.latitude || 0 : 0;

  let lines = opts.lines;
  const totalBeforeLimit = lines.length;
  const arcLimit = opts.heavyCountryLayersAllowed
    ? opts.maxArcs
    : Math.min(opts.maxArcs, 800);

  // Country heatmap stats use the full visible set (before top-N).
  const statsLineFallback = buildLineCoordFallback(lines);
  const statsPoints = getVisiblePointsFromLines(lines, opts.points, statsLineFallback);
  const statsCache = getCountryStatsCache(statsPoints, opts.countriesGeoJSON);

  lines = decorateFlowLines(topByCount(lines, arcLimit));
  const drawnForCount = lines.length;
  const lineFallback = buildLineCoordFallback(lines);
  if (isGlobe) {
    lines = lines.filter((l) =>
      isArcVisibleOnGlobe(l, viewLon, viewLat, opts.points, lineFallback),
    );
  }

  let points = getVisiblePointsFromLines(lines, opts.points, lineFallback);
  const displayCoords = buildDisplayCoordMap(points);
  if (isGlobe) {
    points = points.filter((p) =>
      isOnVisibleGlobeHemisphere(p.lon, p.lat, viewLon, viewLat),
    );
  }

  const layers: unknown[] = [];
  const landColor = cssRgb('--map-land-rgb', opts.mapTilesFailed ? 220 : 0);
  const outlineColor = cssRgb('--map-outline-rgb', opts.mapTilesFailed ? 255 : 140);
  const outlineSoft = cssRgb('--map-outline-rgb', 160);
  const useHeat = opts.showHeatmap;
  const countriesPickable = useHeat || opts.groupBy === 'country' || opts.groupBy === 'continent';
  const heavyOk = opts.heavyCountryLayersAllowed;

  // На глобусе GeoJson-заливка стран даёт артефакты (иглы / ломаный wrap /
  // диск через сферу) — даже как fallback без тайлов. Heatmap и land-fill
  // только для 2D; на globe остаётся basemap (+ подписи/дуги).
  const showCountryFills =
    heavyOk &&
    !!opts.countriesGeoJSON &&
    !isGlobe &&
    (opts.mapTilesFailed || useHeat);

  if (showCountryFills && opts.countriesGeoJSON) {
    const { max, heat } = statsCache;
    layers.push(
      new GeoJsonLayer({
        id: 'countries',
        data: opts.countriesGeoJSON as any,
        pickable: countriesPickable,
        stroked: !isGlobe,
        filled: true,
        wrapLongitude: !isGlobe,
        getFillColor: ((f: GeoFeature): [number, number, number, number] => {
          if (useHeat) {
            const c = heatmapColorRGB(heat.get(f) || 0, max, opts.mapTilesFailed);
            const a = c[3] ?? 210;
            if (opts.focusedCountry && matchCountryFeature(f, opts.focusedCountry)) {
              return [c[0], c[1], c[2], 255];
            }
            return [c[0], c[1], c[2], a];
          }
          return [landColor[0], landColor[1], landColor[2], 255];
        }) as any,
        getLineColor: outlineColor,
        getLineWidth: 1,
        lineWidthMinPixels: 0.5,
        updateTriggers: {
          getFillColor: [
            opts.showHeatmap,
            opts.groupBy,
            statsCache.max,
            isGlobe,
            opts.theme,
            opts.focusedCountry,
            opts.mapTilesFailed,
          ],
          getLineColor: [opts.theme, opts.mapTilesFailed],
        },
        onClick: (info: { object?: GeoFeature }) => {
          if (!info.object || !countriesPickable) return;
          opts.onCountryClick(
            resolveCountryKeyFromFeature(info.object, statsCache.stats),
            info.object,
          );
        },
      }),
    );
  } else if (heavyOk && opts.countriesGeoJSON && countriesPickable && !isGlobe) {
    layers.push(
      new GeoJsonLayer({
        id: 'countries-pick',
        data: opts.countriesGeoJSON as any,
        pickable: true,
        stroked: false,
        filled: true,
        wrapLongitude: true,
        getFillColor: [0, 0, 0, 1] as [number, number, number, number],
        onClick: (info: { object?: GeoFeature }) => {
          if (!info.object) return;
          opts.onCountryClick(
            resolveCountryKeyFromFeature(info.object, statsCache.stats),
            info.object,
          );
        },
      }),
    );
  }

  if (heavyOk && opts.showCountryLabels) {
    let centroids: CountryCentroid[] = buildCountryCentroids(opts.countriesGeoJSON);
    if (isGlobe) {
      centroids = centroids.filter((c) =>
        isOnVisibleGlobeHemisphere(c.lon, c.lat, viewLon, viewLat),
      );
    }
    const labelAlt = isGlobe ? 8e4 : 0;
    const labelColor: [number, number, number, number] =
      opts.theme === 'light' ? [31, 35, 40, 230] : [220, 226, 234, 230];
    layers.push(
      new TextLayer({
        id: 'country-labels',
        data: centroids,
        pickable: false,
        billboard: true,
        characterSet: LABEL_CHARSET,
        getPosition: (d: CountryCentroid) => [d.lon, d.lat, labelAlt],
        getText: (d: CountryCentroid) => d.label,
        getSize: (d: CountryCentroid) => 14 - (d.rank || 3),
        sizeUnits: 'pixels',
        sizeMinPixels: 9,
        sizeMaxPixels: 18,
        getColor: labelColor,
        outlineColor: [...outlineColor, 255] as [number, number, number, number],
        outlineWidth: 4,
        fontSettings: { sdf: true, fontSize: 64, buffer: 4 },
        fontFamily: 'Arial, "Segoe UI", Roboto, sans-serif',
        fontWeight: 700,
        getTextAnchor: 'middle',
        getAlignmentBaseline: 'center',
        parameters: isGlobe ? GLOBE_LAYER_PARAMETERS : undefined,
      }),
    );
  }

  const nodeOpacity = 150;
  layers.push(
    new ArcLayer({
      id: 'arcs',
      data: lines,
      pickable: true,
      // Geodesic arcs avoid bezier + tilt artifacts at the date line (e.g. Kazan → Apia).
      greatCircle: true,
      wrapLongitude: !isGlobe,
      getSourcePosition: (d: MapLine) =>
        displayLonLat(d.src, d.src_lon, d.src_lat, opts.points, lineFallback, displayCoords),
      getTargetPosition: (d: MapLine) =>
        displayLonLat(d.dst, d.dst_lon, d.dst_lat, opts.points, lineFallback, displayCoords),
      getSourceColor: (d: MapLine) => [
        ...arcRGB(d.status, d, opts.monoArcColor, opts.repColorArcs),
        d._flowAlpha || 210,
      ],
      getTargetColor: (d: MapLine) => [
        ...arcRGB(d.status, d, opts.monoArcColor, opts.repColorArcs),
        d._flowAlpha || 210,
      ],
      getWidth: (d: MapLine) => {
        const base = Math.max(1.2, Math.min(7, 1.2 + Math.log2((d.count || 1) + 1) * 0.9));
        if (opts.highlightEdgeKeys?.includes(edgeKey(d.src, d.dst))) return base + 2.5;
        return base;
      },
      widthUnits: 'pixels',
      getHeight: (d: MapLine) => arcHeightWithSpread(d, opts.points, lineFallback, isGlobe),
      getTilt: (d: MapLine) => {
        if (isGlobe) return 0;
        const [sLon] = displayLonLat(
          d.src,
          d.src_lon,
          d.src_lat,
          opts.points,
          lineFallback,
          displayCoords,
        );
        const [tLon] = displayLonLat(
          d.dst,
          d.dst_lon,
          d.dst_lat,
          opts.points,
          lineFallback,
          displayCoords,
        );
        if (arcCrossesAntimeridian(sLon, tLon)) return 0;
        return arcTilt(d) * 0.5;
      },
      autoHighlight: true,
      highlightColor: [255, 255, 255, 140],
      parameters: isGlobe ? GLOBE_LAYER_PARAMETERS : { depthTest: false },
      updateTriggers: {
        getWidth: [opts.highlightEdgeKeys],
        getSourceColor: [opts.monoArcColor, opts.repColorArcs],
      },
      onClick: (info: { object?: MapLine }) => {
        if (info.object) opts.onLineClick(info.object);
      },
    }),
  );

  layers.push(
    new ScatterplotLayer({
      id: 'nodes',
      data: points,
      pickable: true,
      stroked: true,
      filled: true,
      radiusUnits: 'pixels',
      getPosition: (d: MapPointEntry) => displayCoords.get(d.key) ?? [d.lon, d.lat],
      getRadius: (d: MapPointEntry) => {
        const base = Math.max(1.5, Math.min(8, 1.5 + Math.sqrt(d.count || 1) * 0.6));
        if (opts.highlightNodeKeys?.includes(d.key)) return base + 4;
        return base;
      },
      getFillColor: (d: MapPointEntry) =>
        opts.highlightNodeKeys?.includes(d.key)
          ? ([225, 29, 72, 200] as [number, number, number, number])
          : ([88, 166, 255, nodeOpacity] as [number, number, number, number]),
      getLineColor: (d: MapPointEntry) =>
        opts.highlightNodeKeys?.includes(d.key)
          ? ([225, 29, 72, 255] as [number, number, number, number])
          : outlineSoft,
      lineWidthUnits: 'pixels',
      getLineWidth: (d: MapPointEntry) => (opts.highlightNodeKeys?.includes(d.key) ? 2 : 0.7),
      radiusMinPixels: 1.5,
      radiusMaxPixels: 14,
      updateTriggers: {
        getFillColor: [opts.highlightNodeKeys],
        getRadius: [opts.highlightNodeKeys],
        getLineWidth: [opts.highlightNodeKeys],
        getLineColor: [opts.highlightNodeKeys],
      },
      parameters: isGlobe ? GLOBE_LAYER_PARAMETERS : undefined,
      onClick: (info: { object?: MapPointEntry }) => {
        if (info.object) opts.onPointClick(info.object);
      },
    }),
  );

  return { layers, shown: drawnForCount, total: totalBeforeLimit };
}
