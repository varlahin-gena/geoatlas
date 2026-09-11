import type { EmptyMapActionKind, EmptyMapOverlay } from './geoWizard';
import { MapInfoDock, type InfoDockTab } from './MapInfoDock';

export function MapVizOverlays({
  emptyOverlay,
  onEmptyAction,
  loading,
  infoDock,
  monoArcs,
  repColorArcs,
  stats,
}: {
  emptyOverlay: EmptyMapOverlay | null;
  onEmptyAction?: (kind: EmptyMapActionKind) => void;
  loading: boolean;
  infoDock: {
    tab: InfoDockTab;
    onTabChange: (tab: InfoDockTab) => void;
    showLegendTab: boolean;
    showStatsTab: boolean;
  };
  monoArcs: boolean;
  repColorArcs: boolean;
  stats: {
    events: number;
    allowed: number;
    blocked: number;
    connections: number;
    nodes: number;
    countries: number;
    cities: number;
  };
}) {
  return (
    <>
      <div className={`viz-overlay${emptyOverlay ? ' visible' : ''}`}>
        <div className={`viz-overlay-card${emptyOverlay?.action ? ' has-action' : ''}`}>
          <h4>{emptyOverlay?.title || 'Нет данных'}</h4>
          <p>{emptyOverlay?.text || ''}</p>
          {emptyOverlay?.action && onEmptyAction ? (
            <button
              type="button"
              className="btn primary viz-overlay-cta"
              onClick={() => onEmptyAction(emptyOverlay.action!.kind)}
            >
              {emptyOverlay.action.label}
            </button>
          ) : null}
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

      {infoDock.showLegendTab || infoDock.showStatsTab ? (
        <div className="map-end-dock">
          <MapInfoDock
            tab={infoDock.tab}
            onTabChange={infoDock.onTabChange}
            showLegendTab={infoDock.showLegendTab}
            showStatsTab={infoDock.showStatsTab}
            monoArcs={monoArcs}
            repColorArcs={repColorArcs}
            stats={stats}
          />
        </div>
      ) : null}
    </>
  );
}
