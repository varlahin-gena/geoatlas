function statusRGB(status) {
    if (status === 'blocked') return [248, 81, 73];
    if (status === 'unknown') return [110, 118, 129];
    return [63, 185, 80];
}

/** Единый цвет дуг (режим «один цвет»). */
function monoArcRGB() {
    return [88, 166, 255];
}

function arcRGB(status) {
    return monoArcColor ? monoArcRGB() : statusRGB(status);
}

// Приглушённая палитра: страны без трафика = цвет суши, гамма мягче,
// чтобы мелкие значения не заливали полкарты.
function heatmapColorRGB(value, max) {
    if (!max || value <= 0) return cssRgb('--map-land-rgb', 255);   // цвет суши
    const t = Math.pow(value / max, 0.7);
    const stops = [
        { p: 0.0, c: [40, 60, 90]   },
        { p: 0.5, c: [70, 130, 180] },
        { p: 0.8, c: [214, 158, 46] },
        { p: 1.0, c: [220, 60, 50]  },
    ];
    for (let i = 1; i < stops.length; i++) {
        if (t <= stops[i].p) {
            const a = stops[i-1], b = stops[i];
            const f = (t - a.p) / (b.p - a.p);
            const c = a.c.map((v, j) => Math.round(v + (b.c[j] - v) * f));
            return [c[0], c[1], c[2], 210];
        }
    }
    return [220, 60, 50, 210];
}

function matchCountryFeature(feature, country) {
    if (!feature || !country) return false;
    const p = feature.properties || {};
    const candidates = [
        p.name, p.name_long, p.admin, p.sovereignt,
        p.ADMIN, p.NAME, p.NAME_LONG, p.ISO_A2, p.ISO_A3,
        p.iso_a2, p.iso_a3,
    ].filter(Boolean).map(x => String(x).toLowerCase());
    const target = String(country).toLowerCase();
    if (candidates.includes(target)) return true;
    const ruToEn = Object.entries(countryNamesRu)
        .filter(([_, ru]) => ru.toLowerCase() === target)
        .map(([en]) => en.toLowerCase());
    return ruToEn.some(en => candidates.includes(en));
}

function buildCountryStats() {
    const stats = {};
    getVisiblePoints().forEach(p => {
        if (!p || !p.country || p.country === 'Неизвестно') return;
        stats[p.country] = (stats[p.country] || 0) + (p.count || 0);
    });
    return stats;
}

function featureCountryName(feature) {
    const p = feature.properties || {};
    return p.name_long || p.NAME_LONG || p.admin || p.ADMIN || p.name || p.NAME || '';
}

function precomputeFeatureHeat(features, stats) {
    const heat = new Map();
    for (const f of features) {
        let v = 0;
        for (const country of Object.keys(stats)) {
            if (matchCountryFeature(f, country)) { v = stats[country]; break; }
        }
        heat.set(f, v);
    }
    return heat;
}

function statsSignature() {
    return `${_statsCacheVersion}|${currentGroupBy()}|${currentFilter}|${currentSearch}|${minCount}`;
}

function getStatsCache() {
    const sig = statsSignature();
    if (_statsCache && _statsCache.sig === sig) return _statsCache;
    const stats = buildCountryStats();
    const max = Math.max(0, ...Object.values(stats));
    const features = (countriesGeoJSON && countriesGeoJSON.features) ? countriesGeoJSON.features : [];
    const heat = precomputeFeatureHeat(features, stats);
    _statsCache = { stats, max, heat, sig };
    return _statsCache;
}

function polygonBBoxCentroid(coords) {
    const ring = coords[0];
    if (!ring || !ring.length) return null;
    let minX = 180, maxX = -180, minY = 90, maxY = -90;
    for (const p of ring) {
        if (p[0] < minX) minX = p[0];
        if (p[0] > maxX) maxX = p[0];
        if (p[1] < minY) minY = p[1];
        if (p[1] > maxY) maxY = p[1];
    }
    return [(minX + maxX) / 2, (minY + maxY) / 2, (maxX - minX) * (maxY - minY)];
}

function featureCentroid(feature) {
    const g = feature.geometry;
    if (!g) return null;
    if (g.type === 'Polygon') {
        const c = polygonBBoxCentroid(g.coordinates);
        return c ? { lon: c[0], lat: c[1], area: c[2] } : null;
    }
    if (g.type === 'MultiPolygon') {
        let best = null;
        for (const poly of g.coordinates) {
            const c = polygonBBoxCentroid(poly);
            if (!c) continue;
            if (!best || c[2] > best[2]) best = c;
        }
        return best ? { lon: best[0], lat: best[1], area: best[2] } : null;
    }
    return null;
}

function buildCountryCentroids() {
    if (countryCentroidsCache) return countryCentroidsCache;
    if (!countriesGeoJSON || !countriesGeoJSON.features) return [];
    const result = [];
    for (const f of countriesGeoJSON.features) {
        const name = featureCountryName(f);
        if (!name) continue;
        const c = featureCentroid(f);
        if (!c) continue;
        const p = f.properties || {};
        const labelRank = p.LABELRANK ?? p.labelrank ?? p.label_rank;
        let importance;
        if (typeof labelRank === 'number') {
            importance = labelRank;
        } else {
            const area = c.area || 0;
            importance = area > 1000 ? 2 : area > 200 ? 4 : area > 50 ? 6 : 9;
        }
        if (importance > COUNTRY_LABEL_MAX_RANK) continue;
        result.push({
            name, lat: c.lat, lon: c.lon,
            label: ruCountry(name),
            rank: importance,
        });
    }
    countryCentroidsCache = result;
    return result;
}

/** Подпись на лицевой полусфере глобуса относительно центра камеры. */
function isOnVisibleGlobeHemisphere(lon, lat, viewLon, viewLat) {
    const toRad = Math.PI / 180;
    const φ1 = (viewLat || 0) * toRad;
    const λ1 = (viewLon || 0) * toRad;
    const φ2 = lat * toRad;
    const λ2 = lon * toRad;
    // cos(углового расстояния); > 0 — передняя полусфера
    const cosC = Math.sin(φ1) * Math.sin(φ2)
        + Math.cos(φ1) * Math.cos(φ2) * Math.cos(λ2 - λ1);
    // небольшой запас, чтобы не показывать подписи у самого лимба
    return cosC > 0.12;
}

function visibleGlobeCentroids() {
    const all = buildCountryCentroids();
    const viewLon = globeViewState.longitude || 0;
    const viewLat = globeViewState.latitude || 0;
    return all.filter((c) => isOnVisibleGlobeHemisphere(c.lon, c.lat, viewLon, viewLat));
}

let lastGlobeLabelCullKey = '';

/** Обновить слой подписей при повороте камеры (не каждый кадр). */
function maybeRefreshGlobeLabels(force) {
    if (!deckInstance || viewMode !== 'globe' || !showCountryLabels) return;
    const lon = Math.round((globeViewState.longitude || 0) * 2) / 2;
    const lat = Math.round((globeViewState.latitude || 0) * 2) / 2;
    const key = lon + ':' + lat;
    if (!force && key === lastGlobeLabelCullKey) return;
    lastGlobeLabelCullKey = key;
    deckInstance.setProps({ layers: buildDeckLayers('globe') });
}

function lineMatchesSearch(line, pointMap) {
    if (!currentSearch) return true;
    const fields = [
        line.src, line.dst, line.src_label, line.dst_label,
        line.rule, line.proto, line.device, line.last_action,
        line.src_zone, line.dst_zone, line.src_country, line.dst_country
    ];
    if (fields.some(f => normalizeText(f).includes(currentSearch))) return true;
    const srcP = pointMap[line.src], dstP = pointMap[line.dst];
    const ext = [];
    if (srcP) ext.push(srcP.city, srcP.country, srcP.region, srcP.label);
    if (dstP) ext.push(dstP.city, dstP.country, dstP.region, dstP.label);
    return ext.some(v => normalizeText(v).includes(currentSearch));
}

function pointMatchesSearch(key, point) {
    if (!currentSearch) return true;
    return [key, point.label, point.city, point.region, point.country]
        .some(v => normalizeText(v).includes(currentSearch));
}

function getVisibleLines() {
    return allLines.filter(line => {
        if (currentFilter === 'allowed' && line.status !== 'allowed') return false;
        if (currentFilter === 'blocked' && line.status !== 'blocked') return false;
        if ((line.count || 0) < minCount) return false;
        // Self-loop (оба конца схлопнулись в один ключ) — дуга нулевой длины, не рисуем.
        if (line.src && line.src === line.dst) return false;
        if (!lineMatchesSearch(line, allPoints)) return false;
        return hasCoords(line);
    });
}

function getVisiblePoints(visibleLines) {
    const lines = visibleLines || getVisibleLines();
    const active = new Set();
    lines.forEach(l => { active.add(l.src); active.add(l.dst); });
    const fromDrawnArcs = Array.isArray(visibleLines);
    const result = [];
    Object.entries(allPoints).forEach(([key, p]) => {
        if (!p) return;
        // Только концы отрисованных дуг. Раньше при поиске добавляли ещё и
        // совпавшие узлы вне top-N — получались «осиротевшие» маркеры
        // (например центр Канады) без дуги из этой точки.
        if (fromDrawnArcs) {
            if (!active.has(key)) return;
        } else {
            const matchesSearch = currentSearch && pointMatchesSearch(key, p);
            if (!active.has(key) && !matchesSearch) return;
        }
        if (p.lat === 0 && p.lon === 0) return;
        result.push({ key, ...p });
    });
    return result;
}

// ============================================================
// Map (deck.gl)
// ============================================================

// Детерминированный «разброс» дуги по паре узлов —
// чтобы параллельные линии между одними регионами не сливались в пучок.
function arcTilt(d) {
    let h = 0;
    const s = (d.src || '') + '>' + (d.dst || '');
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
    return ((h % 7) - 3) * 2; // -6..+6 градусов (было -15..+15)
}

// Непрозрачная подложка сферы — официальный способ для GlobeView (без BitmapLayer)
const GLOBE_SURFACE = [
    [[-180, 90], [0, 90], [180, 90], [180, -90], [0, -90], [-180, -90]],
];

function buildGlobeBaseLayer() {
    if (!deck.SolidPolygonLayer) return null;
    return new deck.SolidPolygonLayer({
        id: 'globe-base',
        data: GLOBE_SURFACE,
        getPolygon: d => d,
        stroked: false,
        filled: true,
        pickable: false,
        getFillColor: cssRgb('--map-base-rgb', 255),
    });
}

function buildDeckLayers(mode = 'map') {
    const isGlobe = mode === 'globe';
    let lines = getVisibleLines();
    const totalLines = lines.length;
    lines = topByCount(lines, maxArcs);
    const points = getVisiblePoints(lines);
    updateArcCountInfo(lines.length, totalLines);
    const layers = [];
    const landColor = cssRgb('--map-land-rgb', 255);
    const outlineColor = cssRgb('--map-outline-rgb', 255);
    const outlineSoft = cssRgb('--map-outline-rgb', 160);

    if (isGlobe) {
        const base = buildGlobeBaseLayer();
        if (base) layers.push(base);
    }

    if (countriesGeoJSON) {
        const { max, heat } = getStatsCache();
        layers.push(new deck.GeoJsonLayer({
            id: 'countries',
            data: countriesGeoJSON,
            pickable: false,
            stroked: !isGlobe,
            filled: true,
            wrapLongitude: isGlobe,
            getFillColor: f => {
                const c = heatmapEnabled()
                    ? heatmapColorRGB(heat.get(f) || 0, max)
                    : landColor;
                // На глобусе полная непрозрачность — иначе видны щели между треугольниками сетки
                return isGlobe ? [c[0], c[1], c[2], 255] : c;
            },
            getLineColor: outlineColor,
            getLineWidth: 1,
            lineWidthMinPixels: 0.5,
            updateTriggers: {
                getFillColor: [showHeatmap, currentGroupBy(), statsSignature(), isGlobe, NMAuth.getTheme()],
                getLineColor: [NMAuth.getTheme()],
            },
        }));
    }

    if (showCountryLabels) {
        const centroids = isGlobe ? visibleGlobeCentroids() : buildCountryCentroids();
        // Небольшой altitude на глобусе — иначе подписи z-fight с полигонами стран
        const labelAlt = isGlobe ? 8e4 : 0;
        const labelColor = NMAuth.getTheme() === 'light'
            ? [31, 35, 40, 230]
            : [220, 226, 234, 230];
        const viewLon = isGlobe ? (globeViewState.longitude || 0) : 0;
        const viewLat = isGlobe ? (globeViewState.latitude || 0) : 0;
        layers.push(new deck.TextLayer({
            id: 'country-labels',
            data: centroids,
            pickable: false,
            billboard: true,
            characterSet: LABEL_CHARSET,
            getPosition: d => [d.lon, d.lat, labelAlt],
            getText: d => d.label,
            getSize: d => 14 - (d.rank || 3),
            sizeUnits: 'pixels',
            sizeMinPixels: 9,
            sizeMaxPixels: 18,
            getColor: labelColor,
            outlineColor: outlineColor,
            outlineWidth: 4,
            fontSettings: { sdf: true, fontSize: 64, buffer: 4 },
            fontFamily: 'Arial, "Segoe UI", Roboto, sans-serif',
            fontWeight: 700,
            getTextAnchor: 'middle',
            getAlignmentBaseline: 'center',
            // GlobeView: billboard ошибочно cull'ится (deck.gl#9777) —
            // depthTest выключен, видимость с обратной стороны режем фильтром полусферы.
            parameters: isGlobe ? { cullMode: 'none', depthTest: false } : undefined,
            updateTriggers: {
                getText: centroids.map(c => c.label).join('|'),
                getSize: centroids.length,
                getPosition: [labelAlt, viewLon, viewLat],
                getColor: [NMAuth.getTheme()],
                outlineColor: [NMAuth.getTheme()],
            },
        }));
    }

    layers.push(new deck.ArcLayer({
        id: 'arcs',
        data: lines,
        pickable: true,
        greatCircle: isGlobe,
        // Позиции дуг берём из узлов — иначе при расхождении
        // line.src_* и point.lat/lon (avg по ребру vs avg по узлу)
        // дуга стартует «в стороне» от маркера.
        getSourcePosition: d => nodeLonLat(d.src, d.src_lon, d.src_lat),
        getTargetPosition: d => nodeLonLat(d.dst, d.dst_lon, d.dst_lat),
        getSourceColor: d => [...arcRGB(d.status), 210],
        getTargetColor: d => [...arcRGB(d.status), 210],
        getWidth: d => Math.max(1, Math.min(6, 1 + Math.log2((d.count || 1) + 1))),
        widthUnits: 'pixels',
        getHeight: isGlobe
            ? d => {
                const [sLon, sLat] = nodeLonLat(d.src, d.src_lon, d.src_lat);
                const [tLon, tLat] = nodeLonLat(d.dst, d.dst_lon, d.dst_lat);
                const dist = Math.max(1, Math.hypot(Math.abs(tLon - sLon), Math.abs(tLat - sLat)));
                return Math.max(0.05, Math.min(0.35, 10 / dist));
            }
            : d => {
                const [sLon, sLat] = nodeLonLat(d.src, d.src_lon, d.src_lat);
                const [tLon, tLat] = nodeLonLat(d.dst, d.dst_lon, d.dst_lat);
                const dist = Math.max(1, Math.hypot(Math.abs(tLon - sLon), Math.abs(tLat - sLat)));
                return Math.max(0.07, Math.min(0.35, 14 / dist));
            },
        getTilt: isGlobe ? 0 : arcTilt,
        autoHighlight: true,
        highlightColor: [255, 255, 255, 140],
        parameters: { depthTest: isGlobe },
        updateTriggers: {
            getSourceColor: [currentFilter, monoArcColor],
            getTargetColor: [currentFilter, monoArcColor],
            getSourcePosition: [_statsCacheVersion],
            getTargetPosition: [_statsCacheVersion],
        },
        onClick: info => { if (info.object) showLineDetail(info.object); },
    }));

    layers.push(new deck.ScatterplotLayer({
        id: 'nodes',
        data: points,
        pickable: true,
        stroked: true,
        filled: true,
        radiusUnits: 'pixels',
        getPosition: d => [d.lon, d.lat],
        getRadius: d => Math.max(1.5, Math.min(8, 1.5 + Math.sqrt(d.count || 1) * 0.6)),
        getFillColor: [88, 166, 255, 150],
        getLineColor: outlineSoft,
        lineWidthUnits: 'pixels',
        getLineWidth: 0.7,
        radiusMinPixels: 1.5,
        radiusMaxPixels: 9,
        updateTriggers: {
            getLineColor: [NMAuth.getTheme()],
        },
        onClick: info => { if (info.object) showPointDetail(info.object, info.object.key); },
    }));

    return layers;
}

function getDeckTooltip({ object, layer }) {
    if (!object) return null;
    if (layer && layer.id === 'arcs') {
        return {
            html: `<b>${escapeHTML(object.src_label || object.src)} → ${escapeHTML(object.dst_label || object.dst)}</b><br>
                   Статус: ${escapeHTML(object.status)} · События: ${fmtNumber(object.count)}`,
            style: { background: 'rgba(22,27,34,0.95)', color: '#c9d1d9', border: '1px solid #30363d',
                     borderRadius: '6px', padding: '6px 10px', fontSize: '11px' }
        };
    }
    if (layer && layer.id === 'nodes') {
        return {
            html: `<b>${escapeHTML(object.label || object.key)}</b><br>
                   ${escapeHTML(object.city || '')} · ${escapeHTML(ruCountry(object.country))}<br>
                   События: ${fmtNumber(object.count)}`,
            style: { background: 'rgba(22,27,34,0.95)', color: '#c9d1d9', border: '1px solid #30363d',
                     borderRadius: '6px', padding: '6px 10px', fontSize: '11px' }
        };
    }
    return null;
}

function updateArcCountInfo(shown, total) {
    const el = document.getElementById('arcCountInfo');
    if (!el) return;
    if (total > shown) {
        el.textContent = `${fmtNumber(shown)} из ${fmtNumber(total)}`;
        el.style.color = 'var(--orange)';
    } else {
        el.textContent = `${fmtNumber(shown)} связей`;
        el.style.color = 'var(--text-muted)';
    }
    updateArcsTruncHint(shown, total);
}

function initDeck() {
    if (deckInstance) return;
    deckInstance = new deck.Deck({
        parent: document.getElementById('map-host'),
        views: new deck.MapView(),
        initialViewState: mapViewState,
        controller: true,
        layers: buildDeckLayers('map'),
        getTooltip: getDeckTooltip,
        onViewStateChange: ({ viewState }) => {
            mapViewState = viewState;
            deckInstance.setProps({ viewState });
        },
        parameters: { preserveDrawingBuffer: true }
    });
}

function updateDeck() {
    if (!deckInstance) return;
    deckInstance.setProps({ layers: buildDeckLayers('map') });
}

function destroyDeck() {
    stopGlobeAutoRotate();
    if (!deckInstance) return;
    deckInstance.finalize();
    deckInstance = null;
    document.getElementById('map-host').innerHTML = '';
}

function fitDeckToData() {
    const lines = allLines.filter(hasCoords);
    if (!lines.length) return;
    let minLon = 180, maxLon = -180, minLat = 90, maxLat = -90;
    lines.forEach(l => {
        minLon = Math.min(minLon, l.src_lon, l.dst_lon);
        maxLon = Math.max(maxLon, l.src_lon, l.dst_lon);
        minLat = Math.min(minLat, l.src_lat, l.dst_lat);
        maxLat = Math.max(maxLat, l.src_lat, l.dst_lat);
    });
    if (minLon === 180) return;
    const lonSpan = maxLon - minLon, latSpan = maxLat - minLat;
    const zoom = Math.min(6, Math.max(1, 8 - Math.log2(Math.max(lonSpan, latSpan, 1) + 1)));
    mapViewState = { ...mapViewState, longitude: (minLon + maxLon) / 2, latitude: (minLat + maxLat) / 2, zoom };
    if (deckInstance) deckInstance.setProps({ initialViewState: mapViewState });
}

// ============================================================
// Globe (deck.gl GlobeView)
// ============================================================

function getGlobeViewClass() {
    // В standalone bundle экспортируется как _GlobeView (experimental API)
    return deck._GlobeView || deck.GlobeView || null;
}

function stopGlobeAutoRotate() {
    if (globeRotateRAF) {
        cancelAnimationFrame(globeRotateRAF);
        globeRotateRAF = null;
    }
    globeRotateLastTs = 0;
}

function startGlobeAutoRotate() {
    stopGlobeAutoRotate();
    if (!autoRotate || viewMode !== 'globe' || !deckInstance) return;

    function tick(ts) {
        if (!deckInstance || viewMode !== 'globe' || !autoRotate) {
            stopGlobeAutoRotate();
            return;
        }
        if (!globeRotateLastTs) globeRotateLastTs = ts;
        const dt = Math.min(ts - globeRotateLastTs, 50);
        globeRotateLastTs = ts;
        globeViewState = {
            ...globeViewState,
            longitude: ((globeViewState.longitude || 0) + dt * 0.008) % 360,
        };
        deckInstance.setProps({ viewState: globeViewState });
        maybeRefreshGlobeLabels(false);
        globeRotateRAF = requestAnimationFrame(tick);
    }
    globeRotateRAF = requestAnimationFrame(tick);
}

function initGlobe() {
    if (deckInstance) return;
    const GlobeView = getGlobeViewClass();
    if (!GlobeView) {
        toast('GlobeView недоступен в этой версии deck.gl', 'error');
        return;
    }
    try {
        deckInstance = new deck.Deck({
            parent: document.getElementById('globe-host'),
            views: new GlobeView({ resolution: 2 }),
            initialViewState: globeViewState,
            controller: true,
            layers: buildDeckLayers('globe'),
            getTooltip: getDeckTooltip,
            onViewStateChange: ({ viewState }) => {
                globeViewState = viewState;
                deckInstance.setProps({ viewState });
                maybeRefreshGlobeLabels(false);
            },
            style: { background: mapBaseCss(), position: 'absolute', inset: '0' },
            parameters: {
                preserveDrawingBuffer: true,
                clearColor: mapClearColor(),
                cullMode: 'back',
            },
        });
        startGlobeAutoRotate();
    } catch (e) {
        console.error('initGlobe failed:', e);
        deckInstance = null;
        toast('Ошибка инициализации глобуса: ' + e.message, 'error');
    }
}

function updateGlobe() {
    if (!deckInstance) return;
    lastGlobeLabelCullKey = '';
    deckInstance.setProps({ layers: buildDeckLayers('globe') });
}

function destroyGlobe() {
    stopGlobeAutoRotate();
    if (!deckInstance) return;
    deckInstance.finalize();
    deckInstance = null;
    document.getElementById('globe-host').innerHTML = '';
}

// ============================================================
// View mode
// ============================================================

function setViewMode(mode) {
    if (mode === viewMode) return;
    viewMode = mode;

    document.getElementById('mode-map').classList.toggle('active', mode === 'map');
    document.getElementById('mode-globe').classList.toggle('active', mode === 'globe');
    document.getElementById('mode-map-icon').classList.toggle('active', mode === 'map');
    document.getElementById('mode-globe-icon').classList.toggle('active', mode === 'globe');

    document.getElementById('autoRotateWrap').style.display = (mode === 'globe') ? '' : 'none';

    document.getElementById('map-host').classList.toggle('hidden', mode !== 'map');
    document.getElementById('globe-host').classList.toggle('hidden', mode !== 'globe');

    if (mode === 'map') {
        destroyGlobe();
        initDeck();
        updateDeck();
        if (autoFitPending) { fitDeckToData(); autoFitPending = false; }
    } else {
        destroyDeck();
        initGlobe();
        if (!deckInstance) {
            viewMode = 'map';
            document.getElementById('mode-map').classList.add('active');
            document.getElementById('mode-globe').classList.remove('active');
            document.getElementById('mode-map-icon').classList.add('active');
            document.getElementById('mode-globe-icon').classList.remove('active');
            document.getElementById('autoRotateWrap').style.display = 'none';
            document.getElementById('map-host').classList.remove('hidden');
            document.getElementById('globe-host').classList.add('hidden');
            initDeck();
            updateDeck();
            saveUIState();
            return;
        }
        updateGlobe();
        if (autoRotate) startGlobeAutoRotate();
    }
    saveUIState();
    setTimeout(() => resizeCurrentView(), 100);
}

async function loadCountries() {
    try {
        const res = await fetch('/data/countries.geojson');
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        countriesGeoJSON = await res.json();
        countryCentroidsCache = null;
        _statsCacheVersion++;
    } catch (e) {
        console.warn('countries.geojson not loaded:', e);
        countriesGeoJSON = null;
        countryCentroidsCache = null;
    }
}
