import type { Dispatch, SetStateAction } from 'react';
import { SystemHealthPill, UserMenu } from '@/components/Shell';
import { SearchBuilder } from './SearchBuilder';
import { PERIODS } from './mapPeriods';
import { categoryLabel } from './mapReputation';
import type { RepFilterSide } from './mapTypes';

export type MapTopbarProps = {
  search: {
    search: string;
    setSearch: (v: string) => void;
    builderOpen: boolean;
    setBuilderOpen: Dispatch<SetStateAction<boolean>>;
  };
  grouping: {
    groupBy: string;
    setGroupBy: (v: string) => void;
    filter: 'all' | 'allowed' | 'blocked';
    setFilter: (v: 'all' | 'allowed' | 'blocked') => void;
  };
  reputation: {
    reputationEnabled: boolean;
    ipMode: boolean;
    repFilterCount: number;
    repMenuOpen: boolean;
    setRepMenuOpen: Dispatch<SetStateAction<boolean>>;
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
  period: {
    period: string;
    setPeriod: (v: string) => void;
    periodFrom: string;
    setPeriodFrom: (v: string) => void;
    periodTo: string;
    setPeriodTo: (v: string) => void;
    fetchData: () => void | Promise<void>;
  };
};

export function MapTopbar({ search: searchCtl, grouping, reputation, period: periodCtl }: MapTopbarProps) {
  const { search, setSearch, builderOpen, setBuilderOpen } = searchCtl;
  const { groupBy, setGroupBy, filter, setFilter } = grouping;
  const {
    reputationEnabled,
    ipMode,
    repFilterCount,
    repMenuOpen,
    setRepMenuOpen,
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
  const { period, setPeriod, periodFrom, setPeriodFrom, periodTo, setPeriodTo, fetchData } = periodCtl;

  return (
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
        <div className="reputation-filter" id="reputationFilterWrap" data-reputation-only>
          <button
            type="button"
            id="btnReputationFilter"
            className={`rep-filter-btn${(repFilterCount > 0 || repColorArcs) && ipMode ? ' active' : ''}`}
            title={
              ipMode
                ? 'Фильтр и подсветка по репутационным спискам'
                : 'Доступно в режиме Группа: IP'
            }
            disabled={!ipMode}
            onClick={(e) => {
              e.stopPropagation();
              if (!ipMode) return;
              setRepMenuOpen((v) => !v);
            }}
          >
            Репутация
            <span
              id="repFilterBadge"
              className="rep-badge"
              style={{
                display: repFilterCount > 0 && ipMode ? 'inline-flex' : 'none',
              }}
            >
              {repFilterCount}
            </span>
          </button>
          <div
            className={`reputation-menu${repMenuOpen ? ' open' : ''}`}
            id="reputationMenu"
            role="dialog"
            aria-label="Фильтр репутации"
          >
            <div className="rep-menu-head">
              <span>Репутация</span>
              <button
                type="button"
                className="rep-clear"
                id="btnRepFilterClear"
                onClick={() => {
                  setRepCategories(new Set());
                  setRepLists(new Set());
                  setRepSide('any');
                }}
              >
                Сбросить
              </button>
            </div>
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
                id="repColorArcsChk"
                checked={repColorArcs}
                onChange={(e) => setRepColorArcs(e.target.checked)}
              />
              Окрашивать дуги с хитом
            </label>
            <p className="rep-menu-hint">
              Частные и спец. сети (RFC1918, CGNAT, loopback) не учитываются. В деталях дуги
              смотрите «диапазон».
            </p>
            <div className="rep-menu-body" id="reputationMenuBody">
              {!ipMode ? (
                <div className="rep-menu-empty">Переключите «Группа» на IP</div>
              ) : Object.keys(repTree).length === 0 ? (
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
          </div>
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
  );
}
