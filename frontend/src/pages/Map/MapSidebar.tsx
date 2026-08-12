import type { RefObject } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { NavIcon, NAV_ICONS } from '@/components/Shell';
import { isNavActive, type NavItem } from '@/components/nav';
import { fmtNumber } from '@/lib/format';
import { Icon } from './mapIcons';

export type MapSidebarProps = {
  view: {
    viewMode: 'map' | 'globe';
    setViewMode: (mode: 'map' | 'globe') => void;
    autoRotate: boolean;
    setAutoRotate: (v: boolean) => void;
  };
  isAdmin: boolean;
  uploads: {
    logFileRef: RefObject<HTMLInputElement | null>;
    geoFileRef: RefObject<HTMLInputElement | null>;
    uploadFile: (kind: 'logs' | 'geo', file: File) => void | Promise<void>;
  };
  geoWizard?: {
    open: () => void;
    empty: boolean;
  };
  actions: {
    fetchData: () => void | Promise<void>;
    resetView: () => void;
    exportPng: () => void | Promise<void>;
    toggleSidebar: () => void;
  };
  adminLinks: NavItem[];
  viz: {
    minCount: number;
    setMinCount: (n: number) => void;
    arcCountInfo: { shown: number; total: number };
    maxArcs: number;
    setMaxArcs: (n: number) => void;
    showLegend: boolean;
    setShowLegend: (v: boolean) => void;
    showStats: boolean;
    setShowStats: (v: boolean) => void;
    showHeatmap: boolean;
    setShowHeatmap: (v: boolean) => void;
    showCountryLabels: boolean;
    setShowCountryLabels: (v: boolean) => void;
    monoArcs: boolean;
    setMonoArcs: (v: boolean) => void;
  };
  data: {
    autoRefresh: boolean;
    setAutoRefresh: (v: boolean) => void;
    dataSource: 'live' | 'backup';
    selectDataSource: (v: 'live' | 'backup') => void;
    backupAttached: string;
  };
};

export function MapSidebar({
  view,
  isAdmin,
  uploads,
  geoWizard,
  actions,
  adminLinks,
  viz,
  data,
}: MapSidebarProps) {
  const { viewMode, setViewMode, autoRotate, setAutoRotate } = view;
  const { logFileRef, geoFileRef, uploadFile } = uploads;
  const { fetchData, resetView, exportPng, toggleSidebar } = actions;
  const {
    minCount,
    setMinCount,
    arcCountInfo,
    maxArcs,
    setMaxArcs,
    showLegend,
    setShowLegend,
    showStats,
    setShowStats,
    showHeatmap,
    setShowHeatmap,
    showCountryLabels,
    setShowCountryLabels,
    monoArcs,
    setMonoArcs,
  } = viz;
  const { autoRefresh, setAutoRefresh, dataSource, selectDataSource, backupAttached } = data;
  const location = useLocation();

  return (
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
          {geoWizard ? (
            <button
              type="button"
              className="side-btn"
              title="Мастер первой загрузки GeoIP"
              onClick={geoWizard.open}
            >
              <Icon d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
              <span className="label">
                {geoWizard.empty ? 'Мастер GeoIP' : 'Мастер GeoIP'}
              </span>
            </button>
          ) : null}
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
            disabled={dataSource === 'backup'}
            onChange={(e) => setAutoRefresh(e.target.checked)}
          />
          <span>Авто-обновление</span>
        </label>
        <div className="sidebar-section-title" style={{ marginTop: 10 }}>
          Данные
        </div>
        <div className="mode-switch" title={backupAttached ? `Бэкап: ${backupAttached}` : 'Сначала Подключить бэкап в Системе'}>
          <button
            type="button"
            className={dataSource === 'live' ? 'active' : ''}
            onClick={() => selectDataSource('live')}
          >
            Live
          </button>
          <button
            type="button"
            className={dataSource === 'backup' ? 'active' : ''}
            disabled={!backupAttached && dataSource !== 'backup'}
            onClick={() => selectDataSource('backup')}
          >
            Бэкап
          </button>
        </div>
        {backupAttached ? (
          <p className="hint" style={{ marginTop: 6, fontSize: 12 }}>
            Подключён <code>{backupAttached}</code>
          </p>
        ) : (
          <p className="hint" style={{ marginTop: 6, fontSize: 12 }}>
            Нет подключённого бэкапа
          </p>
        )}
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
  );
}
