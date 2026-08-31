import type { Dispatch, SetStateAction } from 'react';
import { categoryLabel } from './mapReputation';
import type { RepFilterSide } from './mapTypes';

export type MapFiltersPanelProps = {
  open: boolean;
  grouping: {
    groupBy: string;
    setGroupBy: (v: string) => void;
    filter: 'all' | 'allowed' | 'blocked';
    setFilter: (v: 'all' | 'allowed' | 'blocked') => void;
    hideIntraCountry: boolean;
    setHideIntraCountry: (v: boolean) => void;
  };
  reputation: {
    reputationEnabled: boolean;
    ipMode: boolean;
    repCategories: Set<string>;
    setRepCategories: Dispatch<SetStateAction<Set<string>>>;
    repLists: Set<string>;
    setRepLists: Dispatch<SetStateAction<Set<string>>>;
    repSide: RepFilterSide;
    setRepSide: (v: RepFilterSide) => void;
    repColorArcs: boolean;
    setRepColorArcs: (v: boolean) => void;
    repTree: Record<string, Set<string>>;
  };
  onReset: () => void;
};

/** Count of non-default filter dimensions for the topbar badge. */
export function countActiveMapFilters(opts: {
  groupBy: string;
  filter: string;
  repFilterCount: number;
  repColorArcs: boolean;
  hideIntraCountry: boolean;
}): number {
  let n = 0;
  if (opts.groupBy !== 'ip') n += 1;
  if (opts.filter !== 'all') n += 1;
  if (opts.repFilterCount > 0) n += 1;
  if (opts.repColorArcs) n += 1;
  if (opts.groupBy === 'city' && opts.hideIntraCountry) n += 1;
  return n;
}

export function MapFiltersPanel({ open, grouping, reputation, onReset }: MapFiltersPanelProps) {
  const { groupBy, setGroupBy, filter, setFilter, hideIntraCountry, setHideIntraCountry } = grouping;
  const {
    reputationEnabled,
    ipMode,
    repCategories,
    setRepCategories,
    repLists,
    setRepLists,
    repSide,
    setRepSide,
    repColorArcs,
    setRepColorArcs,
    repTree,
  } = reputation;

  if (!open) return null;

  return (
    <div className="map-chrome-panel map-filters-panel" role="dialog" aria-label="Фильтры карты">
      <div className="map-chrome-panel-head">
        <span>Фильтры</span>
        <button type="button" className="map-chrome-panel-reset" onClick={onReset}>
          Сбросить
        </button>
      </div>

      <div className="map-chrome-panel-section">
        <div className="map-chrome-panel-label">Группировка</div>
        <select
          className="map-chrome-select"
          value={groupBy}
          onChange={(e) => setGroupBy(e.target.value)}
          aria-label="Группировка"
        >
          <option value="ip">IP</option>
          <option value="subnet">/24</option>
          <option value="city">Город</option>
          <option value="country">Страна</option>
          <option value="continent">Континент</option>
        </select>
      </div>

      {groupBy === 'city' ? (
        <div className="map-chrome-panel-section">
          <label className="side-toggle">
            <input
              type="checkbox"
              checked={hideIntraCountry}
              onChange={(e) => setHideIntraCountry(e.target.checked)}
            />
            <span>Скрыть связи внутри страны</span>
          </label>
        </div>
      ) : null}

      <div className="map-chrome-panel-section">
        <div className="map-chrome-panel-label">Статус</div>
        <div className="map-chrome-filter-tabs">
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
      </div>

      {reputationEnabled ? (
        <div className="map-chrome-panel-section">
          <div className="map-chrome-panel-label">Репутация</div>
          {!ipMode ? (
            <p className="map-chrome-hint">Доступно при группировке IP</p>
          ) : (
            <>
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
                  checked={repColorArcs}
                  onChange={(e) => setRepColorArcs(e.target.checked)}
                />
                Окрашивать дуги с хитом
              </label>
              <p className="rep-menu-hint">
                Частные и спец. сети (RFC1918, CGNAT, loopback) не учитываются. В деталях дуги
                смотрите «диапазон».
              </p>
              <div className="rep-menu-body">
                {Object.keys(repTree).length === 0 ? (
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
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}
