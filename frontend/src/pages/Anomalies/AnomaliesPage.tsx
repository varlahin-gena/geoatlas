import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  ackAnomaly,
  fetchAnomalies,
  type AnomalyEvent,
  type AnomalySummary,
} from '@/api/anomalies';
import { AdminLayout } from '@/components/AdminLayout';
import { ObserveSectionNav } from '@/components/ObserveSectionNav';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import {
  ANOMALY_CODE_OPTIONS,
  ANOMALY_SEVERITY_OPTIONS,
  ANOMALY_SINCE_OPTIONS,
  anomalyMapHref,
  eventCodeLabel,
  matchesAnomalySearch,
  relTime,
  severityLabel,
  sinceIsoHoursAgo,
} from './anomalyDisplay';
import './anomalies.css';

export default function AnomaliesPage() {
  const { toast } = useToast();
  const [rows, setRows] = useState<AnomalyEvent[]>([]);
  const [summary, setSummary] = useState<AnomalySummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [severity, setSeverity] = useState('');
  const [code, setCode] = useState('');
  const [sinceHours, setSinceHours] = useState('24');
  const [includeAcked, setIncludeAcked] = useState(false);
  const [limit, setLimit] = useState('100');
  const [search, setSearch] = useState('');
  const [acking, setAcking] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const hours = Number(sinceHours) || 24;
      const data = await fetchAnomalies({
        since: sinceIsoHoursAgo(hours),
        severity: severity || undefined,
        code: code || undefined,
        include_acked: includeAcked ? '1' : undefined,
        limit: Number(limit) || 100,
      });
      setRows(data.items || []);
      setSummary(data.summary || null);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить аномалии', 'error');
    } finally {
      setLoading(false);
    }
  }, [code, includeAcked, limit, sinceHours, severity, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Аномалии';
  }, []);

  useEffect(() => {
    const t = window.setTimeout(() => void load(), 300);
    return () => window.clearTimeout(t);
  }, [load]);

  const filtered = useMemo(
    () => rows.filter((row) => matchesAnomalySearch(row, search)),
    [rows, search],
  );

  async function hideAlert(item: AnomalyEvent) {
    const fp = item.fingerprint;
    if (!fp || acking) return;
    setAcking(fp);
    try {
      await ackAnomaly(fp);
      if (!includeAcked) {
        setRows((prev) => prev.filter((r) => r.fingerprint !== fp));
      } else {
        setRows((prev) =>
          prev.map((r) => (r.fingerprint === fp ? { ...r, acknowledged: true } : r)),
        );
      }
      setSummary((prev) => {
        if (!prev || includeAcked) return prev;
        const high =
          item.severity === 'high' ? Math.max(0, (prev.high || 0) - 1) : prev.high || 0;
        const warn =
          item.severity === 'warn' ? Math.max(0, (prev.warn || 0) - 1) : prev.warn || 0;
        return { ...prev, high, warn, total: Math.max(0, (prev.total || 0) - 1) };
      });
      toast('Алерт закрыт', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось закрыть алерт', 'error');
    } finally {
      setAcking(null);
    }
  }

  const disabled = summary?.enabled === false;

  return (
    <AdminLayout title="Аномалии">
      <div className="page-content-inner wide">
        <ObserveSectionNav />

        <p className="page-lead">
          Алерты, которые сканер обнаружил в трафике за выбранный период. Закрытие скрывает алерт и
          подавляет повтор на 24 часа. «На карте» открывает контекст события с фильтрами.
        </p>

        {disabled ? (
          <p className="hint warn-banner">Модуль аномалий отключён на сервере (ANOMALY_ENABLED).</p>
        ) : null}
        {summary?.learning ? (
          <p className="hint warn-banner">
            Базовая линия в режиме обучения — часть простых алертов может не появляться первые дни.
          </p>
        ) : null}

        <div className="anomaly-summary-row">
          <div className="anomaly-summary-card">
            <span className="hint">Критично</span>
            <strong className="sev-high">{fmtNumber(summary?.high || 0)}</strong>
          </div>
          <div className="anomaly-summary-card">
            <span className="hint">Предупреждения</span>
            <strong className="sev-warn">{fmtNumber(summary?.warn || 0)}</strong>
          </div>
          <div className="anomaly-summary-card">
            <span className="hint">Всего открытых</span>
            <strong>{fmtNumber(summary?.total || 0)}</strong>
          </div>
          <div className="anomaly-summary-card">
            <span className="hint">Показано в таблице</span>
            <strong>{fmtNumber(filtered.length)}</strong>
          </div>
        </div>

        <div className="toolbar anomaly-toolbar">
          <input
            placeholder="Поиск по заголовку, IP, стране…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <select value={severity} onChange={(e) => setSeverity(e.target.value)} aria-label="Важность">
            {ANOMALY_SEVERITY_OPTIONS.map((opt) => (
              <option key={opt.value || 'all'} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          <select value={code} onChange={(e) => setCode(e.target.value)} aria-label="Тип">
            {ANOMALY_CODE_OPTIONS.map((opt) => (
              <option key={opt.value || 'all'} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          <select value={sinceHours} onChange={(e) => setSinceHours(e.target.value)} aria-label="Период">
            {ANOMALY_SINCE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          <select value={limit} onChange={(e) => setLimit(e.target.value)} aria-label="Лимит">
            {[50, 100, 200].map((n) => (
              <option key={n} value={String(n)}>
                до {n}
              </option>
            ))}
          </select>
          <label className="checkbox anomaly-include-acked">
            <input
              type="checkbox"
              checked={includeAcked}
              onChange={(e) => setIncludeAcked(e.target.checked)}
            />
            <span>С закрытыми</span>
          </label>
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            Обновить
          </button>
        </div>

        <div className="table-wrap">
          <table className="anomalies-table">
            <thead>
              <tr>
                <th scope="col">Важность</th>
                <th scope="col">Обнаружено</th>
                <th scope="col">Тип</th>
                <th scope="col">Описание</th>
                <th scope="col">Источник</th>
                <th scope="col">Назначение</th>
                <th scope="col">Событий</th>
                <th scope="col">Статус</th>
                <th scope="col">
                  <span className="visually-hidden">Действия</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {loading && filtered.length === 0 ? (
                <tr>
                  <td colSpan={9} className="empty">
                    Загрузка…
                  </td>
                </tr>
              ) : filtered.length === 0 ? (
                <tr>
                  <td colSpan={9} className="empty">
                    Нет алертов по текущим фильтрам
                  </td>
                </tr>
              ) : (
                filtered.map((item) => {
                  const fp = item.fingerprint || item.title;
                  const acked = Boolean(item.acknowledged);
                  return (
                    <tr key={fp} className={acked ? 'is-acked' : undefined}>
                      <td>
                        <span className={`anomaly-sev-badge sev-${item.severity || 'warn'}`}>
                          {severityLabel(item.severity)}
                        </span>
                      </td>
                      <td title={item.detected_at ? fmtDate(item.detected_at) : undefined}>
                        {relTime(item.detected_at)}
                      </td>
                      <td>{eventCodeLabel(item)}</td>
                      <td className="anomaly-title-cell">{item.title}</td>
                      <td className="mono">{item.src_ip || '—'}</td>
                      <td className="mono">{item.dst_ip || '—'}</td>
                      <td>{fmtNumber(item.event_count || 0)}</td>
                      <td>{acked ? 'Закрыт' : 'Открыт'}</td>
                      <td className="actions">
                        <Link to={anomalyMapHref(item)} className="btn sm">
                          На карте
                        </Link>
                        {!acked ? (
                          <button
                            type="button"
                            className="btn sm"
                            disabled={acking === item.fingerprint}
                            onClick={() => void hideAlert(item)}
                          >
                            Закрыть
                          </button>
                        ) : null}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </AdminLayout>
  );
}
