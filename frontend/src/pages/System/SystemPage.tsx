import { FormEvent, useCallback, useEffect, useRef, useState } from 'react';
import uPlot from 'uplot';
import { apiFetch } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import { getTheme } from '@/auth/theme';
import 'uplot/dist/uPlot.min.css';
import '@/styles/system.css';

type Tab = 'overview' | 'pipeline' | 'security' | 'charts';

interface SystemStats {
  alerts?: { id: string; severity?: string; message?: string }[];
  ingest?: Record<string, number | string | boolean>;
  containers?: { name: string; cpu?: number; mem?: number; status?: string }[];
  capacity?: { level?: string; message?: string };
  auth_fails?: { username?: string; ip?: string; at?: string; count?: number }[];
}

interface Retention {
  traffic_logs_days?: number;
  edges_days?: number;
  parse_errors_days?: number;
  system_metrics_days?: number;
  updated_at?: string;
}

function chartAxisStroke(): string {
  return getTheme() === 'light' ? '#334155' : '#94a3b8';
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
  const [period, setPeriod] = useState('24h');
  const plotHost = useRef<HTMLDivElement>(null);
  const plotRef = useRef<uPlot | null>(null);

  const loadStats = useCallback(async () => {
    try {
      const data = await apiFetch<SystemStats>('/api/system/stats');
      setStats(data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка stats', 'error');
    }
  }, [toast]);

  const loadRetention = useCallback(async () => {
    try {
      const data = await apiFetch<{ retention?: Retention }>('/api/system/retention');
      setRetention(data.retention || data);
    } catch (e) {
      console.warn('retention load failed', e);
    }
  }, []);

  useEffect(() => {
    document.title = 'ГеоАтлас — Мониторинг';
    void loadStats();
    void loadRetention();
    const id = window.setInterval(() => void loadStats(), 10000);
    return () => window.clearInterval(id);
  }, [loadStats, loadRetention]);

  useEffect(() => {
    try {
      localStorage.setItem('nm.system.tab', tab);
    } catch {
      /* ignore */
    }
  }, [tab]);

  useEffect(() => {
    if (tab !== 'charts' || !plotHost.current) return;
    let cancelled = false;
    (async () => {
      try {
        const data = await apiFetch<{
          series?: { t: string; cpu?: number; mem?: number }[];
        }>(`/api/system/history?period=${encodeURIComponent(period)}`);
        if (cancelled || !plotHost.current) return;
        const series = data.series || [];
        const xs = series.map((s) => Math.floor(new Date(s.t).getTime() / 1000));
        const cpu = series.map((s) => Number(s.cpu) || 0);
        const mem = series.map((s) => Number(s.mem) || 0);
        plotRef.current?.destroy();
        plotRef.current = new uPlot(
          {
            width: plotHost.current.clientWidth || 640,
            height: 280,
            series: [
              {},
              { label: 'CPU %', stroke: '#38bdf8' },
              { label: 'Mem %', stroke: '#a78bfa' },
            ],
            axes: [
              { stroke: chartAxisStroke() },
              { stroke: chartAxisStroke() },
            ],
          },
          [xs, cpu, mem],
          plotHost.current,
        );
      } catch (e) {
        toast(e instanceof Error ? e.message : 'Ошибка history', 'error');
      }
    })();

    const onTheme = () => {
      if (plotRef.current) {
        plotRef.current.redraw();
      }
    };
    document.addEventListener('nm-theme-change', onTheme);
    return () => {
      cancelled = true;
      document.removeEventListener('nm-theme-change', onTheme);
      plotRef.current?.destroy();
      plotRef.current = null;
    };
  }, [tab, period, toast]);

  async function saveRetention(e: FormEvent) {
    e.preventDefault();
    try {
      const data = await apiFetch<{ retention?: Retention }>('/api/system/retention', {
        method: 'PUT',
        body: JSON.stringify(retention),
      });
      setRetention(data.retention || retention);
      toast('TTL сохранён', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  const ingest = stats?.ingest || {};

  return (
    <AdminLayout title="Мониторинг системы">
      <div className="page-content-inner">
        <div className="view-tabs">
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
              data-tab={id}
              className={tab === id ? 'active' : ''}
              onClick={() => setTab(id)}
            >
              {label}
            </button>
          ))}
        </div>

        {tab === 'overview' ? (
          <div className="tab-panel" data-tab="overview">
            <div className="card">
              <h2>Алёрты</h2>
              {!stats?.alerts?.length ? (
                <p className="empty">Нет активных алёртов</p>
              ) : (
                <ul>
                  {stats.alerts.map((a) => (
                    <li key={a.id}>
                      <span className={`badge ${a.severity || ''}`}>{a.severity}</span> {a.message}
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div className="card" style={{ marginTop: 12 }}>
              <h2>Контейнеры</h2>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th scope="col">Имя</th>
                      <th scope="col">CPU</th>
                      <th scope="col">RAM</th>
                      <th scope="col">Статус</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(stats?.containers || []).map((c) => (
                      <tr key={c.name}>
                        <td>{c.name}</td>
                        <td>{fmtNumber(c.cpu)}</td>
                        <td>{fmtNumber(c.mem)}</td>
                        <td>{c.status}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {stats?.capacity?.message ? (
                <p className="hint">Ёмкость: {stats.capacity.message}</p>
              ) : null}
            </div>
          </div>
        ) : null}

        {tab === 'pipeline' ? (
          <div className="tab-panel" data-tab="pipeline">
            <div className="card">
              <h2>Ingest</h2>
              <pre style={{ whiteSpace: 'pre-wrap' }}>{JSON.stringify(ingest, null, 2)}</pre>
            </div>
            <div className="card" style={{ marginTop: 12 }}>
              <h2>Срок хранения (TTL)</h2>
              <form id="retentionForm" className="form-row" onSubmit={saveRetention}>
                {(
                  [
                    ['traffic_logs_days', 'traffic_logs'],
                    ['edges_days', 'edges'],
                    ['parse_errors_days', 'parse_errors'],
                    ['system_metrics_days', 'system_metrics'],
                  ] as const
                ).map(([key, label]) => (
                  <div className="field" key={key}>
                    <label htmlFor={key}>{label} (дней)</label>
                    <input
                      id={key}
                      type="number"
                      min={1}
                      max={730}
                      value={Number(retention[key] ?? 1)}
                      onChange={(e) =>
                        setRetention({ ...retention, [key]: Number(e.target.value) })
                      }
                    />
                  </div>
                ))}
                <button type="submit" className="btn primary" id="retentionSaveBtn">
                  Сохранить
                </button>
              </form>
              <p className="hint" id="retentionUpdatedAt">
                Обновлено: {fmtDate(retention.updated_at)}
              </p>
            </div>
            <div className="card" style={{ marginTop: 12 }}>
              <button
                type="button"
                className="btn"
                onClick={async () => {
                  try {
                    await apiFetch('/api/system/maintenance/backfill', { method: 'POST' });
                    toast('Backfill запущен', 'success');
                  } catch (e) {
                    toast(e instanceof Error ? e.message : 'Ошибка', 'error');
                  }
                }}
              >
                Maintenance backfill
              </button>
            </div>
          </div>
        ) : null}

        {tab === 'security' ? (
          <div className="tab-panel" data-tab="security">
            <div className="card">
              <h2>Неуспешные логины</h2>
              <div className="table-wrap">
                <table className="auth-fails-table">
                  <thead>
                    <tr>
                      <th scope="col">Время</th>
                      <th scope="col">Логин</th>
                      <th scope="col">IP</th>
                      <th scope="col">Count</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(stats?.auth_fails || []).map((f, i) => (
                      <tr key={i}>
                        <td>{fmtDate(f.at)}</td>
                        <td>{f.username}</td>
                        <td>{f.ip}</td>
                        <td>{fmtNumber(f.count)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        ) : null}

        {tab === 'charts' ? (
          <div className="tab-panel" data-tab="charts">
            <div className="period-tabs" style={{ display: 'flex', gap: 6, marginBottom: 12 }}>
              {['1h', '6h', '24h', '7d'].map((p) => (
                <button
                  key={p}
                  type="button"
                  className={period === p ? 'active' : ''}
                  onClick={() => setPeriod(p)}
                >
                  {p}
                </button>
              ))}
            </div>
            <div className="card">
              <div ref={plotHost} />
            </div>
          </div>
        ) : null}
      </div>
    </AdminLayout>
  );
}
