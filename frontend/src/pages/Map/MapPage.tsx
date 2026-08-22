import { useEffect, useState } from 'react';
import { useAuth } from '@/auth/AuthContext';
import { useToast } from '@/components/Toast';
import { useSidebarCollapsed } from '@/components/useSidebarCollapsed';
import { fmtNumber } from '@/lib/format';
import { loadCountriesGeoJSON, type GeoFeatureCollection } from './mapHeatmap';
import { MapDetailPanel } from './mapDetail';
import { buildDeckLayers } from './mapLayers';
import { collectReputationMenuTree } from './mapReputation';
import { MapSidebar } from './MapSidebar';
import { MapTopbar } from './MapTopbar';
import { MapVizOverlays } from './MapVizOverlays';
import type { InfoDockTab } from './MapInfoDock';
import { GeoWizardModal } from './GeoWizardModal';
import { AnomalyStrip } from './AnomalyStrip';
import { useGeoWizard } from './useGeoWizard';
import { useMapAnomalies } from './useMapAnomalies';
import { useMapDetail } from './useMapDetail';
import { useMapEvents } from './useMapEvents';
import { useMapFilters } from './useMapFilters';
import { useMapViewQuery } from './mapQuery';
import { useMapLibreController } from './useMapLibreController';
import { useMapReputation } from './useMapReputation';
import { useMapUploads } from './useMapUploads';
import 'maplibre-gl/dist/maplibre-gl.css';
import '@/styles/index.css';
import './geoWizard.css';
import './anomaly.css';

export default function MapPage() {
  const { isAdmin, reputationEnabled, uiAuthEnabled, theme, user, refresh } = useAuth();
  const { toast } = useToast();
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
    focusedCountry,
    setFocusedCountry,
    clearFocusedCountry,
    applySearchFilter,
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

  const {
    points,
    lines,
    loading,
    fetchError,
    eventStats,
    repFacets,
    autoRefresh,
    setAutoRefresh,
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
  });

  const geoWizard = useGeoWizard({
    isAdmin,
    user,
    uiAuthEnabled,
    toast,
    refreshUser: refresh,
    onGeoReady: fetchData,
  });

  const [viewMode, setViewMode] = useState<'map' | 'globe'>('map');
  const [showLegend, setShowLegend] = useState(true);
  const [showStats, setShowStats] = useState(true);
  const [infoDockOpen, setInfoDockOpen] = useState(true);
  const [infoDockTab, setInfoDockTab] = useState<InfoDockTab>('legend');
  const [showHeatmap, setShowHeatmap] = useState(false);
  const [showCountryLabels, setShowCountryLabels] = useState(false);
  const [monoArcs, setMonoArcs] = useState(false);
  const [autoRotate, setAutoRotate] = useState(true);
  const [countriesGeoJSON, setCountriesGeoJSON] = useState<GeoFeatureCollection | null>(null);
  const [arcCountInfo, setArcCountInfo] = useState({ shown: 0, total: 0 });

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
    viewMode,
    setViewMode,
    autoRotate,
    toast,
  });

  const { logFileRef, geoFileRef, uploadFile } = useMapUploads({
    isAdmin,
    toast,
    fetchData,
  });

  const anomalies = useMapAnomalies();

  useEffect(() => {
    if (infoDockTab === 'legend' && !showLegend) {
      setInfoDockTab(showStats ? 'stats' : 'legend');
    } else if (infoDockTab === 'stats' && !showStats) {
      setInfoDockTab(showLegend ? 'legend' : 'stats');
    }
  }, [showLegend, showStats, infoDockTab]);

  useEffect(() => {
    if (!showLegend && !showStats && infoDockOpen) {
      setInfoDockOpen(false);
    }
  }, [showLegend, showStats, infoDockOpen]);

  function openInfoDock(tab: InfoDockTab) {
    setInfoDockOpen(true);
    setInfoDockTab(tab);
  }

  function closeInfoDock() {
    setInfoDockOpen(false);
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

  useEffect(() => {
    document.title = 'ГеоАтлас — SOC';
    document.body.classList.add('page-map');
    return () => document.body.classList.remove('page-map');
  }, []);

  useEffect(() => {
    void loadCountriesGeoJSON().then(setCountriesGeoJSON);
  }, []);

  // Update deck layers
  useEffect(() => {
    const overlay = overlayRef.current;
    if (!overlay || !mapReady) return;
    if (layersRefreshBusy.current) return;
    layersRefreshBusy.current = true;
    try {
      const result = buildDeckLayers({
        mode: viewMode,
        lines: visibleLines,
        points,
        countriesGeoJSON,
        showHeatmap,
        showCountryLabels,
        monoArcColor: monoArcs,
        repColorArcs,
        groupBy,
        focusedCountry,
        mapTilesFailed,
        heavyCountryLayersAllowed: heavyCountryLayers,
        theme,
        globeView: globeViewRef.current,
        maxArcs,
        onLineClick: openLineDetail,
        onPointClick: openPointDetail,
        onCountryClick: (key, feature) => openCountryDetail(key, feature),
        highlightNodeKeys: undefined,
        highlightEdgeKeys: undefined,
      });
      overlay.setProps({ layers: result.layers as never[] });
      setArcCountInfo({ shown: result.shown, total: result.total });
    } finally {
      layersRefreshBusy.current = false;
    }
  }, [
    mapReady,
    layersTick,
    viewMode,
    visibleLines,
    points,
    countriesGeoJSON,
    showHeatmap,
    showCountryLabels,
    monoArcs,
    repColorArcs,
    groupBy,
    focusedCountry,
    mapTilesFailed,
    heavyCountryLayers,
    theme,
    globeViewRef,
    maxArcs,
    openLineDetail,
    openPointDetail,
    openCountryDetail,
    overlayRef,
    layersRefreshBusy,
  ]);

  const truncHint =
    arcCountInfo.total > arcCountInfo.shown
      ? `Показано ${fmtNumber(arcCountInfo.shown)} из ${fmtNumber(arcCountInfo.total)} связей — увеличьте лимит дуг или сузьте период`
      : '';

  const showGeoEmptyBanner =
    isAdmin && !geoWizard.visible && geoWizard.geo != null && geoWizard.geo.count === 0;

  const displayEmptyOverlay =
    showGeoEmptyBanner && emptyOverlay?.title === 'Нет координат для карты'
      ? null
      : emptyOverlay;

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
    <div className={`app${sidebarCollapsed ? ' sidebar-collapsed' : ''}`}>
      <a className="skip-link" href="#map-main">
        К содержимому
      </a>
      <MapSidebar
        view={{ viewMode, setViewMode }}
        isAdmin={isAdmin}
        uploads={{ logFileRef, geoFileRef, uploadFile }}
        actions={{ fetchData, resetView, exportPng, toggleSidebar }}
        geoWizard={{
          open: geoWizard.open,
          empty: geoWizard.geo == null ? null : geoWizard.geo.count === 0,
        }}
      />

      <main className="main" id="map-main">
        <MapTopbar
          search={{ search, setSearch, builderOpen, setBuilderOpen }}
          grouping={{ groupBy, setGroupBy, filter, setFilter }}
          reputation={{
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
          }}
          period={{
            period,
            setPeriod,
            periodFrom,
            setPeriodFrom,
            periodTo,
            setPeriodTo,
            fetchData,
          }}
          layers={{
            viewMode,
            viz: {
              minCount,
              setMinCount,
              arcCountInfo,
              maxArcs,
              setMaxArcs,
              showLegend,
              setShowLegend: (v: boolean) => {
                setShowLegend(v);
                if (v) openInfoDock('legend');
              },
              showStats,
              setShowStats: (v: boolean) => {
                setShowStats(v);
                if (v) openInfoDock('stats');
              },
              showHeatmap,
              setShowHeatmap,
              showCountryLabels,
              setShowCountryLabels,
              monoArcs,
              setMonoArcs,
            },
            data: {
              autoRefresh,
              setAutoRefresh,
              dataSource,
              selectDataSource,
              backupAttached,
            },
            globe: {
              autoRotate,
              setAutoRotate,
            },
          }}
        />

        <div
          className={`viz-area${loading ? ' is-loading' : ''}${showGeoEmptyBanner ? ' has-geo-empty-banner' : ''}${truncHint ? ' has-viz-hint' : ''}${infoDockOpen ? ' has-map-info-dock' : ''}`}
        >
          <div ref={mapContainer} id="map-host" className="viz-host" />

          <div className="map-top-stack">
            <AnomalyStrip summary={anomalies.summary} />
            {truncHint ? <div className="viz-hint warn">{truncHint}</div> : null}
          </div>

          {showGeoEmptyBanner ? (
            <div className="geo-wizard-banner" role="status">
              <p>База GeoIP пуста — карта не покажет дуги без координат.</p>
              <button type="button" className="btn primary" onClick={geoWizard.open}>
                Мастер GeoIP
              </button>
            </div>
          ) : null}

          <MapVizOverlays
            emptyOverlay={displayEmptyOverlay}
            loading={loading}
            monoArcs={monoArcs}
            repColorArcs={repColorArcs}
            stats={stats}
            infoDock={{
              open: infoDockOpen,
              tab: infoDockTab,
              onTabChange: setInfoDockTab,
              onClose: closeInfoDock,
              showLegendTab: showLegend,
              showStatsTab: showStats,
            }}
          />

          <MapDetailPanel detail={detail} onClose={closeDetail} />
        </div>
      </main>

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
    </div>
  );
}
