import { fmtNumber } from '@/lib/format';

export function MapVizOverlays({
  truncHint,
  emptyOverlay,
  loading,
  showLegend,
  monoArcs,
  repColorArcs,
  showStats,
  stats,
}: {
  truncHint: string;
  emptyOverlay: { title: string; text: string } | null;
  loading: boolean;
  showLegend: boolean;
  monoArcs: boolean;
  repColorArcs: boolean;
  showStats: boolean;
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
      {truncHint ? <div className="viz-hint warn">{truncHint}</div> : null}

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

      {showLegend ? (
        <div className="legend">
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
                  <span className="legend-line" style={{ background: 'var(--orange)' }} />{' '}
                  Репутационный хит
                </div>
              ) : null}
            </>
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
        <div className="stats-floating">
          <div className="stats-item">
            <span className="lbl">Событий</span>
            <span className="val" id="stat-total">
              {fmtNumber(stats.events)}
            </span>
          </div>
          <div className="stats-item">
            <span className="lbl">Allowed</span>
            <span className="val green">{fmtNumber(stats.allowed)}</span>
          </div>
          <div className="stats-item">
            <span className="lbl">Blocked</span>
            <span className="val red">{fmtNumber(stats.blocked)}</span>
          </div>
          <div className="stats-item">
            <span className="lbl">Связей</span>
            <span className="val" id="stat-edges">
              {fmtNumber(stats.connections)}
            </span>
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
      ) : null}
    </>
  );
}
