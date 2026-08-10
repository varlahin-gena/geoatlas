function openDetail(title, sections, actions) {
    document.getElementById('detailTitle').textContent = title;
    const actionsEl = document.getElementById('detailActions');
    actionsEl.innerHTML = '';
    if (actions && actions.length) {
        actionsEl.style.display = 'flex';
        actions.forEach(function (a) {
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'detail-action-btn';
            btn.textContent = a.label;
            btn.addEventListener('click', a.onClick);
            actionsEl.appendChild(btn);
        });
    } else {
        actionsEl.style.display = 'none';
    }
    const body = document.getElementById('detailBody');
    body.innerHTML = '';
    sections.forEach(sec => {
        if (sec.title) {
            const t = document.createElement('div');
            t.className = 'detail-section-title';
            t.textContent = sec.title;
            body.appendChild(t);
        }
        sec.rows.forEach(r => {
            if (r.value === undefined || r.value === null || r.value === '') return;
            const row = document.createElement('div');
            row.className = 'detail-row' + (r.onClick ? ' detail-row-clickable' : '');
            const k = document.createElement('div'); k.className = 'k'; k.textContent = r.key;
            const v = document.createElement('div'); v.className = 'v' + (r.color ? ' ' + r.color : '');
            v.textContent = r.value;
            row.appendChild(k); row.appendChild(v);
            if (r.onClick) {
                row.title = r.hint || 'Открыть связь';
                row.addEventListener('click', r.onClick);
            }
            body.appendChild(row);
        });
    });
    document.getElementById('detailPanel').classList.add('open');
}
function closeDetail() {
    document.getElementById('detailPanel').classList.remove('open');
    document.getElementById('detailActions').style.display = 'none';
}

async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        toast('Скопировано', 'success', 2000);
    } catch (e) {
        toast('Не удалось скопировать', 'error');
    }
}

function applySearchFilter(value) {
    if (typeof clearFocusedCountry === 'function') clearFocusedCountry();
    if (typeof setSearchQuery === 'function') {
        setSearchQuery(value, {
            syncInput: true,
            save: true,
            refresh: true,
            updateOverlay: true,
        });
        return;
    }
    const input = document.getElementById('searchInput');
    input.value = value;
    currentSearch = normalizeText(value);
    refreshMapLayers();
    updateMapOverlay();
}

function updateMapOverlay() {
    const overlay = document.getElementById('vizOverlay');
    const title = document.getElementById('vizOverlayTitle');
    const text = document.getElementById('vizOverlayText');
    if (!overlay || !title || !text) return;

    // Пока идёт явная перезагрузка — не перекрываем спиннер оверлеем «нет данных»
    if (document.getElementById('mapLoading')?.classList.contains('visible')) {
        overlay.classList.remove('visible');
        return;
    }

    if (lastFetchError) {
        overlay.classList.add('visible');
        title.textContent = 'Ошибка загрузки';
        text.textContent = lastFetchError;
        return;
    }

    if (!allLines.length) {
        overlay.classList.add('visible');
        title.textContent = 'Нет событий за период';
        text.textContent = 'Попробуйте расширить период, уменьшить порог minCount или проверить ingest.';
        return;
    }

    const visible = getVisibleLines();
    if (!visible.length) {
        overlay.classList.add('visible');
        title.textContent = 'Ничего не отображается';
        const hints = [];
        if (currentFilter !== 'all') hints.push('фильтр «' + currentFilter + '»');
        if (currentSearch) hints.push('поиск «' + currentSearch + '»');
        if (minCount > 1) hints.push('порог ≥ ' + minCount + ' соб.');
        if (typeof reputationFilterActiveCount === 'function' && reputationFilterActiveCount() > 0 && currentGroupBy() === 'ip') {
            hints.push('репутация');
        }
        text.textContent = hints.length
            ? 'Активные фильтры скрыли все связи: ' + hints.join(', ') + '.'
            : 'Все связи отфильтрованы текущими настройками.';
        return;
    }

    overlay.classList.remove('visible');
}

function updateArcsTruncHint(shown, total) {
    const el = document.getElementById('arcsTruncHint');
    if (!el) return;
    if (total > shown) {
        el.style.display = 'block';
        el.textContent = 'Показано ' + fmtNumber(shown) + ' из ' + fmtNumber(total)
            + ' связей — увеличьте лимит дуг или сузьте период';
    } else {
        el.style.display = 'none';
        el.textContent = '';
    }
}

/** Подпись ключа конца дуги в зависимости от group_by. */
function lineEndpointKeyLabel(groupBy) {
    switch (groupBy) {
        case 'ip': return 'IP';
        case 'subnet': return 'Подсеть';
        case 'country': return 'Страна';
        default: return 'Ключ';
    }
}

/**
 * В режимах грубой группировки (subnet/city/country) порт, zone, rule и т.п.
 * приходят как any()/argMax — это образец, а не полный профиль ребра.
 */
function lineDetailSampleKey(label, coarse) {
    return coarse ? label + ' (пример)' : label;
}

function lineEndpointRows(side, groupBy, coarse) {
    const key = side.key;
    const label = side.label;
    const showLabel = label && label !== key;
    const showCountry = groupBy !== 'country';
    const rows = [
        { key: lineEndpointKeyLabel(groupBy), value: key },
    ];
    if (showLabel) {
        rows.push({ key: 'Метка', value: label });
    }
    rows.push(
        { key: lineDetailSampleKey('Порт', coarse), value: side.port || '' },
        { key: lineDetailSampleKey('Zone', coarse), value: side.zone || '' },
    );
    if (showCountry) {
        rows.push({ key: 'Country', value: ruCountry(side.country) });
    }
    return rows;
}

function showLineDetail(line) {
    const colorByStatus = { allowed: 'green', blocked: 'red', unknown: '' };
    const groupBy = currentGroupBy();
    const coarse = groupBy === 'subnet' || groupBy === 'city' || groupBy === 'country';
    const actions = [
        { label: 'Копировать src', onClick: () => copyToClipboard(line.src || '') },
        { label: 'Копировать dst', onClick: () => copyToClipboard(line.dst || '') },
        { label: 'Поиск src', onClick: () => applySearchFilter(line.src_label || line.src || '') },
        { label: 'Поиск dst', onClick: () => applySearchFilter(line.dst_label || line.dst || '') },
    ];
    openDetail(`${line.src_label || line.src} → ${line.dst_label || line.dst}`, [
        { title: 'Связь', rows: [
            { key: 'Статус', value: line.status, color: colorByStatus[line.status] || '' },
            { key: 'Событий', value: fmtNumber(line.count) },
            { key: 'Allowed', value: fmtNumber(line.allowed_count), color: 'green' },
            { key: 'Blocked', value: fmtNumber(line.blocked_count), color: 'red' },
            { key: 'Bytes out', value: fmtNumber(line.bytes_sent) },
            { key: 'Bytes in',  value: fmtNumber(line.bytes_recv) },
        ]},
        { title: 'Источник', rows: lineEndpointRows({
            key: line.src,
            label: line.src_label,
            port: line.src_port,
            zone: line.src_zone,
            country: line.src_country,
        }, groupBy, coarse) },
        { title: 'Назначение', rows: lineEndpointRows({
            key: line.dst,
            label: line.dst_label,
            port: line.dst_port,
            zone: line.dst_zone,
            country: line.dst_country,
        }, groupBy, coarse) },
        { title: 'Репутация', rows: [].concat(
            typeof reputationDetailRows === 'function'
                ? reputationDetailRows('Src', line.src, line.src_reputation)
                : [{ key: 'Src', value: typeof formatReputationHits === 'function' ? formatReputationHits(line.src_reputation) : '' }],
            typeof reputationDetailRows === 'function'
                ? reputationDetailRows('Dst', line.dst, line.dst_reputation)
                : [{ key: 'Dst', value: typeof formatReputationHits === 'function' ? formatReputationHits(line.dst_reputation) : '' }],
        )},
        { title: 'Параметры', rows: [
            { key: lineDetailSampleKey('Protocol', coarse), value: line.proto || '' },
            { key: lineDetailSampleKey('Rule', coarse), value: line.rule || '' },
            { key: lineDetailSampleKey('Device', coarse), value: line.device || '' },
            // last_action = argMax(action, timestamp) по всему ребру — не any()
            { key: 'Last action', value: line.last_action || '' },
        ]},
    ], actions);
}

function formatConnPeer(line, asSrc) {
    const peerKey = asSrc ? line.dst : line.src;
    const peerLabel = asSrc
        ? (line.dst_label || line.dst)
        : (line.src_label || line.src);
    const port = asSrc ? line.dst_port : line.src_port;
    const country = asSrc ? line.dst_country : line.src_country;
    const parts = [peerLabel || peerKey];
    if (port) parts.push(':' + port);
    if (line.device) parts.push('· ' + line.device);
    else if (country) parts.push('· ' + ruCountry(country));
    parts.push('(' + fmtNumber(line.count) + ')');
    return parts.join(' ');
}

function connectionsForPoint(key) {
    const out = [];
    const inn = [];
    allLines.forEach(function (line) {
        if (!hasCoords(line)) return;
        // Self-loop не показываем как «куда» и «откуда» одновременно.
        if (line.src && line.src === line.dst) return;
        if (line.src === key) out.push(line);
        if (line.dst === key) inn.push(line);
    });
    out.sort(function (a, b) { return (b.count || 0) - (a.count || 0); });
    inn.sort(function (a, b) { return (b.count || 0) - (a.count || 0); });
    return { out: out, inn: inn };
}

function showPointDetail(point, key) {
    const colorByStatus = { allowed: 'green', blocked: 'red', unknown: '' };
    const actions = [
        { label: 'Копировать ключ', onClick: () => copyToClipboard(key || '') },
        { label: 'Искать узел', onClick: () => applySearchFilter(point.label || key || '') },
    ];
    if (point.country && point.country !== 'Неизвестно') {
        actions.push({
            label: 'Искать страну',
            onClick: () => applySearchFilter(ruCountry(point.country)),
        });
    }

    const conn = connectionsForPoint(key);
    const sections = [
        { title: 'Узел', rows: [
            { key: 'Ключ', value: key },
            { key: 'Город', value: point.city || 'Неизвестно' },
            { key: 'Регион', value: point.region || 'Неизвестно' },
            { key: 'Страна', value: ruCountry(point.country) },
            { key: 'Lat / Lon', value: `${point.lat.toFixed(4)}, ${point.lon.toFixed(4)}` },
            { key: 'Событий', value: fmtNumber(point.count) },
        ]},
    ];
    if (typeof reputationDetailRows === 'function' && point.reputation && point.reputation.length) {
        sections.push({ title: 'Репутация', rows: reputationDetailRows('IP', key, point.reputation) });
    } else if (typeof formatReputationHits === 'function' && point.reputation && point.reputation.length) {
        sections[0].rows.push({ key: 'Репутация', value: formatReputationHits(point.reputation) });
    }

    if (conn.out.length) {
        sections.push({
            title: 'Куда (исходящие · ' + conn.out.length + ')',
            rows: conn.out.slice(0, 30).map(function (line) {
                return {
                    key: '→',
                    value: formatConnPeer(line, true),
                    color: colorByStatus[line.status] || '',
                    hint: 'Открыть связь',
                    onClick: function () { showLineDetail(line); },
                };
            }),
        });
    }
    if (conn.inn.length) {
        sections.push({
            title: 'Откуда (входящие · ' + conn.inn.length + ')',
            rows: conn.inn.slice(0, 30).map(function (line) {
                return {
                    key: '←',
                    value: formatConnPeer(line, false),
                    color: colorByStatus[line.status] || '',
                    hint: 'Открыть связь',
                    onClick: function () { showLineDetail(line); },
                };
            }),
        });
    }
    if (!conn.out.length && !conn.inn.length) {
        sections.push({
            title: 'Связи',
            rows: [{ key: 'Нет данных', value: 'Для узла нет дуг с координатами обеих сторон' }],
        });
    }

    openDetail(point.label || key, sections, actions);
}

function renderSparklineSVG(points) {
    if (!points || !points.length) {
        return '<div class="detail-sparkline"><div style="color:var(--text-muted);font-size:11px">Нет данных ряда</div></div>';
    }
    const w = 280, h = 48, pad = 2;
    let max = 1;
    points.forEach(p => {
        const t = (p.allowed || 0) + (p.blocked || 0) || (p.total || 0);
        if (t > max) max = t;
    });
    const n = points.length;
    const step = n <= 1 ? w : (w - pad * 2) / (n - 1);
    function poly(key, color) {
        const coords = points.map((p, i) => {
            const v = p[key] || 0;
            const x = pad + i * step;
            const y = h - pad - (v / max) * (h - pad * 2);
            return x.toFixed(1) + ',' + y.toFixed(1);
        }).join(' ');
        return `<polyline fill="none" stroke="${color}" stroke-width="1.5" points="${coords}" />`;
    }
    return `<div class="detail-sparkline">
      <svg viewBox="0 0 ${w} ${h}" preserveAspectRatio="none">
        ${poly('allowed', 'var(--green, #3fb950)')}
        ${poly('blocked', 'var(--red, #f85149)')}
      </svg>
      <div class="detail-sparkline-legend">
        <span><i style="background:var(--green)"></i>Allowed</span>
        <span><i style="background:var(--red)"></i>Blocked</span>
      </div>
    </div>`;
}

async function fetchCountrySeries(country) {
    if (seriesFetchController) {
        try { seriesFetchController.abort(); } catch (e) {}
    }
    const controller = new AbortController();
    seriesFetchController = controller;
    try {
        const periodQuery = buildPeriodQuery();
        const url = `${API_BASE}/api/events/series?country=${encodeURIComponent(country)}${periodQuery}`;
        const res = await fetch(url, {
            cache: 'no-store',
            credentials: 'same-origin',
            signal: controller.signal,
        });
        if (res.status === 401) {
            location.replace(NMAuth.loginUrl(location.pathname));
            return null;
        }
        if (!res.ok) throw new Error(await res.text() || `HTTP ${res.status}`);
        return await res.json();
    } finally {
        if (seriesFetchController === controller) seriesFetchController = null;
    }
}

function linesForCountry(country) {
    const target = String(country || '').toLowerCase();
    const aliases = new Set([target]);
    Object.entries(countryNamesRu).forEach(([en, ru]) => {
        if (en.toLowerCase() === target || String(ru).toLowerCase() === target) {
            aliases.add(en.toLowerCase());
            aliases.add(String(ru).toLowerCase());
        }
    });
    return getVisibleLines().filter(l => {
        const src = String(l.src_country || '').toLowerCase();
        const dst = String(l.dst_country || '').toLowerCase();
        if (aliases.has(src) || aliases.has(dst)) return true;
        const sp = allPoints[l.src], dp = allPoints[l.dst];
        if (sp && aliases.has(String(sp.country || '').toLowerCase())) return true;
        if (dp && aliases.has(String(dp.country || '').toLowerCase())) return true;
        return false;
    }).sort((a, b) => (b.count || 0) - (a.count || 0));
}

async function showCountryDetail(countryKey, feature) {
    if (!countryKey) return;
    focusedCountry = countryKey;
    _statsCacheVersion++;
    refreshMapLayers();

    const { stats } = getStatsCache();
    const events = stats[countryKey] || 0;
    const topLines = linesForCountry(countryKey).slice(0, 20);
    const colorByStatus = { allowed: 'green', blocked: 'red', unknown: '' };

    const sections = [
        { title: 'Страна', rows: [
            { key: 'Название', value: ruCountry(countryKey) },
            { key: 'Ключ', value: countryKey },
            { key: 'События (узлы)', value: fmtNumber(events) },
            { key: 'Связей на карте', value: fmtNumber(topLines.length) },
        ]},
    ];
    if (topLines.length) {
        sections.push({
            title: 'Топ связей · ' + topLines.length,
            rows: topLines.map(function (line) {
                return {
                    key: line.status || '—',
                    value: (line.src_label || line.src) + ' → ' + (line.dst_label || line.dst)
                        + ' (' + fmtNumber(line.count) + ')',
                    color: colorByStatus[line.status] || '',
                    hint: 'Открыть связь',
                    onClick: function () { showLineDetail(line); },
                };
            }),
        });
    }

    const actions = [
        { label: 'Сбросить фокус', onClick: function () { clearFocusedCountry(); closeDetail(); } },
        { label: 'Искать страну', onClick: function () { applySearchFilter(ruCountry(countryKey)); } },
    ];
    openDetail(ruCountry(countryKey), sections, actions);

    // Sparkline async append
    const body = document.getElementById('detailBody');
    const sparkHost = document.createElement('div');
    sparkHost.innerHTML = '<div class="detail-sparkline"><div style="color:var(--text-muted);font-size:11px">Загрузка ряда…</div></div>';
    body.insertBefore(sparkHost, body.firstChild);
    try {
        const data = await fetchCountrySeries(countryKey);
        if (!data) return;
        // If user clicked another country meanwhile
        if (focusedCountry !== countryKey) return;
        sparkHost.innerHTML = renderSparklineSVG(data.points || []);
        const title = document.createElement('div');
        title.className = 'detail-section-title';
        title.textContent = 'Динамика (bucket ' + (data.bucket_sec || '?') + 's)';
        sparkHost.insertBefore(title, sparkHost.firstChild);
    } catch (e) {
        if (typeof isAbortError === 'function' && isAbortError(e)) return;
        if (focusedCountry !== countryKey) return;
        sparkHost.innerHTML = '<div class="detail-sparkline"><div style="color:var(--red);font-size:11px">Ряд недоступен: '
            + escapeHTML(e.message || e) + '</div></div>';
    }
}
