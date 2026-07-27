function setMapLoading(on) {
    const el = document.getElementById('mapLoading');
    const area = document.querySelector('.viz-area');
    if (el) {
        el.classList.toggle('visible', !!on);
        el.setAttribute('aria-busy', on ? 'true' : 'false');
    }
    if (area) area.classList.toggle('is-loading', !!on);
}

async function fetchData(opts) {
    const silent = !!(opts && opts.silent);
    // Фоновый poll не мешает явной перестройке карты
    if (silent && document.querySelector('.viz-area.is-loading')) return;

    const gen = ++dataFetchGen;
    if (!silent) setMapLoading(true);
    try {
        const groupBy = document.getElementById('groupBy').value;
        const periodQuery = buildPeriodQuery();
        // IP/subnet дают больше уникальных рёбер — берём верхний лимит API.
        const apiLimit = (groupBy === 'ip' || groupBy === 'subnet') ? 50000 : 10000;

        // ВАЖНО: filter не передаём в backend — фильтруем локально на клиенте
        const url = `${API_BASE}/api/events`
            + `?group_by=${encodeURIComponent(groupBy)}`
            + `&limit=${apiLimit}`
            + periodQuery;

        const res = await fetch(url, { cache: 'no-store', credentials: 'same-origin' });
        if (res.status === 401) {
            location.replace(NMAuth.loginUrl(location.pathname));
            return;
        }
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || `HTTP ${res.status}`);
        }
        const data = await res.json();
        if (gen !== dataFetchGen) return;
        allPoints  = data.points || {};
        allLines   = data.lines  || [];
        lastStats  = data.stats  || {};
        lastPeriodInfo = {
            group_by: data.group_by,
            from: data.from,
            to: data.to,
            minutes: data.minutes,
            hours: data.hours,
            days: data.days,
            period: data.period,
        };
        lastFetchError = null;
        backendHealthy = true;

        // Инвалидация кэша stats при новых данных
        _statsCacheVersion++;

        renderStats();
        if (typeof updateReputationMenuUI === 'function') updateReputationMenuUI();
        updateMapOverlay();
        NMUI.fetchSystemHealth();

        refreshMapLayers();
        if (autoFitPending) { fitDeckToData(); autoFitPending = false; }
    } catch (err) {
        if (gen !== dataFetchGen) return;
        console.error(err);
        backendHealthy = false;
        lastFetchError = err.message || String(err);
        updateMapOverlay();
        NMUI.fetchSystemHealth();
        toast('Ошибка загрузки данных: ' + err.message, 'error');
    } finally {
        if (!silent && gen === dataFetchGen) {
            setMapLoading(false);
            updateMapOverlay();
        }
    }
}

function renderStats() {
    let totalEvents = 0;
    let allowedEvents = 0;
    let blockedEvents = 0;
    allLines.forEach(function (l) {
        const c = l.count || 0;
        totalEvents += c;
        if (l.status === 'allowed') allowedEvents += c;
        else if (l.status === 'blocked') blockedEvents += c;
    });

    const uniqueCountries = new Set();
    const uniqueCities = new Set();
    Object.values(allPoints).forEach(p => {
        if (p.country && p.country !== 'Неизвестно') uniqueCountries.add(p.country);
        if (p.city && p.city !== 'Неизвестно') uniqueCities.add(p.city);
    });

    document.getElementById('stat-total').textContent     = fmtNumber(totalEvents);
    document.getElementById('stat-allowed').textContent   = fmtNumber(allowedEvents);
    document.getElementById('stat-blocked').textContent   = fmtNumber(blockedEvents);
    document.getElementById('stat-edges').textContent     = fmtNumber(allLines.length);
    document.getElementById('stat-nodes').textContent     = fmtNumber(Object.keys(allPoints).length);
    document.getElementById('stat-countries').textContent = fmtNumber(uniqueCountries.size);
    document.getElementById('stat-cities').textContent    = fmtNumber(uniqueCities.size);
}

function setFilter(f) {
    currentFilter = f;
    applyFilterUI();
    saveUIState();
    refreshMapLayers();
    updateMapOverlay();
}

function onMinCountChange() {
    const el = document.getElementById('minCount');
    minCount = parseInt(el.value, 10) || 1;
    document.getElementById('minCountVal').textContent = minCount;
    saveUIState();
    refreshMapLayers();
    updateMapOverlay();
}

function onMaxArcsChange() {
    const el = document.getElementById('maxArcs');
    maxArcs = parseInt(el.value, 10) || MAX_ARCS_DEFAULT;
    document.getElementById('maxArcsVal').textContent = fmtNumber(maxArcs);
    saveUIState();
    refreshMapLayers();
    updateMapOverlay();
}

function refreshMap() { fetchData(); }

function resetView() {
    currentFilter = 'all';
    autoFitPending = true;
    currentSearch = '';
    minCount = 1;
    maxArcs = MAX_ARCS_DEFAULT;
    focusedCountry = null;
    document.getElementById('searchInput').value = '';
    document.getElementById('groupBy').value = 'city';
    document.getElementById('periodPreset').value = '1d';
    document.getElementById('periodFrom').value = '';
    document.getElementById('periodTo').value = '';
    periodCustomOpen = false;
    updateCustomPeriodLabel();
    syncPeriodCustomPanel();
    document.getElementById('minCount').value = 1;
    document.getElementById('minCountVal').textContent = '1';
    document.getElementById('maxArcs').value = MAX_ARCS_DEFAULT;
    document.getElementById('maxArcsVal').textContent = fmtNumber(MAX_ARCS_DEFAULT);
    if (typeof clearReputationFilters === 'function') {
        repFilterCategories.clear();
        repFilterLists.clear();
        repFilterSide = 'any';
        const sideEl = document.getElementById('repFilterSide');
        if (sideEl) sideEl.value = 'any';
        if (typeof updateReputationMenuUI === 'function') updateReputationMenuUI();
    }
    applyFilterUI();
    mapViewState = { ...DEFAULT_MAP_VIEW };
    globeViewState = { ...DEFAULT_GLOBE_VIEW };
    if (maplibreMap) {
        const vs = viewMode === 'globe' ? globeViewState : mapViewState;
        maplibreMap.jumpTo({
            center: [vs.longitude, vs.latitude],
            zoom: vs.zoom,
            bearing: vs.bearing || 0,
            pitch: vs.pitch || 0,
        });
        if (viewMode === 'globe' && autoRotate) startGlobeAutoRotate();
    }
    saveUIState();
    refreshMap();
}

function onToggleLegend() {
    showLegend = document.getElementById('toggleLegendChk').checked;
    document.getElementById('legendBox').classList.toggle('hidden', !showLegend);
}
function onToggleStats() {
    showStats = document.getElementById('toggleStatsChk').checked;
    document.getElementById('statsFloating').classList.toggle('hidden', !showStats);
}
function onToggleHeatmap() {
    showHeatmap = document.getElementById('toggleHeatmapChk').checked;
    saveUIState();
    refreshMapLayers();
}
function onToggleCountryLabels() {
    showCountryLabels = document.getElementById('toggleCountryLabelsChk').checked;
    saveUIState();
    refreshMapLayers();
}
function syncLegendMode() {
    const byStatus = document.getElementById('legendByStatus');
    const mono = document.getElementById('legendMono');
    const title = document.getElementById('legendTitle');
    if (!byStatus || !mono) return;
    byStatus.style.display = monoArcColor ? 'none' : '';
    mono.style.display = monoArcColor ? '' : 'none';
    if (title) title.textContent = monoArcColor ? 'Связи' : 'Статус трафика';
}
function onToggleMonoArcs() {
    monoArcColor = document.getElementById('toggleMonoArcsChk').checked;
    syncLegendMode();
    saveUIState();
    refreshMapLayers();
}
