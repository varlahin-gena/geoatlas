import type { AnomalyEvent, AnomalySummary } from '@/api/anomalies';
import { fmtNumber } from '@/lib/format';
import { AnomalyPanel } from './AnomalyPanel';

export type InfoDockTab = 'legend' | 'stats' | 'anomalies';

function MapLegend({ monoArcs, repColorArcs }: { monoArcs: boolean; repColorArcs: boolean }) {
  return (
    <div className="legend map-info-dock-legend">
      <div className="legend-title">Статус трафика</div>
      {monoArcs ? (
        <div className="legend-row">
          <span className="legend-line" style={{ background: 'var(--accent)' }} /> Связь
        </div>
      ) : (
        <>
          <div className="legend-row">
            <span className="legend-line" style={{ background: 'var(--green)' }} /> Разрешённый
          </div>
          <div className="legend-row">
            <span className="legend-line" style={{ background: 'var(--red)' }} /> Заблокированный
          </div>
          {repColorArcs ? (
            <div className="legend-row">
              <span className="legend-line" style={{ background: 'var(--orange)' }} /> Репутационный хит
            </div>
          ) : null}
        </>
      )}
      <div className="legend-row" style={{ marginTop: 6, color: 'var(--text-muted)', fontSize: 11 }}>
        Толщина линии / размер точки — кол-во событий
      </div>
    </div>
  );
}

function MapStatsGrid({
  stats,
}: {
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
    <div className="map-info-dock-stats">
      <div className="stats-item">
        <span className="lbl">Событий</span>
        <span className="val">{fmtNumber(stats.events)}</span>
      </div>
      <div className="stats-item">
        <span className="lbl">Разрешено</span>
        <span className="val green">{fmtNumber(stats.allowed)}</span>
      </div>
      <div className="stats-item">
        <span className="lbl">Заблокировано</span>
        <span className="val red">{fmtNumber(stats.blocked)}</span>
      </div>
      <div className="stats-item">
        <span className="lbl">Связей</span>
        <span className="val">{fmtNumber(stats.connections)}</span>
      </div>
      <div className="stats-item">
        <span className="lbl">Узлов</span>
        <span className="val">{fmtNumber(stats.nodes)}</span>
      </div>
      <div className="stats-item">
        <span className="lbl">Стран</span>
        <span className="val">{fmtNumber(stats.countries)}</span>
      </div>
      <div className="stats-item">
        <span className="lbl">Городов</span>
        <span className="val">{fmtNumber(stats.cities)}</span>
      </div>
    </div>
  );
}

export function MapInfoDock({
  open,
  tab,
  onTabChange,
  onClose,
  showLegendTab,
  showStatsTab,
  summary,
  monoArcs,
  repColorArcs,
  stats,
  anomalyItems,
  onAnomalyShow,
  onAnomalyAck,
}: {
  open: boolean;
  tab: InfoDockTab;
  onTabChange: (tab: InfoDockTab) => void;
  onClose: () => void;
  showLegendTab: boolean;
  showStatsTab: boolean;
  summary: AnomalySummary | null;
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
  anomalyItems: AnomalyEvent[];
  onAnomalyShow: (item: AnomalyEvent) => void;
  onAnomalyAck: (fingerprint: string) => void;
}) {
  if (!open) return null;

  const anomalyTotal = summary?.total || 0;
  const tabs: { id: InfoDockTab; label: string; badge?: number }[] = [];
  if (showLegendTab) tabs.push({ id: 'legend', label: 'Легенда' });
  if (showStatsTab) tabs.push({ id: 'stats', label: 'Статистика' });
  tabs.push({ id: 'anomalies', label: 'Аномалии', badge: anomalyTotal || undefined });

  const activeTab = tabs.some((t) => t.id === tab) ? tab : tabs[0]?.id;

  return (
    <aside className="map-info-dock" role="complementary" aria-label="Информация о карте">
      <header className="map-info-dock-head">
        <nav className="map-info-dock-tabs" role="tablist" aria-label="Вкладки инфо-панели">
          {tabs.map((t) => (
            <button
              key={t.id}
              type="button"
              role="tab"
              className={activeTab === t.id ? 'active' : ''}
              aria-selected={activeTab === t.id}
              onClick={() => onTabChange(t.id)}
            >
              {t.label}
              {t.badge ? <span className="map-info-dock-tab-badge">{t.badge}</span> : null}
            </button>
          ))}
        </nav>
        <button type="button" className="btn ghost map-info-dock-close" onClick={onClose} aria-label="Закрыть">
          ×
        </button>
      </header>
      <div className="map-info-dock-body" role="tabpanel">
        {activeTab === 'legend' ? <MapLegend monoArcs={monoArcs} repColorArcs={repColorArcs} /> : null}
        {activeTab === 'stats' ? <MapStatsGrid stats={stats} /> : null}
        {activeTab === 'anomalies' ? (
          <AnomalyPanel
            embedded
            items={anomalyItems}
            onShow={onAnomalyShow}
            onAck={onAnomalyAck}
          />
        ) : null}
      </div>
    </aside>
  );
}
