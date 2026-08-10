import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import maplibregl from 'maplibre-gl';
import { MapboxOverlay } from '@deck.gl/mapbox';
import { Link, useLocation } from 'react-router-dom';
import { apiFetch, authHeaders } from '@/api/client';
import { useAuth } from '@/auth/AuthContext';
import { SystemHealthPill, UserMenu, NavIcon, NAV_ICONS } from '@/components/Shell';
import { filterNav, isNavActive, PAGE_NAV } from '@/components/nav';
import { useToast } from '@/components/Toast';
import { compileSearchQuery, evaluateSearchAst } from '@/lib/search';
import { fmtNumber } from '@/lib/format';
import { SearchBuilder } from './SearchBuilder';
import {
  DEFAULT_GLOBE_VIEW,
  DEFAULT_MAP_VIEW,
  MAP_STYLE_DARK,
  MAP_STYLE_LIGHT,
  buildPeriodQuery,
  emptyStyleFallback,
  mapRuCountry,
} from './mapConstants';
import {
  getCountryStatsCache,
  lineMatchesFocusedCountry,
  loadCountriesGeoJSON,
  type GeoFeature,
  type GeoFeatureCollection,
} from './mapHeatmap';
import {
  buildCountryDetailBase,
  buildLineDetail,
  buildPointDetail,
  fetchCountrySeries,
  linesForCountry,
  MapDetailPanel,
  renderSparklineSVG,
} from './mapDetail';
import { buildDeckLayers, hasCoords } from './mapLayers';
import {
  categoryLabel,
  collectReputationMenuTree,
  lineMatchesReputation,
  reputationFilterActiveCount,
} from './mapReputation';
import {
  applyGlobeFitZoom,
  applyMapFitZoom,
  applyMapProjection,
  readViewStateFromMap,
} from './mapViewport';
import type {
  DetailState,
  EventsPayload,
  MapLine,
  MapPoint,
  MapPointEntry,
  RepFilterSide,
  ViewState,
} from './mapTypes';
import 'maplibre-gl/dist/maplibre-gl.css';
import '@/styles/index.css';

const PERIODS = [
  ['15m', '15 минут'],
  ['30m', '30 минут'],
  ['1h', '1 час'],
  ['3h', '3 часа'],
  ['6h', '6 часов'],
  ['12h', '12 часов'],
  ['1d', '1 день'],
  ['3d', '3 дня'],
  ['7d', '7 дней'],
  ['14d', '14 дней'],
  ['30d', '30 дней'],
  ['custom', 'Свой диапазон…'],
] as const;

function Icon({ d, paths }: { d?: string; paths?: string[] }) {
  return (
    <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      {d ? <path d={d} /> : null}
      {(paths || []).map((p) => (
        <path key={p} d={p} />
      ))}
    </svg>
  );
}

async function copyToClipboard(text: string, toast: (m: string, t?: string) => void) {
  try {
    await navigator.clipboard.writeText(text);
    toast('Скопировано', 'success');
  } catch {
    toast('Не удалось скопировать', 'error');
  }
}

export default function MapPage() {
  const { isAdmin, reputationEnabled, uiAuthEnabled, theme } = useAuth();
  const { toast } = useToast();
  const location = useLocation();
  const mapContainer = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);
  const logFileRef = useRef<HTMLInputElement>(null);
  const geoFileRef = useRef<HTMLInputElement>(null);
  const seriesAbortRef = useRef<AbortController | null>(null);
  const rotateRafRef = useRef<number | null>(null);
  const rotateLastTsRef = useRef(0);
  const userInteractingRef = useRef(false);
  const lastGlobeCullKeyRef = useRef('');
  const mapTilesFailedRef = useRef(true);
  const basemapOkRef = useRef(false);
  const lastThemeBasemapRef = useRef<string | null>(null);
  const heavyLayersRef = useRef(false);
  const basemapGenRef = useRef(0);
  const layersRefreshBusy = useRef(false);

  const [period, setPeriod] = useState('1d');
  const [periodFrom, setPeriodFrom] = useState('');
  const [periodTo, setPeriodTo] = useState('');
  const [groupBy, setGroupBy] = useState('city');
  const [filter, setFilter] = useState<'all' | 'allowed' | 'blocked'>('all');
  const [search, setSearch] = useState('');
  const [builderOpen, setBuilderOpen] = useState(false);
  const [minCount, setMinCount] = useState(1);
  const [maxArcs, setMaxArcs] = useState(5000);
  const [points, setPoints] = useState<Record<string, MapPoint>>({});
  const [lines, setLines] = useState<MapLine[]>([]);
  const [viewMode, setViewMode] = useState<'map' | 'globe'>('map');
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [showLegend, setShowLegend] = useState(true);
  const [showStats, setShowStats] = useState(true);
  const [showHeatmap, setShowHeatmap] = useState(false);
  const [showCountryLabels, setShowCountryLabels] = useState(false);
  const [monoArcs, setMonoArcs] = useState(false);
  const [autoRotate, setAutoRotate] = useState(true);
  const [countriesGeoJSON, setCountriesGeoJSON] = useState<GeoFeatureCollection | null>(null);
  const [heavyCountryLayers, setHeavyCountryLayers] = useState(false);
  const [mapTilesFailed, setMapTilesFailed] = useState(true);
  const [focusedCountry, setFocusedCountry] = useState<string | null>(null);
  const [detail, setDetail] = useState<DetailState | null>(null);
  const [globeView, setGlobeView] = useState<ViewState>({ ...DEFAULT_GLOBE_VIEW });
  const [arcCountInfo, setArcCountInfo] = useState({ shown: 0, total: 0 });
  const [mapReady, setMapReady] = useState(false);
  const [layersTick, setLayersTick] = useState(0);

  const [repMenuOpen, setRepMenuOpen] = useState(false);
  const [repCategories, setRepCategories] = useState<Set<string>>(() => new Set());
  const [repLists, setRepLists] = useState<Set<string>>(() => new Set());
  const [repSide, setRepSide] = useState<RepFilterSide>('any');
  const [repColorArcs, setRepColorArcs] = useState(false);

  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    try {
      return localStorage.getItem('nm.mapSidebarCollapsed') === '1';
    } catch {
      return false;
    }
  });

  const viewModeRef = useRef(viewMode);
  const autoRotateRef = useRef(autoRotate);
  const globeViewRef = useRef(globeView);
  viewModeRef.current = viewMode;
  autoRotateRef.current = autoRotate;
  globeViewRef.current = globeView;
  heavyLayersRef.current = heavyCountryLayers;
  mapTilesFailedRef.current = mapTilesFailed;

  useEffect(() => {
    document.title = 'ГеоАтлас — SOC';
    document.body.classList.add('page-map');
    return () => document.body.classList.remove('page-map');
  }, []);

  useEffect(() => {
    void loadCountriesGeoJSON().then(setCountriesGeoJSON);
  }, []);

  const compiled = useMemo(() => compileSearchQuery(search), [search]);
  const periodQuery = useMemo(
    () => buildPeriodQuery(period, periodFrom, periodTo),
    [period, periodFrom, periodTo],
  );

  const repActive =
    groupBy === 'ip' && reputationFilterActiveCount(repCategories, repLists) > 0;

  const visibleLines = useMemo(() => {
    return lines.filter((line) => {
      if (!hasCoords(line)) return false;
      if (line.src && line.src === line.dst) return false;
      if ((line.count || 0) < minCount) return false;
      if (filter === 'allowed' && line.status !== 'allowed') return false;
      if (filter === 'blocked' && line.status !== 'blocked') return false;
      if (!lineMatchesFocusedCountry(line, focusedCountry, points)) return false;
      if (repActive && !lineMatchesReputation(line, repCategories, repLists, repSide)) {
        return false;
      }
      if (compiled.mode === 'empty') return true;
      if (compiled.mode === 'error' || !compiled.ast) return true;
      const srcP = points[line.src];
      const dstP = points[line.dst];
      const fieldValues = {
        all: [
          line.src,
          line.dst,
          line.rule,
          line.device,
          line.proto,
          line.src_country,
          line.dst_country,
          mapRuCountry(line.src_country),
          mapRuCountry(line.dst_country),
          srcP?.city,
          dstP?.city,
        ],
        ip: [line.src, line.dst],
        country: [
          line.src_country,
          line.dst_country,
          mapRuCountry(line.src_country),
          mapRuCountry(line.dst_country),
        ],
        city: [srcP?.city, dstP?.city],
        rule: [line.rule],
        device: [line.device],
        src: [line.src, line.src_country],
        dst: [line.dst, line.dst_country],
        proto: [line.proto],
        zone: [line.src_zone, line.dst_zone],
      };
      return evaluateSearchAst(compiled.ast, fieldValues);
    });
  }, [
    lines,
    points,
    minCount,
    filter,
    compiled,
    focusedCountry,
    repActive,
    repCategories,
    repLists,
    repSide,
  ]);

  const stats = useMemo(() => {
    let events = 0;
    let allowed = 0;
    let blocked = 0;
    const nodeSet = new Set<string>();
    const countrySet = new Set<string>();
    const citySet = new Set<string>();
    for (const line of visibleLines) {
      const c = Number(line.count) || 0;
      events += c;
      if (line.status === 'allowed') allowed += c;
      else if (line.status === 'blocked') blocked += c;
      nodeSet.add(line.src);
      nodeSet.add(line.dst);
      if (line.src_country) countrySet.add(line.src_country);
      if (line.dst_country) countrySet.add(line.dst_country);
      const s = points[line.src];
      const d = points[line.dst];
      if (s?.city) citySet.add(s.city);
      if (d?.city) citySet.add(d.city);
    }
    return {
      events,
      allowed,
      blocked,
      connections: visibleLines.length,
      nodes: nodeSet.size,
      countries: countrySet.size,
      cities: citySet.size,
    };
  }, [visibleLines, points]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const apiLimit = groupBy === 'ip' || groupBy === 'subnet' ? 50000 : 10000;
      const url = `/api/events?group_by=${encodeURIComponent(groupBy)}&limit=${apiLimit}${periodQuery}`;
      const data = await apiFetch<EventsPayload>(url);
      setPoints(data.points || {});
      setLines(data.lines || []);
      setFetchError(null);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Ошибка загрузки';
      setFetchError(msg);
      toast(msg, 'error');
    } finally {
      setLoading(false);
    }
  }, [groupBy, periodQuery, toast]);

  useEffect(() => {
    void fetchData();
  }, [fetchData]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => void fetchData(), 30000);
    return () => window.clearInterval(id);
  }, [autoRefresh, fetchData]);

  const stopGlobeAutoRotate = useCallback(() => {
    if (rotateRafRef.current) {
      cancelAnimationFrame(rotateRafRef.current);
      rotateRafRef.current = null;
    }
    rotateLastTsRef.current = 0;
  }, []);

  const startGlobeAutoRotate = useCallback(() => {
    stopGlobeAutoRotate();
    if (!autoRotateRef.current || viewModeRef.current !== 'globe' || !mapRef.current) return;

    const tick = (ts: number) => {
      const map = mapRef.current;
      if (
        !map ||
        viewModeRef.current !== 'globe' ||
        !autoRotateRef.current ||
        userInteractingRef.current ||
        document.hidden
      ) {
        stopGlobeAutoRotate();
        return;
      }
      if (!rotateLastTsRef.current) rotateLastTsRef.current = ts;
      const dt = Math.min(ts - rotateLastTsRef.current, 50);
      rotateLastTsRef.current = ts;
      const lng = map.getCenter().lng + dt * 0.008;
      map.jumpTo({ center: [lng, map.getCenter().lat] });
      const vs = readViewStateFromMap(map);
      setGlobeView(vs);
      globeViewRef.current = vs;
      const lon = Math.round((vs.longitude || 0) * 4) / 4;
      const lat = Math.round((vs.latitude || 0) * 4) / 4;
      const key = `${lon}:${lat}`;
      if (key !== lastGlobeCullKeyRef.current) {
        lastGlobeCullKeyRef.current = key;
        setLayersTick((t) => t + 1);
      }
      rotateRafRef.current = requestAnimationFrame(tick);
    };
    rotateRafRef.current = requestAnimationFrame(tick);
  }, [stopGlobeAutoRotate]);

  const beginRemoteBasemapUpgrade = useCallback(
    (styleUrl: string) => {
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
        // style.load never arrived — stay on whatever is current; only force
        // empty fallback if we never had a successful remote style.
        if (basemapOkRef.current) return;
        settled = true;
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
    },
    [],
  );

  // Init map once
  useEffect(() => {
    if (!mapContainer.current || mapRef.current) return;
    const host = mapContainer.current;
    host.innerHTML = '';

    setMapTilesFailed(true);
    const vs = viewMode === 'globe' ? DEFAULT_GLOBE_VIEW : DEFAULT_MAP_VIEW;
    let map: maplibregl.Map;
    try {
      map = new maplibregl.Map({
        container: host,
        style: emptyStyleFallback(viewMode) as maplibregl.StyleSpecification,
        center: [vs.longitude, vs.latitude],
        zoom: vs.zoom,
        bearing: vs.bearing,
        pitch: vs.pitch,
        attributionControl: {},
        fadeDuration: 0,
        // Needed for PNG export from canvas
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        ...({ preserveDrawingBuffer: true } as any),
      });
    } catch (e) {
      console.error('MapLibre init failed:', e);
      toast('Ошибка инициализации карты', 'error');
      return;
    }

    map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'bottom-right');
    mapRef.current = map;

    map.on('error', (e) => {
      const msg = (e && e.error && e.error.message) || String(e.error || '');
      if (!/Failed to fetch|NetworkError|AJAX|tile|style|load/i.test(msg) && !e.error) return;
      // After a successful remote style, keep it — transient tile errors must not
      // wipe the basemap to an empty black background (2D/globe both break).
      if (basemapOkRef.current || mapTilesFailedRef.current) {
        if (basemapOkRef.current) {
          console.warn('Basemap tile/style warning (keeping remote style)', e.error || e);
        }
        return;
      }
      setMapTilesFailed(true);
      try {
        map.setStyle(emptyStyleFallback(viewModeRef.current));
        applyMapProjection(map, viewModeRef.current);
      } catch {
        /* ignore */
      }
      setLayersTick((t) => t + 1);
    });

    let readyOnce = false;
    const onReady = () => {
      if (readyOnce) return;
      readyOnce = true;
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
          setGlobeView(gvs);
          globeViewRef.current = gvs;
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

      const remoteStyleUrl = theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK;
      lastThemeBasemapRef.current = theme;
      beginRemoteBasemapUpgrade(remoteStyleUrl);

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
      if (!readyOnce) onReady();
    }, 300);

    map.on('move', () => {
      const vsNow = readViewStateFromMap(map);
      if (viewModeRef.current === 'globe') {
        setGlobeView(vsNow);
        globeViewRef.current = vsNow;
        const lon = Math.round((vsNow.longitude || 0) * 4) / 4;
        const lat = Math.round((vsNow.latitude || 0) * 4) / 4;
        const key = `${lon}:${lat}`;
        if (key !== lastGlobeCullKeyRef.current) {
          lastGlobeCullKeyRef.current = key;
          setLayersTick((t) => t + 1);
        }
      }
    });
    map.on('zoomend', () => {
      if (layersRefreshBusy.current) return;
      setLayersTick((t) => t + 1);
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

  // View mode switch
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
      // Empty fallback must be globe-tinted (sphere = background). Re-apply when
      // remote tiles failed so the globe is visible without GeoJSON fills.
      if (mapTilesFailedRef.current) {
        try {
          map.setStyle(emptyStyleFallback('globe'));
          map.once('style.load', () => {
            applyMapProjection(map, 'globe');
            const gvs = applyGlobeFitZoom(map, {
              longitude: globeViewRef.current.longitude ?? DEFAULT_GLOBE_VIEW.longitude,
              latitude: globeViewRef.current.latitude ?? DEFAULT_GLOBE_VIEW.latitude,
              bearing: globeViewRef.current.bearing ?? DEFAULT_GLOBE_VIEW.bearing,
            });
            setGlobeView(gvs);
            globeViewRef.current = gvs;
            setLayersTick((t) => t + 1);
          });
        } catch {
          /* ignore */
        }
      }
      const gvs = applyGlobeFitZoom(map, {
        longitude: globeViewRef.current.longitude ?? DEFAULT_GLOBE_VIEW.longitude,
        latitude: globeViewRef.current.latitude ?? DEFAULT_GLOBE_VIEW.latitude,
        bearing: globeViewRef.current.bearing ?? DEFAULT_GLOBE_VIEW.bearing,
      });
      setGlobeView(gvs);
      globeViewRef.current = gvs;
      if (autoRotate) startGlobeAutoRotate();
    } else {
      applyMapProjection(map, 'map');
      if (mapTilesFailedRef.current) {
        try {
          map.setStyle(emptyStyleFallback('map'));
          map.once('style.load', () => {
            applyMapProjection(map, 'map');
            applyMapFitZoom(map, { ...DEFAULT_MAP_VIEW });
            setLayersTick((t) => t + 1);
          });
        } catch {
          /* ignore */
        }
      }
      applyMapFitZoom(map, { ...DEFAULT_MAP_VIEW });
    }
    if (overlayRef.current) {
      overlayRef.current.setProps({ views: undefined });
    }
    setLayersTick((t) => t + 1);
    window.setTimeout(() => {
      try {
        map.resize();
      } catch {
        /* ignore */
      }
      applyMapProjection(map, viewModeRef.current);
      if (viewModeRef.current === 'globe') {
        const gvs = applyGlobeFitZoom(map);
        setGlobeView(gvs);
        globeViewRef.current = gvs;
      } else {
        applyMapFitZoom(map);
      }
      try {
        map.triggerRepaint();
      } catch {
        /* ignore */
      }
      setLayersTick((t) => t + 1);
    }, 100);
  }, [viewMode, mapReady, autoRotate, startGlobeAutoRotate, stopGlobeAutoRotate, toast]);

  useEffect(() => {
    if (viewMode === 'globe' && autoRotate && mapReady) startGlobeAutoRotate();
    else stopGlobeAutoRotate();
  }, [autoRotate, viewMode, mapReady, startGlobeAutoRotate, stopGlobeAutoRotate]);

  const closeDetail = useCallback(() => {
    setDetail(null);
    if (seriesAbortRef.current) {
      try {
        seriesAbortRef.current.abort();
      } catch {
        /* ignore */
      }
      seriesAbortRef.current = null;
    }
  }, []);

  const clearFocusedCountry = useCallback(() => {
    setFocusedCountry(null);
  }, []);

  const applySearchFilter = useCallback((value: string) => {
    setFocusedCountry(null);
    setSearch(value);
  }, []);

  const openLineDetail = useCallback(
    (line: MapLine) => {
      setDetail(
        buildLineDetail(line, groupBy, [
          {
            label: 'Копировать src',
            onClick: () => void copyToClipboard(line.src || '', toast),
          },
          {
            label: 'Копировать dst',
            onClick: () => void copyToClipboard(line.dst || '', toast),
          },
          {
            label: 'Поиск src',
            onClick: () => applySearchFilter(line.src_label || line.src || ''),
          },
          {
            label: 'Поиск dst',
            onClick: () => applySearchFilter(line.dst_label || line.dst || ''),
          },
        ]),
      );
    },
    [groupBy, toast, applySearchFilter],
  );

  const openPointDetail = useCallback(
    (point: MapPointEntry) => {
      const actions = [
        {
          label: 'Копировать ключ',
          onClick: () => void copyToClipboard(point.key || '', toast),
        },
        {
          label: 'Искать узел',
          onClick: () => applySearchFilter(point.label || point.key || ''),
        },
      ];
      if (point.country && point.country !== 'Неизвестно') {
        actions.push({
          label: 'Искать страну',
          onClick: () => applySearchFilter(mapRuCountry(point.country)),
        });
      }
      setDetail(buildPointDetail(point, lines, actions, openLineDetail));
    },
    [lines, toast, applySearchFilter, openLineDetail],
  );

  const openCountryDetail = useCallback(
    (countryKey: string, _feature?: GeoFeature) => {
      if (!countryKey) return;
      setFocusedCountry(countryKey);
      const pointsForStats: MapPointEntry[] = Object.entries(points)
        .filter(([, p]) => p && !(p.lat === 0 && p.lon === 0))
        .map(([key, p]) => ({ key, ...p }));
      const { stats: countryStats } = getCountryStatsCache(pointsForStats, countriesGeoJSON);
      const events = countryStats[countryKey] || 0;
      const topLines = linesForCountry(countryKey, visibleLines, points).slice(0, 20);
      const base = buildCountryDetailBase(
        countryKey,
        events,
        topLines,
        [
          {
            label: 'Сбросить фокус',
            onClick: () => {
              clearFocusedCountry();
              closeDetail();
            },
          },
          {
            label: 'Искать страну',
            onClick: () => applySearchFilter(mapRuCountry(countryKey)),
          },
        ],
        openLineDetail,
      );
      setDetail(base);

      if (seriesAbortRef.current) {
        try {
          seriesAbortRef.current.abort();
        } catch {
          /* ignore */
        }
      }
      const controller = new AbortController();
      seriesAbortRef.current = controller;
      void (async () => {
        try {
          const data = await fetchCountrySeries(countryKey, periodQuery, controller.signal);
          setDetail((prev) => {
            if (!prev || prev.countryKey !== countryKey) return prev;
            return {
              ...prev,
              sparklineLoading: false,
              sparklineHtml: renderSparklineSVG(data.points || []),
              bucketSec: data.bucket_sec,
            };
          });
        } catch (e) {
          if ((e as { name?: string })?.name === 'AbortError') return;
          setDetail((prev) => {
            if (!prev || prev.countryKey !== countryKey) return prev;
            return {
              ...prev,
              sparklineLoading: false,
              sparklineError: e instanceof Error ? e.message : String(e),
            };
          });
        }
      })();
    },
    [
      points,
      countriesGeoJSON,
      visibleLines,
      periodQuery,
      clearFocusedCountry,
      closeDetail,
      applySearchFilter,
      openLineDetail,
    ],
  );

  // Update deck layers
  useEffect(() => {
    const overlay = overlayRef.current;
    if (!overlay || !mapReady) return;
    if (layersRefreshBusy.current) return;
    layersRefreshBusy.current = true;
    try {
      const result = buildDeckLayers({
        mode: viewMode,
        lines: visibleLines,
        points,
        countriesGeoJSON,
        showHeatmap,
        showCountryLabels,
        monoArcColor: monoArcs,
        repColorArcs,
        groupBy,
        focusedCountry,
        mapTilesFailed,
        heavyCountryLayersAllowed: heavyCountryLayers,
        theme,
        globeView,
        maxArcs,
        onLineClick: openLineDetail,
        onPointClick: openPointDetail,
        onCountryClick: (key, feature) => openCountryDetail(key, feature),
      });
      overlay.setProps({ layers: result.layers as never[] });
      setArcCountInfo({ shown: result.shown, total: result.total });
    } finally {
      layersRefreshBusy.current = false;
    }
  }, [
    mapReady,
    layersTick,
    viewMode,
    visibleLines,
    points,
    countriesGeoJSON,
    showHeatmap,
    showCountryLabels,
    monoArcs,
    repColorArcs,
    groupBy,
    focusedCountry,
    mapTilesFailed,
    heavyCountryLayers,
    theme,
    globeView,
    maxArcs,
    openLineDetail,
    openPointDetail,
    openCountryDetail,
  ]);

  // Close reputation menu on outside click
  useEffect(() => {
    if (!repMenuOpen) return;
    const onDoc = (e: MouseEvent) => {
      const t = e.target as Node;
      const wrap = document.getElementById('reputationFilterWrap');
      if (wrap && wrap.contains(t)) return;
      setRepMenuOpen(false);
    };
    document.addEventListener('click', onDoc);
    return () => document.removeEventListener('click', onDoc);
  }, [repMenuOpen]);

  const adminLinks = filterNav(PAGE_NAV, {
    isAdmin,
    reputationEnabled,
    uiAuthEnabled,
    adminLinksOnly: true,
  });

  function toggleSidebar() {
    setSidebarCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem('nm.mapSidebarCollapsed', next ? '1' : '0');
      } catch {
        /* ignore */
      }
      return next;
    });
  }

  function resetView() {
    const map = mapRef.current;
    if (!map) return;
    if (viewMode === 'globe') {
      const gvs = applyGlobeFitZoom(map, {
        longitude: DEFAULT_GLOBE_VIEW.longitude,
        latitude: DEFAULT_GLOBE_VIEW.latitude,
        bearing: DEFAULT_GLOBE_VIEW.bearing,
      });
      setGlobeView(gvs);
      globeViewRef.current = gvs;
    } else {
      applyMapFitZoom(map, { ...DEFAULT_MAP_VIEW });
    }
    setLayersTick((t) => t + 1);
  }

  async function exportPng() {
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

  async function uploadFile(kind: 'logs' | 'geo', file: File) {
    try {
      const res = await fetch(kind === 'logs' ? '/upload-logs' : '/upload-geo', {
        method: 'POST',
        credentials: 'same-origin',
        headers: authHeaders({
          'Content-Type': kind === 'logs' ? 'text/plain' : 'text/csv',
        }),
        body: file,
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      toast(kind === 'logs' ? 'Логи загружены' : 'GeoIP загружен', 'success');
      if (kind === 'logs') void fetchData();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка загрузки', 'error');
    }
  }

  const emptyOverlay = useMemo(() => {
    if (loading) return null;
    if (fetchError) {
      return { title: 'Ошибка загрузки', text: fetchError };
    }
    if (!lines.length) {
      return {
        title: 'Нет событий за период',
        text: 'Попробуйте расширить период, уменьшить порог minCount или проверить ingest.',
      };
    }
    if (!visibleLines.length) {
      const hints: string[] = [];
      if (filter !== 'all') hints.push(`фильтр «${filter}»`);
      if (search) hints.push(`поиск «${search}»`);
      if (minCount > 1) hints.push(`порог ≥ ${minCount} соб.`);
      if (repActive) hints.push('репутация');
      return {
        title: 'Ничего не отображается',
        text: hints.length
          ? `Активные фильтры скрыли все связи: ${hints.join(', ')}.`
          : 'Все связи отфильтрованы текущими настройками.',
      };
    }
    if (compiled.mode === 'error') {
      return { title: 'Ошибка поиска', text: compiled.error || '' };
    }
    return null;
  }, [
    loading,
    fetchError,
    lines.length,
    visibleLines.length,
    filter,
    search,
    minCount,
    repActive,
    compiled,
  ]);

  const ipMode = groupBy === 'ip';
  const repFilterCount = reputationFilterActiveCount(repCategories, repLists);
  const repTree = useMemo(() => collectReputationMenuTree(lines), [lines]);
  const truncHint =
    arcCountInfo.total > arcCountInfo.shown
      ? `Показано ${fmtNumber(arcCountInfo.shown)} из ${fmtNumber(arcCountInfo.total)} связей — увеличьте лимит дуг или сузьте период`
      : '';

  return (
    <div className={`app${sidebarCollapsed ? ' sidebar-collapsed' : ''}`} id="app">
      <a className="skip-link" href="#map-main">
        К содержимому
      </a>
      <aside className="sidebar" aria-label="Навигация">
        <div className="sidebar-header">
          <img className="logo" src="/logo.png" alt="" width={28} height={28} />
          <div className="title">ГеоАтлас</div>
        </div>

        <div className="sidebar-section collapse-hide">
          <div className="sidebar-section-title">Режим</div>
          <div className="mode-switch">
            <button
              type="button"
              id="mode-map"
              className={viewMode === 'map' ? 'active' : ''}
              onClick={() => setViewMode('map')}
            >
              🗺 Map
            </button>
            <button
              type="button"
              id="mode-globe"
              className={viewMode === 'globe' ? 'active' : ''}
              onClick={() => setViewMode('globe')}
            >
              🌐 Globe
            </button>
          </div>
        </div>

        <div className="sidebar-section">
          <div className="mode-switch-icons">
            <button
              type="button"
              id="mode-map-icon"
              className={viewMode === 'map' ? 'active' : ''}
              title="Map (2D)"
              onClick={() => setViewMode('map')}
            >
              <Icon paths={['M1 6v15l7-3 8 3 7-3V3l-7 3-8-3-7 3z', 'M8 3v15M16 6v15']} />
            </button>
            <button
              type="button"
              id="mode-globe-icon"
              className={viewMode === 'globe' ? 'active' : ''}
              title="Globe (3D)"
              onClick={() => setViewMode('globe')}
            >
              <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <path d="M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20" />
              </svg>
            </button>
          </div>
        </div>

        {isAdmin ? (
          <div className="sidebar-section">
            <div className="sidebar-section-title">Загрузка</div>
            <input
              ref={logFileRef}
              type="file"
              accept=".log,.txt,.cef,.syslog"
              style={{ display: 'none' }}
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) void uploadFile('logs', f);
                e.target.value = '';
              }}
            />
            <input
              ref={geoFileRef}
              type="file"
              accept=".csv"
              style={{ display: 'none' }}
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) void uploadFile('geo', f);
                e.target.value = '';
              }}
            />
            <button
              type="button"
              className="side-btn"
              title="Загрузить логи"
              onClick={() => logFileRef.current?.click()}
            >
              <Icon d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12" />
              <span className="label">Загрузить логи</span>
            </button>
            <button
              type="button"
              className="side-btn"
              title="Обновить GeoIP"
              onClick={() => geoFileRef.current?.click()}
            >
              <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <path d="M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20" />
              </svg>
              <span className="label">Обновить GeoIP</span>
            </button>
          </div>
        ) : null}

        <div className="sidebar-section">
          <div className="sidebar-section-title">Вид</div>
          <button type="button" className="side-btn" title="Обновить" onClick={() => void fetchData()}>
            <Icon paths={['M23 4v6h-6M1 20v-6h6', 'M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15']} />
            <span className="label">Обновить</span>
          </button>
          <button type="button" className="side-btn" title="Сбросить вид" onClick={resetView}>
            <Icon paths={['M3 12a9 9 0 1 0 3-6.7L3 8', 'M3 3v5h5']} />
            <span className="label">Сбросить вид</span>
          </button>
          <button type="button" className="side-btn" title="Экспорт PNG" onClick={() => void exportPng()}>
            <Icon paths={['M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3']} />
            <span className="label">Экспорт PNG</span>
          </button>
        </div>

        {isAdmin ? (
          <div className="sidebar-section" id="adminNavSection">
            <div className="sidebar-section-title">Администрирование</div>
            {adminLinks.map((item) => {
              const active = isNavActive(item, location.pathname);
              return (
                <Link
                  key={item.href}
                  to={item.href}
                  className={`side-btn${active ? ' active' : ''}`}
                  aria-current={active ? 'page' : undefined}
                  title={item.label}
                >
                  <NavIcon kind={NAV_ICONS[item.href] || 'map'} />
                  <span className="label">{item.label}</span>
                </Link>
              );
            })}
          </div>
        ) : null}

        <div className="sidebar-section collapse-hide">
          <div className="sidebar-section-title">Порог событий на связь</div>
          <input
            type="range"
            className="side-range"
            min={1}
            max={50}
            value={minCount}
            onChange={(e) => setMinCount(Number(e.target.value))}
          />
          <div className="side-range-label">
            <span>
              от <b>{minCount}</b> соб.
            </span>
            <span id="arcCountInfo" style={{ color: arcCountInfo.total > arcCountInfo.shown ? 'var(--orange)' : 'var(--text-muted)' }}>
              {arcCountInfo.total > arcCountInfo.shown
                ? `${fmtNumber(arcCountInfo.shown)} из ${fmtNumber(arcCountInfo.total)}`
                : `${fmtNumber(arcCountInfo.shown)} связей`}
            </span>
          </div>
        </div>

        <div className="sidebar-section collapse-hide">
          <div className="sidebar-section-title">Лимит дуг</div>
          <input
            type="range"
            className="side-range"
            min={100}
            max={20000}
            step={100}
            value={maxArcs}
            onChange={(e) => setMaxArcs(Number(e.target.value))}
          />
          <div className="side-range-label">
            <span>
              до <b>{fmtNumber(maxArcs)}</b> дуг
            </span>
          </div>
        </div>

        <div className="sidebar-section collapse-hide">
          <div className="sidebar-section-title">Отображение</div>
          <label className="side-toggle">
            <input type="checkbox" checked={showLegend} onChange={(e) => setShowLegend(e.target.checked)} />
            <span>Легенда</span>
          </label>
          <label className="side-toggle">
            <input type="checkbox" checked={showStats} onChange={(e) => setShowStats(e.target.checked)} />
            <span>Статистика</span>
          </label>
          <label
            className="side-toggle"
            title="2D: заливка стран (на глобусе heatmap отключён)"
            style={{ display: viewMode === 'globe' ? 'none' : undefined }}
          >
            <input
              type="checkbox"
              id="toggleHeatmapChk"
              checked={showHeatmap}
              onChange={(e) => setShowHeatmap(e.target.checked)}
            />
            <span>Heatmap стран</span>
          </label>
          <label className="side-toggle">
            <input
              type="checkbox"
              id="toggleCountryLabelsChk"
              checked={showCountryLabels}
              onChange={(e) => setShowCountryLabels(e.target.checked)}
            />
            <span>Названия стран</span>
          </label>
          <label className="side-toggle" title="Все дуги одним цветом">
            <input type="checkbox" checked={monoArcs} onChange={(e) => setMonoArcs(e.target.checked)} />
            <span>Один цвет линий</span>
          </label>
          <label className="side-toggle">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            <span>Авто-обновление</span>
          </label>
          <label
            className="side-toggle"
            id="autoRotateWrap"
            style={{ display: viewMode === 'globe' ? undefined : 'none' }}
          >
            <input
              type="checkbox"
              id="autoRotate"
              checked={autoRotate}
              onChange={(e) => setAutoRotate(e.target.checked)}
            />
            <span>Авто-вращение глобуса</span>
          </label>
        </div>

        <div className="sidebar-collapse-btn">
          <button
            type="button"
            className="side-btn"
            id="btnToggleSidebar"
            title="Развернуть / свернуть меню"
            onClick={toggleSidebar}
          >
            <svg
              className="icon"
              id="collapseIcon"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <path d="M15 18l-6-6 6-6" />
            </svg>
            <span className="label">Свернуть меню</span>
          </button>
        </div>
      </aside>

      <main className="main" id="map-main">
        <header className="topbar">
          <div className="search-box">
            <svg className="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="11" cy="11" r="8" />
              <path d="M21 21l-4.35-4.35" />
            </svg>
            <input
              type="text"
              id="searchInput"
              placeholder="Поиск: IP, страна, city:Москва, rule:block…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <button
              type="button"
              id="btnSearchBuilder"
              className={`search-builder-toggle${builderOpen ? ' active' : ''}`}
              aria-label="Открыть расширенный поиск"
              aria-expanded={builderOpen}
              aria-controls="searchBuilderPanel"
              title="Расширенный поиск"
              onClick={() => setBuilderOpen((v) => !v)}
            >
              <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M3 5h18" />
                <path d="M6 12h12" />
                <path d="M10 19h4" />
              </svg>
            </button>
            <SearchBuilder
              open={builderOpen}
              onOpenChange={setBuilderOpen}
              search={search}
              onApply={setSearch}
            />
          </div>

          <div className="group-control">
            <span>Группа:</span>
            <select id="groupBy" value={groupBy} onChange={(e) => setGroupBy(e.target.value)}>
              <option value="ip">IP</option>
              <option value="subnet">/24</option>
              <option value="city">Город</option>
              <option value="country">Страна</option>
            </select>
          </div>

          <div className="filter-tabs">
            <button
              type="button"
              className={filter === 'all' ? 'active' : ''}
              onClick={() => setFilter('all')}
            >
              Все
            </button>
            <button
              type="button"
              className={`allowed${filter === 'allowed' ? ' active' : ''}`}
              onClick={() => setFilter('allowed')}
            >
              Разрешённые
            </button>
            <button
              type="button"
              className={`blocked${filter === 'blocked' ? ' active' : ''}`}
              onClick={() => setFilter('blocked')}
            >
              Заблокированные
            </button>
          </div>

          {reputationEnabled ? (
            <div className="reputation-filter" id="reputationFilterWrap" data-reputation-only>
              <button
                type="button"
                id="btnReputationFilter"
                className={`rep-filter-btn${(repFilterCount > 0 || repColorArcs) && ipMode ? ' active' : ''}`}
                title={
                  ipMode
                    ? 'Фильтр и подсветка по репутационным спискам'
                    : 'Доступно в режиме Группа: IP'
                }
                disabled={!ipMode}
                onClick={(e) => {
                  e.stopPropagation();
                  if (!ipMode) return;
                  setRepMenuOpen((v) => !v);
                }}
              >
                Репутация
                <span
                  id="repFilterBadge"
                  className="rep-badge"
                  style={{
                    display: repFilterCount > 0 && ipMode ? 'inline-flex' : 'none',
                  }}
                >
                  {repFilterCount}
                </span>
              </button>
              <div
                className={`reputation-menu${repMenuOpen ? ' open' : ''}`}
                id="reputationMenu"
                role="dialog"
                aria-label="Фильтр репутации"
              >
                <div className="rep-menu-head">
                  <span>Репутация</span>
                  <button
                    type="button"
                    className="rep-clear"
                    id="btnRepFilterClear"
                    onClick={() => {
                      setRepCategories(new Set());
                      setRepLists(new Set());
                      setRepSide('any');
                    }}
                  >
                    Сбросить
                  </button>
                </div>
                <div className="rep-menu-side">
                  <label htmlFor="repFilterSide">Сторона</label>
                  <select
                    id="repFilterSide"
                    value={repSide}
                    onChange={(e) => setRepSide(e.target.value as RepFilterSide)}
                  >
                    <option value="any">src или dst</option>
                    <option value="src">только src</option>
                    <option value="dst">только dst</option>
                    <option value="both">оба конца</option>
                  </select>
                </div>
                <label className="rep-color-toggle">
                  <input
                    type="checkbox"
                    id="repColorArcsChk"
                    checked={repColorArcs}
                    onChange={(e) => setRepColorArcs(e.target.checked)}
                  />
                  Окрашивать дуги с хитом
                </label>
                <p className="rep-menu-hint">
                  Частные и спец. сети (RFC1918, CGNAT, loopback) не учитываются. В деталях дуги
                  смотрите «диапазон».
                </p>
                <div className="rep-menu-body" id="reputationMenuBody">
                  {!ipMode ? (
                    <div className="rep-menu-empty">Переключите «Группа» на IP</div>
                  ) : Object.keys(repTree).length === 0 ? (
                    <div className="rep-menu-empty">Нет совпадений на карте</div>
                  ) : (
                    Object.keys(repTree)
                      .sort()
                      .map((cat) => (
                        <div key={cat}>
                          <label className="rep-cat">
                            <input
                              type="checkbox"
                              checked={repCategories.has(cat)}
                              onChange={(e) => {
                                setRepCategories((prev) => {
                                  const next = new Set(prev);
                                  if (e.target.checked) next.add(cat);
                                  else next.delete(cat);
                                  return next;
                                });
                              }}
                            />{' '}
                            <strong>{categoryLabel(cat)}</strong>{' '}
                            <span className="rep-cat-key">({cat})</span>
                          </label>
                          {Array.from(repTree[cat])
                            .sort()
                            .map((list) => (
                              <label className="rep-list" key={list}>
                                <input
                                  type="checkbox"
                                  checked={repLists.has(list)}
                                  onChange={(e) => {
                                    setRepLists((prev) => {
                                      const next = new Set(prev);
                                      if (e.target.checked) next.add(list);
                                      else next.delete(list);
                                      return next;
                                    });
                                  }}
                                />{' '}
                                {list}
                              </label>
                            ))}
                        </div>
                      ))
                  )}
                </div>
              </div>
            </div>
          ) : null}

          <div className="period-control">
            <span>Период:</span>
            <select
              id="periodPreset"
              title="Период данных"
              value={period}
              onChange={(e) => setPeriod(e.target.value)}
            >
              {PERIODS.map(([v, label]) => (
                <option key={v} value={v}>
                  {label}
                </option>
              ))}
            </select>
            <div className={`period-custom${period === 'custom' ? ' visible' : ''}`}>
              <label htmlFor="periodFrom">От</label>
              <input
                type="datetime-local"
                id="periodFrom"
                value={periodFrom}
                onChange={(e) => setPeriodFrom(e.target.value)}
              />
              <label htmlFor="periodTo">До</label>
              <input
                type="datetime-local"
                id="periodTo"
                value={periodTo}
                onChange={(e) => setPeriodTo(e.target.value)}
              />
              <button type="button" className="period-apply-btn" onClick={() => void fetchData()}>
                Применить
              </button>
            </div>
          </div>

          <div className="topbar-spacer" />
          <div id="userBarHost">
            <UserMenu />
          </div>
          <SystemHealthPill />
        </header>

        <div className={`viz-area${loading ? ' is-loading' : ''}`}>
          <div ref={mapContainer} id="map-host" className="viz-host" />

          {truncHint ? (
            <div className="viz-hint warn" id="arcsTruncHint">
              {truncHint}
            </div>
          ) : null}

          <div className={`viz-overlay${emptyOverlay ? ' visible' : ''}`} id="vizOverlay">
            <div className="viz-overlay-card">
              <h4 id="vizOverlayTitle">{emptyOverlay?.title || 'Нет данных'}</h4>
              <p id="vizOverlayText">{emptyOverlay?.text || ''}</p>
            </div>
          </div>

          <div
            className={`map-loading${loading ? ' visible' : ''}`}
            id="mapLoading"
            aria-live="polite"
            aria-busy={loading}
          >
            <div className="map-loading-spinner" aria-hidden="true" />
            <span>Загрузка данных…</span>
          </div>

          {showLegend ? (
            <div className="legend" id="legendBox">
              <div className="legend-title" id="legendTitle">
                Статус трафика
              </div>
              {monoArcs ? (
                <div id="legendMono" className="legend-row">
                  <span className="legend-line" style={{ background: 'var(--accent)' }} /> Связь
                </div>
              ) : (
                <div id="legendByStatus">
                  <div className="legend-row">
                    <span className="legend-line" style={{ background: 'var(--green)' }} /> Разрешённый
                  </div>
                  <div className="legend-row">
                    <span className="legend-line" style={{ background: 'var(--red)' }} /> Заблокированный
                  </div>
                  <div
                    className="legend-row"
                    id="legendRepRow"
                    style={{ display: repColorArcs ? undefined : 'none' }}
                  >
                    <span className="legend-line" style={{ background: 'var(--orange)' }} />{' '}
                    Репутационный хит
                  </div>
                </div>
              )}
              <div
                className="legend-row"
                style={{ marginTop: 6, color: 'var(--text-muted)', fontSize: 11 }}
              >
                Толщина линии / размер точки — кол-во событий
              </div>
            </div>
          ) : null}

          {showStats ? (
            <div className="stats-floating" id="statsFloating">
              <div className="stats-item">
                <span className="lbl">Событий</span>
                <span className="val" id="stat-total">
                  {fmtNumber(stats.events)}
                </span>
              </div>
              <div className="stats-item">
                <span className="lbl">Allowed</span>
                <span className="val green" id="stat-allowed">
                  {fmtNumber(stats.allowed)}
                </span>
              </div>
              <div className="stats-item">
                <span className="lbl">Blocked</span>
                <span className="val red" id="stat-blocked">
                  {fmtNumber(stats.blocked)}
                </span>
              </div>
              <div className="stats-item">
                <span className="lbl">Связей</span>
                <span className="val" id="stat-edges">
                  {fmtNumber(stats.connections)}
                </span>
              </div>
              <div className="stats-item">
                <span className="lbl">Узлов</span>
                <span className="val" id="stat-nodes">
                  {fmtNumber(stats.nodes)}
                </span>
              </div>
              <div className="stats-item">
                <span className="lbl">Стран</span>
                <span className="val" id="stat-countries">
                  {fmtNumber(stats.countries)}
                </span>
              </div>
              <div className="stats-item">
                <span className="lbl">Городов</span>
                <span className="val" id="stat-cities">
                  {fmtNumber(stats.cities)}
                </span>
              </div>
            </div>
          ) : null}

          <MapDetailPanel
            detail={detail}
            onClose={closeDetail}
          />
        </div>
      </main>
    </div>
  );
}
