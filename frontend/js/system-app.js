'use strict';

const STATS_REFRESH_MS   = 5000;
const HISTORY_REFRESH_MS = 30000;

const CONTAINERS = ['backend', 'clickhouse', 'syslog-ng', 'frontend'];

const COLORS = {
    backend:        '#58a6ff',
    clickhouse:     '#bc8cff',
    'syslog-ng':    '#d29922',
    frontend:       '#39c5cf',
    rate:           '#3fb950',
    db_rate:        '#58a6ff',
    lag:            '#d29922',
    buffer:         '#bc8cff',
    storage:        '#f85149',
};

let currentPeriod = '1h';
let currentTab = 'overview';
let statsTimer = null;
let historyTimer = null;
let isFetchingHistory = false;
let charts = {};
let chartsNeedResize = false;
const TAB_KEY = 'nm.systemTab';

function fmtNumber(n) {
    if (n == null || isNaN(n)) return '—';
    return Number(n).toLocaleString('ru-RU');
}

function fmtBytes(bytes) {
    if (bytes == null || isNaN(bytes)) return '—';
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB';
    return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB';
}

function fmtDuration(sec) {
    if (sec == null || isNaN(sec)) return '—';
    if (sec < 60) return Math.round(sec) + ' с';
    if (sec < 3600) return Math.round(sec/60) + ' мин';
    if (sec < 86400) return (sec/3600).toFixed(1) + ' ч';
    return (sec/86400).toFixed(1) + ' д';
}

function fmtPercent(ratio) {
    if (ratio == null || isNaN(ratio)) return '—';
    return (ratio * 100).toFixed(1) + '%';
}

function isAutoRefresh() {
    return document.getElementById('autoRefreshChk').checked;
}

// Безопасный fill: возвращает gradient если bbox валиден, иначе плоский цвет
function makeFill(u, color) {
    const bbox = u && u.bbox;
    if (!bbox || !isFinite(bbox.top) || !isFinite(bbox.height) || bbox.height <= 0) {
        return color + '33';
    }
    try {
        const grad = u.ctx.createLinearGradient(0, bbox.top, 0, bbox.top + bbox.height);
        grad.addColorStop(0, color + '55');
        grad.addColorStop(1, color + '00');
        return grad;
    } catch (e) {
        return color + '33';
    }
}

function cssVar(name, fallback) {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return v || fallback;
}
function chartAxisStroke() { return cssVar('--border', '#30363d'); }
function chartTextColor() { return cssVar('--text-muted', '#8b949e'); }
function formatSeriesValue(v, opts) {
    if (v == null || isNaN(v)) return '—';
    if (opts.isPercent) return Number(v).toFixed(2) + '%';
    if (opts.isBytes) return fmtBytes(v);
    if (opts.isInt) return Math.round(v).toLocaleString('ru-RU');
    if (Math.abs(v) < 1) return Number(v).toFixed(3);
    if (Math.abs(v) < 100) return Number(v).toFixed(1);
    return Math.round(v).toLocaleString('ru-RU');
}

function buildChartLegend(series) {
    const legend = document.createElement('div');
    legend.className = 'chart-legend';
    series.forEach(function (s) {
        const item = document.createElement('div');
        item.className = 'chart-legend-item';
        item.innerHTML =
            '<span class="chart-legend-marker" style="background:' + escapeHTML(s.color) + ';"></span>' +
            '<span class="chart-legend-label">' + escapeHTML(s.label) + '</span>' +
            '<span class="chart-legend-value">—</span>';
        legend.appendChild(item);
    });
    return legend;
}

function updateCustomLegend(u, legendEl, opts) {
    if (!u || !legendEl) return;
    const valueEls = legendEl.querySelectorAll('.chart-legend-value');
    const idx = u.cursor && u.cursor.idx != null ? u.cursor.idx : null;
    const data = u.data || [];

    valueEls.forEach(function (el, i) {
        const seriesIdx = i + 1;
        let v = null;
        if (idx != null && data[seriesIdx]) {
            v = data[seriesIdx][idx];
        } else if (data[seriesIdx] && data[seriesIdx].length) {
            // без курсора — последнее валидное значение
            for (let j = data[seriesIdx].length - 1; j >= 0; j--) {
                if (data[seriesIdx][j] != null && !isNaN(data[seriesIdx][j])) {
                    v = data[seriesIdx][j];
                    break;
                }
            }
        }
        el.textContent = formatSeriesValue(v, opts);
    });
}

function makeChart(elId, series, opts) {
    opts = opts || {};
    const el = document.getElementById(elId);
    if (!el) return null;
    el.innerHTML = '';

    const legend = buildChartLegend(series);
    const plotHost = document.createElement('div');
    plotHost.className = 'chart-plot-host';
    plotHost.style.height = (el.clientHeight || 240) + 'px';
    el.appendChild(legend);
    el.appendChild(plotHost);

    const isPercent = !!opts.isPercent;
    const isBytes   = !!opts.isBytes;
    const isInt     = !!opts.isInt;
    const axisStroke = chartAxisStroke();
    const textColor = chartTextColor();
    const legendHeight = 28;
    const fmtOpts = { isPercent: isPercent, isBytes: isBytes, isInt: isInt };

    const chartOpts = {
        width: plotHost.clientWidth || el.clientWidth || 600,
        height: Math.max(140, (el.clientHeight || 240) - legendHeight),
        cursor: {
            drag: { x: false, y: false },
            sync: { key: 'sync' }
        },
        legend: { show: false, live: false },
        hooks: {
            setCursor: [
                function (u) { updateCustomLegend(u, legend, fmtOpts); }
            ],
            setData: [
                function (u) { updateCustomLegend(u, legend, fmtOpts); }
            ]
        },
        scales: {
            x: { time: true },
            y: { auto: true },
        },
        axes: [
            {
                stroke: textColor,
                grid: { stroke: axisStroke, width: 1 },
                ticks: { stroke: axisStroke },
            },
            {
                stroke: textColor,
                grid: { stroke: axisStroke, width: 1 },
                ticks: { stroke: axisStroke },
                values: function (u, vals) {
                    return vals.map(function (v) {
                        if (v == null) return '';
                        if (isPercent) return v.toFixed(1) + '%';
                        if (isBytes)   return fmtBytes(v);
                        if (isInt)     return Math.round(v).toLocaleString('ru-RU');
                        if (Math.abs(v) < 1) return v.toFixed(2);
                        if (Math.abs(v) < 100) return v.toFixed(1);
                        return Math.round(v).toLocaleString('ru-RU');
                    });
                },
            }
        ],
        series: [{ label: 'Время' }].concat(series.map(function (s) {
            return {
                label: s.label,
                stroke: s.color,
                width: 2,
                fill: function (u) { return makeFill(u, s.color); },
                points: { show: false },
            };
        })),
    };

    const chart = new uPlot(chartOpts, [[]], plotHost);
    updateCustomLegend(chart, legend, fmtOpts);
    return chart;
}

function showNoDataOverlay(chart, show) {
    const root = chart && chart.root;
    if (!root) return;
    let overlay = root.querySelector('.no-data-overlay');
    if (show) {
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.className = 'no-data-overlay';
            overlay.textContent = 'Нет данных за выбранный период';
            root.style.position = 'relative';
            root.appendChild(overlay);
        }
    } else if (overlay) {
        overlay.remove();
    }
}

function updateChart(chart, xs, seriesData) {
    if (!chart) return;

    const hasAnyTime = xs && xs.length > 0;
    const hasAnyValue = seriesData.some(function (arr) {
        return arr.some(function (v) { return v != null && !isNaN(v); });
    });

    if (!hasAnyTime || !hasAnyValue) {
        const hasData = chart.data && chart.data[0] && chart.data[0].length > 0;
        if (!hasData) showNoDataOverlay(chart, true);
        return;
    }

    showNoDataOverlay(chart, false);

    try {
        chart.setData([xs].concat(seriesData));
    } catch (e) {
        console.error('setData error:', e, { xs: xs.length, series: seriesData.length });
    }
}

function resizeCharts() {
    Object.values(charts).forEach(function (c) {
        if (!c) return;
        try {
            const host = c.over && c.over.parentNode && c.over.parentNode.parentNode;
            if (host) c.setSize({ width: host.clientWidth, height: host.clientHeight });
        } catch (e) {}
    });
}
window.addEventListener('resize', resizeCharts);

function renderHealthPill(stats) {
    const pill = document.getElementById('healthPill');
    const text = document.getElementById('healthText');
    pill.classList.remove('ok', 'warn', 'bad');

    const alerts = stats.alerts || [];
    const hasError = alerts.some(function (a) { return a.level === 'error'; });
    const hasWarn  = alerts.some(function (a) { return a.level === 'warn'; });

    if (hasError) {
        pill.classList.add('bad');
        text.textContent = alerts.length + ' проблем';
    } else if (hasWarn) {
        pill.classList.add('warn');
        text.textContent = alerts.length + ' предупр.';
    } else {
        pill.classList.add('ok');
        text.textContent = 'Всё ОК';
    }
}

function renderPipeline(stats) {
    const row = document.getElementById('pipelineRow');
    row.innerHTML = '';

    const ingest   = (stats.pipeline && stats.pipeline.ingest)   || {};
    const rate     = (stats.pipeline && stats.pipeline.rate)     || {};
    const tlogs    = (stats.storage && stats.storage.traffic_logs) || {};
    const queueDepth = ingest.queue_depth || 0;
    const queueCapacity = ingest.queue_capacity || 0;
    const queuePct = queueCapacity > 0 ? (queueDepth / queueCapacity) : 0;

    const inputEps = rate.input_events_per_sec != null ? rate.input_events_per_sec : rate.events_per_sec;
    const udpEps   = rate.udp_events_per_sec != null ? rate.udp_events_per_sec : 0;
    const tcpEps   = rate.tcp_events_per_sec != null ? rate.tcp_events_per_sec : 0;
    const taggedTotal = (ingest.udp_received_total || 0) + (ingest.tcp_received_total || 0);
    const untaggedEps = Math.max(0, (inputEps || 0) - udpEps - tcpEps);

    let syslogMeta = 'udp: ' + fmtNumber(udpEps) + '/s · tcp: ' + fmtNumber(tcpEps) + '/s';
    if (untaggedEps > 0 && taggedTotal === 0) {
        syslogMeta += ' · без метки: ' + fmtNumber(untaggedEps) + '/s';
    }

    const stages = [
        {
            name: 'Syslog-NG',
            value: fmtNumber(inputEps || 0) + ' eps',
            meta: syslogMeta,
            status: 'ok',
        },
        {
            name: 'Backend Ingest',
            value: fmtNumber(rate.events_per_sec || 0) + ' eps',
            meta: 'conn: ' + fmtNumber(ingest.connections || 0)
                + ', buf: ' + fmtNumber(ingest.buffered_lines || 0)
                + ', q: ' + fmtNumber(queueDepth) + '/' + fmtNumber(queueCapacity)
                + (queueCapacity > 0 ? ' (' + fmtPercent(queuePct) + ')' : '')
                + ((ingest.dropped_total || 0) > 0
                    ? ', drop: ' + fmtNumber(ingest.dropped_total) + ' (' + fmtNumber(rate.drops_per_sec || 0) + '/s)'
                    : ''),
            status: (rate.drops_per_sec || 0) >= 100 ? 'bad'
                  : queuePct >= 0.9 ? 'bad'
                  : queuePct >= 0.75 ? 'warn'
                  : (rate.drops_per_sec || 0) > 0 || (ingest.dropped_total || 0) > 0 ? 'warn'
                  : (ingest.buffered_lines || 0) > 100000 ? 'bad'
                  : (ingest.buffered_lines || 0) > 10000  ? 'warn'
                  : 'ok',
        },
        { name: 'ClickHouse', value: fmtNumber(tlogs.row_count), meta: 'строк в БД', status: 'ok' },
    ];

    stages.forEach(function (s, i) {
        if (i > 0) {
            const arrow = document.createElement('div');
            arrow.className = 'pipeline-arrow';
            arrow.textContent = '→';
            row.appendChild(arrow);
        }
        const stage = document.createElement('div');
        stage.className = 'pipeline-stage ' + s.status;
        stage.innerHTML =
            '<div class="stage-name">' + escapeHTML(s.name) + '</div>' +
            '<div class="stage-value">' + escapeHTML(s.value) + '</div>' +
            '<div class="stage-meta">' + escapeHTML(s.meta) + '</div>';
        row.appendChild(stage);
    });
}

function renderStatusStrip(stats) {
    const rate = (stats.pipeline && stats.pipeline.rate) || {};
    const ingest = (stats.pipeline && stats.pipeline.ingest) || {};
    const inputEps = rate.input_events_per_sec != null ? rate.input_events_per_sec : rate.events_per_sec;
    const queueDepth = ingest.queue_depth || 0;
    const queueCapacity = ingest.queue_capacity || 0;
    const queuePct = queueCapacity > 0 ? (queueDepth / queueCapacity) : 0;

    const epsEl = document.getElementById('statusEps');
    const lagEl = document.getElementById('statusLag');
    const queueEl = document.getElementById('statusQueue');
    const bufEl = document.getElementById('statusBuffer');
    const capWrap = document.getElementById('statusCapacityWrap');
    const capEl = document.getElementById('statusCapacity');

    if (epsEl) epsEl.textContent = fmtNumber(inputEps != null ? Math.round(inputEps) : null);

    if (lagEl) {
        lagEl.textContent = fmtDuration(ingest.lag_sec);
        lagEl.classList.remove('ok', 'warn', 'bad');
        const lag = ingest.lag_sec || 0;
        if (lag >= 60) lagEl.classList.add('bad');
        else if (lag >= 10) lagEl.classList.add('warn');
        else lagEl.classList.add('ok');
    }

    if (queueEl) {
        queueEl.textContent = queueCapacity > 0
            ? fmtNumber(queueDepth) + '/' + fmtNumber(queueCapacity)
            : fmtNumber(queueDepth);
        queueEl.classList.remove('ok', 'warn', 'bad');
        if (queuePct >= 0.9) queueEl.classList.add('bad');
        else if (queuePct >= 0.75) queueEl.classList.add('warn');
        else queueEl.classList.add('ok');
    }

    if (bufEl) {
        const buf = ingest.buffered_lines || 0;
        bufEl.textContent = fmtNumber(buf);
        bufEl.classList.remove('ok', 'warn', 'bad');
        if (buf > 100000) bufEl.classList.add('bad');
        else if (buf > 10000) bufEl.classList.add('warn');
        else bufEl.classList.add('ok');
    }

    const profile = stats.install_profile || {};
    const cap = (profile.capacity) || {};
    if (capWrap && capEl && cap.expected_eps_max) {
        const currentEps = inputEps != null ? inputEps : 0;
        const pct = (currentEps / cap.expected_eps_max) * 100;
        capWrap.hidden = false;
        capEl.textContent = Math.round(pct) + '%';
        capEl.classList.remove('ok', 'warn', 'bad');
        if (pct > 125) capEl.classList.add('bad');
        else if (pct > 90) capEl.classList.add('warn');
        else capEl.classList.add('ok');
    } else if (capWrap) {
        capWrap.hidden = true;
    }
}

function fillKvGrid(el, rows) {
    if (!el) return;
    el.innerHTML = '';
    rows.forEach(function (kv) {
        const row = document.createElement('div');
        row.className = 'kv-row';
        row.innerHTML = '<span class="k">' + escapeHTML(kv[0]) + '</span><span class="v">' + escapeHTML(String(kv[1])) + '</span>';
        el.appendChild(row);
    });
}

function renderContainers(stats) {
    const row = document.getElementById('containersRow');
    if (!row) return;
    row.innerHTML = '';

    CONTAINERS.forEach(function (name) {
        const m = (stats.containers && stats.containers[name]) || {};
        const cpu = m.cpu_pct != null ? m.cpu_pct : 0;
        const mem = m.mem_bytes != null ? m.mem_bytes : 0;
        const isUp = mem > 0;

        const chip = document.createElement('div');
        chip.className = 'container-chip';
        chip.innerHTML =
            '<div class="name">' +
                '<span class="dot ' + (isUp ? '' : 'bad') + '"></span>' +
                escapeHTML(name) +
            '</div>' +
            '<div class="metrics">' +
                '<span>CPU <b>' + cpu.toFixed(1) + '%</b></span>' +
                '<span>Mem <b>' + fmtBytes(mem) + '</b></span>' +
            '</div>';
        row.appendChild(chip);
    });
}

function renderStorage(stats) {
    const storageList = document.getElementById('storageList');
    const ingestList = document.getElementById('ingestList');

    const traffic = (stats.storage && stats.storage.traffic_logs) || {};
    const geo = (stats.storage && stats.storage.geo_ranges) || {};
    const ch = (stats.storage && stats.storage.clickhouse) || {};
    const parseErr = (stats.pipeline && stats.pipeline.parse_errors) || {};
    const ingest = (stats.pipeline && stats.pipeline.ingest) || {};
    const queueDepth = ingest.queue_depth || 0;
    const queueCapacity = ingest.queue_capacity || 0;
    const queueRatio = queueCapacity > 0 ? (queueDepth / queueCapacity) : null;

    fillKvGrid(storageList, [
        ['traffic_logs', fmtNumber(traffic.row_count) + ' / ' + fmtBytes(traffic.bytes_on_disk)],
        ['geo_ranges', fmtNumber(geo.row_count) + ' диапазонов'],
        ['active parts', fmtNumber(ch.active_parts)],
    ]);

    fillKvGrid(ingestList, [
        ['Лаг', fmtDuration(ingest.lag_sec)],
        ['Buffered', fmtNumber(ingest.buffered_lines || 0)],
        ['Queue', fmtNumber(queueDepth) + ' / ' + fmtNumber(queueCapacity)
            + (queueRatio != null ? ' (' + fmtPercent(queueRatio) + ')' : '')],
        ['Received', fmtNumber(ingest.received_total || 0)],
        ['Inserted', fmtNumber(ingest.inserted_total || 0)],
        ['Skipped', fmtNumber(ingest.skipped_total || 0)],
        ['Parse err.', fmtNumber(ingest.parse_errors_total || 0)],
        ['Connections', fmtNumber(ingest.connections || 0)],
        ['Parse (1h)', fmtNumber(parseErr.count_1h || 0)],
        ['Uptime', fmtDuration(stats.uptime_sec)],
    ]);
}

function fillRetentionForm(r) {
    if (!r) return;
    const set = function (id, v) {
        const el = document.getElementById(id);
        if (el) el.value = v;
    };
    const setCur = function (id, v) {
        const el = document.getElementById(id);
        if (el) el.textContent = (v != null && v !== '') ? ('сейчас: ' + v + ' дн.') : 'сейчас: —';
    };
    set('retTrafficLogs', r.traffic_logs_days);
    set('retEdges', r.edges_days);
    set('retParseErrors', r.parse_errors_days);
    set('retSystemMetrics', r.system_metrics_days);
    setCur('retTrafficLogsCur', r.traffic_logs_days);
    setCur('retEdgesCur', r.edges_days);
    setCur('retParseErrorsCur', r.parse_errors_days);
    setCur('retSystemMetricsCur', r.system_metrics_days);
    const meta = document.getElementById('retentionUpdatedAt');
    if (meta) {
        meta.textContent = r.updated_at
            ? ('Сохранено: ' + r.updated_at)
            : '';
    }
}

async function loadRetention() {
    try {
        const res = await fetch('/api/system/retention', { credentials: 'same-origin' });
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        fillRetentionForm(data.retention || data);
    } catch (e) {
        console.warn('retention load failed', e);
    }
}

async function saveRetention(ev) {
    if (ev) ev.preventDefault();
    const toast = window.NMUI && NMUI.toast ? NMUI.toast : function () {};
    const body = {
        traffic_logs_days: Number(document.getElementById('retTrafficLogs').value),
        edges_days: Number(document.getElementById('retEdges').value),
        parse_errors_days: Number(document.getElementById('retParseErrors').value),
        system_metrics_days: Number(document.getElementById('retSystemMetrics').value),
    };
    const btn = document.getElementById('retentionSaveBtn');
    if (btn) btn.disabled = true;
    try {
        const res = await fetch('/api/system/retention', {
            method: 'PUT',
            credentials: 'same-origin',
            headers: NMAuth.nmAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify(body),
        });
        const data = await res.json().catch(function () { return {}; });
        if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
        fillRetentionForm(data.retention || body);
        toast('TTL сохранён и применён', 'success');
    } catch (e) {
        toast(e.message || 'Ошибка сохранения TTL', 'error');
    } finally {
        if (btn) btn.disabled = false;
    }
}

function renderComponentHealth(stats) {
    const grid = document.getElementById('componentHealthGrid');
    if (!grid) return;
    grid.innerHTML = '';

    const components = [
        {
            name: 'Backend',
            health: (stats.health && stats.health.backend) || {},
            fallbackState: 'unknown',
            meta: [
                'goroutines: ' + fmtNumber(stats.backend_info && stats.backend_info.num_goroutine),
                'heap: ' + (((stats.backend_info && stats.backend_info.heap_alloc_mb) != null)
                    ? stats.backend_info.heap_alloc_mb.toFixed(1) + ' MB'
                    : '—')
            ]
        },
        {
            name: 'Ingest',
            health: (stats.health && stats.health.ingest) || {},
            fallbackState: 'unknown',
            meta: [
                'conn: ' + fmtNumber(stats.pipeline && stats.pipeline.ingest && stats.pipeline.ingest.connections),
                'lag: ' + fmtDuration(stats.pipeline && stats.pipeline.ingest && stats.pipeline.ingest.lag_sec)
            ]
        }
    ];

    components.forEach(function (item) {
        const h = item.health || {};
        let stateText = h.state_text || item.fallbackState || 'unknown';
        let css = 'warn';

        if (h.up != null) {
            stateText = h.up >= 1 ? 'up' : 'down';
            css = h.up >= 1 ? 'ok' : 'bad';
        } else if (h.state != null) {
            if (h.state > 0) css = 'ok';
            else if (h.state < 0) css = 'bad';
            else css = 'warn';
        }

        const card = document.createElement('div');
        card.className = 'health-card ' + css;

        let html =
            '<div class="health-head">' +
                '<div class="health-name">' + escapeHTML(item.name) + '</div>' +
                '<div class="health-state">' + escapeHTML(String(stateText)) + '</div>' +
            '</div>' +
            '<div class="health-meta">' + escapeHTML(item.meta.join(' · ')) + '</div>';

        if (h.last_error) {
            html += '<div class="health-meta health-error">last_error: ' + escapeHTML(String(h.last_error)) + '</div>';
        }

        card.innerHTML = html;
        grid.appendChild(card);
    });
}

function renderInstallProfile(stats) {
    const section = document.getElementById('installProfileSection');
    const profile = stats.install_profile;
    if (!section) return;

    if (!profile || !profile.profile) {
        section.style.display = 'none';
        return;
    }

    const host = profile.host || {};
    const limits = profile.limits || {};
    const ch = limits.clickhouse || {};
    const be = limits.backend || {};
    const syslog = limits.syslog_ng || {};
    const cap = profile.capacity || {};

    const rate = (stats.pipeline && stats.pipeline.rate) || {};
    const currentEps = rate.input_events_per_sec != null ? rate.input_events_per_sec
        : (rate.events_per_sec != null ? rate.events_per_sec : 0);
    const maxEps = cap.expected_eps_max;
    const pct = maxEps ? (currentEps / maxEps) * 100 : 0;

    // Полный блок только при нагрузке > 90% (или если нет расчётной ёмкости — скрыт)
    const showFull = maxEps && pct > 90;
    section.style.display = showFull ? '' : 'none';
    if (!showFull) return;

    document.getElementById('profileBadge').textContent = profile.profile;

    const hostRows = [
        ['Профиль', (profile.profile_label || profile.profile)],
        ['CPU хоста', fmtNumber(host.cpu_cores) + ' ядер'],
        ['RAM хоста', host.ram_mb ? (host.ram_mb / 1024).toFixed(1) + ' GiB' : '—'],
        ['Диск (свободно)', host.disk_gb_avail ? host.disk_gb_avail + ' GiB' : '—'],
        ['Создан', profile.generated_at ? new Date(profile.generated_at).toLocaleString('ru-RU') : '—'],
    ];

    const limitRows = [
        ['ClickHouse', (ch.memory_gb || '—') + ' GiB / ' + (ch.cpus || '—') + ' CPU'],
        ['Backend', (be.memory_gb || '—') + ' GiB / ' + (be.cpus || '—') + ' CPU'],
        ['Ingest workers', fmtNumber(be.ingest_workers)],
        ['Очередь ingest', fmtNumber(be.ingest_queue_size)],
        ['Batch size', fmtNumber(be.ingest_batch_size)],
        ['syslog-ng', (syslog.memory_mb || '—') + ' MiB / ' + (syslog.cpus || '—') + ' CPU'],
        ['Расчётная EPS', 'до ' + fmtNumber(maxEps) + ' /с'],
    ];

    fillKvGrid(document.getElementById('profileHostList'), hostRows);
    fillKvGrid(document.getElementById('profileLimitsList'), limitRows);

    const meter = document.getElementById('capacityMeter');
    const fill = document.getElementById('capacityFill');
    const label = document.getElementById('capacityLabel');
    const hint = document.getElementById('capacityHint');

    meter.style.display = '';
    const displayPct = Math.min(100, Math.min(150, pct));
    fill.style.width = displayPct + '%';
    fill.classList.remove('warn', 'bad');
    if (pct > 125) fill.classList.add('bad');
    else if (pct > 90) fill.classList.add('warn');

    label.textContent = fmtNumber(Math.round(currentEps)) + ' / ' + fmtNumber(maxEps) + ' eps';
    hint.textContent = pct > 125
        ? 'Нагрузка превышает расчётную ёмкость профиля — рассмотрите upgrade или ./scripts/tune-resources.sh'
        : 'Нагрузка близка к лимиту профиля';
}

function renderEdgesAgg(stats) {
    const ea = stats.edges_agg || {};
    const badge = document.getElementById('edgesAggBadge');
    const primary = document.getElementById('edgesAggPrimary');
    const secondary = document.getElementById('edgesAggSecondary');
    const details = document.getElementById('edgesAggDetails');
    const hint = document.getElementById('edgesAggHint');
    if (!badge || !primary || !hint) return;

    const state = ea.state || 'idle';
    const phase = ea.phase || '';
    badge.textContent = phase ? (state + '/' + phase) : state;

    const primaryRows = [
        ['Raw / agg', fmtNumber(ea.raw_rows) + ' / ' + fmtNumber(ea.agg_rows)],
        ['Карта', ea.map_source || '—'],
        ['Backfill', ea.days_total
            ? (fmtNumber(ea.days_done) + ' / ' + fmtNumber(ea.days_total))
            : '—'],
    ];

    const secondaryRows = [
        ['Сообщение', ea.message || '—'],
        ['prefer_agg', ea.prefer_agg ? 'да' : 'нет'],
        ['geo prefer_agg', ea.geo_prefer_agg ? 'да' : 'нет'],
        ['Запущено', ea.started_at ? new Date(ea.started_at).toLocaleString('ru-RU') : '—'],
        ['Обновлено', ea.updated_at ? new Date(ea.updated_at).toLocaleString('ru-RU') : '—'],
    ];

    fillKvGrid(primary, primaryRows);
    fillKvGrid(secondary, secondaryRows);

    if (details) {
        const shouldOpen = state === 'running' || state === 'error';
        if (shouldOpen) details.open = true;
        else if (state === 'ready' || state === 'idle') details.open = false;
    }

    if (state === 'running' && phase === 'schema') {
        hint.textContent = 'Идёт DROP/CREATE MV — карта временно на сырых traffic_logs.';
    } else if (state === 'running') {
        hint.textContent = 'Backfill агрегатов — карта на traffic_logs до state=ready.';
    } else if (state === 'ready') {
        hint.textContent = 'Агрегаты готовы — /api/events предпочитает edges_daily.';
    } else if (state === 'error') {
        hint.textContent = 'Ошибка Ensure*/backfill — перезапустите backend или см. логи.';
    } else {
        hint.textContent = '';
    }
}

function renderAlertsInto(listEl, counterEl, alerts) {
    if (!listEl) return;
    listEl.innerHTML = '';
    if (counterEl) counterEl.textContent = alerts.length ? '(' + alerts.length + ')' : '';

    const chromeAlerts = document.getElementById('chromeAlerts');
    if (chromeAlerts) chromeAlerts.classList.toggle('chrome-alerts--empty', alerts.length === 0);

    if (alerts.length === 0) {
        const row = document.createElement('div');
        row.className = 'alert-row info';
        row.innerHTML = '<div class="empty">Активных алёртов нет</div>';
        listEl.appendChild(row);
        return;
    }

    const maxShow = alerts.length;
    alerts.slice(0, maxShow).forEach(function (a) {
        const row = document.createElement('div');
        row.className = 'alert-row ' + a.level;
        row.innerHTML =
            '<span class="level">' + escapeHTML(a.level) + '</span>' +
            '<div>' +
                '<div class="code">' + escapeHTML(a.code || 'no_code') + '</div>' +
                '<span class="target">' + escapeHTML(a.target) + '</span>' +
                escapeHTML(a.message) +
            '</div>';
        listEl.appendChild(row);
    });

}

function renderAlerts(stats) {
    const alerts = stats.alerts || [];
    renderAlertsInto(
        document.getElementById('alertsList'),
        document.getElementById('alertsCount'),
        alerts
    );

    const badge = document.getElementById('securityTabBadge');
    if (badge) {
        const fails = (stats.failed_logins || []).length;
        if (fails > 0) {
            badge.hidden = false;
            badge.textContent = String(fails);
            badge.classList.remove('warn');
        } else {
            badge.hidden = true;
        }
    }
}

function fmtDateTime(iso) {
    if (!iso) return '—';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return escapeHTML(String(iso));
    return d.toLocaleString('ru-RU');
}

function renderFailedLogins(stats) {
    const host = document.getElementById('failedLoginsHost');
    const counter = document.getElementById('failedLoginsCount');
    const details = document.getElementById('failedLoginsDetails');
    const rows = stats.failed_logins || [];
    if (!host) return;

    if (counter) counter.textContent = rows.length ? '(' + rows.length + ')' : '(нет)';

    if (rows.length === 0) {
        host.innerHTML = '<div class="auth-fails-empty">За последние 24 ч неуспешных попыток нет (или backend перезапускался)</div>';
        if (details) details.open = false;
        return;
    }

    if (details) details.open = true;

    let html =
      '<table class="auth-fails-table"><thead><tr>' +
        '<th scope="col">Учётная запись</th>' +
        '<th scope="col">Адрес</th>' +
        '<th scope="col">Попыток</th>' +
        '<th scope="col">Первая</th>' +
        '<th scope="col">Последняя</th>' +
      '</tr></thead><tbody>';

    rows.forEach(function (r) {
        const locked = r.locked
            ? '<span class="badge-locked" title="' + escapeHTML(r.locked_until ? ('до ' + fmtDateTime(r.locked_until)) : 'lockout') + '">lockout</span>'
            : '';
        html += '<tr>' +
            '<td>' + escapeHTML(r.username || '—') + locked + '</td>' +
            '<td class="mono">' + escapeHTML(r.ip || '—') + '</td>' +
            '<td><b>' + escapeHTML(String(r.count || 0)) + '</b></td>' +
            '<td>' + fmtDateTime(r.first_at) + '</td>' +
            '<td>' + fmtDateTime(r.last_at) + '</td>' +
            '</tr>';
    });
    html += '</tbody></table>';
    host.innerHTML = html;
}

function renderFooter(stats) {
    const heap = stats.backend_info && stats.backend_info.heap_alloc_mb;
    const goro = stats.backend_info && stats.backend_info.num_goroutine;
    const ts = new Date(stats.timestamp).toLocaleString('ru-RU');
    const text = 'обновлено: ' + ts +
        ' · backend heap: ' + (heap ? heap.toFixed(1) + ' MB' : '—') +
        ' · goroutines: ' + (goro != null ? goro : '—');
    ['footerInfo', 'footerInfoOverview'].forEach(function (id) {
        const el = document.getElementById(id);
        if (el) el.textContent = text;
    });
}

function renderHistoryMeta(history) {
    const el = document.getElementById('historyMeta');
    if (!el) return;
    if (!history) {
        el.textContent = '';
        return;
    }
    el.innerHTML =
        '<span>Период: <b>' + escapeHTML(history.period || currentPeriod) + '</b></span>' +
        '<span>Шаг: <b>' + escapeHTML(String(history.step_sec || '—')) + ' сек</b></span>' +
        '<span>Окно: <b>' + escapeHTML(fmtDateTime(history.from)) + '</b> - <b>' + escapeHTML(fmtDateTime(history.to)) + '</b></span>';
}

function initCharts() {
    const safeMake = function (id, series, opts) {
        try {
            return makeChart(id, series, opts);
        } catch (e) {
            console.error('makeChart failed for', id, e);
            return null;
        }
    };

    charts.events = safeMake('chart-events', [
        { label: 'Ingest rate (live)', color: COLORS.rate },
        { label: 'DB ingest (1m avg)', color: COLORS.db_rate },
    ]);
    charts.lag = safeMake('chart-lag', [
        { label: 'Lag (sec)', color: COLORS.lag },
    ]);
    charts.cpu = safeMake('chart-cpu', CONTAINERS.map(function (c) {
        return { label: c, color: COLORS[c] };
    }), { isPercent: true });
    charts.mem = safeMake('chart-mem', CONTAINERS.map(function (c) {
        return { label: c, color: COLORS[c] };
    }), { isBytes: true });
    charts.buffer = safeMake('chart-buffer', [
        { label: 'Buffered lines', color: COLORS.buffer },
    ], { isInt: true });
    charts.storage = safeMake('chart-storage', [
        { label: 'traffic_logs (MB)', color: COLORS.storage },
    ]);
}

function alignSeries(history, keys) {
    const tsSet = new Set();
    keys.forEach(function (k) {
        const arr = (history.series && history.series[k]) || [];
        arr.forEach(function (p) {
            tsSet.add(Math.floor(new Date(p.t).getTime() / 1000));
        });
    });
    const xs = Array.from(tsSet).sort(function (a, b) { return a - b; });

    const seriesData = keys.map(function (k) {
        const arr = (history.series && history.series[k]) || [];
        const map = new Map();
        arr.forEach(function (p) {
            map.set(Math.floor(new Date(p.t).getTime() / 1000), p.v);
        });
        return xs.map(function (t) { return map.has(t) ? map.get(t) : null; });
    });

    return { xs: xs, seriesData: seriesData };
}

function updateAllCharts(history) {
    if (!history || !history.series) return;
    const summary = {};

    function applyChart(chartName, keys, transform) {
        const aligned = alignSeries(history, keys);
        const validCount = aligned.seriesData.reduce(function (sum, arr) {
            return sum + arr.filter(function (v) { return v != null && !isNaN(v); }).length;
        }, 0);
        summary[chartName] = 'xs=' + aligned.xs.length + ' valid=' + validCount;
        const data = transform ? transform(aligned.seriesData) : aligned.seriesData;
        updateChart(charts[chartName], aligned.xs, data);
    }

    applyChart('events', [
        'pipeline.rate.events_per_sec',
        'pipeline.ingest.events_per_sec_db'
    ]);
    applyChart('lag', ['pipeline.ingest.lag_sec']);
    applyChart('cpu', CONTAINERS.map(function (c) { return 'container.' + c + '.cpu_pct'; }));
    applyChart('mem', CONTAINERS.map(function (c) { return 'container.' + c + '.mem_bytes'; }));
    applyChart('buffer', ['pipeline.ingest.buffered_lines']);
    applyChart('storage',
        ['storage.traffic_logs.bytes_on_disk'],
        function (seriesData) {
            return seriesData.map(function (arr) {
                return arr.map(function (v) { return v == null ? null : v / 1024 / 1024; });
            });
        }
    );

    console.log('charts update:', summary);
}

async function fetchStats() {
    try {
        const res = await fetch('/api/system/stats');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        renderHealthPill(data);
        renderStatusStrip(data);
        renderComponentHealth(data);
        renderInstallProfile(data);
        renderEdgesAgg(data);
        renderPipeline(data);
        renderContainers(data);
        renderStorage(data);
        renderAlerts(data);
        renderFailedLogins(data);
        renderFooter(data);
    } catch (e) {
        console.error('fetchStats:', e);
        const pill = document.getElementById('healthPill');
        pill.classList.remove('ok', 'warn');
        pill.classList.add('bad');
        document.getElementById('healthText').textContent = 'API ошибка';
    }
}

function setActiveTab(tab) {
    const valid = { overview: 1, pipeline: 1, security: 1, charts: 1 };
    if (!valid[tab]) tab = 'overview';
    currentTab = tab;

    document.querySelectorAll('.view-tabs button').forEach(function (btn) {
        const on = btn.dataset.tab === tab;
        btn.classList.toggle('active', on);
        btn.setAttribute('aria-selected', on ? 'true' : 'false');
    });

    document.querySelectorAll('.tab-panel').forEach(function (panel) {
        const on = panel.dataset.tab === tab;
        panel.classList.toggle('active', on);
        panel.hidden = !on;
    });

    const periodTabs = document.getElementById('periodTabs');
    if (periodTabs) periodTabs.hidden = tab !== 'charts';

    try { localStorage.setItem(TAB_KEY, tab); } catch (e) {}

    if (tab === 'charts') {
        requestAnimationFrame(function () {
            resizeCharts();
            chartsNeedResize = false;
        });
    } else {
        chartsNeedResize = true;
    }
}

function initTabs() {
    let initial = 'overview';
    try {
        const saved = localStorage.getItem(TAB_KEY);
        if (saved) initial = saved;
    } catch (e) {}

    document.querySelectorAll('.view-tabs button').forEach(function (btn) {
        btn.addEventListener('click', function () {
            setActiveTab(btn.dataset.tab);
        });
    });

    setActiveTab(initial);
}

async function fetchHistory() {
    try {
        const url = '/api/system/history?period=' + encodeURIComponent(currentPeriod);
        const res = await fetch(url);
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        renderHistoryMeta(data);
        updateAllCharts(data);
    } catch (e) {
        console.error('fetchHistory:', e);
        renderHistoryMeta(null);
    }
}

async function fetchHistorySafe() {
    if (isFetchingHistory) return;
    isFetchingHistory = true;
    try { await fetchHistory(); } finally { isFetchingHistory = false; }
}

async function refresh() {
    await Promise.all([fetchStats(), fetchHistory()]);
}

function startAutoRefresh() {
    stopAutoRefresh();
    statsTimer = setInterval(function () {
        if (isAutoRefresh()) fetchStats();
    }, STATS_REFRESH_MS);
    historyTimer = setInterval(function () {
        if (isAutoRefresh()) fetchHistorySafe();
    }, HISTORY_REFRESH_MS);
}

function stopAutoRefresh() {
    if (statsTimer)   { clearInterval(statsTimer);   statsTimer = null; }
    if (historyTimer) { clearInterval(historyTimer); historyTimer = null; }
}

document.querySelectorAll('.period-tabs button').forEach(function (btn) {
    btn.addEventListener('click', function () {
        document.querySelectorAll('.period-tabs button').forEach(function (b) {
            b.classList.remove('active');
        });
        btn.classList.add('active');
        currentPeriod = btn.dataset.period;
        fetchHistorySafe();
    });
});

document.getElementById('autoRefreshChk').addEventListener('change', function (e) {
    if (e.target.checked) startAutoRefresh(); else stopAutoRefresh();
});


document.addEventListener('nm-theme-change', function () {
    Object.values(charts).forEach(function (c) {
        if (!c) return;
        try { c.destroy(); } catch (e) {}
    });
    charts = {};
    initCharts();
    fetchHistorySafe();
});

window.addEventListener('load', async function () {
    const user = await NMAuth.requireLogin({ admin: true });
    if (!user) return;
    NMAuth.renderUserBar(user, document.getElementById('userBarHost'));
    initTabs();
    initCharts();
    const form = document.getElementById('retentionForm');
    if (form) form.addEventListener('submit', saveRetention);
    loadRetention();
    refresh();
    startAutoRefresh();
});
