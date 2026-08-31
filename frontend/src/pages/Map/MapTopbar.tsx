import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react';
import { SystemHealthPill, UserMenu } from '@/components/Shell';
import { SearchBuilder } from './SearchBuilder';
import { countActiveMapFilters, MapFiltersPanel } from './MapFiltersPanel';
import { MapLayersPanel } from './MapLayersPanel';
import { PERIODS } from './mapPeriods';
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
    hideIntraCountry: boolean;
    setHideIntraCountry: (v: boolean) => void;
  };
  reputation: {
    reputationEnabled: boolean;
    ipMode: boolean;
    repFilterCount: number;
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
  layers: {
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
};

type ChromePanel = 'filters' | 'layers' | null;

export function MapTopbar({
  search: searchCtl,
  grouping,
  reputation,
  period: periodCtl,
  layers,
}: MapTopbarProps) {
  const { search, setSearch, builderOpen, setBuilderOpen } = searchCtl;
  const { groupBy, setGroupBy, filter, setFilter, hideIntraCountry, setHideIntraCountry } = grouping;
  const {
    reputationEnabled,
    ipMode,
    repFilterCount,
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

  const [panel, setPanel] = useState<ChromePanel>(null);
  const filtersWrapRef = useRef<HTMLDivElement>(null);
  const layersWrapRef = useRef<HTMLDivElement>(null);

  const filtersActive = countActiveMapFilters({
    groupBy,
    filter,
    repFilterCount,
    repColorArcs,
    hideIntraCountry,
  });

  function openPanel(next: ChromePanel) {
    setPanel((prev) => (prev === next ? null : next));
    if (next) setBuilderOpen(false);
  }

  function openBuilder() {
    setBuilderOpen((v) => {
      const next = !v;
      if (next) setPanel(null);
      return next;
    });
  }

  useEffect(() => {
    if (builderOpen) setPanel(null);
  }, [builderOpen]);

  useEffect(() => {
    if (!panel) return;
    function onDoc(e: MouseEvent) {
      const t = e.target as Node;
      if (panel === 'filters' && filtersWrapRef.current?.contains(t)) return;
      if (panel === 'layers' && layersWrapRef.current?.contains(t)) return;
      setPanel(null);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setPanel(null);
    }
    document.addEventListener('mousedown', onDoc);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDoc);
      document.removeEventListener('keydown', onKey);
    };
  }, [panel]);

  function resetFilters() {
    setGroupBy('ip');
    setFilter('all');
    setHideIntraCountry(false);
    setRepCategories(new Set());
    setRepLists(new Set());
    setRepSide('any');
    setRepColorArcs(false);
  }

  return (
    <header className="topbar">
      <div className="search-box">
        <svg className="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="11" cy="11" r="8" />
          <path d="M21 21l-4.35-4.35" />
        </svg>
        <input
          type="text"
          placeholder="Поиск: IP, страна, action=allow, src_ip=10.0.0.1…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <button
          type="button"
          className={`search-builder-toggle${builderOpen ? ' active' : ''}`}
          aria-label="Открыть расширенный поиск"
          aria-expanded={builderOpen}
          aria-controls="searchBuilderPanel"
          title="Расширенный поиск"
          onClick={openBuilder}
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

      <div className="period-control">
        <span>Период:</span>
        <select
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

      <div className="map-chrome-trigger" ref={filtersWrapRef}>
        <button
          type="button"
          className={`map-chrome-btn${panel === 'filters' || filtersActive > 0 ? ' active' : ''}`}
          aria-expanded={panel === 'filters'}
          aria-haspopup="dialog"
          onClick={() => openPanel('filters')}
        >
          Фильтры
          {filtersActive > 0 ? <span className="map-chrome-badge">{filtersActive}</span> : null}
        </button>
        <MapFiltersPanel
          open={panel === 'filters'}
          grouping={grouping}
          reputation={{
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
          }}
          onReset={resetFilters}
        />
      </div>

      <div className="map-chrome-trigger" ref={layersWrapRef}>
        <button
          type="button"
          className={`map-chrome-btn${panel === 'layers' ? ' active' : ''}`}
          aria-expanded={panel === 'layers'}
          aria-haspopup="dialog"
          onClick={() => openPanel('layers')}
        >
          Слои
        </button>
        <MapLayersPanel
          open={panel === 'layers'}
          viewMode={layers.viewMode}
          viz={layers.viz}
          data={layers.data}
          globe={layers.globe}
        />
      </div>

      <div className="topbar-spacer" />
      <UserMenu />
      <SystemHealthPill />
    </header>
  );
}
