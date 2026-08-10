import { FormEvent, useCallback, useEffect, useMemo, useRef, useState, type RefObject } from 'react';
import uPlot from 'uplot';
import { apiFetch } from '@/api/client';
import { AdminSidebar, UserMenu } from '@/components/Shell';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber, escapeHTML } from '@/lib/format';
import { getTheme } from '@/auth/theme';
import 'uplot/dist/uPlot.min.css';
import '@/styles/system.css';

type Tab = 'overview' | 'pipeline' | 'security' | 'charts';

const CONTAINERS = ['backend', 'clickhouse', 'syslog-ng', 'frontend'] as const;
const PERIODS = [
  ['1h', '1ч'],
  ['6h', '6ч'],
  ['24h', '24ч'],
  ['7d', '7д'],
] as const;

interface Alert {
  level?: string;
  code?: string;
  target?: string;
  message?: string;
}

interface FailedLogin {
  username?: string;
  ip?: string;
  count?: number;
  first_at?: string;
  last_at?: string;
  locked?: boolean;
  locked_until?: string;
}

interface EdgesAgg {
  state?: string;
  phase?: string;
  message?: string;
  raw_rows?: number;
  agg_rows?: number;
  days_total?: number;
  days_done?: number;
  map_source?: string;
  prefer_agg?: boolean;
  geo_prefer_agg?: boolean;
  started_at?: string;
  updated_at?: string;
}

interface SystemStats {
  alerts?: Alert[];
  containers?: Record<string, { cpu_pct?: number; mem_bytes?: number }>;
  health?: Record<string, Record<string, unknown>>;
  pipeline?: Record<string, Record<string, number>>;
  storage?: Record<string, Record<string, number>>;
  backend_info?: { num_goroutine?: number; heap_alloc_mb?: number; go_version?: string };
  install_profile?: {
    profile?: string;
    profile_label?: string;
    host?: Record<string, unknown>;
    limits?: Record<string, unknown>;
    capacity?: { expected_eps_min?: number; expected_eps_max?: number };
  };
  edges_agg?: EdgesAgg;
  failed_logins?: FailedLogin[];
  uptime_sec?: number;
  timestamp?: string;
}

interface Retention {
  traffic_logs_days?: number;
  edges_days?: number;
  parse_errors_days?: number;
  system_metrics_days?: number;
  updated_at?: string;
}

interface HistoryPoint {
  t: string;
  v: number;
}

interface HistoryPayload {
  period?: string;
  from?: string;
  to?: string;
  series?: Record<string, HistoryPoint[]>;
}

function num(v: unknown): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

function fmtBytes(bytes: unknown): string {
  const b = num(bytes);
  if (b < 1024) return `${fmtNumber(b)} Б`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} КБ`;
  if (b < 1024 * 1024 * 1024) return `${(b / (1024 * 1024)).toFixed(1)} МБ`;
  return `${(b / (1024 * 1024 * 1024)).toFixed(2)} ГБ`;
}

/** Axis ticks share one unit (from max split) so labels stay ordered and readable. */
function fmtBytesAxisTicks(splits: number[]): string[] {
  const finite = splits.filter((v) => v != null && Number.isFinite(v));
  const maxAbs = finite.reduce((m, v) => Math.max(m, Math.abs(v)), 0);
  let div = 1;
  let suffix = 'Б';
  let digits = 0;
  if (maxAbs >= 1024 * 1024 * 1024) {
    div = 1024 * 1024 * 1024;
    suffix = 'ГБ';
    digits = 2;
  } else if (maxAbs >= 1024 * 1024) {
    div = 1024 * 1024;
    suffix = 'МБ';
    digits = 1;
  } else if (maxAbs >= 1024) {
    div = 1024;
    suffix = 'КБ';
    digits = 1;
  }
  return splits.map((v) => {
    if (v == null || !Number.isFinite(v)) return '';
    if (div === 1) return `${fmtNumber(v)} ${suffix}`;
    return `${(v / div).toFixed(digits)} ${suffix}`;
  });
}

function fmtLag(sec: unknown): string {
  if (sec == null || sec === '') return '—';
  const s = num(sec);
  if (s < 1) return '<1 с';
  if (s < 60) return `${Math.round(s)} с`;
  if (s < 3600) return `${Math.round(s / 60)} мин`;
  return `${(s / 3600).toFixed(1)} ч`;
}

function fmtUptime(sec: unknown): string {
  const s = num(sec);
  if (s < 60) return `${Math.round(s)} с`;
  if (s < 3600) return `${Math.round(s / 60)} мин`;
  return `${(s / 3600).toFixed(1)} ч`;
}

function toneClass(kind: 'ok' | 'warn' | 'bad' | ''): string {
  return kind || '';
}

function queueTone(depth: number, capacity: number): 'ok' | 'warn' | 'bad' {
  if (!capacity) return 'ok';
  const r = depth / capacity;
  if (r >= 0.9) return 'bad';
  if (r >= 0.75) return 'warn';
  return 'ok';
}

function fmtPercent(ratio: number): string {
  return `${(ratio * 100).toFixed(1)}%`;
}

function pipelineIngestStatus(
  rate: Record<string, number>,
  ingest: Record<string, number>,
  queuePct: number,
): 'ok' | 'warn' | 'bad' {
  const drops = num(rate.drops_per_sec);
  const bufDrops = num(rate.buffer_drops_per_sec);
  const dropped = num(ingest.dropped_total);
  const bufDropped = num(ingest.buffer_drops_total);
  const buffered = num(ingest.buffered_lines);
  if (drops >= 100 || bufDrops >= 100) return 'bad';
  if (queuePct >= 0.9) return 'bad';
  if (queuePct >= 0.75) return 'warn';
  if (drops > 0 || bufDrops > 0 || dropped > 0 || bufDropped > 0) return 'warn';
  if (buffered > 100000) return 'bad';
  if (buffered > 10000) return 'warn';
  if (num(ingest.circuit_open) >= 1) return 'warn';
  return 'ok';
}

function dropsTone(admPerSec: number, bufPerSec: number): 'ok' | 'warn' | 'bad' {
  const total = admPerSec + bufPerSec;
  if (total >= 100) return 'bad';
  if (total > 0) return 'warn';
  return 'ok';
}

function fmtDropAt(iso: unknown): string {
  if (iso == null || iso === '') return '—';
  const s = String(iso);
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  return d.toLocaleString();
}

function edgesAggHint(edges?: EdgesAgg): string {
  if (!edges) return '';
  if (edges.message) return edges.message;
  const state = edges.state || 'idle';
  const phase = edges.phase || '';
  if (state === 'running' && phase === 'schema') {
    return 'Идёт DROP/CREATE MV — карта временно на сырых traffic_logs.';
  }
  if (state === 'running') {
    return 'Backfill агрегатов — карта на traffic_logs до state=ready.';
  }
  if (state === 'ready') {
    return 'Агрегаты готовы — /api/events предпочитает edges_daily.';
  }
  if (state === 'error') {
    return 'Ошибка Ensure*/backfill — перезапустите backend или см. логи.';
  }
  return '';
}

function lagTone(sec: number): 'ok' | 'warn' | 'bad' | '' {
  if (!sec) return '';
  if (sec >= 60) return 'bad';
  if (sec >= 10) return 'warn';
  return 'ok';
}

function bufferTone(n: number): 'ok' | 'warn' | 'bad' {
  if (n >= 100000) return 'bad';
  if (n >= 10000) return 'warn';
  return 'ok';
}

function capacityTone(pct: number): 'ok' | 'warn' | 'bad' {
  if (pct >= 125) return 'bad';
  if (pct >= 90) return 'warn';
  return 'ok';
}

function chartAxisStroke(): string {
  return getTheme() === 'light' ? '#334155' : '#94a3b8';
}

function alignSeries(
  series: Record<string, HistoryPoint[]> | undefined,
  keys: string[],
): { xs: number[]; ys: number[][] } {
  const maps = keys.map((k) => {
    const m = new Map<number, number>();
    for (const p of series?.[k] || []) {
      const t = Math.floor(new Date(p.t).getTime() / 1000);
      if (Number.isFinite(t)) m.set(t, num(p.v));
    }
    return m;
  });
  const xs = Array.from(
    new Set(maps.flatMap((m) => Array.from(m.keys()))),
  ).sort((a, b) => a - b);
  const ys = maps.map((m) => xs.map((t) => (m.has(t) ? (m.get(t) as number) : null as unknown as number)));
  return { xs, ys };
}

type ChartFormatOpts = { isBytes?: boolean; isPercent?: boolean; isInt?: boolean };

function formatSeriesValue(v: number | null | undefined, opts?: ChartFormatOpts): string {
  if (v == null || Number.isNaN(Number(v))) return '—';
  if (opts?.isPercent) return `${Number(v).toFixed(2)}%`;
  if (opts?.isBytes) return fmtBytes(v);
  if (opts?.isInt) return fmtNumber(Math.round(Number(v)));
  const n = Number(v);
  if (Math.abs(n) < 1) return n.toFixed(3);
  if (Math.abs(n) < 100) return n.toFixed(1);
  return fmtNumber(Math.round(n));
}

function buildChartLegend(labels: string[], colors: string[]): HTMLDivElement {
  const legend = document.createElement('div');
  legend.className = 'chart-legend';
  labels.forEach((label, i) => {
    const item = document.createElement('div');
    item.className = 'chart-legend-item';
    const color = colors[i % colors.length];
    item.innerHTML =
      `<span class="chart-legend-marker" style="background:${escapeHTML(color)};"></span>` +
      `<span class="chart-legend-label">${escapeHTML(label)}</span>` +
      `<span class="chart-legend-value">—</span>`;
    legend.appendChild(item);
  });
  return legend;
}

function updateCustomLegend(u: uPlot, legendEl: HTMLElement, opts?: ChartFormatOpts): void {
  const valueEls = legendEl.querySelectorAll('.chart-legend-value');
  const idx = u.cursor?.idx ?? null;
  const data = u.data || [];
  valueEls.forEach((el, i) => {
    const seriesIdx = i + 1;
    const series = data[seriesIdx];
    let v: number | null = null;
    if (idx != null && series) {
      const raw = series[idx];
      v = raw == null || Number.isNaN(raw) ? null : raw;
    } else if (series?.length) {
      for (let j = series.length - 1; j >= 0; j--) {
        const raw = series[j];
        if (raw != null && !Number.isNaN(raw)) {
          v = raw;
          break;
        }
      }
    }
    el.textContent = formatSeriesValue(v, opts);
  });
}

function makeChart(
  host: HTMLElement,
  title: string,
  labels: string[],
  xs: number[],
  ys: number[][],
  opts?: ChartFormatOpts,
): uPlot {
  host.innerHTML = '';
  const titleEl = document.createElement('div');
  titleEl.className = 'chart-title';
  titleEl.textContent = title;
  const colors = ['#38bdf8', '#a78bfa', '#fbbf24', '#2dd4bf', '#f472b6', '#94a3b8'];
  const legend = buildChartLegend(labels, colors);
  const plotHost = document.createElement('div');
  plotHost.className = 'chart-plot-host';
  host.appendChild(titleEl);
  host.appendChild(legend);
  host.appendChild(plotHost);

  const series: uPlot.Series[] = [{ label: 'Время' }];
  labels.forEach((label, i) => {
    series.push({
      label,
      stroke: colors[i % colors.length],
      width: 1.5,
      fill: i === 0 && labels.length === 1 ? `${colors[0]}22` : undefined,
      points: { show: false },
    });
  });

  const chromeH = titleEl.offsetHeight + legend.offsetHeight + 8;
  const height = Math.max(140, (host.clientHeight || 220) - chromeH);
  const plot = new uPlot(
    {
      width: host.clientWidth || 480,
      height,
      series,
      legend: { show: false },
      cursor: { drag: { x: false, y: false } },
      hooks: {
        setCursor: [(u) => updateCustomLegend(u, legend, opts)],
        setData: [(u) => updateCustomLegend(u, legend, opts)],
      },
      axes: [
        { stroke: chartAxisStroke(), grid: { stroke: 'rgba(148,163,184,0.12)' } },
        {
          stroke: chartAxisStroke(),
          grid: { stroke: 'rgba(148,163,184,0.12)' },
          values: (_u, splits) => {
            if (opts?.isBytes) return fmtBytesAxisTicks(splits);
            return splits.map((v) => {
              if (v == null || Number.isNaN(v)) return '';
              if (opts?.isPercent) return `${v.toFixed(1)}%`;
              if (opts?.isInt) return fmtNumber(Math.round(v));
              return formatSeriesValue(v, opts);
            });
          },
        },
      ],
      scales: { x: { time: true } },
    },
    [xs, ...ys],
    plotHost,
  );
  updateCustomLegend(plot, legend, opts);
  return plot;
}

export default function SystemPage() {
  const { toast } = useToast();
  const [tab, setTab] = useState<Tab>(() => {
    try {
      return (localStorage.getItem('nm.system.tab') as Tab) || 'overview';
    } catch {
      return 'overview';
    }
  });
  const [stats, setStats] = useState<SystemStats | null>(null);
  const [retention, setRetention] = useState<Retention>({
    traffic_logs_days: 30,
    edges_days: 30,
    parse_errors_days: 7,
    system_metrics_days: 7,
  });
  const [period, setPeriod] = useState('1h');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const [themeTick, setThemeTick] = useState(0);
  const chartEvents = useRef<HTMLDivElement>(null);
  const chartLag = useRef<HTMLDivElement>(null);
  const chartCpu = useRef<HTMLDivElement>(null);
  const chartMem = useRef<HTMLDivElement>(null);
  const chartBuffer = useRef<HTMLDivElement>(null);
  const chartStorage = useRef<HTMLDivElement>(null);
  const plotsRef = useRef<uPlot[]>([]);

  const loadStats = useCallback(async () => {
    try {
      const data = await apiFetch<SystemStats>('/api/system/stats');
      setStats(data);
      setUpdatedAt(new Date());
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка stats', 'error');
    }
  }, [toast]);

  const loadRetention = useCallback(async () => {
    try {
      const data = await apiFetch<{ retention?: Retention } & Retention>('/api/system/retention');
      setRetention(data.retention || data);
    } catch {
      /* optional */
    }
  }, []);

  useEffect(() => {
    document.title = 'ГеоАтлас — Мониторинг';
    document.body.classList.add('page-admin');
    return () => document.body.classList.remove('page-admin');
  }, []);

  useEffect(() => {
    void loadStats();
    void loadRetention();
  }, [loadStats, loadRetention]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => void loadStats(), 5000);
    return () => window.clearInterval(id);
  }, [autoRefresh, loadStats]);

  useEffect(() => {
    try {
      localStorage.setItem('nm.system.tab', tab);
    } catch {
      /* ignore */
    }
  }, [tab]);

  useEffect(() => {
    const onTheme = () => setThemeTick((n) => n + 1);
    document.addEventListener('nm-theme-change', onTheme);
    return () => document.removeEventListener('nm-theme-change', onTheme);
  }, []);

  const paintHistory = useCallback(
    (data: HistoryPayload) => {
      plotsRef.current.forEach((p) => p.destroy());
      plotsRef.current = [];

      const mk = (
        ref: RefObject<HTMLDivElement | null>,
        title: string,
        labels: string[],
        keys: string[],
        opts?: ChartFormatOpts,
      ) => {
        if (!ref.current) return;
        const { xs, ys } = alignSeries(data.series, keys);
        if (!xs.length) {
          ref.current.innerHTML = `<div class="chart-title">${title}</div><div class="empty" style="padding:24px">Нет данных</div>`;
          return;
        }
        plotsRef.current.push(makeChart(ref.current, title, labels, xs, ys, opts));
      };

      mk(chartEvents, 'События / сек', ['Ingest rate (live)', 'DB ingest (1m avg)'], [
        'pipeline.rate.events_per_sec',
        'pipeline.ingest.events_per_sec_db',
      ]);
      mk(chartLag, 'Лаг ingest (сек)', ['Lag (sec)'], ['pipeline.ingest.lag_sec']);
      mk(
        chartCpu,
        'CPU контейнеров (%)',
        [...CONTAINERS],
        CONTAINERS.map((c) => `container.${c}.cpu_pct`),
        { isPercent: true },
      );
      mk(
        chartMem,
        'Память контейнеров',
        [...CONTAINERS],
        CONTAINERS.map((c) => `container.${c}.mem_bytes`),
        { isBytes: true },
      );
      mk(
        chartBuffer,
        'Буфер импортера',
        ['Buffered lines'],
        ['pipeline.ingest.buffered_lines'],
        { isInt: true },
      );
      mk(
        chartStorage,
        'Размер хранилища',
        ['traffic_logs'],
        ['storage.traffic_logs.bytes_on_disk'],
        { isBytes: true },
      );
    },
    [],
  );

  useEffect(() => {
    if (tab !== 'charts') {
      plotsRef.current.forEach((p) => p.destroy());
      plotsRef.current = [];
      return;
    }
    let cancelled = false;
    let fetching = false;
    const loadHistory = async () => {
      if (fetching) return;
      fetching = true;
      try {
        const data = await apiFetch<HistoryPayload>(
          `/api/system/history?period=${encodeURIComponent(period)}`,
        );
        if (cancelled) return;
        paintHistory(data);
      } catch (e) {
        if (!cancelled) toast(e instanceof Error ? e.message : 'Ошибка history', 'error');
      } finally {
        fetching = false;
      }
    };
    void loadHistory();
    // SoT: history timer 30s while auto-refresh on (independent of stats 5s).
    const histId =
      autoRefresh
        ? window.setInterval(() => {
            void loadHistory();
          }, 30000)
        : 0;
    return () => {
      cancelled = true;
      if (histId) window.clearInterval(histId);
      plotsRef.current.forEach((p) => p.destroy());
      plotsRef.current = [];
    };
  }, [tab, period, toast, themeTick, autoRefresh, paintHistory]);

  async function saveRetention(e: FormEvent) {
    e.preventDefault();
    try {
      const data = await apiFetch<{ retention?: Retention }>('/api/system/retention', {
        method: 'PUT',
        body: JSON.stringify({
          traffic_logs_days: retention.traffic_logs_days,
          edges_days: retention.edges_days,
          parse_errors_days: retention.parse_errors_days,
          system_metrics_days: retention.system_metrics_days,
        }),
      });
      setRetention(data.retention || retention);
      toast('TTL сохранён', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  const pipeline = stats?.pipeline || {};
  const ingest = pipeline.ingest || {};
  const rate = pipeline.rate || {};
  const storage = stats?.storage || {};
  const alerts = stats?.alerts || [];
  const failed = stats?.failed_logins || [];
  const edges = stats?.edges_agg;

  const eps = num(rate.input_events_per_sec ?? rate.events_per_sec);
  const lag = num(ingest.lag_sec);
  const qDepth = num(ingest.queue_depth);
  const qCap = num(ingest.queue_capacity);
  const queuePct = qCap > 0 ? qDepth / qCap : 0;
  const buffered = num(ingest.buffered_lines);
  const dropsPerSec = num(rate.drops_per_sec);
  const bufferDropsPerSec = num(rate.buffer_drops_per_sec);
  const qBytes = num(ingest.queue_bytes);
  const qBytesCap = num(ingest.queue_bytes_capacity);
  const lastDropAt =
    (stats?.health?.ingest?.last_drop_at as string | undefined) ||
    (ingest.last_drop_at as unknown as string | undefined) ||
    '';
  const epsMax = num(stats?.install_profile?.capacity?.expected_eps_max);
  const capPct = epsMax > 0 ? Math.round((eps / epsMax) * 100) : 0;
  const showInstallProfile = !!stats?.install_profile?.profile && !!epsMax && capPct > 90;
  const parseErr1h = num(pipeline.parse_errors?.count_1h ?? ingest.parse_errors_1h);
  const uptimeSec = num(stats?.uptime_sec ?? ingest.uptime_sec);
  const ingestStageStatus = pipelineIngestStatus(rate as Record<string, number>, ingest as Record<string, number>, queuePct);
  const edgesHint = edgesAggHint(edges);
  const edgesBadge = edges?.phase
    ? `${edges.state || 'idle'}/${edges.phase}`
    : edges?.state || '—';

  const healthLevel = useMemo(() => {
    const hasError = alerts.some((a) => a.level === 'error');
    const hasWarn = alerts.some((a) => a.level === 'warn');
    if (hasError) return 'bad' as const;
    if (hasWarn) return 'warn' as const;
    return 'ok' as const;
  }, [alerts]);

  const healthText =
    healthLevel === 'bad'
      ? `${alerts.length} проблем`
      : healthLevel === 'warn'
        ? `${alerts.length} предупр.`
        : 'Всё ОК';

  const backendHealth = stats?.health?.backend || {};
  const ingestHealth = stats?.health?.ingest || {};

  return (
    <div id="adminApp" className="app">
      <AdminSidebar />
      <div className="admin-main">
        <header className="header">
          <div className="title-block">
            <h1>Мониторинг системы</h1>
            <div className="subtitle">ГеоАтлас · pipeline / containers / storage</div>
          </div>
          <div className="spacer" />
          <div className="period-tabs" id="periodTabs" hidden={tab !== 'charts'}>
            {PERIODS.map(([v, label]) => (
              <button
                key={v}
                type="button"
                data-period={v}
                className={period === v ? 'active' : ''}
                onClick={() => setPeriod(v)}
              >
                {label}
              </button>
            ))}
          </div>
          <label className="toggle">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />{' '}
            Авто-обновление
          </label>
          <div className={`health-pill ${healthLevel}`} id="healthPill">
            <span className="dot" />
            <span id="healthText">{stats ? healthText : '— загрузка —'}</span>
          </div>
          <div id="userBarHost">
            <UserMenu />
          </div>
        </header>

        <main className="content" id="system-main">
          <div className="content-chrome">
            <section
              className={`chrome-section chrome-alerts${alerts.length ? '' : ' chrome-alerts--empty'}`}
              id="chromeAlerts"
            >
              <div className="chrome-section-head">
                <span className="accent-dot" style={{ background: 'var(--orange)' }} />
                <span>Алёрты</span>
                <span className="chrome-section-meta" id="alertsCount">
                  {alerts.length ? alerts.length : ''}
                </span>
              </div>
              <div className="alerts" id="alertsList">
                {!alerts.length ? (
                  <div className="alert-row info">
                    <span className="empty">Активных алертов нет</span>
                  </div>
                ) : (
                  alerts.map((a, i) => (
                    <div key={`${a.code}-${i}`} className={`alert-row ${a.level || ''}`}>
                      <span className="level">{a.level}</span>
                      {a.code ? <span className="code">{a.code}</span> : null}
                      {a.target ? <span className="target">{a.target}</span> : null}
                      <span>{a.message}</span>
                    </div>
                  ))
                )}
              </div>
            </section>

            <div className="status-strip" id="statusStrip" aria-label="Ключевые метрики">
              <div className="status-metric">
                <span className="sm-label">EPS</span>
                <span className="sm-value" id="statusEps">
                  {fmtNumber(eps)}
                </span>
              </div>
              <div className="status-metric">
                <span className="sm-label">Лаг</span>
                <span className={`sm-value ${toneClass(lagTone(lag))}`} id="statusLag">
                  {fmtLag(ingest.lag_sec)}
                </span>
              </div>
              <div className="status-metric">
                <span className="sm-label">Очередь</span>
                <span className={`sm-value ${toneClass(queueTone(qDepth, qCap))}`} id="statusQueue">
                  {qCap > 0
                    ? `${fmtNumber(qDepth)}/${fmtNumber(qCap)}`
                    : fmtNumber(qDepth)}
                </span>
              </div>
              <div className="status-metric">
                <span className="sm-label">Буфер</span>
                <span className={`sm-value ${toneClass(bufferTone(buffered))}`} id="statusBuffer">
                  {fmtNumber(buffered)}
                </span>
              </div>
              <div className="status-metric">
                <span className="sm-label">Drops</span>
                <span
                  className={`sm-value ${toneClass(dropsTone(dropsPerSec, bufferDropsPerSec))}`}
                  id="statusDrops"
                  title={`admission ${fmtNumber(dropsPerSec)}/s · buffer ${fmtNumber(bufferDropsPerSec)}/s`}
                >
                  {fmtNumber(dropsPerSec + bufferDropsPerSec)}/s
                </span>
              </div>
              <div className="status-metric" id="statusCapacityWrap" hidden={!epsMax}>
                <span className="sm-label">Ёмкость</span>
                <span className={`sm-value ${toneClass(capacityTone(capPct))}`} id="statusCapacity">
                  {capPct}%
                </span>
              </div>
            </div>

            <nav className="view-tabs" id="viewTabs" role="tablist" aria-label="Разделы мониторинга">
              {(
                [
                  ['overview', 'Обзор'],
                  ['pipeline', 'Pipeline'],
                  ['security', 'Безопасность'],
                  ['charts', 'Графики'],
                ] as const
              ).map(([id, label]) => (
                <button
                  key={id}
                  type="button"
                  role="tab"
                  data-tab={id}
                  className={tab === id ? 'active' : ''}
                  aria-selected={tab === id}
                  onClick={() => setTab(id)}
                >
                  {label}
                  {id === 'security' && failed.length ? (
                    <span className="tab-badge" id="securityTabBadge">
                      {failed.length}
                    </span>
                  ) : null}
                </button>
              ))}
            </nav>
          </div>

          <div className="tab-panels">
            {tab === 'overview' ? (
              <div className="tab-panel active" id="tab-overview" role="tabpanel">
                {showInstallProfile ? (
                  <section className="card card-compact" id="installProfileSection">
                    <h3 className="card-title">
                      Профиль установки{' '}
                      <span className="profile-badge" id="profileBadge">
                        {stats?.install_profile?.profile_label ||
                          stats?.install_profile?.profile ||
                          '—'}
                      </span>
                    </h3>
                    <div className="capacity-meter" id="capacityMeter">
                      <div className="meter-label">
                        Нагрузка к ёмкости:{' '}
                        <span id="capacityLabel">
                          {fmtNumber(Math.round(eps))} / {fmtNumber(epsMax)} eps
                        </span>
                      </div>
                      <div className="capacity-track">
                        <div
                          className={`capacity-fill ${capacityTone(capPct)}`}
                          id="capacityFill"
                          style={{ width: `${Math.min(150, Math.min(100, capPct))}%` }}
                        />
                      </div>
                      <div className="capacity-hint" id="capacityHint">
                        {capPct > 125
                          ? 'Нагрузка превышает расчётную ёмкость профиля — рассмотрите upgrade или ./scripts/tune-resources.sh'
                          : 'Нагрузка близка к лимиту профиля'}
                      </div>
                    </div>
                  </section>
                ) : null}

                <section className="row cols-2">
                  <div className="card card-compact">
                    <h3 className="card-title">Health компонентов</h3>
                    <div className="health-grid" id="componentHealthGrid">
                      {([
                        {
                          name: 'Backend',
                          health: backendHealth,
                          meta: `goroutines: ${fmtNumber(stats?.backend_info?.num_goroutine)} · heap: ${
                            stats?.backend_info?.heap_alloc_mb != null
                              ? `${Number(stats.backend_info.heap_alloc_mb).toFixed(1)} MB`
                              : '—'
                          }`,
                        },
                        {
                          name: 'Ingest',
                          health: ingestHealth,
                          meta: `conn: ${fmtNumber(ingest.connections)} · lag: ${fmtLag(ingest.lag_sec)}`,
                        },
                      ] as const).map((item) => {
                        const h = item.health || {};
                        let stateText = String(h.state_text || 'unknown');
                        let css: 'ok' | 'warn' | 'bad' = 'warn';
                        if (h.up != null) {
                          const up = Number(h.up);
                          stateText = up >= 1 ? 'up' : 'down';
                          css = up >= 1 ? 'ok' : 'bad';
                        } else if (h.state != null) {
                          const st = Number(h.state);
                          if (st > 0) css = 'ok';
                          else if (st < 0) css = 'bad';
                          else css = 'warn';
                          if (h.state_text == null) stateText = String(h.state);
                        }
                        return (
                          <div key={item.name} className={`health-card ${css}`}>
                            <div className="health-head">
                              <span className="health-name">{item.name}</span>
                              <span className="health-state">{stateText}</span>
                            </div>
                            <div className="health-meta">{item.meta}</div>
                          </div>
                        );
                      })}
                    </div>
                  </div>

                  <div className="card card-compact">
                    <h3 className="card-title">Контейнеры</h3>
                    <div className="container-strip" id="containersRow">
                      {CONTAINERS.map((name) => {
                        const c = stats?.containers?.[name];
                        const up = num(c?.mem_bytes) > 0;
                        return (
                          <div key={name} className={`container-chip${up ? ' up' : ''}`}>
                            <span className="dot" />
                            <span className="name">{name}</span>
                            <span className="metrics">
                              CPU {c?.cpu_pct != null ? `${Number(c.cpu_pct).toFixed(1)}%` : '—'} · Mem{' '}
                              {c?.mem_bytes != null ? fmtBytes(c.mem_bytes) : '—'}
                            </span>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                </section>

                <section className="card card-compact edges-card" id="edgesAggSection">
                  <h3 className="card-title">
                    Агрегаты рёбер{' '}
                    <span className="profile-badge" id="edgesAggBadge">
                      {edgesBadge}
                    </span>
                  </h3>
                  <div className="kv-grid cols-3" id="edgesAggPrimary">
                    <div className="kv-row">
                      <span className="k">Raw / agg</span>
                      <span className="v">
                        {fmtNumber(edges?.raw_rows)} / {fmtNumber(edges?.agg_rows)}
                      </span>
                    </div>
                    <div className="kv-row">
                      <span className="k">Карта</span>
                      <span className="v">{edges?.map_source || '—'}</span>
                    </div>
                    <div className="kv-row">
                      <span className="k">Backfill</span>
                      <span className="v">
                        {edges?.days_total
                          ? `${fmtNumber(edges.days_done)} / ${fmtNumber(edges.days_total)}`
                          : '—'}
                      </span>
                    </div>
                  </div>
                  <div className="capacity-hint" id="edgesAggHint">
                    {edgesHint}
                  </div>
                  <details
                    className="details-toggle"
                    id="edgesAggDetails"
                    open={edges?.state === 'running' || edges?.state === 'error'}
                  >
                    <summary>+ Подробности</summary>
                    <div className="kv-grid cols-2" id="edgesAggSecondary">
                      <div className="kv-row">
                        <span className="k">Сообщение</span>
                        <span className="v">{edges?.message || '—'}</span>
                      </div>
                      <div className="kv-row">
                        <span className="k">prefer_agg</span>
                        <span className="v">{edges?.prefer_agg ? 'да' : 'нет'}</span>
                      </div>
                      <div className="kv-row">
                        <span className="k">geo prefer_agg</span>
                        <span className="v">{edges?.geo_prefer_agg ? 'да' : 'нет'}</span>
                      </div>
                      <div className="kv-row">
                        <span className="k">Обновлено</span>
                        <span className="v">{fmtDate(edges?.updated_at)}</span>
                      </div>
                      <div className="kv-row">
                        <span className="k">Старт</span>
                        <span className="v">{fmtDate(edges?.started_at)}</span>
                      </div>
                    </div>
                  </details>
                </section>

                <div className="footer-info" id="footerInfoOverview">
                  обновлено: {updatedAt ? updatedAt.toLocaleString('ru-RU') : '—'}
                  {stats?.backend_info
                    ? ` · backend heap: ${Number(stats.backend_info.heap_alloc_mb || 0).toFixed(1)} MB · goroutines: ${fmtNumber(stats.backend_info.num_goroutine)}`
                    : ''}
                </div>
              </div>
            ) : null}

            {tab === 'pipeline' ? (
              <div className="tab-panel active" id="tab-pipeline" role="tabpanel">
                <section className="card card-compact">
                  <h3 className="card-title">Pipeline</h3>
                  <div className="pipeline" id="pipelineRow">
                    <div className="pipeline-stage ok">
                      <div className="stage-name">Syslog-NG</div>
                      <div className="stage-value">{fmtNumber(eps)} eps</div>
                      <div className="stage-meta">
                        udp: {fmtNumber(rate.udp_events_per_sec)}/s · tcp:{' '}
                        {fmtNumber(rate.tcp_events_per_sec)}/s
                      </div>
                    </div>
                    <div className="pipeline-arrow">→</div>
                    <div className={`pipeline-stage ${ingestStageStatus}`}>
                      <div className="stage-name">Backend Ingest</div>
                      <div className="stage-value">
                        {fmtNumber(rate.events_per_sec || 0)} eps
                      </div>
                      <div className="stage-meta">
                        conn: {fmtNumber(ingest.connections)}, buf: {fmtNumber(buffered)}, q:{' '}
                        {fmtNumber(qDepth)}/{fmtNumber(qCap)}
                        {qCap > 0 ? ` (${fmtPercent(queuePct)})` : ''}
                        {num(ingest.dropped_total) > 0
                          ? `, drop: ${fmtNumber(ingest.dropped_total)} (${fmtNumber(dropsPerSec)}/s)`
                          : ''}
                        {num(ingest.buffer_drops_total) > 0
                          ? `, buffer_drop: ${fmtNumber(ingest.buffer_drops_total)} (${fmtNumber(bufferDropsPerSec)}/s)`
                          : ''}
                        {num(ingest.circuit_open) >= 1 ? ', circuit: open' : ''}
                      </div>
                    </div>
                    <div className="pipeline-arrow">→</div>
                    <div className="pipeline-stage ok">
                      <div className="stage-name">ClickHouse</div>
                      <div className="stage-value">
                        {fmtNumber(storage.traffic_logs?.row_count)}
                      </div>
                      <div className="stage-meta">строк в БД</div>
                    </div>
                  </div>
                </section>

                <section className="row cols-2">
                  <div className="card card-compact">
                    <h3 className="card-title">Ingest</h3>
                    <div className="kv-grid cols-2" id="ingestList">
                      {(
                        [
                          ['Лаг', fmtLag(ingest.lag_sec)],
                          [
                            'Queue',
                            `${fmtNumber(qDepth)} / ${fmtNumber(qCap)}${
                              qCap > 0 ? ` (${fmtPercent(queuePct)})` : ''
                            }`,
                          ],
                          [
                            'Queue bytes',
                            qBytesCap > 0
                              ? `${fmtBytes(qBytes)} / ${fmtBytes(qBytesCap)}`
                              : fmtBytes(qBytes),
                          ],
                          ['Buffered', fmtNumber(buffered)],
                          [
                            'Dropped',
                            `${fmtNumber(ingest.dropped_total)} (${fmtNumber(dropsPerSec)}/s)`,
                          ],
                          [
                            'Buffer drops',
                            `${fmtNumber(ingest.buffer_drops_total)} (${fmtNumber(bufferDropsPerSec)}/s)`,
                          ],
                          ['Last drop', fmtDropAt(lastDropAt)],
                          ['Received', fmtNumber(ingest.received_total)],
                          ['Inserted', fmtNumber(ingest.inserted_total)],
                          ['Skipped', fmtNumber(ingest.skipped_total)],
                          ['Parse err.', fmtNumber(ingest.parse_errors_total)],
                          ['Connections', fmtNumber(ingest.connections)],
                          ['Parse (1h)', fmtNumber(parseErr1h)],
                          ['Circuit', num(ingest.circuit_open) >= 1 ? 'open' : 'closed'],
                          ['Uptime', fmtUptime(uptimeSec)],
                        ] as const
                      ).map(([k, v]) => (
                        <div className="kv-row" key={k}>
                          <span className="k">{k}</span>
                          <span className="v">{v}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="card card-compact">
                    <h3 className="card-title">Хранилище</h3>
                    <div className="kv-grid cols-2" id="storageList">
                      <div className="kv-row">
                        <span className="k">traffic_logs</span>
                        <span className="v">
                          {fmtNumber(storage.traffic_logs?.row_count)} /{' '}
                          {fmtBytes(storage.traffic_logs?.bytes_on_disk)}
                        </span>
                      </div>
                      <div className="kv-row">
                        <span className="k">active parts</span>
                        <span className="v">{fmtNumber(storage.clickhouse?.active_parts)}</span>
                      </div>
                      <div className="kv-row">
                        <span className="k">geo_ranges</span>
                        <span className="v">
                          {fmtNumber(storage.geo_ranges?.row_count)} диапазонов
                        </span>
                      </div>
                    </div>
                  </div>
                </section>

                <section className="card card-compact">
                  <h3 className="card-title">Срок хранения</h3>
                  <form id="retentionForm" className="form-row" onSubmit={saveRetention}>
                    {(
                      [
                        ['traffic_logs_days', 'Логи трафика (дни)', 'retTrafficLogs'],
                        ['edges_days', 'Агрегаты рёбер (дни)', 'retEdges'],
                        ['parse_errors_days', 'Ошибки парсинга (дни)', 'retParseErrors'],
                        ['system_metrics_days', 'Метрики системы (дни)', 'retSystemMetrics'],
                      ] as const
                    ).map(([key, label, id]) => (
                      <div className="field" key={key}>
                        <label htmlFor={id}>{label}</label>
                        <input
                          id={id}
                          name={key}
                          type="number"
                          min={1}
                          max={730}
                          value={Number(retention[key] ?? 1)}
                          onChange={(e) =>
                            setRetention({ ...retention, [key]: Number(e.target.value) })
                          }
                        />
                        <span className="hint">сейчас: {Number(retention[key] ?? 0)} дн.</span>
                      </div>
                    ))}
                    <button type="submit" className="btn primary" id="retentionSaveBtn">
                      Сохранить
                    </button>
                  </form>
                  <p className="hint" id="retentionUpdatedAt">
                    Уменьшение TTL удалит старые партиции при следующем merge/drop в ClickHouse.
                    {retention.updated_at ? ` Обновлено: ${fmtDate(retention.updated_at)}` : ''}
                  </p>
                </section>
              </div>
            ) : null}

            {tab === 'security' ? (
              <div className="tab-panel active" id="tab-security" role="tabpanel">
                <section className="card" id="failedLoginsSection">
                  <details className="section-details" id="failedLoginsDetails" open>
                    <summary className="card-title" style={{ color: 'var(--red)' }}>
                      ■ Неуспешные попытки входа{' '}
                      <span id="failedLoginsCount">({failed.length ? failed.length : 'нет'})</span>
                    </summary>
                    <div id="failedLoginsHost">
                      {!failed.length ? (
                        <p className="auth-fails-empty empty">Нет неуспешных попыток</p>
                      ) : (
                        <div className="table-wrap">
                          <table className="auth-fails-table">
                            <thead>
                              <tr>
                                <th scope="col">Логин</th>
                                <th scope="col">IP</th>
                                <th scope="col">Count</th>
                                <th scope="col">Первая</th>
                                <th scope="col">Последняя</th>
                                <th scope="col">Блок</th>
                              </tr>
                            </thead>
                            <tbody>
                              {failed.map((f, i) => (
                                <tr key={i}>
                                  <td>{f.username}</td>
                                  <td>{f.ip}</td>
                                  <td>{fmtNumber(f.count)}</td>
                                  <td>{fmtDate(f.first_at)}</td>
                                  <td>{fmtDate(f.last_at)}</td>
                                  <td>
                                    {f.locked ? (
                                      <span className="badge-locked">locked</span>
                                    ) : (
                                      '—'
                                    )}
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </div>
                  </details>
                </section>
              </div>
            ) : null}

            {tab === 'charts' ? (
              <div className="tab-panel active" id="tab-charts" role="tabpanel">
                <section className="row cols-2">
                  <div className="card chart-card">
                    <div className="chart-host" ref={chartEvents} style={{ height: 240 }} />
                  </div>
                  <div className="card chart-card">
                    <div className="chart-host" ref={chartLag} style={{ height: 240 }} />
                  </div>
                </section>
                <section className="row cols-2">
                  <div className="card chart-card">
                    <div className="chart-host" ref={chartCpu} style={{ height: 280 }} />
                  </div>
                  <div className="card chart-card">
                    <div className="chart-host" ref={chartMem} style={{ height: 280 }} />
                  </div>
                </section>
                <section className="row cols-2">
                  <div className="card chart-card">
                    <div className="chart-host" ref={chartBuffer} style={{ height: 200 }} />
                  </div>
                  <div className="card chart-card">
                    <div className="chart-host" ref={chartStorage} style={{ height: 200 }} />
                  </div>
                </section>
              </div>
            ) : null}
          </div>
        </main>
      </div>
    </div>
  );
}
