import type { AnomalyEvent, AnomalySummary } from '@/api/anomalies';
import { MapInfoDock, type InfoDockTab } from './MapInfoDock';

export function MapVizOverlays({
  emptyOverlay,
  loading,
  infoDock,
  monoArcs,
  repColorArcs,
  stats,
}: {
  emptyOverlay: { title: string; text: string } | null;
  loading: boolean;
  infoDock: {
    open: boolean;
    tab: InfoDockTab;
    onTabChange: (tab: InfoDockTab) => void;
    onClose: () => void;
    showLegendTab: boolean;
    showStatsTab: boolean;
    summary: AnomalySummary | null;
    anomalyItems: AnomalyEvent[];
    onAnomalyShow: (item: AnomalyEvent) => void;
    onAnomalyAck: (fingerprint: string) => void;
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
        <div className="viz-overlay-card">
          <h4>{emptyOverlay?.title || 'Нет данных'}</h4>
          <p>{emptyOverlay?.text || ''}</p>
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

      {infoDock.open ? (
        <div className="map-end-dock">
          <MapInfoDock
            open={infoDock.open}
            tab={infoDock.tab}
            onTabChange={infoDock.onTabChange}
            onClose={infoDock.onClose}
            showLegendTab={infoDock.showLegendTab}
            showStatsTab={infoDock.showStatsTab}
            summary={infoDock.summary}
            monoArcs={monoArcs}
            repColorArcs={repColorArcs}
            stats={stats}
            anomalyItems={infoDock.anomalyItems}
            onAnomalyShow={infoDock.onAnomalyShow}
            onAnomalyAck={infoDock.onAnomalyAck}
          />
        </div>
      ) : null}
    </>
  );
}
