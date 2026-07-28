'use strict';

function bindUI() {
    const $ = (id) => document.getElementById(id);

    $('mode-map')?.addEventListener('click', () => setViewMode('map'));
    $('mode-globe')?.addEventListener('click', () => setViewMode('globe'));
    $('mode-map-icon')?.addEventListener('click', () => setViewMode('map'));
    $('mode-globe-icon')?.addEventListener('click', () => setViewMode('globe'));

    $('btnUploadLogs')?.addEventListener('click', () => $('logFile').click());
    $('btnUploadGeo')?.addEventListener('click', () => $('geoFile').click());
    $('btnRefreshMap')?.addEventListener('click', () => refreshMap());
    $('btnResetView')?.addEventListener('click', () => resetView());
    $('btnExportPng')?.addEventListener('click', () => exportPNG());
    $('btnToggleSidebar')?.addEventListener('click', () => toggleSidebar());
    $('btnCloseDetail')?.addEventListener('click', () => {
        clearFocusedCountry();
        closeDetail();
    });
    $('btnPeriodApply')?.addEventListener('click', () => applyCustomPeriod());

    $('minCount')?.addEventListener('input', () => onMinCountChange());
    $('maxArcs')?.addEventListener('input', () => onMaxArcsChange());

    $('toggleLegendChk')?.addEventListener('change', () => onToggleLegend());
    $('toggleStatsChk')?.addEventListener('change', () => onToggleStats());
    $('toggleHeatmapChk')?.addEventListener('change', () => onToggleHeatmap());
    $('toggleCountryLabelsChk')?.addEventListener('change', () => onToggleCountryLabels());
    $('toggleMonoArcsChk')?.addEventListener('change', () => onToggleMonoArcs());

    $('groupBy')?.addEventListener('change', () => {
        saveUIState();
        // На глобусе после смены группировки нужен однократный resync GlobeView.
        if (typeof requestGlobeViewResync === 'function') requestGlobeViewResync();
        refreshMap();
    });
    $('periodPreset')?.addEventListener('change', () => onPeriodPresetChange());
    $('periodPreset')?.addEventListener('mousedown', function () {
        this.dataset.periodPrev = this.value;
    });
    $('periodPreset')?.addEventListener('click', function () {
        if (this.value === 'custom' && this.dataset.periodPrev === 'custom' && !periodCustomOpen) {
            openCustomPeriodPanel();
        }
    });

    $('filter-all')?.addEventListener('click', () => setFilter('all'));
    $('filter-allowed')?.addEventListener('click', () => setFilter('allowed'));
    $('filter-blocked')?.addEventListener('click', () => setFilter('blocked'));

    if (typeof bindReputationMenu === 'function') bindReputationMenu();

    $('logFile')?.addEventListener('change', async function () {
        if (this.files && this.files.length) await uploadLogs();
        this.value = '';
    });
    $('geoFile')?.addEventListener('change', async function () {
        if (this.files && this.files.length) await uploadGeo();
        this.value = '';
    });
    $('searchInput')?.addEventListener('input', function () {
        const val = this.value;
        clearTimeout(searchDebounceTimer);
        searchDebounceTimer = setTimeout(() => {
            if (typeof setSearchQuery === 'function') {
                setSearchQuery(val, {
                    syncInput: false,
                    save: true,
                    refresh: true,
                    updateOverlay: true,
                    keepBuilderOpen: true,
                });
            } else {
                currentSearch = normalizeText(val);
                saveUIState();
                refreshMapLayers();
                updateMapOverlay();
            }
        }, SEARCH_DEBOUNCE_MS);
    });
    if (typeof bindSearchBuilderUI === 'function') bindSearchBuilderUI();
    $('autoRotate')?.addEventListener('change', function () {
        autoRotate = this.checked;
        if (viewMode === 'globe') {
            if (autoRotate) startGlobeAutoRotate();
            else stopGlobeAutoRotate();
        }
    });
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            if (typeof searchBuilderPanelOpen === 'function' && searchBuilderPanelOpen()) {
                searchBuilderForceOpen = false;
                syncSearchBuilderUI();
            }
            clearFocusedCountry();
            closeDetail();
        }
    });
    document.addEventListener('visibilitychange', () => {
        if (viewMode !== 'globe') return;
        if (document.hidden) stopGlobeAutoRotate();
        else if (autoRotate) startGlobeAutoRotate();
    });
}

async function init() {
    bindUI();
    nmCurrentUser = await NMAuth.requireLogin();
    if (!nmCurrentUser) return;
    nmIsAdmin = NMAuth.applyAdminVisibility(nmCurrentUser);
    NMAuth.renderUserBar(nmCurrentUser, document.getElementById('userBarHost'));
    document.addEventListener('nm-theme-change', applyMapTheme);

    loadUIState();
    applyViewFromURL();
    applyFilterUI();
    document.getElementById('toggleHeatmapChk').checked = showHeatmap;
    document.getElementById('toggleCountryLabelsChk').checked = showCountryLabels;
    document.getElementById('toggleMonoArcsChk').checked = monoArcColor;
    document.getElementById('autoRotate').checked = autoRotate;
    syncLegendMode();

    document.getElementById('mode-map').classList.toggle('active', viewMode === 'map');
    document.getElementById('mode-globe').classList.toggle('active', viewMode === 'globe');
    document.getElementById('mode-map-icon').classList.toggle('active', viewMode === 'map');
    document.getElementById('mode-globe-icon').classList.toggle('active', viewMode === 'globe');
    document.getElementById('autoRotateWrap').style.display = (viewMode === 'globe') ? '' : 'none';
    if (typeof syncHeatmapToggleVisibility === 'function') syncHeatmapToggleVisibility(viewMode);
    document.getElementById('maxArcs').value = maxArcs;
    document.getElementById('maxArcsVal').textContent = fmtNumber(maxArcs);
    syncViewToURL();

    // Карта сразу: basemap не ждёт countries.geojson /api/events.
    initMapView();
    await Promise.all([loadCountries(), fetchData()]);
    NMUI.startSystemHealthPolling({ isAdmin: nmIsAdmin });

    setInterval(() => { if (isAutoRefresh()) fetchData({ silent: true }); }, REFRESH_DATA_MS);
}

init();
