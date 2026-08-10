import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import maplibregl from 'maplibre-gl';
import { MapboxOverlay } from '@deck.gl/mapbox';
import { ArcLayer, ScatterplotLayer } from '@deck.gl/layers';
import { Link, useLocation } from 'react-router-dom';
import { apiFetch, authHeaders } from '@/api/client';
import { useAuth } from '@/auth/AuthContext';
import { SystemHealthPill, UserMenu } from '@/components/Shell';
import { filterNav, isNavActive, PAGE_NAV } from '@/components/nav';
import { useToast } from '@/components/Toast';
import { compileSearchQuery, evaluateSearchAst, ruCountry } from '@/lib/search';
import { fmtNumber } from '@/lib/format';
import { SearchBuilder } from './SearchBuilder';
import 'maplibre-gl/dist/maplibre-gl.css';
import '@/styles/index.css';

const MAP_STYLE_DARK = 'https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json';
const MAP_STYLE_LIGHT = 'https://basemaps.cartocdn.com/gl/positron-gl-style/style.json';
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

interface MapPoint {
  lat: number;
  lon: number;
  country?: string;
  city?: string;
  label?: string;
  count?: number;
}

interface MapLine {
  src: string;
  dst: string;
  count?: number;
  allowed?: number;
  blocked?: number;
  src_country?: string;
  dst_country?: string;
  rule?: string;
  device?: string;
  proto?: string;
}

interface EventsPayload {
  points?: Record<string, MapPoint>;
  lines?: MapLine[];
  period?: string;
  source?: string;
}

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

export default function MapPage() {
  const { isAdmin, reputationEnabled, uiAuthEnabled, theme } = useAuth();
  const { toast } = useToast();
  const location = useLocation();
  const mapContainer = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);
  const logFileRef = useRef<HTMLInputElement>(null);
  const geoFileRef = useRef<HTMLInputElement>(null);

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
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [showLegend, setShowLegend] = useState(true);
  const [showStats, setShowStats] = useState(true);
  const [monoArcs, setMonoArcs] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    try {
      return localStorage.getItem('nm.mapSidebarCollapsed') === '1';
    } catch {
      return false;
    }
  });

  useEffect(() => {
    document.title = 'ГеоАтлас — SOC';
    document.body.classList.add('page-map');
    return () => document.body.classList.remove('page-map');
  }, []);

  const compiled = useMemo(() => compileSearchQuery(search), [search]);

  const visibleLines = useMemo(() => {
    const filtered = lines.filter((line) => {
      const total = Number(line.count) || 0;
      if (total < minCount) return false;
      if (filter === 'allowed' && !(Number(line.allowed) > 0)) return false;
      if (filter === 'blocked' && !(Number(line.blocked) > 0)) return false;
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
          ruCountry(line.src_country),
          ruCountry(line.dst_country),
          srcP?.city,
          dstP?.city,
        ],
        ip: [line.src, line.dst],
        country: [
          line.src_country,
          line.dst_country,
          ruCountry(line.src_country),
          ruCountry(line.dst_country),
        ],
        city: [srcP?.city, dstP?.city],
        rule: [line.rule],
        device: [line.device],
        src: [line.src, line.src_country],
        dst: [line.dst, line.dst_country],
        proto: [line.proto],
        zone: [],
      };
      return evaluateSearchAst(compiled.ast, fieldValues);
    });
    return filtered
      .slice()
      .sort((a, b) => (Number(b.count) || 0) - (Number(a.count) || 0))
      .slice(0, maxArcs);
  }, [lines, points, minCount, filter, compiled, maxArcs]);

  const stats = useMemo(() => {
    let events = 0;
    let allowed = 0;
    let blocked = 0;
    const nodeSet = new Set<string>();
    const countrySet = new Set<string>();
    const citySet = new Set<string>();
    for (const line of visibleLines) {
      events += Number(line.count) || 0;
      allowed += Number(line.allowed) || 0;
      blocked += Number(line.blocked) || 0;
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

  const periodQuery = useMemo(() => {
    if (period === 'custom' && periodFrom && periodTo) {
      return `&from=${encodeURIComponent(periodFrom)}&to=${encodeURIComponent(periodTo)}`;
    }
    return `&period=${encodeURIComponent(period)}`;
  }, [period, periodFrom, periodTo]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const apiLimit = groupBy === 'ip' || groupBy === 'subnet' ? 50000 : 10000;
      const url = `/api/events?group_by=${encodeURIComponent(groupBy)}&limit=${apiLimit}${periodQuery}`;
      const data = await apiFetch<EventsPayload>(url);
      setPoints(data.points || {});
      setLines(data.lines || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка загрузки', 'error');
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

  useEffect(() => {
    if (!mapContainer.current || mapRef.current) return;
    const map = new maplibregl.Map({
      container: mapContainer.current,
      style: theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK,
      center: [20, 18],
      zoom: 1.8,
      attributionControl: true,
    });
    map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'bottom-right');
    const overlay = new MapboxOverlay({ interleaved: true, layers: [] });
    map.addControl(overlay as unknown as maplibregl.IControl);
    mapRef.current = map;
    overlayRef.current = overlay;
    return () => {
      map.remove();
      mapRef.current = null;
      overlayRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    map.setStyle(theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK);
  }, [theme]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !('setProjection' in map)) return;
    try {
      (map as maplibregl.Map & { setProjection: (p: string) => void }).setProjection(
        viewMode === 'globe' ? 'globe' : 'mercator',
      );
    } catch {
      /* ignore */
    }
  }, [viewMode]);

  useEffect(() => {
    const overlay = overlayRef.current;
    if (!overlay) return;
    const arcs = visibleLines
      .map((line) => {
        const s = points[line.src];
        const d = points[line.dst];
        if (!s || !d) return null;
        const blocked = Number(line.blocked) > 0;
        const color = monoArcs
          ? [56, 189, 248, 160]
          : blocked
            ? [239, 68, 68, 180]
            : [56, 189, 248, 160];
        return {
          sourcePosition: [s.lon, s.lat] as [number, number],
          targetPosition: [d.lon, d.lat] as [number, number],
          count: Number(line.count) || 1,
          color,
        };
      })
      .filter(Boolean) as {
      sourcePosition: [number, number];
      targetPosition: [number, number];
      count: number;
      color: number[];
    }[];

    const pointEntries = Object.entries(points).filter(
      ([, p]) => p && Number.isFinite(p.lat) && Number.isFinite(p.lon),
    );

    overlay.setProps({
      layers: [
        new ArcLayer({
          id: 'arcs',
          data: arcs,
          getSourcePosition: (d: (typeof arcs)[0]) => d.sourcePosition,
          getTargetPosition: (d: (typeof arcs)[0]) => d.targetPosition,
          getSourceColor: (d: (typeof arcs)[0]) => d.color,
          getTargetColor: (d: (typeof arcs)[0]) => d.color,
          getWidth: (d: (typeof arcs)[0]) => Math.max(1, Math.log10(d.count + 1) * 2),
          pickable: true,
        }),
        new ScatterplotLayer({
          id: 'nodes',
          data: pointEntries,
          getPosition: ([, p]: [string, MapPoint]) => [p.lon, p.lat],
          getRadius: ([, p]: [string, MapPoint]) =>
            Math.max(40000, Math.log10((p.count || 1) + 1) * 25000),
          getFillColor: [255, 255, 255, 200],
          radiusUnits: 'meters',
          pickable: true,
        }),
      ],
    });
  }, [visibleLines, points, monoArcs]);

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
    mapRef.current?.easeTo({ center: [20, 18], zoom: 1.8, pitch: 0, bearing: 0 });
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

  const empty = !loading && visibleLines.length === 0;

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
              className={viewMode === 'map' ? 'active' : ''}
              onClick={() => setViewMode('map')}
            >
              🗺 Map
            </button>
            <button
              type="button"
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
              className={viewMode === 'map' ? 'active' : ''}
              title="Map (2D)"
              onClick={() => setViewMode('map')}
            >
              <Icon paths={['M1 6v15l7-3 8 3 7-3V3l-7 3-8-3-7 3z', 'M8 3v15M16 6v15']} />
            </button>
            <button
              type="button"
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
            <span>{fmtNumber(visibleLines.length)} дуг</span>
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
            <div className="reputation-filter" title="Фильтр репутации — в разработке SPA">
              <button type="button" className="rep-filter-btn" disabled>
                Репутация
              </button>
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

          <div className={`viz-overlay${empty ? ' visible' : ''}`} id="vizOverlay">
            <div className="viz-overlay-card">
              <h4>Нет событий за период</h4>
              <p>
                {compiled.mode === 'error'
                  ? `Ошибка поиска: ${compiled.error}`
                  : 'Измените период, фильтры или загрузите логи.'}
              </p>
            </div>
          </div>

          <div
            className={`map-loading${loading ? ' visible' : ''}`}
            aria-live="polite"
            aria-busy={loading}
          >
            <div className="map-loading-spinner" aria-hidden="true" />
            <span>Загрузка данных…</span>
          </div>

          {showLegend ? (
            <div className="legend">
              <div className="legend-title">Статус трафика</div>
              <div className="legend-row">
                <span className="legend-line" style={{ background: '#38bdf8' }} />
                <span>Разрешённые</span>
              </div>
              <div className="legend-row">
                <span className="legend-line" style={{ background: '#ef4444' }} />
                <span>Заблокированные</span>
              </div>
            </div>
          ) : null}

          {showStats ? (
            <div className="stats-floating">
              <span>
                События <b>{fmtNumber(stats.events)}</b>
              </span>
              <span>
                Разреш. <b>{fmtNumber(stats.allowed)}</b>
              </span>
              <span>
                Блок. <b>{fmtNumber(stats.blocked)}</b>
              </span>
              <span>
                Связи <b>{fmtNumber(stats.connections)}</b>
              </span>
              <span>
                Узлы <b>{fmtNumber(stats.nodes)}</b>
              </span>
              <span>
                Страны <b>{fmtNumber(stats.countries)}</b>
              </span>
              <span>
                Города <b>{fmtNumber(stats.cities)}</b>
              </span>
            </div>
          ) : null}
        </div>
      </main>
    </div>
  );
}
