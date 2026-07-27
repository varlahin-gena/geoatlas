function statusRGB(status) {
    if (status === 'blocked') return [248, 81, 73];
    if (status === 'unknown') return [110, 118, 129];
    return [63, 185, 80];
}

/** Единый цвет дуг (режим «один цвет»). */
function monoArcRGB() {
    return [88, 166, 255];
}

function arcRGB(status, line) {
    if (monoArcColor) return monoArcRGB();
    if (typeof repColorArcs !== 'undefined' && repColorArcs &&
        line && typeof lineHasReputationHits === 'function' && lineHasReputationHits(line)) {
        return [210, 153, 34];
    }
    return statusRGB(status);
}

// Приглушённая палитра: страны без трафика = цвет суши, гамма мягче.
function heatmapColorRGB(value, max) {
    if (!max || value <= 0) return cssRgb('--map-land-rgb', mapTilesFailed ? 255 : 0);
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

function resolveCountryKeyFromFeature(feature) {
    if (!feature) return '';
    const stats = getStatsCache().stats || {};
    for (const country of Object.keys(stats)) {
        if (matchCountryFeature(feature, country)) return country;
    }
    return featureCountryName(feature);
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
    return `${_statsCacheVersion}|${currentGroupBy()}|${currentFilter}|${currentSearch}|${minCount}|${focusedCountry || ''}`;
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

function isOnVisibleGlobeHemisphere(lon, lat, viewLon, viewLat) {
    const toRad = Math.PI / 180;
    const φ1 = (viewLat || 0) * toRad;
    const λ1 = (viewLon || 0) * toRad;
    const φ2 = lat * toRad;
    const λ2 = lon * toRad;
    const cosC = Math.sin(φ1) * Math.sin(φ2)
        + Math.cos(φ1) * Math.cos(φ2) * Math.cos(λ2 - λ1);
    return cosC > 0.12;
}

function visibleGlobeCentroids() {
    const all = buildCountryCentroids();
    const viewLon = globeViewState.longitude || 0;
    const viewLat = globeViewState.latitude || 0;
    return all.filter((c) => isOnVisibleGlobeHemisphere(c.lon, c.lat, viewLon, viewLat));
}

let lastGlobeLabelCullKey = '';

function maybeRefreshGlobeLabels(force) {
    if (viewMode !== 'globe' || !showCountryLabels) return;
    const lon = Math.round((globeViewState.longitude || 0) * 2) / 2;
    const lat = Math.round((globeViewState.latitude || 0) * 2) / 2;
    const key = lon + ':' + lat;
    if (!force && key === lastGlobeLabelCullKey) return;
    lastGlobeLabelCullKey = key;
    refreshMapLayers();
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

function lineMatchesFocusedCountry(line) {
    if (!focusedCountry) return true;
    const target = String(focusedCountry).toLowerCase();
    const src = String(line.src_country || '').toLowerCase();
    const dst = String(line.dst_country || '').toLowerCase();
    if (src === target || dst === target) return true;
    const ru = Object.entries(countryNamesRu)
        .filter(([en]) => en.toLowerCase() === target)
        .map(([, ruName]) => ruName.toLowerCase());
    if (ru.some(r => src === r || dst === r)) return true;
    // Also match via point countries
    const sp = allPoints[line.src], dp = allPoints[line.dst];
    if (sp && String(sp.country || '').toLowerCase() === target) return true;
    if (dp && String(dp.country || '').toLowerCase() === target) return true;
    return false;
}

function getVisibleLines() {
    const ipMode = typeof currentGroupBy === 'function' && currentGroupBy() === 'ip';
    const repActive = ipMode && typeof reputationFilterActiveCount === 'function' && reputationFilterActiveCount() > 0;
    return allLines.filter(line => {
        if (currentFilter === 'allowed' && line.status !== 'allowed') return false;
        if (currentFilter === 'blocked' && line.status !== 'blocked') return false;
        if ((line.count || 0) < minCount) return false;
        if (line.src && line.src === line.dst) return false;
        if (!lineMatchesSearch(line, allPoints)) return false;
        if (!lineMatchesFocusedCountry(line)) return false;
        if (repActive && typeof lineMatchesReputation === 'function' && !lineMatchesReputation(line)) return false;
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

/** Rank-based alpha + corridor tilt for country pairs. */
function decorateFlowLines(lines) {
    if (!lines.length) return lines;
    const sorted = [...lines].sort((a, b) => (b.count || 0) - (a.count || 0));
    const n = sorted.length;
    const corridorTilt = new Map();
    return sorted.map((line, idx) => {
        const rankT = n <= 1 ? 1 : 1 - idx / (n - 1);
        const alpha = Math.round(60 + rankT * 150);
        const pairKey = [
            String(line.src_country || ''),
            String(line.dst_country || ''),
        ].sort().join('>');
        let tilt = corridorTilt.get(pairKey);
        if (tilt === undefined) {
            let h = 0;
            for (let i = 0; i < pairKey.length; i++) h = (h * 31 + pairKey.charCodeAt(i)) | 0;
            tilt = ((h % 7) - 3) * 2;
            corridorTilt.set(pairKey, tilt);
        }
        return Object.assign({}, line, { _flowAlpha: alpha, _flowTilt: tilt, _flowRank: idx });
    });
}

function arcTilt(d) {
    if (typeof d._flowTilt === 'number') return d._flowTilt;
    let h = 0;
    const s = (d.src || '') + '>' + (d.dst || '');
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
    return ((h % 7) - 3) * 2;
}

/** Max latitude along a great-circle between two endpoints. */
function greatCircleMaxLat(lon1, lat1, lon2, lat2) {
    let maxLat = Math.max(lat1, lat2);
    for (let i = 1; i < 9; i++) {
        const [, lat] = greatCirclePoint(lon1, lat1, lon2, lat2, i / 10);
        maxLat = Math.max(maxLat, lat);
    }
    return maxLat;
}

/** Arc height for 2D flat (non–great-circle) ArcLayer. */
function mapArcHeight(d) {
    const [sLon, sLat] = nodeLonLat(d.src, d.src_lon, d.src_lat);
    const [tLon, tLat] = nodeLonLat(d.dst, d.dst_lon, d.dst_lat);
    let dLon = Math.abs(tLon - sLon);
    if (dLon > 180) dLon = 360 - dLon;
    const dist = Math.max(1, Math.hypot(dLon, Math.abs(tLat - sLat)));
    // Без greatCircle дуга идёт по хорде; лёгкий pitch карты поднимает «горб» в кадр.
    return Math.max(0.15, Math.min(0.35, 0.1 + dist / 160));
}

/** Arc height on globe — ниже, чтобы дуги не вылезали над сферой. */
function globeArcHeight(d) {
    const [sLon, sLat] = nodeLonLat(d.src, d.src_lon, d.src_lat);
    const [tLon, tLat] = nodeLonLat(d.dst, d.dst_lon, d.dst_lat);
    let dLon = Math.abs(tLon - sLon);
    if (dLon > 180) dLon = 360 - dLon;
    const dist = Math.max(1, Math.hypot(dLon, Math.abs(tLat - sLat)));
    return Math.max(0.06, Math.min(0.18, 0.04 + dist / 380));
}

function buildDeckLayers(mode = 'map') {
    const isGlobe = mode === 'globe';
    let lines = getVisibleLines();
    const totalBeforeLimit = lines.length;
    lines = topByCount(lines, maxArcs);
    lines = decorateFlowLines(lines);

    const points = getVisiblePoints(lines);
    updateArcCountInfo(lines.length, totalBeforeLimit);

    const layers = [];
    const landColor = cssRgb('--map-land-rgb', mapTilesFailed ? 220 : 0);
    const outlineColor = cssRgb('--map-outline-rgb', mapTilesFailed ? 255 : 140);
    const outlineSoft = cssRgb('--map-outline-rgb', 160);
    const useHeat = heatmapEnabled();
    const countriesPickable = useHeat || currentGroupBy() === 'country';

    // Fallback land fill when basemap tiles unavailable; otherwise transparent unless heatmap.
    if (countriesGeoJSON && (mapTilesFailed || useHeat)) {
        const { max, heat } = getStatsCache();
        layers.push(new deck.GeoJsonLayer({
            id: 'countries',
            data: countriesGeoJSON,
            pickable: countriesPickable,
            stroked: !isGlobe,
            filled: true,
            wrapLongitude: true,
            getFillColor: f => {
                if (useHeat) {
                    const c = heatmapColorRGB(heat.get(f) || 0, max);
                    if (focusedCountry && matchCountryFeature(f, focusedCountry)) {
                        return [c[0], c[1], c[2], 255];
                    }
                    return c;
                }
                return landColor;
            },
            getLineColor: outlineColor,
            getLineWidth: 1,
            lineWidthMinPixels: 0.5,
            updateTriggers: {
                getFillColor: [showHeatmap, currentGroupBy(), statsSignature(), isGlobe, NMAuth.getTheme(), focusedCountry, mapTilesFailed],
                getLineColor: [NMAuth.getTheme(), mapTilesFailed],
            },
            onClick: info => {
                if (!info.object || !countriesPickable) return;
                if (typeof showCountryDetail === 'function') {
                    showCountryDetail(resolveCountryKeyFromFeature(info.object), info.object);
                }
            },
        }));
    } else if (countriesGeoJSON && countriesPickable) {
        // Invisible pick layer for country clicks when basemap is present.
        layers.push(new deck.GeoJsonLayer({
            id: 'countries-pick',
            data: countriesGeoJSON,
            pickable: true,
            stroked: false,
            filled: true,
            wrapLongitude: true,
            getFillColor: [0, 0, 0, 1],
            updateTriggers: {
                getFillColor: [focusedCountry],
            },
            onClick: info => {
                if (!info.object) return;
                if (typeof showCountryDetail === 'function') {
                    showCountryDetail(resolveCountryKeyFromFeature(info.object), info.object);
                }
            },
        }));
    }

    if (showCountryLabels) {
        const centroids = isGlobe ? visibleGlobeCentroids() : buildCountryCentroids();
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

    const nodeOpacity = 150;
    // На MapLibre globe НЕ ставить cullMode:'back' / depthCompare:'always' —
    // трубки ArcLayer вырезаются до обрывков по лимбу (см. deck.gl maplibre example:
    // parameters: { cullMode: 'none' }).
    layers.push(new deck.ArcLayer({
        id: 'arcs',
        data: lines,
        pickable: true,
        // greatCircle на mercator гонит трансатлантику через Арктику — дуги
        // обрезаются сверху. На карте рисуем плоские дуги по хорде; на глобусе — 3D.
        greatCircle: false,
        wrapLongitude: true,
        getSourcePosition: d => nodeLonLat(d.src, d.src_lon, d.src_lat),
        getTargetPosition: d => nodeLonLat(d.dst, d.dst_lon, d.dst_lat),
        getSourceColor: d => [...arcRGB(d.status, d), d._flowAlpha || 210],
        getTargetColor: d => [...arcRGB(d.status, d), d._flowAlpha || 210],
        getWidth: d => Math.max(1.2, Math.min(7, 1.2 + Math.log2((d.count || 1) + 1) * 0.9)),
        widthUnits: 'pixels',
        getHeight: d => isGlobe ? globeArcHeight(d) : mapArcHeight(d),
        getTilt: isGlobe ? 0 : d => arcTilt(d) * 0.5,
        autoHighlight: true,
        highlightColor: [255, 255, 255, 140],
        parameters: isGlobe
            ? { cullMode: 'none' }
            : { depthTest: false },
        updateTriggers: {
            getSourceColor: [currentFilter, monoArcColor, typeof repColorArcs !== 'undefined' && repColorArcs, typeof reputationFilterActiveCount === 'function' ? reputationFilterActiveCount() : 0, focusedCountry],
            getTargetColor: [currentFilter, monoArcColor, typeof repColorArcs !== 'undefined' && repColorArcs, typeof reputationFilterActiveCount === 'function' ? reputationFilterActiveCount() : 0, focusedCountry],
            getSourcePosition: [_statsCacheVersion],
            getTargetPosition: [_statsCacheVersion],
            getHeight: [isGlobe],
            getTilt: [isGlobe],
            getWidth: [maxArcs],
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
        getFillColor: [88, 166, 255, nodeOpacity],
        getLineColor: outlineSoft,
        lineWidthUnits: 'pixels',
        getLineWidth: 0.7,
        radiusMinPixels: 1.5,
        radiusMaxPixels: 14,
        // Billboard-точки на глобусе тоже ломает back-face culling.
        parameters: isGlobe ? { cullMode: 'none' } : undefined,
        updateTriggers: {
            getLineColor: [NMAuth.getTheme()],
            getFillColor: [nodeOpacity],
        },
        onClick: info => { if (info.object) showPointDetail(info.object, info.object.key); },
    }));

    return layers;
}

function getDeckTooltip({ object, layer }) {
    if (!object) return null;
    const tipStyle = {
        background: 'rgba(22,27,34,0.95)', color: '#c9d1d9', border: '1px solid #30363d',
        borderRadius: '6px', padding: '6px 10px', fontSize: '11px',
    };
    if (layer && layer.id === 'arcs') {
        return {
            html: `<b>${escapeHTML(object.src_label || object.src)} → ${escapeHTML(object.dst_label || object.dst)}</b><br>
                   Статус: ${escapeHTML(object.status)} · События: ${fmtNumber(object.count)}`,
            style: tipStyle,
        };
    }
    if (layer && layer.id === 'nodes') {
        return {
            html: `<b>${escapeHTML(object.label || object.key)}</b><br>
                   ${escapeHTML(object.city || '')} · ${escapeHTML(ruCountry(object.country))}<br>
                   События: ${fmtNumber(object.count)}`,
            style: tipStyle,
        };
    }
    if (layer && (layer.id === 'countries' || layer.id === 'countries-pick')) {
        const name = resolveCountryKeyFromFeature(object);
        const { stats } = getStatsCache();
        const cnt = stats[name] || 0;
        return {
            html: `<b>${escapeHTML(ruCountry(name))}</b><br>События: ${fmtNumber(cnt)}`,
            style: tipStyle,
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

function syncViewStateFromMap() {
    if (!maplibreMap) return;
    const c = maplibreMap.getCenter();
    const vs = {
        longitude: ((c.lng + 540) % 360) - 180,
        latitude: c.lat,
        zoom: maplibreMap.getZoom(),
        bearing: maplibreMap.getBearing(),
        pitch: maplibreMap.getPitch(),
    };
    if (viewMode === 'globe') {
        globeViewState = vs;
    } else {
        mapViewState = vs;
    }
}

function refreshMapLayers() {
    if (!deckOverlay) return;
    const layers = buildDeckLayers(viewMode);
    deckOverlay.setProps({
        layers,
        getTooltip: getDeckTooltip,
    });
}

// Aliases used across older call sites
function updateDeck() { refreshMapLayers(); }
function updateGlobe() { refreshMapLayers(); }

function stopGlobeAutoRotate() {
    if (globeRotateRAF) {
        cancelAnimationFrame(globeRotateRAF);
        globeRotateRAF = null;
    }
    globeRotateLastTs = 0;
}

function startGlobeAutoRotate() {
    stopGlobeAutoRotate();
    if (!autoRotate || viewMode !== 'globe' || !maplibreMap) return;

    function tick(ts) {
        if (!maplibreMap || viewMode !== 'globe' || !autoRotate || mapUserInteracting || document.hidden) {
            stopGlobeAutoRotate();
            return;
        }
        if (!globeRotateLastTs) globeRotateLastTs = ts;
        const dt = Math.min(ts - globeRotateLastTs, 50);
        globeRotateLastTs = ts;
        const lng = maplibreMap.getCenter().lng + dt * 0.008;
        maplibreMap.jumpTo({ center: [lng, maplibreMap.getCenter().lat] });
        syncViewStateFromMap();
        maybeRefreshGlobeLabels(false);
        globeRotateRAF = requestAnimationFrame(tick);
    }
    globeRotateRAF = requestAnimationFrame(tick);
}

function applyMapProjection(mode) {
    if (!maplibreMap || typeof maplibreMap.setProjection !== 'function') return false;
    try {
        maplibreMap.setProjection({ type: mode === 'globe' ? 'globe' : 'mercator' });
        return true;
    } catch (e) {
        console.warn('setProjection failed:', e);
        return false;
    }
}

function emptyStyleFallback() {
    const bg = mapBaseCss();
    return {
        version: 8,
        sources: {},
        layers: [
            { id: 'background', type: 'background', paint: { 'background-color': bg } },
        ],
    };
}

function initMapView() {
    if (maplibreMap) return;
    const host = document.getElementById('map-host');
    if (!host) return;
    host.innerHTML = '';

    if (typeof maplibregl === 'undefined') {
        toast('MapLibre GL не загружен', 'error');
        return;
    }
    if (typeof deck === 'undefined' || !deck.MapboxOverlay) {
        toast('deck.gl MapboxOverlay недоступен', 'error');
        return;
    }

    const theme = (typeof NMAuth !== 'undefined' && NMAuth.getTheme()) || 'dark';
    const styleUrl = theme === 'light' ? MAP_STYLE_LIGHT : MAP_STYLE_DARK;
    const vs = viewMode === 'globe' ? globeViewState : mapViewState;

    try {
        maplibreMap = new maplibregl.Map({
            container: host,
            style: styleUrl,
            center: [vs.longitude || 37.6, vs.latitude || 55.7],
            zoom: vs.zoom || 2.5,
            bearing: vs.bearing || 0,
            pitch: vs.pitch || 0,
            attributionControl: true,
            preserveDrawingBuffer: true,
            fadeDuration: 0,
        });
    } catch (e) {
        console.error('MapLibre init failed:', e);
        toast('Ошибка инициализации карты: ' + e.message, 'error');
        maplibreMap = null;
        return;
    }

    maplibreMap.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'bottom-right');

    maplibreMap.on('error', (e) => {
        const msg = (e && e.error && e.error.message) || String(e.error || '');
        if (/Failed to fetch|NetworkError|AJAX|tile|style|load/i.test(msg) || e.error) {
            if (!mapTilesFailed) {
                mapTilesFailed = true;
                console.warn('Basemap tiles/style failed, using geojson fallback', e.error || e);
                try {
                    maplibreMap.setStyle(emptyStyleFallback());
                } catch (err) {}
                refreshMapLayers();
            }
        }
    });

    let readyOnce = false;
    const onReady = () => {
        if (readyOnce) return;
        readyOnce = true;
        if (viewMode === 'globe') {
            if (!applyMapProjection('globe')) {
                toast('Globe projection недоступен — остаёмся в 2D', 'error');
                viewMode = 'map';
            }
        } else {
            applyMapProjection('map');
        }

        deckOverlay = new deck.MapboxOverlay({
            interleaved: false,
            layers: buildDeckLayers(viewMode),
            getTooltip: getDeckTooltip,
            parameters: { preserveDrawingBuffer: true },
        });
        maplibreMap.addControl(deckOverlay);
        deckInstance = deckOverlay;

        syncViewStateFromMap();
        refreshMapLayers();
        if (viewMode === 'globe' && autoRotate) startGlobeAutoRotate();
    };

    if (maplibreMap.isStyleLoaded()) onReady();
    else maplibreMap.once('load', onReady);
    // If remote style never loads (offline), still initialize overlay on fallback style.
    setTimeout(() => {
        if (readyOnce || !maplibreMap) return;
        if (!mapTilesFailed) {
            mapTilesFailed = true;
            try { maplibreMap.setStyle(emptyStyleFallback()); } catch (e) {}
        }
        maplibreMap.once('load', onReady);
        // empty style usually loads sync/quick
        setTimeout(onReady, 200);
    }, 8000);

    maplibreMap.on('move', () => {
        syncViewStateFromMap();
        if (viewMode === 'globe') maybeRefreshGlobeLabels(false);
    });
    maplibreMap.on('zoomend', () => {
        syncViewStateFromMap();
        refreshMapLayers();
    });
    maplibreMap.on('mousedown', () => {
        mapUserInteracting = true;
        stopGlobeAutoRotate();
    });
    maplibreMap.on('mouseup', () => {
        mapUserInteracting = false;
        if (viewMode === 'globe' && autoRotate) startGlobeAutoRotate();
    });
    maplibreMap.on('dragstart', () => {
        mapUserInteracting = true;
        stopGlobeAutoRotate();
    });
    maplibreMap.on('dragend', () => {
        mapUserInteracting = false;
        if (viewMode === 'globe' && autoRotate) startGlobeAutoRotate();
    });
    maplibreMap.on('touchstart', () => {
        mapUserInteracting = true;
        stopGlobeAutoRotate();
    });
    maplibreMap.on('touchend', () => {
        mapUserInteracting = false;
        if (viewMode === 'globe' && autoRotate) startGlobeAutoRotate();
    });
}

function destroyMapView() {
    stopGlobeAutoRotate();
    if (deckOverlay && maplibreMap) {
        try { maplibreMap.removeControl(deckOverlay); } catch (e) {}
    }
    deckOverlay = null;
    deckInstance = null;
    if (maplibreMap) {
        try { maplibreMap.remove(); } catch (e) {}
        maplibreMap = null;
    }
    const host = document.getElementById('map-host');
    if (host) host.innerHTML = '';
}

/** Intermediate point on a great-circle, t ∈ [0, 1]. */
function greatCirclePoint(lon1, lat1, lon2, lat2, t) {
    const φ1 = lat1 * Math.PI / 180;
    const λ1 = lon1 * Math.PI / 180;
    const φ2 = lat2 * Math.PI / 180;
    let dλ = (lon2 - lon1) * Math.PI / 180;
    if (dλ > Math.PI) dλ -= 2 * Math.PI;
    if (dλ < -Math.PI) dλ += 2 * Math.PI;
    const λ2 = λ1 + dλ;
    const x1 = Math.cos(φ1) * Math.cos(λ1);
    const y1 = Math.cos(φ1) * Math.sin(λ1);
    const z1 = Math.sin(φ1);
    const x2 = Math.cos(φ2) * Math.cos(λ2);
    const y2 = Math.cos(φ2) * Math.sin(λ2);
    const z2 = Math.sin(φ2);
    const ω = Math.acos(Math.max(-1, Math.min(1, x1 * x2 + y1 * y2 + z1 * z2)));
    if (!(ω > 1e-6)) return [lon1, lat1];
    const a = Math.sin((1 - t) * ω) / Math.sin(ω);
    const b = Math.sin(t * ω) / Math.sin(ω);
    const x = a * x1 + b * x2;
    const y = a * y1 + b * y2;
    const z = a * z1 + b * z2;
    return [
        Math.atan2(y, x) * 180 / Math.PI,
        Math.atan2(z, Math.hypot(x, y)) * 180 / Math.PI,
    ];
}

function fitDeckToData() {
    const lines = allLines.filter(hasCoords);
    if (!lines.length || !maplibreMap) return;
    let minLon = 180, maxLon = -180, minLat = 90, maxLat = -90;
    const expand = (lon, lat) => {
        if (!Number.isFinite(lon) || !Number.isFinite(lat)) return;
        minLon = Math.min(minLon, lon);
        maxLon = Math.max(maxLon, lon);
        minLat = Math.min(minLat, lat);
        maxLat = Math.max(maxLat, lat);
    };
    lines.forEach(l => {
        expand(l.src_lon, l.src_lat);
        expand(l.dst_lon, l.dst_lat);
        // Учитываем северный «горб» great-circle, иначе fitBounds режет дуги сверху.
        const peakLat = greatCircleMaxLat(l.src_lon, l.src_lat, l.dst_lon, l.dst_lat);
        expand((l.src_lon + l.dst_lon) / 2, peakLat);
        for (let i = 1; i <= 7; i++) {
            const [lon, lat] = greatCirclePoint(l.src_lon, l.src_lat, l.dst_lon, l.dst_lat, i / 8);
            expand(lon, lat);
        }
    });
    if (minLon === 180) return;
    const pad = 0.2;
    const lonPad = Math.max(1.5, (maxLon - minLon) * pad);
    const latPad = Math.max(2, (maxLat - minLat) * pad);
    // Чуть больше запаса на север — туда уходят дуги в перспективе.
    const northPad = Math.max(3, latPad * 1.6);
    try {
        maplibreMap.fitBounds(
            [[minLon - lonPad, minLat - latPad], [maxLon + lonPad, Math.min(82, maxLat + northPad)]],
            { padding: { top: 80, bottom: 52, left: 52, right: 52 }, maxZoom: 5, duration: 0 }
        );
        // Небольшой pitch, чтобы плоские дуги читались как «горбы», а не как прямые.
        if ((maplibreMap.getPitch() || 0) < 12) {
            maplibreMap.jumpTo({ pitch: 20 });
        }
        syncViewStateFromMap();
    } catch (e) {
        const lonSpan = maxLon - minLon, latSpan = maxLat - minLat;
        const zoom = Math.min(5.5, Math.max(1, 8 - Math.log2(Math.max(lonSpan, latSpan, 1) + 1)));
        maplibreMap.jumpTo({
            center: [(minLon + maxLon) / 2, (minLat + maxLat) / 2],
            zoom,
        });
        syncViewStateFromMap();
    }
}

function setViewMode(mode) {
    if (mode === viewMode && maplibreMap) {
        // still update chrome
    }
    const prev = viewMode;
    viewMode = mode;

    document.getElementById('mode-map')?.classList.toggle('active', mode === 'map');
    document.getElementById('mode-globe')?.classList.toggle('active', mode === 'globe');
    document.getElementById('mode-map-icon')?.classList.toggle('active', mode === 'map');
    document.getElementById('mode-globe-icon')?.classList.toggle('active', mode === 'globe');
    document.getElementById('autoRotateWrap').style.display = (mode === 'globe') ? '' : 'none';

    if (!maplibreMap) {
        initMapView();
        saveUIState();
        return;
    }

    stopGlobeAutoRotate();
    if (mode === 'globe') {
        if (!applyMapProjection('globe')) {
            toast('Globe projection недоступен', 'error');
            viewMode = 'map';
            document.getElementById('mode-map')?.classList.add('active');
            document.getElementById('mode-globe')?.classList.remove('active');
            document.getElementById('mode-map-icon')?.classList.add('active');
            document.getElementById('mode-globe-icon')?.classList.remove('active');
            document.getElementById('autoRotateWrap').style.display = 'none';
            applyMapProjection('map');
            refreshMapLayers();
            saveUIState();
            return;
        }
        maplibreMap.jumpTo({
            center: [globeViewState.longitude || 30, globeViewState.latitude || 30],
            zoom: globeViewState.zoom || 1.2,
            bearing: globeViewState.bearing || 0,
            pitch: 0,
        });
        if (autoRotate) startGlobeAutoRotate();
    } else {
        applyMapProjection('map');
        maplibreMap.jumpTo({
            center: [mapViewState.longitude || 37.6, mapViewState.latitude || 55.7],
            zoom: mapViewState.zoom || 2.5,
            bearing: mapViewState.bearing || 0,
            pitch: mapViewState.pitch || 0,
        });
    }
    syncViewStateFromMap();
    lastGlobeLabelCullKey = '';
    // Сброс views у overlay — чтобы подхватить GlobeView после setProjection.
    if (deckOverlay && typeof deckOverlay.setProps === 'function') {
        deckOverlay.setProps({ views: undefined, layers: buildDeckLayers(viewMode) });
    } else {
        refreshMapLayers();
    }
    saveUIState();
    setTimeout(() => resizeCurrentView(), 100);
    if (prev !== mode) { /* mode changed */ }
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

function clearFocusedCountry() {
    if (!focusedCountry) return;
    focusedCountry = null;
    _statsCacheVersion++;
    refreshMapLayers();
}
