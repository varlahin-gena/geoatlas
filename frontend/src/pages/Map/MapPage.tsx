import { useEffect, useState } from 'react';
import { useAuth } from '@/auth/AuthContext';
import { filterNav, PAGE_NAV } from '@/components/nav';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';
import { loadCountriesGeoJSON, type GeoFeatureCollection } from './mapHeatmap';
import { MapDetailPanel } from './mapDetail';
import { buildDeckLayers } from './mapLayers';
import { MapSidebar } from './MapSidebar';
import { MapTopbar } from './MapTopbar';
import { MapVizOverlays } from './MapVizOverlays';
import { GeoWizardModal } from './GeoWizardModal';
import { useGeoWizard } from './useGeoWizard';
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

export default function MapPage() {
  const { isAdmin, reputationEnabled, uiAuthEnabled, theme, user, refresh } = useAuth();
  const { toast } = useToast();
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
    points,
    lines,
    loading,
    fetchError,
    eventStats,
    autoRefresh,
    setAutoRefresh,
    dataSource,
    selectDataSource,
    backupAttached,
    periodQuery,
    fetchData,
  } = useMapEvents(toast, {
    filter: view.filter,
    maxArcs: view.debouncedMaxArcs,
    focusedCountry: view.focusedCountry,
    search: view.debouncedSearch,
  });

  const {
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
    repActive,
    repFilterCount,
    repTree,
    ipMode,
  } = useMapReputation(lines, groupBy);

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
    focusedCountry: view.focusedCountry,
  });
  const {
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
    globeView,
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

  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    try {
      return localStorage.getItem('nm.mapSidebarCollapsed') === '1';
    } catch {
      return false;
    }
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
        globeView,
        maxArcs,
        onLineClick: openLineDetail,
        onPointClick: openPointDetail,
        onCountryClick: (key, feature) => openCountryDetail(key, feature),
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
    globeView,
    maxArcs,
    openLineDetail,
    openPointDetail,
    openCountryDetail,
    overlayRef,
    layersRefreshBusy,
  ]);

  const adminLinks = filterNav(PAGE_NAV, {
    isAdmin,
    reputationEnabled,
    uiAuthEnabled,
    adminLinksOnly: true,
  });

  function toggleSidebar() {
    setSidebarCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem('nm.mapSidebarCollapsed', next ? '1' : '0');
      } catch {
        /* ignore */
      }
      return next;
    });
  }

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
    <div className={`app${sidebarCollapsed ? ' sidebar-collapsed' : ''}`} id="app">
      <a className="skip-link" href="#map-main">
        К содержимому
      </a>
      <MapSidebar
        view={{ viewMode, setViewMode, autoRotate, setAutoRotate }}
        isAdmin={isAdmin}
        uploads={{ logFileRef, geoFileRef, uploadFile }}
        actions={{ fetchData, resetView, exportPng, toggleSidebar }}
        adminLinks={adminLinks}
        geoWizard={{
          open: geoWizard.open,
          empty: geoWizard.geo != null && geoWizard.geo.count === 0,
        }}
        viz={{
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
        }}
        data={{
          autoRefresh,
          setAutoRefresh,
          dataSource,
          selectDataSource,
          backupAttached,
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
        />

        <div
          className={`viz-area${loading ? ' is-loading' : ''}${showGeoEmptyBanner ? ' has-geo-empty-banner' : ''}${truncHint ? ' has-viz-hint' : ''}${showLegend ? ' has-map-legend' : ''}`}
        >
          <div ref={mapContainer} id="map-host" className="viz-host" />

          {showGeoEmptyBanner ? (
            <div className="geo-wizard-banner" role="status">
              <p>База GeoIP пуста — карта не покажет дуги без координат.</p>
              <button type="button" className="btn primary" onClick={geoWizard.open}>
                Мастер GeoIP
              </button>
            </div>
          ) : null}

          <MapVizOverlays
            truncHint={truncHint}
            emptyOverlay={displayEmptyOverlay}
            loading={loading}
            showLegend={showLegend}
            monoArcs={monoArcs}
            repColorArcs={repColorArcs}
            showStats={showStats}
            stats={stats}
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
          onDryRun={(file) => void geoWizard.runDryRun(file)}
          onCommit={() => void geoWizard.commitUpload()}
          onWaitForCurl={() => void geoWizard.waitForGeo()}
        />
      ) : null}
    </div>
  );
}
