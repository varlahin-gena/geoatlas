import type maplibregl from 'maplibre-gl';
import { DEFAULT_GLOBE_VIEW, DEFAULT_MAP_VIEW } from './mapConstants';
import type { ViewState } from './mapTypes';

export function computeGlobeFitZoom(lat: number, width: number, height: number): number {
  const padding = 6;
  const w = Math.max(1, width || 1);
  const h = Math.max(1, height || 1);
  const targetDiameterPx = Math.max(64, Math.min(w, h) - padding * 2);
  const latClamped = Math.max(-60, Math.min(60, lat || 0));
  const mercatorScaleCorrection = Math.max(0.25, Math.cos((latClamped * Math.PI) / 180));
  const requiredWorldCircumferencePx = targetDiameterPx * Math.PI * mercatorScaleCorrection;
  return Math.log2(requiredWorldCircumferencePx / 512);
}

export function computeMapFitZoom(width: number, height: number): number {
  const padding = 8;
  const w = Math.max(1, width || 1);
  const h = Math.max(1, height || 1);
  const targetPx = Math.max(64, Math.max(w, h) - padding * 2);
  return Math.log2(targetPx / 512);
}

export function applyGlobeFitZoom(
  map: maplibregl.Map,
  opts?: Partial<ViewState>,
): ViewState {
  try {
    map.resize();
  } catch {
    /* ignore */
  }
  const container = map.getContainer();
  const cur = map.getCenter();
  const lat = opts?.latitude ?? cur.lat ?? DEFAULT_GLOBE_VIEW.latitude;
  const lon = opts?.longitude ?? cur.lng ?? DEFAULT_GLOBE_VIEW.longitude;
  const bearing = opts?.bearing ?? DEFAULT_GLOBE_VIEW.bearing;
  const zoom = computeGlobeFitZoom(lat, container.clientWidth, container.clientHeight);
  const vs: ViewState = {
    longitude: lon,
    latitude: lat,
    zoom,
    pitch: 0,
    bearing: bearing || 0,
  };
  map.jumpTo({
    center: [lon, lat],
    zoom,
    bearing: bearing || 0,
    pitch: 0,
  });
  return vs;
}

export function applyMapFitZoom(
  map: maplibregl.Map,
  opts?: Partial<ViewState>,
): ViewState {
  try {
    map.resize();
  } catch {
    /* ignore */
  }
  const container = map.getContainer();
  const lon = opts?.longitude ?? DEFAULT_MAP_VIEW.longitude;
  const lat = opts?.latitude ?? DEFAULT_MAP_VIEW.latitude;
  const bearing = opts?.bearing ?? DEFAULT_MAP_VIEW.bearing;
  const zoom = computeMapFitZoom(container.clientWidth, container.clientHeight);
  const vs: ViewState = {
    longitude: lon,
    latitude: lat,
    zoom,
    pitch: 0,
    bearing: bearing || 0,
  };
  map.jumpTo({
    center: [lon, lat],
    zoom,
    bearing: bearing || 0,
    pitch: 0,
  });
  return vs;
}

export function applyMapProjection(map: maplibregl.Map, mode: 'map' | 'globe'): boolean {
  if (!('setProjection' in map) || typeof (map as maplibregl.Map & { setProjection?: unknown }).setProjection !== 'function') {
    return false;
  }
  try {
    const m = map as maplibregl.Map & {
      setProjection: (p: { type: string }) => void;
      setFog?: (fog: Record<string, unknown> | null) => void;
    };
    m.setProjection({ type: mode === 'globe' ? 'globe' : 'mercator' });
    if (typeof m.setFog === 'function') {
      if (mode === 'globe') {
        m.setFog({
          color: 'rgba(13, 17, 23, 0.65)',
          'high-color': 'rgba(20, 28, 40, 0.25)',
          'horizon-blend': 0.015,
          'space-color': 'rgb(5, 8, 12)',
          'star-intensity': 0,
        });
      } else {
        m.setFog(null);
      }
    }
    return true;
  } catch (e) {
    console.warn('setProjection failed:', e);
    return false;
  }
}

export function readViewStateFromMap(map: maplibregl.Map): ViewState {
  const c = map.getCenter();
  return {
    longitude: ((c.lng + 540) % 360) - 180,
    latitude: c.lat,
    zoom: map.getZoom(),
    bearing: map.getBearing(),
    pitch: map.getPitch(),
  };
}

/** 0.25° grid — rebuild hemisphere culling only when the camera crosses a cell. */
export function globeCullKey(longitude?: number, latitude?: number): string {
  const lon = Math.round((longitude || 0) * 4) / 4;
  const lat = Math.round((latitude || 0) * 4) / 4;
  return `${lon}:${lat}`;
}
