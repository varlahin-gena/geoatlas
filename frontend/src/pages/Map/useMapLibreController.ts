import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type MutableRefObject,
  type RefObject,
  type SetStateAction,
} from 'react';
import maplibregl from 'maplibre-gl';
import { MapboxOverlay } from '@deck.gl/mapbox';
import type { Theme } from '@/auth/theme';
import type { ToastKind } from '@/components/Toast';
import {
  DEFAULT_GLOBE_VIEW,
  DEFAULT_MAP_VIEW,
  MAP_STYLE_DARK,
  MAP_STYLE_LIGHT,
  emptyStyleFallback,
} from './mapConstants';
import {
  applyGlobeFitZoom,
  applyMapFitZoom,
  applyMapProjection,
  globeCullKey,
  readViewStateFromMap,
} from './mapViewport';
import type { ViewState } from './mapTypes';
import { useGlobeAutoRotate } from './useGlobeAutoRotate';

type ToastFn = (msg: string, kind?: ToastKind) => void;

/**
 * Imperative MapLibre + deck overlay lifecycle: init, basemap fallback,
 * theme upgrades, view-mode projection, and layer tick bumping.
 */
export function useMapLibreController(opts: {
  theme: Theme;
  viewMode: 'map' | 'globe';
  setViewMode: Dispatch<SetStateAction<'map' | 'globe'>>;
  autoRotate: boolean;
  toast: ToastFn;
}) {
  const { theme, viewMode, setViewMode, autoRotate, toast } = opts;

  const mapContainer = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);
  const mapTilesFailedRef = useRef(true);
  const basemapOkRef = useRef(false);
  const lastThemeBasemapRef = useRef<string | null>(null);
  const heavyLayersRef = useRef(false);
  const basemapGenRef = useRef(0);
  const layersRefreshBusy = useRef(false);

  const [heavyCountryLayers, setHeavyCountryLayers] = useState(false);
  const [mapTilesFailed, setMapTilesFailed] = useState(true);
  const [mapReady, setMapReady] = useState(false);
  const [layersTick, setLayersTick] = useState(0);
  const globeViewRef = useRef<ViewState>({ ...DEFAULT_GLOBE_VIEW });
  const lastGlobeCullKeyRef = useRef('');

  const bumpLayersTick = useCallback(() => {
    setLayersTick((t) => t + 1);
  }, []);

  const {
    startGlobeAutoRotate,
    stopGlobeAutoRotate,
    userInteractingRef,
    viewModeRef,
    autoRotateRef,
  } = useGlobeAutoRotate({
    mapRef,
    viewMode,
    autoRotate,
    mapReady,
    globeViewRef,
  });

  heavyLayersRef.current = heavyCountryLayers;
  mapTilesFailedRef.current = mapTilesFailed;

  const beginRemoteBasemapUpgrade = useCallback((styleUrl: string) => {
    const map = mapRef.current;
    if (!map || !styleUrl) return;
    const gen = ++basemapGenRef.current;
    let settled = false;
    const center = map.getCenter();
    const zoom = map.getZoom();
    const bearing = map.getBearing();
    const pitch = map.getPitch();

    const finishOk = () => {
      if (settled || gen !== basemapGenRef.current || !mapRef.current) return;
      settled = true;
      basemapOkRef.current = true;
      mapTilesFailedRef.current = false;
      setMapTilesFailed(false);
      try {
        map.jumpTo({ center, zoom, bearing, pitch });
        applyMapProjection(map, viewModeRef.current);
        map.triggerRepaint();
      } catch {
        /* ignore */
      }
      setLayersTick((t) => t + 1);
    };

    map.once('style.load', finishOk);
    try {
      map.setStyle(styleUrl);
    } catch (e) {
      console.warn('Remote basemap upgrade failed to start:', e);
      return;
    }

    window.setTimeout(() => {
      if (settled || gen !== basemapGenRef.current || !mapRef.current) return;
      if (basemapOkRef.current) return;
      settled = true;
      mapTilesFailedRef.current = true;
      setMapTilesFailed(true);
      console.warn('Remote basemap slow/unavailable, keeping local style');
      try {
        map.setStyle(emptyStyleFallback(viewModeRef.current));
        applyMapProjection(map, viewModeRef.current);
      } catch {
        /* ignore */
      }
      try {
        map.jumpTo({ center, zoom, bearing, pitch });
        applyMapProjection(map, viewModeRef.current);
      } catch {
        /* ignore */
      }
      setLayersTick((t) => t + 1);
    }, 12000);
  }, [viewModeRef]);

  // Init map once — start with remote basemap (CARTO). Empty style only as fallback.
  useEffect(() => {
    if (!mapContainer.current || mapRef.current) return;
    const host = mapContainer.current;
    host.innerHTML = '';

    const initialStyleUrl = theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK;
    lastThemeBasemapRef.current = theme;
    setMapTilesFailed(false);
    mapTilesFailedRef.current = false;
    basemapOkRef.current = false;

    const vs = viewMode === 'globe' ? DEFAULT_GLOBE_VIEW : DEFAULT_MAP_VIEW;
    let map: maplibregl.Map;
    try {
      map = new maplibregl.Map({
        container: host,
        style: initialStyleUrl,
        center: [vs.longitude, vs.latitude],
        zoom: vs.zoom,
        bearing: vs.bearing,
        pitch: vs.pitch,
        attributionControl: {},
        fadeDuration: 0,
        // Needed for PNG export from canvas
        ...({ preserveDrawingBuffer: true } as Record<string, unknown>),
      });
    } catch (e) {
      console.error('MapLibre init failed:', e);
      toast('Ошибка инициализации карты', 'error');
      return;
    }

    map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'bottom-right');
    mapRef.current = map;

    const fallbackToEmpty = (why: string) => {
      if (basemapOkRef.current || mapTilesFailedRef.current) return;
      console.warn('Basemap fallback:', why);
      mapTilesFailedRef.current = true;
      setMapTilesFailed(true);
      try {
        map.setStyle(emptyStyleFallback(viewModeRef.current));
        applyMapProjection(map, viewModeRef.current);
      } catch {
        /* ignore */
      }
      setLayersTick((t) => t + 1);
    };

    map.on('error', (e) => {
      const msg = (e && e.error && e.error.message) || String(e.error || '');
      if (!/Failed to fetch|NetworkError|AJAX|tile|style|load/i.test(msg) && !e.error) return;
      if (basemapOkRef.current) {
        console.warn('Basemap tile/style warning (keeping remote style)', e.error || e);
        return;
      }
      fallbackToEmpty(msg || 'map error');
    });

    let readyOnce = false;
    const onReady = () => {
      if (readyOnce) return;
      readyOnce = true;
      if (!mapTilesFailedRef.current) {
        basemapOkRef.current = true;
        mapTilesFailedRef.current = false;
        setMapTilesFailed(false);
      }

      if (viewModeRef.current === 'globe') {
        if (!applyMapProjection(map, 'globe')) {
          toast('Globe projection недоступен — остаёмся в 2D', 'error');
          setViewMode('map');
        } else {
          const gvs = applyGlobeFitZoom(map, {
            longitude: DEFAULT_GLOBE_VIEW.longitude,
            latitude: DEFAULT_GLOBE_VIEW.latitude,
            bearing: DEFAULT_GLOBE_VIEW.bearing,
          });
          globeViewRef.current = gvs;
          lastGlobeCullKeyRef.current = globeCullKey(gvs.longitude, gvs.latitude);
        }
      } else {
        applyMapProjection(map, 'map');
        applyMapFitZoom(map, { ...DEFAULT_MAP_VIEW });
      }

      const overlay = new MapboxOverlay({
        interleaved: false,
        layers: [],
      });
      map.addControl(overlay as unknown as maplibregl.IControl);
      overlayRef.current = overlay;
      setMapReady(true);

      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          const enable = () => {
            heavyLayersRef.current = true;
            setHeavyCountryLayers(true);
          };
          if (typeof requestIdleCallback === 'function') {
            requestIdleCallback(enable, { timeout: 120 });
          } else {
            enable();
          }
        });
      });

      if (viewModeRef.current === 'globe' && autoRotateRef.current) {
        startGlobeAutoRotate();
      }
    };

    if (map.isStyleLoaded()) onReady();
    else map.once('load', onReady);
    const t = window.setTimeout(() => {
      if (!readyOnce) {
        fallbackToEmpty('initial style timeout');
        onReady();
      }
    }, 12000);

    map.on('move', () => {
      const vsNow = readViewStateFromMap(map);
      if (viewModeRef.current !== 'globe') return;
      globeViewRef.current = vsNow;
      const key = globeCullKey(vsNow.longitude, vsNow.latitude);
      if (key !== lastGlobeCullKeyRef.current) {
        lastGlobeCullKeyRef.current = key;
        bumpLayersTick();
      }
    });
    map.on('zoomend', () => {
      if (layersRefreshBusy.current) return;
      setLayersTick((tick) => tick + 1);
    });

    const onInteractStart = () => {
      userInteractingRef.current = true;
      stopGlobeAutoRotate();
    };
    const onInteractEnd = () => {
      userInteractingRef.current = false;
      if (viewModeRef.current === 'globe' && autoRotateRef.current) startGlobeAutoRotate();
    };
    map.on('mousedown', onInteractStart);
    map.on('mouseup', onInteractEnd);
    map.on('dragstart', onInteractStart);
    map.on('dragend', onInteractEnd);
    map.on('touchstart', onInteractStart);
    map.on('touchend', onInteractEnd);

    return () => {
      window.clearTimeout(t);
      stopGlobeAutoRotate();
      overlayRef.current = null;
      try {
        map.remove();
      } catch {
        /* ignore */
      }
      mapRef.current = null;
      setMapReady(false);
      heavyLayersRef.current = false;
      setHeavyCountryLayers(false);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Theme basemap — first load is handled in onReady; only react to later theme changes.
  useEffect(() => {
    if (!mapReady || !mapRef.current) return;
    if (lastThemeBasemapRef.current === null) {
      lastThemeBasemapRef.current = theme;
      return;
    }
    if (lastThemeBasemapRef.current === theme) return;
    lastThemeBasemapRef.current = theme;
    basemapOkRef.current = false;
    const styleUrl = theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK;
    beginRemoteBasemapUpgrade(styleUrl);
  }, [theme, mapReady, beginRemoteBasemapUpgrade]);

  // View mode switch — never setStyle(empty) here: that races the CARTO upgrade
  // while mapTilesFailedRef is still true and wipes a loading/loaded basemap.
  useEffect(() => {
    const map = mapRef.current;
    if (!mapReady || !map) return;
    stopGlobeAutoRotate();
    lastGlobeCullKeyRef.current = '';
    if (viewMode === 'globe') {
      if (!applyMapProjection(map, 'globe')) {
        toast('Globe projection недоступен', 'error');
        setViewMode('map');
        return;
      }
      const gvs = applyGlobeFitZoom(map, {
        longitude: globeViewRef.current.longitude ?? DEFAULT_GLOBE_VIEW.longitude,
        latitude: globeViewRef.current.latitude ?? DEFAULT_GLOBE_VIEW.latitude,
        bearing: globeViewRef.current.bearing ?? DEFAULT_GLOBE_VIEW.bearing,
      });
      globeViewRef.current = gvs;
      lastGlobeCullKeyRef.current = globeCullKey(gvs.longitude, gvs.latitude);
      if (autoRotate) startGlobeAutoRotate();
    } else {
      applyMapProjection(map, 'map');
      applyMapFitZoom(map, { ...DEFAULT_MAP_VIEW });
    }
    if (overlayRef.current) {
      overlayRef.current.setProps({ views: undefined });
    }
    setLayersTick((tick) => tick + 1);
    window.setTimeout(() => {
      try {
        map.resize();
      } catch {
        /* ignore */
      }
      applyMapProjection(map, viewModeRef.current);
      if (viewModeRef.current === 'globe') {
        const gvs = applyGlobeFitZoom(map);
        globeViewRef.current = gvs;
        lastGlobeCullKeyRef.current = globeCullKey(gvs.longitude, gvs.latitude);
      } else {
        applyMapFitZoom(map);
      }
      try {
        map.triggerRepaint();
      } catch {
        /* ignore */
      }
      setLayersTick((tick) => tick + 1);
    }, 100);
  }, [
    viewMode,
    mapReady,
    autoRotate,
    startGlobeAutoRotate,
    stopGlobeAutoRotate,
    toast,
    setViewMode,
    globeViewRef,
    viewModeRef,
  ]);

  function resetView() {
    const map = mapRef.current;
    if (!map) return;
    if (viewMode === 'globe') {
      const gvs = applyGlobeFitZoom(map, {
        longitude: DEFAULT_GLOBE_VIEW.longitude,
        latitude: DEFAULT_GLOBE_VIEW.latitude,
        bearing: DEFAULT_GLOBE_VIEW.bearing,
      });
      globeViewRef.current = gvs;
      lastGlobeCullKeyRef.current = globeCullKey(gvs.longitude, gvs.latitude);
    } else {
      applyMapFitZoom(map, { ...DEFAULT_MAP_VIEW });
    }
    setLayersTick((tick) => tick + 1);
  }

  function exportPng() {
    const canvas = mapContainer.current?.querySelector('canvas');
    if (!canvas) {
      toast('Карта ещё не готова', 'warn');
      return;
    }
    const a = document.createElement('a');
    a.href = (canvas as HTMLCanvasElement).toDataURL('image/png');
    a.download = `geoatlas-${Date.now()}.png`;
    a.click();
  }

  return {
    mapContainer: mapContainer as RefObject<HTMLDivElement | null>,
    mapRef,
    overlayRef,
    layersRefreshBusy,
    heavyCountryLayers,
    mapTilesFailed,
    globeViewRef: globeViewRef as MutableRefObject<ViewState>,
    mapReady,
    layersTick,
    resetView,
    exportPng,
  };
}
