import { fmtNumber } from '@/lib/format';

export type MapLayersPanelProps = {
  open: boolean;
  viewMode: 'map' | 'globe';
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
  globe: {
    autoRotate: boolean;
    setAutoRotate: (v: boolean) => void;
  };
};

export function MapLayersPanel({ open, viewMode, viz, data, globe }: MapLayersPanelProps) {
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
  const { autoRotate, setAutoRotate } = globe;

  if (!open) return null;

  return (
    <div className="map-chrome-panel map-layers-panel" role="dialog" aria-label="Слои карты">
      <div className="map-chrome-panel-head">
        <span>Слои</span>
      </div>

      <div className="map-chrome-panel-section">
        <div className="map-chrome-panel-label">Порог событий на связь</div>
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
          <span
            style={{
              color: arcCountInfo.total > arcCountInfo.shown ? 'var(--orange)' : 'var(--text-muted)',
            }}
          >
            {arcCountInfo.total > arcCountInfo.shown
              ? `${fmtNumber(arcCountInfo.shown)} из ${fmtNumber(arcCountInfo.total)}`
              : `${fmtNumber(arcCountInfo.shown)} связей`}
          </span>
        </div>
      </div>

      <div className="map-chrome-panel-section">
        <div className="map-chrome-panel-label">Лимит дуг</div>
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

      <div className="map-chrome-panel-section">
        <div className="map-chrome-panel-label">Оверлеи</div>
        <label className="side-toggle">
          <input type="checkbox" checked={showLegend} onChange={(e) => setShowLegend(e.target.checked)} />
          <span>Легенда (инфо-панель)</span>
        </label>
        <label className="side-toggle">
          <input type="checkbox" checked={showStats} onChange={(e) => setShowStats(e.target.checked)} />
          <span>Статистика (инфо-панель)</span>
        </label>
        <label
          className="side-toggle"
          title="2D: заливка стран (на глобусе heatmap отключён)"
          style={{ display: viewMode === 'globe' ? 'none' : undefined }}
        >
          <input
            type="checkbox"
            checked={showHeatmap}
            onChange={(e) => setShowHeatmap(e.target.checked)}
          />
          <span>Heatmap стран</span>
        </label>
        <label className="side-toggle">
          <input
            type="checkbox"
            checked={showCountryLabels}
            onChange={(e) => setShowCountryLabels(e.target.checked)}
          />
          <span>Названия стран</span>
        </label>
        <label className="side-toggle" title="Все дуги одним цветом">
          <input type="checkbox" checked={monoArcs} onChange={(e) => setMonoArcs(e.target.checked)} />
          <span>Один цвет линий</span>
        </label>
      </div>

      <div className="map-chrome-panel-section">
        <div className="map-chrome-panel-label">Данные</div>
        <label className="side-toggle">
          <input
            type="checkbox"
            checked={autoRefresh}
            disabled={dataSource === 'backup'}
            onChange={(e) => setAutoRefresh(e.target.checked)}
          />
          <span>Авто-обновление</span>
        </label>
        <div
          className="mode-switch mode-switch-data"
          title={
            backupAttached
              ? `Резервная копия: ${backupAttached}`
              : 'Сначала подключите резервную копию в Мониторинге системы'
          }
        >
          <button
            type="button"
            className={dataSource === 'live' ? 'active' : ''}
            title="Прямой эфир"
            onClick={() => selectDataSource('live')}
          >
            Прямой эфир
          </button>
          <button
            type="button"
            className={dataSource === 'backup' ? 'active' : ''}
            disabled={!backupAttached && dataSource !== 'backup'}
            title="Резервная копия"
            onClick={() => selectDataSource('backup')}
          >
            Резервная копия
          </button>
        </div>
        {backupAttached ? (
          <p className="hint" style={{ marginTop: 6, fontSize: 12 }}>
            Подключена <code>{backupAttached}</code>
          </p>
        ) : (
          <p className="hint" style={{ marginTop: 6, fontSize: 12 }}>
            Нет подключённой резервной копии
          </p>
        )}
        <label
          className="side-toggle"
          style={{ display: viewMode === 'globe' ? undefined : 'none' }}
        >
          <input
            type="checkbox"
            checked={autoRotate}
            onChange={(e) => setAutoRotate(e.target.checked)}
          />
          <span>Авто-вращение глобуса</span>
        </label>
      </div>
    </div>
  );
}
