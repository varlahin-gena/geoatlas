import { FormEvent, useEffect, useMemo, useState } from 'react';
import { putRetention } from '@/api/system';
import { AdminLayout } from '@/components/AdminLayout';
import { ObserveSectionNav } from '@/components/ObserveSectionNav';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';
import 'uplot/dist/uPlot.min.css';
import '@/styles/system.css';
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
import { PERIODS, type Tab } from './systemTypes';
import { SystemBackupTab } from './SystemBackupTab';
import { SystemAuditTab } from './SystemAuditTab';
import { SystemChartsTab } from './SystemChartsTab';
import { SystemOverviewTab } from './SystemOverviewTab';
import { SystemPipelineTab } from './SystemPipelineTab';
import { SystemSecurityTab } from './SystemSecurityTab';
import { useSystemCharts } from './useSystemCharts';
import { useSystemStatsPolling } from './useSystemStatsPolling';

export default function SystemPage() {
  const { toast } = useToast();
  const [tab, setTab] = useState<Tab>(() => {
    try {
      return (localStorage.getItem('nm.system.tab') as Tab) || 'overview';
    } catch {
      return 'overview';
    }
  });
  const [period, setPeriod] = useState('1h');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [themeTick, setThemeTick] = useState(0);

  const { stats, retention, setRetention, updatedAt } = useSystemStatsPolling(toast, autoRefresh);
  const charts = useSystemCharts(toast, { tab, period, autoRefresh, themeTick });

  useEffect(() => {
    document.title = 'ГеоАтлас — Мониторинг системы';
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem('nm.system.tab', tab);
    } catch {
      /* ignore */
    }
  }, [tab]);

  useEffect(() => {
    const onTheme = () => setThemeTick((n) => n + 1);
    document.addEventListener('ga-theme-change', onTheme);
    return () => document.removeEventListener('ga-theme-change', onTheme);
  }, []);

  async function saveRetention(e: FormEvent) {
    e.preventDefault();
    try {
      const data = await putRetention({
        traffic_logs_days: retention.traffic_logs_days,
        edges_days: retention.edges_days,
        parse_errors_days: retention.parse_errors_days,
        system_metrics_days: retention.system_metrics_days,
      });
      if (data.retention) setRetention(data.retention);
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
  const ingestStageStatus = pipelineIngestStatus(rate, ingest, queuePct);
  const syslogng = pipeline.syslogng || {};
  const syslogngDropsPerSec = num(syslogng.drops_per_sec);
  const syslogngFifo = num(
    (stats?.install_profile?.limits?.syslog_ng as { fifo_size?: number } | undefined)?.fifo_size,
  );
  const syslogngUpRaw = stats?.health?.syslogng?.up;
  const syslogngUp = syslogngUpRaw == null ? undefined : num(syslogngUpRaw as number);
  const syslogStageStatus = pipelineSyslogStatus(syslogng, syslogngUp, syslogngFifo, syslogngDropsPerSec);
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
      className="ga-system"
      title="Мониторинг системы"
      subtitle="pipeline / containers / storage"
      mainClassName="content"
      showSystemHealth={false}
      actions={
        <>
          <div className="period-tabs" hidden={tab !== 'charts'}>
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
          <div className={`health-pill ${healthLevel}`}>
            <span className="dot" />
            <span>{stats ? healthText : '— загрузка —'}</span>
          </div>
        </>
      }
    >
      <ObserveSectionNav />
      <div className="content-chrome">
        <section
          className={`chrome-section chrome-alerts${alerts.length ? '' : ' chrome-alerts--empty'}`}
        >
          <div className="chrome-section-head">
            <span className="accent-dot" style={{ background: 'var(--orange)' }} />
            <span>Алёрты</span>
            <span className="chrome-section-meta">{alerts.length ? alerts.length : ''}</span>
          </div>
          <div className="alerts">
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

        <div className="status-strip" aria-label="Ключевые метрики">
          <div className="status-metric">
            <span className="sm-label">EPS</span>
            <span className="sm-value">{fmtNumber(eps)}</span>
          </div>
          <div className="status-metric">
            <span className="sm-label">Лаг</span>
            <span className={`sm-value ${toneClass(lagTone(lag))}`}>{fmtLag(ingest.lag_sec)}</span>
          </div>
          <div className="status-metric">
            <span className="sm-label">Очередь</span>
            <span className={`sm-value ${toneClass(queueTone(qDepth, qCap))}`}>
              {qCap > 0 ? `${fmtNumber(qDepth)}/${fmtNumber(qCap)}` : fmtNumber(qDepth)}
            </span>
          </div>
          <div className="status-metric">
            <span className="sm-label">Буфер</span>
            <span className={`sm-value ${toneClass(bufferTone(buffered))}`}>{fmtNumber(buffered)}</span>
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
          <div className="status-metric" hidden={!epsMax}>
            <span className="sm-label">Ёмкость</span>
            <span className={`sm-value ${toneClass(capacityTone(capPct))}`}>{capPct}%</span>
          </div>
        </div>

        <nav className="view-tabs" role="tablist" aria-label="Разделы мониторинга">
          {(
            [
              ['overview', 'Обзор'],
              ['pipeline', 'Конвейер'],
              ['backup', 'Резервное копирование'],
              ['security', 'Безопасность'],
              ['audit', 'Журнал аудита'],
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
                <span className="tab-badge">{failed.length}</span>
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
            ingest={ingest}
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
            syslogng={syslogng}
            syslogngDropsPerSec={syslogngDropsPerSec}
            syslogngFifo={syslogngFifo}
            rate={rate}
            ingest={ingest}
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

        {tab === 'audit' ? <SystemAuditTab /> : null}

        {tab === 'charts' ? (
          <SystemChartsTab
            chartEvents={charts.chartEvents}
            chartLag={charts.chartLag}
            chartCpu={charts.chartCpu}
            chartMem={charts.chartMem}
            chartBuffer={charts.chartBuffer}
            chartStorage={charts.chartStorage}
          />
        ) : null}
      </div>
    </AdminLayout>
  );
}
