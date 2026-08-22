import type { RefObject } from 'react';
import { useAuth } from '@/auth/AuthContext';
import { filterNav, PAGE_NAV } from '@/components/nav';
import { NavSections } from '@/components/NavSections';
import { SidebarCollapseButton, SidebarShell } from '@/components/SidebarShell';
import { useNavBadges } from '@/components/useNavBadges';
import { Icon } from './mapIcons';

export type MapSidebarProps = {
  view: {
    viewMode: 'map' | 'globe';
    setViewMode: (mode: 'map' | 'globe') => void;
  };
  isAdmin: boolean;
  uploads: {
    logFileRef: RefObject<HTMLInputElement | null>;
    geoFileRef: RefObject<HTMLInputElement | null>;
    uploadFile: (kind: 'logs' | 'geo', file: File) => void | Promise<void>;
  };
  geoWizard?: {
    open: () => void;
    empty: boolean | null;
  };
  actions: {
    fetchData: () => void | Promise<void>;
    resetView: () => void;
    exportPng: () => void | Promise<void>;
    toggleSidebar: () => void;
  };
};

export function MapSidebar({
  view,
  isAdmin,
  uploads,
  geoWizard,
  actions,
}: MapSidebarProps) {
  const { viewMode, setViewMode } = view;
  const { logFileRef, geoFileRef, uploadFile } = uploads;
  const { fetchData, resetView, exportPng, toggleSidebar } = actions;
  const { reputationEnabled, uiAuthEnabled } = useAuth();
  const badges = useNavBadges();
  const navItems = filterNav(PAGE_NAV, {
    isAdmin,
    reputationEnabled,
    uiAuthEnabled,
  });

  return (
    <SidebarShell>
      <NavSections
        items={navItems}
        badges={badges}
        middle={
          <>
        <div className="sidebar-section collapse-hide">
          <div className="sidebar-section-title">Режим</div>
          <div className="mode-switch">
            <button
              type="button"
              className={viewMode === 'map' ? 'active' : ''}
              onClick={() => setViewMode('map')}
            >
              Карта
            </button>
            <button
              type="button"
              className={viewMode === 'globe' ? 'active' : ''}
              onClick={() => setViewMode('globe')}
            >
              Глобус
            </button>
          </div>
        </div>

        <div className="sidebar-section">
          <div className="mode-switch-icons">
            <button
              type="button"
              className={viewMode === 'map' ? 'active' : ''}
              title="Карта (2D)"
              onClick={() => setViewMode('map')}
            >
              <Icon paths={['M1 6v15l7-3 8 3 7-3V3l-7 3-8-3-7 3z', 'M8 3v15M16 6v15']} />
            </button>
            <button
              type="button"
              className={viewMode === 'globe' ? 'active' : ''}
              title="Глобус (3D)"
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
            {geoWizard?.empty === true ? (
              <button
                type="button"
                className="side-btn"
                title="Мастер первой загрузки GeoIP"
                onClick={geoWizard.open}
              >
                <Icon d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                <span className="label">Мастер GeoIP</span>
              </button>
            ) : (
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
            )}
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
          </>
        }
      />

      <SidebarCollapseButton onToggle={toggleSidebar} />
    </SidebarShell>
  );
}
