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
import 'maplibre-gl/dist/maplibre-gl.css';
import '@/styles/index.css';

const MAP_STYLE_DARK = 'https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json';
const MAP_STYLE_LIGHT = 'https://basemaps.cartocdn.com/gl/positron-gl-style/style.json';

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
  last_action?: string;
}

interface EventsPayload {
  points?: Record<string, MapPoint>;
  lines?: MapLine[];
  period?: string;
  source?: string;
}

export default function MapPage() {
  const { isAdmin, reputationEnabled, uiAuthEnabled, theme } = useAuth();
  const { toast } = useToast();
  const mapContainer = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const overlayRef = useRef<MapboxOverlay | null>(null);

  const [period, setPeriod] = useState('1d');
  const [groupBy, setGroupBy] = useState('city');
  const [filter, setFilter] = useState<'all' | 'allowed' | 'blocked'>('all');
  const [search, setSearch] = useState('');
  const [minCount, setMinCount] = useState(1);
  const [points, setPoints] = useState<Record<string, MapPoint>>({});
  const [lines, setLines] = useState<MapLine[]>([]);
  const [meta, setMeta] = useState('');
  const [viewMode, setViewMode] = useState<'map' | 'globe'>('map');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    document.title = 'ГеоАтлас';
    document.body.classList.add('page-map');
    return () => document.body.classList.remove('page-map');
  }, []);

  const compiled = useMemo(() => compileSearchQuery(search), [search]);

  const visibleLines = useMemo(() => {
    return lines.filter((line) => {
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
        country: [line.src_country, line.dst_country, ruCountry(line.src_country), ruCountry(line.dst_country)],
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
  }, [lines, points, minCount, filter, compiled]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const apiLimit = groupBy === 'ip' || groupBy === 'subnet' ? 50000 : 10000;
      const url = `/api/events?group_by=${encodeURIComponent(groupBy)}&limit=${apiLimit}&period=${encodeURIComponent(period)}`;
      const data = await apiFetch<EventsPayload>(url);
      setPoints(data.points || {});
      setLines(data.lines || []);
      setMeta(`${data.source || ''} · ${data.period || period} · линий ${fmtNumber((data.lines || []).length)}`);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка загрузки', 'error');
    } finally {
      setLoading(false);
    }
  }, [groupBy, period, toast]);

  useEffect(() => {
    void fetchData();
    const id = window.setInterval(() => void fetchData(), 30000);
    return () => window.clearInterval(id);
  }, [fetchData]);

  useEffect(() => {
    if (!mapContainer.current || mapRef.current) return;
    const map = new maplibregl.Map({
      container: mapContainer.current,
      style: theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK,
      center: [20, 18],
      zoom: 1.8,
      attributionControl: true,
    });
    map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'top-right');
    const overlay = new MapboxOverlay({ interleaved: true, layers: [] });
    map.addControl(overlay as unknown as maplibregl.IControl);
    mapRef.current = map;
    overlayRef.current = overlay;
    return () => {
      map.remove();
      mapRef.current = null;
      overlayRef.current = null;
    };
    // init once
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    const style = theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK;
    map.setStyle(style);
  }, [theme]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    if (viewMode === 'globe' && 'setProjection' in map) {
      try {
        (map as maplibregl.Map & { setProjection: (p: string) => void }).setProjection('globe');
      } catch {
        /* older builds */
      }
    } else if ('setProjection' in map) {
      try {
        (map as maplibregl.Map & { setProjection: (p: string) => void }).setProjection('mercator');
      } catch {
        /* ignore */
      }
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
        return {
          sourcePosition: [s.lon, s.lat] as [number, number],
          targetPosition: [d.lon, d.lat] as [number, number],
          count: Number(line.count) || 1,
          color: blocked ? [239, 68, 68, 180] : [56, 189, 248, 160],
        };
      })
      .filter(Boolean) as {
      sourcePosition: [number, number];
      targetPosition: [number, number];
      count: number;
      color: number[];
    }[];

    const pointEntries = Object.entries(points).filter(([, p]) => p && Number.isFinite(p.lat) && Number.isFinite(p.lon));

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
          getRadius: ([, p]: [string, MapPoint]) => Math.max(40000, Math.log10((p.count || 1) + 1) * 25000),
          getFillColor: [255, 255, 255, 200],
          radiusUnits: 'meters',
          pickable: true,
        }),
      ],
    });
  }, [visibleLines, points]);

  const location = useLocation();
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    try {
      return localStorage.getItem('nm.mapSidebarCollapsed') === '1';
    } catch {
      return false;
    }
  });

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

  return (
    <div className={`app map-app${sidebarCollapsed ? ' sidebar-collapsed' : ''}`} id="app">
      <aside className="sidebar" aria-label="Навигация">
        <div className="sidebar-header">
          <img className="logo" src="/logo.png" alt="" width={28} height={28} />
          <div className="title">ГеоАтлас</div>
        </div>
        <div className="sidebar-section collapse-hide">
          <div className="sidebar-section-title">Карта</div>
          <label className="field">
            Период
            <select value={period} onChange={(e) => setPeriod(e.target.value)}>
              <option value="1h">1ч</option>
              <option value="6h">6ч</option>
              <option value="1d">1д</option>
              <option value="7d">7д</option>
              <option value="30d">30д</option>
            </select>
          </label>
          <label className="field">
            Группировка
            <select id="groupBy" value={groupBy} onChange={(e) => setGroupBy(e.target.value)}>
              <option value="ip">IP</option>
              <option value="subnet">/24</option>
              <option value="city">Город</option>
              <option value="country">Страна</option>
            </select>
          </label>
          <label className="field">
            Фильтр
            <select value={filter} onChange={(e) => setFilter(e.target.value as typeof filter)}>
              <option value="all">Все</option>
              <option value="allowed">Разрешённые</option>
              <option value="blocked">Заблокированные</option>
            </select>
          </label>
          <label className="field">
            Поиск
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="country:Россия"
            />
          </label>
          {compiled.mode === 'error' ? <div className="hint error">{compiled.error}</div> : null}
          <label className="field">
            Мин. событий: {minCount}
            <input
              type="range"
              min={1}
              max={100}
              value={minCount}
              onChange={(e) => setMinCount(Number(e.target.value))}
            />
          </label>
          <div className="actions" style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <button type="button" className="btn" onClick={() => setViewMode('map')}>
              2D
            </button>
            <button type="button" className="btn" onClick={() => setViewMode('globe')}>
              3D
            </button>
            <button type="button" className="btn" disabled={loading} onClick={() => void fetchData()}>
              Обновить
            </button>
          </div>
          <div className="hint" style={{ marginTop: 8 }}>
            {meta}
            {loading ? ' · загрузка…' : ''} · видно {fmtNumber(visibleLines.length)}
          </div>
          <div className="actions" style={{ marginTop: 8, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <label className="btn">
              Логи
              <input
                type="file"
                hidden
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  try {
                    const res = await fetch('/upload-logs', {
                      method: 'POST',
                      credentials: 'same-origin',
                      headers: authHeaders({ 'Content-Type': 'text/plain' }),
                      body: file,
                    });
                    if (!res.ok) throw new Error(`HTTP ${res.status}`);
                    toast('Логи загружены', 'success');
                    void fetchData();
                  } catch (err) {
                    toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                  }
                }}
              />
            </label>
            <label className="btn">
              GeoIP
              <input
                type="file"
                accept=".csv,text/csv"
                hidden
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  try {
                    const res = await fetch('/upload-geo', {
                      method: 'POST',
                      credentials: 'same-origin',
                      headers: authHeaders({ 'Content-Type': 'text/csv' }),
                      body: file,
                    });
                    if (!res.ok) throw new Error(`HTTP ${res.status}`);
                    toast('GeoIP загружен', 'success');
                  } catch (err) {
                    toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                  }
                }}
              />
            </label>
          </div>
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
      <div className="main">
        <header className="topbar map-topbar">
          <div className="topbar-spacer" />
          <SystemHealthPill />
          <div id="userBarHost">
            <UserMenu />
          </div>
        </header>
        <main id="map-main" style={{ position: 'relative', minHeight: 0, height: '100%' }}>
          <div ref={mapContainer} style={{ position: 'absolute', inset: 0 }} />
        </main>
      </div>
    </div>
  );
}
