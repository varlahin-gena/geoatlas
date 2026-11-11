import { useCallback, useEffect, useMemo, useState, Fragment } from 'react';
import { Link } from 'react-router-dom';
import {
  ackAnomaly,
  assignAnomaly,
  fetchAnomalies,
  fetchAnomalyEpisodes,
  type AnomalyEvent,
  type AnomalySummary,
} from '@/api/anomalies';
import { listUserDirectory, type UserDirectoryEntry } from '@/api/users';
import { useAuth } from '@/auth/AuthContext';
import { AdminLayout } from '@/components/AdminLayout';
import { EmptyState, TableSkeleton } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import { COPY } from '@/lib/glossary';
import { fmtDate, fmtNumber } from '@/lib/format';
import {
  ANOMALY_CODE_OPTIONS,
  ANOMALY_SINCE_OPTIONS,
  anomalyCodeLabel,
  anomalyEndpointLabel,
  anomalyMapHref,
  compareAnomalies,
  eventCodeLabel,
  formatAnomalyDescription,
  investigateHref,
  matchesAnomalySearch,
  nextAnomalySort,
  relTime,
  severityLabel,
  sinceIsoHoursAgo,
  type AnomalySortDir,
  type AnomalySortKey,
} from './anomalyDisplay';
import { AnomalyPeersPanel } from './AnomalyPeersPanel';
import { rememberMapAlert } from '@/pages/Map/AnomalyActiveBanner';
import './anomalies.css';

function EpisodesPanel({
  sinceHours,
  includeAcked,
}: {
  sinceHours: string;
  includeAcked: boolean;
}) {
  const [episodes, setEpisodes] = useState<import('@/api/anomalies').AnomalyEpisode[]>([]);
  const [open, setOpen] = useState(false);
  const { toast } = useToast();

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    void fetchAnomalyEpisodes({
      since: sinceIsoHoursAgo(Number(sinceHours) || 24),
      include_acked: includeAcked ? '1' : undefined,
    })
      .then((data) => {
        if (!cancelled) setEpisodes(data.episodes || []);
      })
      .catch((e) => {
        if (!cancelled) toast(e instanceof Error ? e.message : COPY.episodesLoadFailed, 'error');
      });
    return () => {
      cancelled = true;
    };
  }, [open, sinceHours, includeAcked, toast]);

  if (!open) {
    return (
      <button type="button" className="btn sm" onClick={() => setOpen(true)}>
        Показать эпизоды корреляции
      </button>
    );
  }

  return (
    <section className="card card-compact anomaly-episodes-panel">
      <div className="anomaly-episodes-head">
        <h3 className="card-title">Эпизоды (корреляция по IP / час)</h3>
        <button type="button" className="btn sm" onClick={() => setOpen(false)}>
          Скрыть
        </button>
      </div>
      {!episodes.length ? <p className="hint">Нет эпизодов за период.</p> : null}
      {episodes.map((ep) => (
        <div key={ep.episode_id || ep.anchor_ip} className="anomaly-episode-row">
          <strong>{ep.anchor_ip || ep.episode_id?.slice(0, 8) || '—'}</strong>
          <span className={`sev-${ep.max_severity}`}>{severityLabel(ep.max_severity)}</span>
          <span>{fmtNumber(ep.alert_count ?? 0)} алертов</span>
          <span className="hint">
            {(ep.codes || []).map(anomalyCodeLabel).filter(Boolean).join(', ')}
          </span>
          <span className="hint">{relTime(ep.updated_at)}</span>
        </div>
      ))}
    </section>
  );
}

function closeAnomalyMore(target: EventTarget | null) {
  const root =
    target instanceof Element ? (target.closest('details') as HTMLDetailsElement | null) : null;
  if (root) root.open = false;
}

function AnomalyRowActions({
  item,
  acked,
  expanded,
  canPeers,
  acking,
  onTogglePeers,
  onHide,
}: {
  item: AnomalyEvent;
  acked: boolean;
  expanded: boolean;
  canPeers: boolean;
  acking: boolean;
  onTogglePeers: () => void;
  onHide: () => void;
}) {
  return (
    <div className="anomaly-row-actions">
      {item.fingerprint ? (
        <Link to={investigateHref(item.fingerprint)} className="btn sm primary">
          Разбор
        </Link>
      ) : null}
      <details className="anomaly-more">
        <summary className="btn sm ghost">Ещё</summary>
        <div className="anomaly-more-menu" role="menu">
          {canPeers ? (
            <button
              type="button"
              role="menuitem"
              className="anomaly-more-item"
              aria-expanded={expanded}
              onClick={(e) => {
                closeAnomalyMore(e.currentTarget);
                onTogglePeers();
              }}
            >
              {expanded ? 'Скрыть связи' : 'Связи'}
            </button>
          ) : null}
          <Link
            to={anomalyMapHref(item)}
            role="menuitem"
            className="anomaly-more-item"
            onClick={(e) => {
              closeAnomalyMore(e.currentTarget);
              rememberMapAlert(item);
            }}
          >
            На карте
          </Link>
          {!acked ? (
            <>
              <div className="anomaly-more-sep" role="separator" />
              <button
                type="button"
                role="menuitem"
                className="anomaly-more-item"
                disabled={acking}
                onClick={(e) => {
                  closeAnomalyMore(e.currentTarget);
                  onHide();
                }}
              >
                Скрыть на 24 ч
              </button>
            </>
          ) : null}
        </div>
      </details>
    </div>
  );
}

function AnomalyDescriptionCell({ item }: { item: AnomalyEvent }) {
  const parts = formatAnomalyDescription(item);
  return (
    <div className="anomaly-desc">
      <div className="anomaly-desc-subject" title={parts.subject}>
        {parts.subject}
      </div>
      {parts.detail ? (
        <div className="anomaly-desc-detail mono" title={parts.detail}>
          {parts.detail}
        </div>
      ) : null}
      {parts.delta ? (
        <div className="anomaly-desc-delta" title={parts.delta}>
          {parts.delta}
        </div>
      ) : null}
    </div>
  );
}

export default function AnomaliesPage() {
  const { toast } = useToast();
  const { user } = useAuth();
  const [rows, setRows] = useState<AnomalyEvent[]>([]);
  const [summary, setSummary] = useState<AnomalySummary | null>(null);
  const [directory, setDirectory] = useState<UserDirectoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [severity, setSeverity] = useState('');
  const [code, setCode] = useState('');
  const [sinceHours, setSinceHours] = useState('24');
  const [includeAcked, setIncludeAcked] = useState(false);
  const [limit, setLimit] = useState('100');
  const [search, setSearch] = useState('');
  const [sortKey, setSortKey] = useState<AnomalySortKey>('detected_at');
  const [sortDir, setSortDir] = useState<AnomalySortDir>('desc');
  const [acking, setAcking] = useState<string | null>(null);
  const [assigning, setAssigning] = useState<string | null>(null);
  const [expandedFp, setExpandedFp] = useState<string | null>(null);

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

  useEffect(() => {
    let cancelled = false;
    void listUserDirectory()
      .then((data) => {
        if (!cancelled) setDirectory(data.users || []);
      })
      .catch(() => {
        if (!cancelled) setDirectory([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const filtersActive =
    Boolean(severity || code || search) ||
    includeAcked ||
    sinceHours !== '24' ||
    limit !== '100';

  function resetFilters() {
    setSeverity('');
    setCode('');
    setSinceHours('24');
    setIncludeAcked(false);
    setLimit('100');
    setSearch('');
  }

  const filtered = useMemo(
    () => rows.filter((row) => matchesAnomalySearch(row, search)),
    [rows, search],
  );

  const sorted = useMemo(() => {
    const list = [...filtered];
    list.sort((a, b) => compareAnomalies(a, b, sortKey, sortDir));
    return list;
  }, [filtered, sortKey, sortDir]);

  function onSort(nextKey: AnomalySortKey) {
    const next = nextAnomalySort(sortKey, sortDir, nextKey);
    setSortKey(next.key);
    setSortDir(next.dir);
  }

  const showSummary =
    (summary?.high || 0) > 0 ||
    (summary?.warn || 0) > 0 ||
    (summary?.total || 0) > 0 ||
    filtered.length > 0;

  const showEpisodes = (summary?.total || 0) > 0 || filtered.length > 0 || includeAcked;

  const fioByUsername = useMemo(() => {
    const map = new Map<string, string>();
    for (const u of directory) {
      const fio = (u.full_name || '').trim();
      if (u.username && fio) map.set(u.username, fio);
    }
    if (user?.username) {
      const selfFio = (user.full_name || '').trim();
      if (selfFio) map.set(user.username, selfFio);
    }
    return map;
  }, [directory, user?.full_name, user?.username]);

  const personLabel = useCallback(
    (username: string | undefined | null) => {
      const login = (username || '').trim();
      if (!login) return '';
      return fioByUsername.get(login) || login;
    },
    [fioByUsername],
  );

  const assigneeOptions = useMemo(() => {
    const names = new Set(directory.map((u) => u.username));
    if (user?.username) names.add(user.username);
    for (const row of rows) {
      if (row.assigned_to) names.add(row.assigned_to);
      if (row.ack_by) names.add(row.ack_by);
    }
    return [...names]
      .filter(Boolean)
      .map((username) => ({ username, label: personLabel(username) || username }))
      .sort((a, b) => a.label.localeCompare(b.label, 'ru'));
  }, [directory, personLabel, rows, user?.username]);

  function toggleSeverityFilter(next: string) {
    setSeverity((cur) => (cur === next ? '' : next));
  }

  async function hideAlert(item: AnomalyEvent) {
    const fp = item.fingerprint;
    if (!fp || acking) return;
    setAcking(fp);
    try {
      const res = await ackAnomaly(fp);
      const closer = res.ack_by || user?.username || '';
      if (!includeAcked) {
        setRows((prev) => prev.filter((r) => r.fingerprint !== fp));
      } else {
        setRows((prev) =>
          prev.map((r) =>
            r.fingerprint === fp
              ? {
                  ...r,
                  acknowledged: true,
                  ack_by: closer || r.ack_by,
                  assigned_to: r.assigned_to || closer,
                }
              : r,
          ),
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
      toast(
        closer
          ? `Алерт скрыт на 24 ч (${personLabel(closer) || closer})`
          : 'Алерт скрыт на 24 ч',
        'success',
      );
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось скрыть алерт', 'error');
    } finally {
      setAcking(null);
    }
  }

  async function onAssign(item: AnomalyEvent, assignedTo: string) {
    const fp = item.fingerprint;
    if (!fp || !assignedTo || assigning) return;
    setAssigning(fp);
    try {
      await assignAnomaly(fp, assignedTo);
      setRows((prev) =>
        prev.map((r) => (r.fingerprint === fp ? { ...r, assigned_to: assignedTo } : r)),
      );
      toast(`Назначено на ${personLabel(assignedTo) || assignedTo}`, 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось назначить исполнителя', 'error');
    } finally {
      setAssigning(null);
    }
  }

  const moduleOff = summary?.module_loaded === false;
  const scanPaused = !moduleOff && summary?.enabled === false;
  const searchActive = Boolean(search.trim());

  return (
    <AdminLayout title="Аномалии">
      <div className="page-content-inner wide">
        {moduleOff ? (
          <p className="hint warn-banner">Модуль аномалий отключён на сервере (ANOMALY_ENABLED).</p>
        ) : null}
        {scanPaused ? (
          <p className="hint warn-banner">
            Сканирование приостановлено в{' '}
            <Link to="/anomalies/engine">настройках движка</Link> — журнал показывает ранее найденные алерты.
          </p>
        ) : null}
        {!moduleOff && !scanPaused && summary?.learning ? (
          <p className="anomaly-learning-note" role="status">
            Режим обучения — «новая страна» и «периодическая связь» пока не ищутся. Параметры — в{' '}
            <Link to="/anomalies/engine">движке</Link>.
          </p>
        ) : null}

        {showEpisodes ? (
          <EpisodesPanel sinceHours={sinceHours} includeAcked={includeAcked} />
        ) : null}

        {showSummary ? (
          <div className="anomaly-summary-row" role="group" aria-label="Сводка по важности">
            <button
              type="button"
              className={`anomaly-summary-chip${severity === 'high' ? ' is-active' : ''}`}
              aria-pressed={severity === 'high'}
              onClick={() => toggleSeverityFilter('high')}
            >
              <span className="anomaly-summary-label">Критично</span>
              <strong className="sev-high">{fmtNumber(summary?.high || 0)}</strong>
            </button>
            <button
              type="button"
              className={`anomaly-summary-chip${severity === 'warn' ? ' is-active' : ''}`}
              aria-pressed={severity === 'warn'}
              onClick={() => toggleSeverityFilter('warn')}
            >
              <span className="anomaly-summary-label">Предупреждения</span>
              <strong className="sev-warn">{fmtNumber(summary?.warn || 0)}</strong>
            </button>
            <button
              type="button"
              className={`anomaly-summary-chip${!severity ? ' is-active' : ''}`}
              aria-pressed={!severity}
              onClick={() => setSeverity('')}
            >
              <span className="anomaly-summary-label">Всего открытых</span>
              <strong>{fmtNumber(summary?.total || 0)}</strong>
            </button>
          </div>
        ) : null}

        <div className="toolbar anomaly-toolbar">
          <div className="anomaly-toolbar-search">
            <input
              type="search"
              className="anomaly-toolbar-control"
              placeholder="Поиск по заголовку, IP, стране…"
              aria-label="Поиск по заголовку, IP, стране"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          {searchActive ? (
            <span className="hint anomaly-search-count" role="status">
              Найдено: {fmtNumber(filtered.length)}
            </span>
          ) : null}
          <div className="anomaly-toolbar-filters">
            <select
              className="anomaly-toolbar-control"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              aria-label="Тип"
            >
              {ANOMALY_CODE_OPTIONS.map((opt) => (
                <option key={opt.value || 'all'} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
            <select
              className="anomaly-toolbar-control"
              value={sinceHours}
              onChange={(e) => setSinceHours(e.target.value)}
              aria-label="Период"
            >
              {ANOMALY_SINCE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
            <select
              className="anomaly-toolbar-control"
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
              aria-label="Лимит"
            >
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
              <span>Со скрытыми</span>
            </label>
          </div>
          <div className="anomaly-toolbar-actions">
            <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
              Обновить
            </button>
          </div>
        </div>

        <div className="table-wrap">
          <table className="anomalies-table">
            <thead>
              <tr>
                <th scope="col">Важность</th>
                <th
                  scope="col"
                  aria-sort={
                    sortKey === 'detected_at'
                      ? sortDir === 'asc'
                        ? 'ascending'
                        : 'descending'
                      : 'none'
                  }
                >
                  <button
                    type="button"
                    className={`anomaly-sort-btn${sortKey === 'detected_at' ? ' is-active' : ''}`}
                    onClick={() => onSort('detected_at')}
                  >
                    Обнаружено
                    <span className="anomaly-sort-ind" aria-hidden="true">
                      {sortKey === 'detected_at' ? (sortDir === 'asc' ? '↑' : '↓') : ''}
                    </span>
                  </button>
                </th>
                <th scope="col">Тип</th>
                <th scope="col">Описание</th>
                <th scope="col">Источник</th>
                <th scope="col">Цель</th>
                <th scope="col">Исполнитель</th>
                <th
                  scope="col"
                  aria-sort={
                    sortKey === 'event_count'
                      ? sortDir === 'asc'
                        ? 'ascending'
                        : 'descending'
                      : 'none'
                  }
                >
                  <button
                    type="button"
                    className={`anomaly-sort-btn${sortKey === 'event_count' ? ' is-active' : ''}`}
                    onClick={() => onSort('event_count')}
                  >
                    Событий
                    <span className="anomaly-sort-ind" aria-hidden="true">
                      {sortKey === 'event_count' ? (sortDir === 'asc' ? '↑' : '↓') : ''}
                    </span>
                  </button>
                </th>
                <th scope="col">Статус</th>
                <th scope="col">
                  <span className="visually-hidden">Действия</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {loading && sorted.length === 0 ? (
                <TableSkeleton cols={10} rows={6} />
              ) : sorted.length === 0 ? (
                <tr>
                  <td colSpan={10} className="empty">
                    <EmptyState
                      compact
                      title="Нет алертов по текущим фильтрам"
                      description={
                        filtersActive
                          ? 'Сбросьте фильтры или расширьте период — возможно, события уже есть за другое окно.'
                          : 'Когда движок зафиксирует аномалию, она появится здесь.'
                      }
                      action={
                        filtersActive ? (
                          <button type="button" className="btn" onClick={resetFilters}>
                            Сбросить фильтры
                          </button>
                        ) : (
                          <Link className="btn" to="/">
                            Открыть карту
                          </Link>
                        )
                      }
                    />
                  </td>
                </tr>
              ) : (
                sorted.map((item) => {
                  const fp = item.fingerprint || item.title;
                  const acked = Boolean(item.acknowledged);
                  const expanded = expandedFp === item.fingerprint;
                  const canPeers = Boolean(item.src_ip || item.map?.q);
                  const detail =
                    item.detail && typeof item.detail === 'object' && !Array.isArray(item.detail)
                      ? (item.detail as Record<string, unknown>)
                      : undefined;
                  const networkFallback =
                    (typeof detail?.network === 'string' && detail.network.trim()) ||
                    (item.device || '').trim() ||
                    undefined;
                  const srcLabel = anomalyEndpointLabel(
                    item.src_ip,
                    item.src_city,
                    item.src_country,
                    networkFallback,
                  );
                  const dstLabel = anomalyEndpointLabel(
                    item.dst_ip,
                    item.dst_city,
                    item.dst_country,
                  );
                  return (
                    <Fragment key={fp}>
                      <tr
                        className={[acked ? 'is-acked' : '', expanded ? 'is-expanded' : '']
                          .filter(Boolean)
                          .join(' ') || undefined}
                      >
                      <td>
                        <span className={`anomaly-sev-badge sev-${item.severity || 'warn'}`}>
                          {severityLabel(item.severity)}
                        </span>
                      </td>
                      <td title={item.detected_at ? fmtDate(item.detected_at) : undefined}>
                        {relTime(item.detected_at)}
                      </td>
                      <td className="anomaly-type-cell" title={eventCodeLabel(item)}>
                        {eventCodeLabel(item)}
                      </td>
                      <td className="anomaly-title-cell">
                        <AnomalyDescriptionCell item={item} />
                      </td>
                      <td
                        className={[
                          'anomaly-endpoint-cell',
                          item.src_ip || (networkFallback && srcLabel === networkFallback)
                            ? 'mono'
                            : undefined,
                        ]
                          .filter(Boolean)
                          .join(' ')}
                        title={srcLabel || undefined}
                      >
                        {srcLabel ? srcLabel : <span className="anomaly-empty-cell">—</span>}
                      </td>
                      <td
                        className={['anomaly-endpoint-cell', item.dst_ip ? 'mono' : undefined]
                          .filter(Boolean)
                          .join(' ')}
                        title={dstLabel || undefined}
                      >
                        {dstLabel ? dstLabel : <span className="anomaly-empty-cell">—</span>}
                      </td>
                      <td>
                        {acked ? (
                          <span
                            title={
                              item.ack_by
                                ? `Скрыл: ${personLabel(item.ack_by) || item.ack_by}`
                                : undefined
                            }
                          >
                            {personLabel(item.assigned_to || item.ack_by) || '—'}
                          </span>
                        ) : (
                          <select
                            className="anomaly-assignee-select"
                            aria-label="Исполнитель"
                            value={item.assigned_to || ''}
                            disabled={assigning === item.fingerprint}
                            onChange={(e) => {
                              const v = e.target.value;
                              if (v) void onAssign(item, v);
                            }}
                          >
                            <option value="">Не назначен</option>
                            {assigneeOptions.map((opt) => (
                              <option key={opt.username} value={opt.username} title={opt.username}>
                                {opt.label}
                              </option>
                            ))}
                          </select>
                        )}
                      </td>
                      <td className="mono">{fmtNumber(item.event_count || 0)}</td>
                      <td>
                        {acked
                          ? item.ack_by
                            ? `Скрыт (${personLabel(item.ack_by) || item.ack_by})`
                            : 'Скрыт'
                          : 'Открыт'}
                      </td>
                      <td className="actions">
                        <AnomalyRowActions
                          item={item}
                          acked={acked}
                          expanded={expanded}
                          canPeers={canPeers}
                          acking={acking === item.fingerprint}
                          onTogglePeers={() =>
                            setExpandedFp((cur) =>
                              cur === item.fingerprint ? null : item.fingerprint || null,
                            )
                          }
                          onHide={() => void hideAlert(item)}
                        />
                      </td>
                    </tr>
                    {expanded && canPeers ? (
                      <tr className="anomaly-peers-row">
                        <td colSpan={10}>
                          <AnomalyPeersPanel item={item} />
                        </td>
                      </tr>
                    ) : null}
                    </Fragment>
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
