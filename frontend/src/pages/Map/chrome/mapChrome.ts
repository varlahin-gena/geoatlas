import type { Dispatch, SetStateAction } from 'react';
import type { MapRefreshSec } from '../mapRefreshInterval';
import type { MapSidebarProps } from '../MapSidebar';
import type { MapTopbarProps } from '../MapTopbar';
import type { RepFilterSide } from '../mapTypes';

type ViewMode = 'map' | 'globe';

export type MapChromeSidebarInput = {
  toggleSidebar: () => void;
};

export function buildMapSidebarProps(input: MapChromeSidebarInput): MapSidebarProps {
  return {
    actions: {
      toggleSidebar: input.toggleSidebar,
    },
  };
}

export type MapChromeTopbarInput = {
  search: string;
  setSearch: (v: string) => void;
  builderOpen: boolean;
  setBuilderOpen: Dispatch<SetStateAction<boolean>>;
  groupBy: string;
  setGroupBy: (v: string) => void;
  filter: 'all' | 'allowed' | 'blocked';
  setFilter: (v: 'all' | 'allowed' | 'blocked') => void;
  hideIntraCountry: boolean;
  setHideIntraCountry: (v: boolean) => void;
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
  period: string;
  setPeriod: (v: string) => void;
  periodFrom: string;
  setPeriodFrom: (v: string) => void;
  periodTo: string;
  setPeriodTo: (v: string) => void;
  fetchData: () => void | Promise<void>;
  viewMode: ViewMode;
  setViewMode: (mode: ViewMode) => void;
  resetView: () => void;
  exportPng: () => void | Promise<void>;
  learningNote: string | null;
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
  autoRefresh: boolean;
  setAutoRefresh: (v: boolean) => void;
  refreshIntervalSec: MapRefreshSec;
  setRefreshIntervalSec: (v: MapRefreshSec) => void;
  dataSource: 'live' | 'backup';
  selectDataSource: (v: 'live' | 'backup') => void;
  backupAttached: string;
  autoRotate: boolean;
  setAutoRotate: (v: boolean) => void;
  setInfoDockTab: (tab: 'legend' | 'stats') => void;
  focusedCountry: string | null;
  clearFocusedCountry: () => void;
};

export function buildMapTopbarProps(input: MapChromeTopbarInput): MapTopbarProps {
  return {
    search: {
      search: input.search,
      setSearch: input.setSearch,
      builderOpen: input.builderOpen,
      setBuilderOpen: input.setBuilderOpen,
      huntSave: {
        period: input.period,
        periodFrom: input.periodFrom,
        periodTo: input.periodTo,
        groupBy: input.groupBy,
        filter: input.filter,
        focusedCountry: input.focusedCountry,
        maxArcs: input.maxArcs,
      },
    },
    grouping: {
      groupBy: input.groupBy,
      setGroupBy: input.setGroupBy,
      filter: input.filter,
      setFilter: input.setFilter,
      hideIntraCountry: input.hideIntraCountry,
      setHideIntraCountry: input.setHideIntraCountry,
      focusedCountry: input.focusedCountry,
      clearFocusedCountry: input.clearFocusedCountry,
    },
    reputation: {
      reputationEnabled: input.reputationEnabled,
      ipMode: input.ipMode,
      repFilterCount: input.repFilterCount,
      repCategories: input.repCategories,
      setRepCategories: input.setRepCategories,
      repLists: input.repLists,
      setRepLists: input.setRepLists,
      repSide: input.repSide,
      setRepSide: input.setRepSide,
      repColorArcs: input.repColorArcs,
      setRepColorArcs: input.setRepColorArcs,
      repTree: input.repTree,
    },
    period: {
      period: input.period,
      setPeriod: input.setPeriod,
      periodFrom: input.periodFrom,
      setPeriodFrom: input.setPeriodFrom,
      periodTo: input.periodTo,
      setPeriodTo: input.setPeriodTo,
      fetchData: input.fetchData,
    },
    learningNote: input.learningNote,
    layers: {
      viewMode: input.viewMode,
      setViewMode: input.setViewMode,
      tools: {
        fetchData: input.fetchData,
        resetView: input.resetView,
        exportPng: input.exportPng,
      },
      viz: {
        minCount: input.minCount,
        setMinCount: input.setMinCount,
        arcCountInfo: input.arcCountInfo,
        maxArcs: input.maxArcs,
        setMaxArcs: input.setMaxArcs,
        showLegend: input.showLegend,
        setShowLegend: (v: boolean) => {
          input.setShowLegend(v);
          if (v) input.setInfoDockTab('legend');
        },
        showStats: input.showStats,
        setShowStats: (v: boolean) => {
          input.setShowStats(v);
          if (v) input.setInfoDockTab('stats');
        },
        showHeatmap: input.showHeatmap,
        setShowHeatmap: input.setShowHeatmap,
        showCountryLabels: input.showCountryLabels,
        setShowCountryLabels: input.setShowCountryLabels,
        monoArcs: input.monoArcs,
        setMonoArcs: input.setMonoArcs,
      },
      data: {
        autoRefresh: input.autoRefresh,
        setAutoRefresh: input.setAutoRefresh,
        refreshIntervalSec: input.refreshIntervalSec,
        setRefreshIntervalSec: input.setRefreshIntervalSec,
        dataSource: input.dataSource,
        selectDataSource: input.selectDataSource,
        backupAttached: input.backupAttached,
      },
      globe: {
        autoRotate: input.autoRotate,
        setAutoRotate: input.setAutoRotate,
      },
    },
  };
}
