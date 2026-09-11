import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';
import { useToast } from '@/components/Toast';
import { useSidebarCollapsed } from '@/components/useSidebarCollapsed';
import { fmtNumber } from '@/lib/format';
import { GA_MAP_COMMAND, isMapCommandDetail } from '@/lib/mapCommandBus';
import { loadCountriesGeoJSON, type GeoFeatureCollection } from './mapHeatmap';
import type { EmptyMapActionKind } from './geoWizard';
import { MapDetailPanel } from './mapDetail';
import { collectReputationMenuTree } from './mapReputation';
import { MapChromeProvider, useMapChrome } from './MapChromeContext';
import { MapSidebar } from './MapSidebar';
import { MapTopbar } from './MapTopbar';
import { MapVizOverlays } from './MapVizOverlays';
import { GeoWizardModal } from './GeoWizardModal';
import { AnomalyStrip } from './AnomalyStrip';
import {
  AnomalyActiveBanner,
  clearMapAlertMemory,
  readMapAlert,
} from './AnomalyActiveBanner';
import { useGeoWizard } from './useGeoWizard';
import { useMapAnomalies } from './useMapAnomalies';
import { useMapDetail } from './useMapDetail';
import { useMapDeckLayers } from './viz/useMapDeckLayers';
import { useMapEvents } from './useMapEvents';
import { useMapFilters } from './useMapFilters';
import { useMapViewQuery } from './mapQuery';
import { mapQueryWarning } from './mapQueryCost';
import { useMapLibreController } from './useMapLibreController';
import { useMapReputation } from './useMapReputation';
import { useMapUploads } from './useMapUploads';
import { useMapVizChrome } from './useMapVizChrome';
import 'maplibre-gl/dist/maplibre-gl.css';
import '@/styles/index.css';
import './geoWizard.css';
import './anomaly.css';

function MapChromeShell({
  sidebarCollapsed,
  mapContainer,
  loading,
  showGeoEmptyBanner,
  truncHint,
  infoDockVisible,
  activeAlert,
  returnToLiveMap,
  anomaliesSummary,
  displayEmptyOverlay,
  onEmptyAction,
  monoArcs,
  repColorArcs,
  stats,
  infoDockTab,
  setInfoDockTab,
  showLegend,
  showStats,
  detail,
  closeDetail,
}: {
  sidebarCollapsed: boolean;
  mapContainer: React.RefObject<HTMLDivElement | null>;
  loading: boolean;
  showGeoEmptyBanner: boolean;
  truncHint: string;
  infoDockVisible: boolean;
  activeAlert: ReturnType<typeof readMapAlert>;
  returnToLiveMap: () => void;
  anomaliesSummary: ReturnType<typeof useMapAnomalies>['summary'];
  displayEmptyOverlay: ReturnType<typeof useMapFilters>['emptyOverlay'];
  onEmptyAction: (kind: EmptyMapActionKind) => void;
  monoArcs: boolean;
  repColorArcs: boolean;
  stats: ReturnType<typeof useMapFilters>['stats'];
  infoDockTab: 'legend' | 'stats';
  setInfoDockTab: (tab: 'legend' | 'stats') => void;
  showLegend: boolean;
  showStats: boolean;
  detail: ReturnType<typeof useMapDetail>['detail'];
  closeDetail: () => void;
}) {
  const { sidebar, topbar } = useMapChrome();

  return (
    <div className={`app${sidebarCollapsed ? ' sidebar-collapsed' : ''}`}>
      <a className="skip-link" href="#map-main">
        К содержимому
      </a>
      <MapSidebar {...sidebar} />

      <main className="main" id="map-main">
        <MapTopbar {...topbar} />

        <div
          className={`viz-area${loading ? ' is-loading' : ''}${showGeoEmptyBanner ? ' has-geo-empty-banner' : ''}${truncHint ? ' has-viz-hint' : ''}${infoDockVisible ? ' has-map-info-dock' : ''}`}
        >
          <div ref={mapContainer} id="map-host" className="viz-host" />

          <div className="map-top-stack">
            {activeAlert ? (
              <AnomalyActiveBanner alert={activeAlert} onReturnLive={returnToLiveMap} />
            ) : (
              <AnomalyStrip summary={anomaliesSummary} />
            )}
            {truncHint ? <div className="viz-hint warn">{truncHint}</div> : null}
          </div>

          {showGeoEmptyBanner ? (
            <div className="geo-wizard-banner" role="status">
              <p>База GeoIP пуста — загрузите координаты через карточку на карте.</p>
            </div>
          ) : null}

          <MapVizOverlays
            emptyOverlay={displayEmptyOverlay}
            onEmptyAction={onEmptyAction}
            loading={loading}
            monoArcs={monoArcs}
            repColorArcs={repColorArcs}
            stats={stats}
            infoDock={{
              tab: infoDockTab,
              onTabChange: setInfoDockTab,
              showLegendTab: showLegend,
              showStatsTab: showStats,
            }}
          />

          <MapDetailPanel detail={detail} onClose={closeDetail} />
        </div>
      </main>
    </div>
  );
}

export default function MapPage() {
  const { isAdmin, reputationEnabled, uiAuthEnabled, theme, user, refresh } = useAuth();
  const { toast } = useToast();
  const navigate = useNavigate();
  const { collapsed: sidebarCollapsed, toggle: toggleSidebar } = useSidebarCollapsed();
  const view = useMapViewQuery();
  const {
    period,
    setPeriod,
    periodFrom,
    setPeriodFrom,
    periodTo,
    setPeriodTo,
    groupBy,
    setGroupBy,
    filter,
    setFilter,
    search,
    setSearch,
    builderOpen,
    setBuilderOpen,
    minCount,
    setMinCount,
    maxArcs,
    setMaxArcs,
    hideIntraCountry,
    setHideIntraCountry,
    focusedCountry,
    setFocusedCountry,
    clearFocusedCountry,
    applySearchFilter,
    alertFingerprint,
    resetToLiveView,
  } = view;

  const {
    repCategories,
    setRepCategories,
    repLists,
    setRepLists,
    repSide,
    setRepSide,
    repColorArcs,
    setRepColorArcs,
    repActive,
    repFilterCount,
    ipMode,
    repCategoryList,
    repListList,
  } = useMapReputation(groupBy);

  useEffect(() => {
    function onCmd(e: Event) {
      const detail = (e as CustomEvent).detail;
      if (!isMapCommandDetail(detail)) return;
      if (detail.type === 'set-period') {
        setPeriod(detail.period);
        return;
      }
      if (detail.type === 'reset-filters') {
        setGroupBy('ip');
        setFilter('all');
        setHideIntraCountry(false);
        setRepCategories(new Set());
        setRepLists(new Set());
        setRepSide('any');
        setRepColorArcs(false);
        setSearch('');
        return;
      }
      if (detail.type === 'focus-search') {
        const input = document.querySelector<HTMLInputElement>('.topbar .search-box input');
        input?.focus();
        input?.select();
      }
    }
    document.addEventListener(GA_MAP_COMMAND, onCmd);
    return () => document.removeEventListener(GA_MAP_COMMAND, onCmd);
  }, [
    setPeriod,
    setGroupBy,
    setFilter,
    setHideIntraCountry,
    setRepCategories,
    setRepLists,
    setRepSide,
    setRepColorArcs,
    setSearch,
  ]);

  const {
    points,
    lines,
    loading,
    fetchError,
    eventStats,
    queryCost,
    requestedLimit,
    effectiveLimit,
    repFacets,
    autoRefresh,
    setAutoRefresh,
    refreshIntervalSec,
    setRefreshIntervalSec,
    dataSource,
    selectDataSource,
    backupAttached,
    periodQuery,
    fetchData,
  } = useMapEvents(toast, {
    period,
    periodFrom,
    periodTo,
    groupBy,
    filter,
    maxArcs: view.debouncedMaxArcs,
    focusedCountry,
    search: view.debouncedSearch,
    repCategories: repCategoryList,
    repLists: repListList,
    repSide,
    repActive,
  });

  const repTree = useMemo(() => {
    const keys = Object.keys(repFacets || {});
    if (keys.length) {
      const tree: Record<string, Set<string>> = {};
      for (const cat of keys) {
        tree[cat] = new Set(repFacets[cat]);
      }
      return tree;
    }
    return collectReputationMenuTree(lines);
  }, [repFacets, lines]);

  const { visibleLines, stats, emptyOverlay } = useMapFilters({
    lines,
    points,
    loading,
    fetchError,
    rawPairs: eventStats.rawPairs,
    skippedNoGeo: eventStats.skippedNoGeo,
    repActive,
    repCategories,
    repLists,
    repSide,
    filter: view.filter,
    search: view.search,
    minCount: view.minCount,
    focusedCountry,
    groupBy,
    hideIntraCountry,
    isAdmin,
  });

  const geoWizard = useGeoWizard({
    isAdmin,
    user,
    uiAuthEnabled,
    toast,
    refreshUser: refresh,
    onGeoReady: fetchData,
  });

  const viz = useMapVizChrome();
  const [countriesGeoJSON, setCountriesGeoJSON] = useState<GeoFeatureCollection | null>(null);

  const {
    mapContainer,
    overlayRef,
    layersRefreshBusy,
    heavyCountryLayers,
    mapTilesFailed,
    globeViewRef,
    mapReady,
    layersTick,
    resetView,
    exportPng,
  } = useMapLibreController({
    theme,
    viewMode: viz.viewMode,
    setViewMode: viz.setViewMode,
    autoRotate: viz.autoRotate,
    toast,
  });

  const { logFileRef, geoFileRef, uploadFile } = useMapUploads({
    isAdmin,
    toast,
    fetchData,
  });

  const anomalies = useMapAnomalies();
  const activeAlert = useMemo(
    () => (alertFingerprint ? readMapAlert(alertFingerprint) : null),
    [alertFingerprint],
  );

  function returnToLiveMap() {
    clearMapAlertMemory();
    resetToLiveView();
    if (dataSource !== 'live') selectDataSource('live');
  }

  const { detail, closeDetail, openLineDetail, openPointDetail, openCountryDetail } = useMapDetail({
    groupBy,
    lines,
    points,
    visibleLines,
    countriesGeoJSON,
    periodQuery,
    dataSource,
    toast,
    applySearchFilter,
    clearFocusedCountry,
    setFocusedCountry,
  });

  const { arcCountInfo } = useMapDeckLayers({
    overlayRef,
    mapReady,
    layersRefreshBusy,
    layersTick,
    viewMode: viz.viewMode,
    visibleLines,
    points,
    countriesGeoJSON,
    showHeatmap: viz.showHeatmap,
    showCountryLabels: viz.showCountryLabels,
    monoArcs: viz.monoArcs,
    repColorArcs,
    groupBy,
    focusedCountry,
    mapTilesFailed,
    heavyCountryLayers,
    theme,
    globeViewRef,
    maxArcs,
    onLineClick: openLineDetail,
    onPointClick: openPointDetail,
    onCountryClick: openCountryDetail,
  });

  useEffect(() => {
    document.title = 'ГеоАтлас — SOC';
    document.body.classList.add('page-map');
    return () => document.body.classList.remove('page-map');
  }, []);

  useEffect(() => {
    void loadCountriesGeoJSON().then(setCountriesGeoJSON);
  }, []);

  const truncHint = useMemo(() => {
    const queryWarn = mapQueryWarning({
      cost: queryCost,
      requestedLimit: eventStats.limitRequested || requestedLimit,
      effectiveLimit: eventStats.limitApplied || effectiveLimit,
      limitCapped: eventStats.limitCapped,
      source: eventStats.source,
    });
    if (queryWarn) return queryWarn;
    if (arcCountInfo.total > arcCountInfo.shown) {
      return `Показано ${fmtNumber(arcCountInfo.shown)} из ${fmtNumber(arcCountInfo.total)} связей — увеличьте лимит дуг или сузьте период`;
    }
    return '';
  }, [
    queryCost,
    eventStats.limitRequested,
    eventStats.limitApplied,
    eventStats.limitCapped,
    eventStats.source,
    requestedLimit,
    effectiveLimit,
    arcCountInfo.total,
    arcCountInfo.shown,
  ]);

  const showGeoEmptyBanner =
    isAdmin && !geoWizard.visible && geoWizard.geo != null && geoWizard.geo.count === 0;

  const displayEmptyOverlay = emptyOverlay;

  function onEmptyAction(kind: EmptyMapActionKind) {
    if (kind === 'open-geo-wizard') {
      geoWizard.open();
      return;
    }
    if (kind === 'open-geo-missing') {
      navigate('/geo-missing');
      return;
    }
    if (kind === 'set-period-7d') {
      setPeriod('7d');
      return;
    }
    if (kind === 'reset-filters') {
      setGroupBy('ip');
      setFilter('all');
      setHideIntraCountry(false);
      setRepCategories(new Set());
      setRepLists(new Set());
      setRepSide('any');
      setRepColorArcs(false);
      setSearch('');
      return;
    }
    if (kind === 'clear-search') {
      setSearch('');
    }
  }

  const reloadGeoStatus = geoWizard.reloadStatus;

  useEffect(() => {
    if (!isAdmin) return;
    const refreshGeo = () => void reloadGeoStatus();
    const onVisibility = () => {
      if (document.visibilityState === 'visible') refreshGeo();
    };
    window.addEventListener('focus', refreshGeo);
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      window.removeEventListener('focus', refreshGeo);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [isAdmin, reloadGeoStatus]);

  return (
    <MapChromeProvider
      sidebarInput={{
        viewMode: viz.viewMode,
        setViewMode: viz.setViewMode,
        isAdmin,
        logFileRef,
        geoFileRef,
        uploadFile,
        fetchData,
        resetView,
        exportPng,
        toggleSidebar,
        geoWizardOpen: geoWizard.open,
        geoWizardEmpty: geoWizard.geo == null ? null : geoWizard.geo.count === 0,
      }}
      topbarInput={{
        search,
        setSearch,
        builderOpen,
        setBuilderOpen,
        groupBy,
        setGroupBy,
        filter,
        setFilter,
        hideIntraCountry,
        setHideIntraCountry,
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
        period,
        setPeriod,
        periodFrom,
        setPeriodFrom,
        periodTo,
        setPeriodTo,
        fetchData,
        viewMode: viz.viewMode,
        minCount,
        setMinCount,
        arcCountInfo,
        maxArcs,
        setMaxArcs,
        showLegend: viz.showLegend,
        setShowLegend: viz.setShowLegend,
        showStats: viz.showStats,
        setShowStats: viz.setShowStats,
        showHeatmap: viz.showHeatmap,
        setShowHeatmap: viz.setShowHeatmap,
        showCountryLabels: viz.showCountryLabels,
        setShowCountryLabels: viz.setShowCountryLabels,
        monoArcs: viz.monoArcs,
        setMonoArcs: viz.setMonoArcs,
        autoRefresh,
        setAutoRefresh,
        refreshIntervalSec,
        setRefreshIntervalSec,
        dataSource,
        selectDataSource,
        backupAttached,
        autoRotate: viz.autoRotate,
        setAutoRotate: viz.setAutoRotate,
        setInfoDockTab: viz.setInfoDockTab,
        focusedCountry,
      }}
    >
      <MapChromeShell
        sidebarCollapsed={sidebarCollapsed}
        mapContainer={mapContainer}
        loading={loading}
        showGeoEmptyBanner={showGeoEmptyBanner}
        truncHint={truncHint}
        infoDockVisible={viz.infoDockVisible}
        activeAlert={activeAlert}
        returnToLiveMap={returnToLiveMap}
        anomaliesSummary={anomalies.summary}
        displayEmptyOverlay={displayEmptyOverlay}
        onEmptyAction={onEmptyAction}
        monoArcs={viz.monoArcs}
        repColorArcs={repColorArcs}
        stats={stats}
        infoDockTab={viz.infoDockTab}
        setInfoDockTab={viz.setInfoDockTab}
        showLegend={viz.showLegend}
        showStats={viz.showStats}
        detail={detail}
        closeDetail={closeDetail}
      />

      {geoWizard.visible ? (
        <GeoWizardModal
          step={geoWizard.step}
          setStep={geoWizard.setStep}
          busy={geoWizard.busy}
          geo={geoWizard.geo}
          preview={geoWizard.preview}
          pendingFile={geoWizard.pendingFile}
          pollNote={geoWizard.pollNote}
          curlSnippet={geoWizard.curlSnippet}
          fileRef={geoWizard.fileRef}
          onDismiss={() => void geoWizard.dismiss()}
          onCloseSuccess={() => void geoWizard.closeAfterSuccess()}
          onMoreUpload={geoWizard.moreUpload}
          onDryRun={(file) => void geoWizard.runDryRun(file)}
          onCommit={() => void geoWizard.commitUpload()}
          onWaitForCurl={() => void geoWizard.waitForGeo()}
        />
      ) : null}
    </MapChromeProvider>
  );
}
