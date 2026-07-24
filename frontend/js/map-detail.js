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
    const input = document.getElementById('searchInput');
    input.value = value;
    currentSearch = normalizeText(value);
    if (viewMode === 'map') updateDeck();
    else updateGlobe();
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
