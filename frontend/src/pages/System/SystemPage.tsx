import { FormEvent, useCallback, useEffect, useMemo, useRef, useState, type RefObject } from 'react';
import uPlot from 'uplot';
import {
  fetchRetention,
  fetchSystemHistory,
  fetchSystemStats,
  putRetention,
} from '@/api/system';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';
import { usePolling } from '@/lib/usePolling';
import 'uplot/dist/uPlot.min.css';
import '@/styles/system.css';
import { alignSeries, makeChart, type ChartFormatOpts } from './systemCharts';
import {
  bufferTone,
  capacityTone,
  dropsTone,
  edgesAggHint,
  lagTone,
  num,
  pipelineIngestStatus,
  pipelineSyslogStatus,
  queueTone,
  toneClass,
  fmtLag,
} from './systemFormat';
import {
  CONTAINERS,
  PERIODS,
  type HistoryPayload,
  type Retention,
  type SystemStats,
  type Tab,
} from './systemTypes';
import { SystemBackupTab } from './SystemBackupTab';
import { SystemChartsTab } from './SystemChartsTab';
import { SystemOverviewTab } from './SystemOverviewTab';
import { SystemPipelineTab } from './SystemPipelineTab';
import { SystemSecurityTab } from './SystemSecurityTab';

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
  const historyFetching = useRef(false);

  const loadStats = useCallback(async () => {
    try {
      const data = await fetchSystemStats();
      setStats(data);
      setUpdatedAt(new Date());
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка stats', 'error');
    }
  }, [toast]);

  const loadRetention = useCallback(async () => {
    try {
      const data = await fetchRetention();
      setRetention(data.retention || data);
    } catch {
      /* optional */
    }
  }, []);

  useEffect(() => {
    document.title = 'ГеоАтлас — Мониторинг';
  }, []);

  useEffect(() => {
    void loadRetention();
  }, [loadRetention]);

  useEffect(() => {
    if (!autoRefresh) void loadStats();
  }, [autoRefresh, loadStats]);

  usePolling(
    async () => {
      await loadStats();
    },
    5000,
    autoRefresh,
  );

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
          const host = ref.current;
          host.replaceChildren();
          const titleEl = document.createElement('div');
          titleEl.className = 'chart-title';
          titleEl.textContent = title;
          const empty = document.createElement('div');
          empty.className = 'empty';
          empty.style.padding = '24px';
          empty.textContent = 'Нет данных';
          host.append(titleEl, empty);
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
        'Буферы',
        ['Ingest buffered', 'syslog-ng queued'],
        ['pipeline.ingest.buffered_lines', 'pipeline.syslogng.queued'],
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

  const loadHistory = useCallback(async () => {
    if (historyFetching.current) return;
    historyFetching.current = true;
    try {
      const data = await fetchSystemHistory(period);
      paintHistory(data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка history', 'error');
    } finally {
      historyFetching.current = false;
    }
  }, [period, paintHistory, toast]);

  useEffect(() => {
    if (tab !== 'charts') {
      plotsRef.current.forEach((p) => p.destroy());
      plotsRef.current = [];
      return;
    }
    void loadHistory();
    return () => {
      plotsRef.current.forEach((p) => p.destroy());
      plotsRef.current = [];
    };
  }, [tab, period, themeTick, loadHistory]);

  usePolling(
    async () => {
      await loadHistory();
    },
    30000,
    autoRefresh && tab === 'charts',
    { runImmediately: false },
  );

  async function saveRetention(e: FormEvent) {
    e.preventDefault();
    try {
      const data = await putRetention({
        traffic_logs_days: retention.traffic_logs_days,
        edges_days: retention.edges_days,
        parse_errors_days: retention.parse_errors_days,
        system_metrics_days: retention.system_metrics_days,
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
  const alerts = useMemo(() => stats?.alerts || [], [stats?.alerts]);
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
  const syslogng = pipeline.syslogng || {};
  const syslogngDropsPerSec = num(syslogng.drops_per_sec);
  const syslogngFifo = num(
    (stats?.install_profile?.limits?.syslog_ng as { fifo_size?: number } | undefined)?.fifo_size,
  );
  const syslogngUpRaw = stats?.health?.syslogng?.up;
  const syslogngUp = syslogngUpRaw == null ? undefined : num(syslogngUpRaw);
  const syslogStageStatus = pipelineSyslogStatus(
    syslogng as Record<string, number>,
    syslogngUp,
    syslogngFifo,
    syslogngDropsPerSec,
  );
  const syslogEps = num(syslogng.events_per_sec) || eps;
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
    <AdminLayout
      className="nm-system"
      title="Мониторинг системы"
      subtitle="pipeline / containers / storage"
      mainClassName="content"
      showSystemHealth={false}
      actions={
        <>
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
        </>
      }
    >
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
                  className={`sm-value ${toneClass(dropsTone(dropsPerSec + syslogngDropsPerSec, bufferDropsPerSec))}`}
                  id="statusDrops"
                  title={`admission ${fmtNumber(dropsPerSec)}/s · buffer ${fmtNumber(bufferDropsPerSec)}/s · syslog-ng ${fmtNumber(syslogngDropsPerSec)}/s`}
                >
                  {fmtNumber(dropsPerSec + bufferDropsPerSec + syslogngDropsPerSec)}/s
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
                  ['backup', 'Резервное копирование'],
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
              <SystemOverviewTab
                showInstallProfile={showInstallProfile}
                stats={stats}
                eps={eps}
                epsMax={epsMax}
                capPct={capPct}
                backendHealth={backendHealth}
                ingestHealth={ingestHealth}
                ingest={ingest as Record<string, number>}
                edges={edges}
                edgesBadge={edgesBadge}
                edgesHint={edgesHint}
                updatedAt={updatedAt}
              />
            ) : null}

            {tab === 'pipeline' ? (
              <SystemPipelineTab
                syslogEps={syslogEps}
                syslogStageStatus={syslogStageStatus}
                syslogng={syslogng as Record<string, number>}
                syslogngDropsPerSec={syslogngDropsPerSec}
                syslogngFifo={syslogngFifo}
                rate={rate as Record<string, number>}
                ingest={ingest as Record<string, number>}
                ingestStageStatus={ingestStageStatus}
                buffered={buffered}
                qDepth={qDepth}
                qCap={qCap}
                queuePct={queuePct}
                dropsPerSec={dropsPerSec}
                bufferDropsPerSec={bufferDropsPerSec}
                storage={storage}
                qBytes={qBytes}
                qBytesCap={qBytesCap}
                lastDropAt={lastDropAt}
                parseErr1h={parseErr1h}
                uptimeSec={uptimeSec}
                retention={retention}
                setRetention={setRetention}
                saveRetention={saveRetention}
              />
            ) : null}

            {tab === 'backup' ? <SystemBackupTab /> : null}

            {tab === 'security' ? <SystemSecurityTab failed={failed} /> : null}

            {tab === 'charts' ? (
              <SystemChartsTab
                chartEvents={chartEvents}
                chartLag={chartLag}
                chartCpu={chartCpu}
                chartMem={chartMem}
                chartBuffer={chartBuffer}
                chartStorage={chartStorage}
              />
            ) : null}
          </div>
    </AdminLayout>
  );
}
