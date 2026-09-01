import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  ackAnomaly,
  assignAnomaly,
  fetchAnomalies,
  type AnomalyEvent,
} from '@/api/anomalies';
import { createSearchTemplate } from '@/api/searchTemplates';
import type { MapLine } from '@/api/eventsTypes';
import { listUserDirectory, type UserDirectoryEntry } from '@/api/users';
import { useAuth } from '@/auth/AuthContext';
import { AdminLayout } from '@/components/AdminLayout';
import { ObserveSectionNav } from '@/components/ObserveSectionNav';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import { AnomalyPeersPanel } from '@/pages/Anomalies/AnomalyPeersPanel';
import {
  absoluteAppHref,
  anomalyEventsQuery,
  anomalyMapHref,
  downloadTextFile,
  eventCodeLabel,
  investigateHref,
  investigationTemplateName,
  peersLinesToCsv,
  severityLabel,
  sinceIsoHoursAgo,
} from '@/pages/Anomalies/anomalyDisplay';
import { rememberMapAlert } from '@/pages/Map/AnomalyActiveBanner';
import { mapViewToHuntState } from '@/pages/Hunts/huntMapState';
import { promptSaveHuntFromMap } from '@/pages/Hunts/saveHuntFromMap';
import './investigate.css';

async function findAnomalyByFingerprint(
  fingerprint: string,
  signal?: AbortSignal,
): Promise<{ item: AnomalyEvent | null; related: AnomalyEvent[] }> {
  const data = await fetchAnomalies(
    {
      since: sinceIsoHoursAgo(168),
      include_acked: '1',
      limit: 200,
    },
    { signal, cache: 'no-store' },
  );
  const items = data.items || [];
  const item = items.find((row) => row.fingerprint === fingerprint) || null;
  const related =
    item?.episode_id != null && item.episode_id !== ''
      ? items.filter((row) => row.episode_id === item.episode_id && row.fingerprint !== item.fingerprint)
      : [];
  return { item, related };
}

export default function InvestigatePage() {
  const { toast } = useToast();
  const { user } = useAuth();
  const [params] = useSearchParams();
  const alertFp = (params.get('alert') || '').trim();

  const [item, setItem] = useState<AnomalyEvent | null>(null);
  const [related, setRelated] = useState<AnomalyEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [missing, setMissing] = useState(false);
  const [directory, setDirectory] = useState<UserDirectoryEntry[]>([]);
  const [acking, setAcking] = useState(false);
  const [assigning, setAssigning] = useState(false);
  const [savingTpl, setSavingTpl] = useState(false);
  const [peerLines, setPeerLines] = useState<MapLine[]>([]);

  const load = useCallback(async (fp: string, signal?: AbortSignal) => {
    setLoading(true);
    setMissing(false);
    try {
      const found = await findAnomalyByFingerprint(fp, signal);
      if (signal?.aborted) return;
      setItem(found.item);
      setRelated(found.related);
      setMissing(!found.item);
      setPeerLines([]);
    } catch (e) {
      if (signal?.aborted) return;
      setItem(null);
      setMissing(true);
      toast(e instanceof Error ? e.message : 'Не удалось загрузить алерт', 'error');
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Разбор';
  }, []);

  useEffect(() => {
    if (!alertFp) {
      setItem(null);
      setMissing(true);
      setLoading(false);
      return;
    }
    const ac = new AbortController();
    void load(alertFp, ac.signal);
    return () => ac.abort();
  }, [alertFp, load]);

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
    if (item?.assigned_to) names.add(item.assigned_to);
    if (item?.ack_by) names.add(item.ack_by);
    return [...names]
      .filter(Boolean)
      .map((username) => ({ username, label: personLabel(username) || username }))
      .sort((a, b) => a.label.localeCompare(b.label, 'ru'));
  }, [directory, item?.ack_by, item?.assigned_to, personLabel, user?.username]);

  async function copyText(label: string, text: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast(`${label} скопирована`, 'success');
    } catch {
      toast('Не удалось скопировать в буфер', 'error');
    }
  }

  async function hideAlert() {
    if (!item?.fingerprint || acking) return;
    setAcking(true);
    try {
      const res = await ackAnomaly(item.fingerprint);
      const closer = res.ack_by || user?.username || '';
      setItem((prev) =>
        prev
          ? {
              ...prev,
              acknowledged: true,
              ack_by: closer || prev.ack_by,
              assigned_to: prev.assigned_to || closer,
            }
          : prev,
      );
      toast(closer ? `Алерт закрыт (${personLabel(closer) || closer})` : 'Алерт закрыт', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось закрыть алерт', 'error');
    } finally {
      setAcking(false);
    }
  }

  async function onAssign(assignedTo: string) {
    if (!item?.fingerprint || !assignedTo || assigning) return;
    setAssigning(true);
    try {
      await assignAnomaly(item.fingerprint, assignedTo);
      setItem((prev) => (prev ? { ...prev, assigned_to: assignedTo } : prev));
      toast(`Назначено на ${personLabel(assignedTo) || assignedTo}`, 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось назначить исполнителя', 'error');
    } finally {
      setAssigning(false);
    }
  }

  function onDownloadCsv() {
    if (!item || peerLines.length === 0) {
      toast('Нет связей для выгрузки', 'error');
      return;
    }
    const safe = (item.fingerprint || 'peers').replace(/[^\w.-]+/g, '_').slice(0, 40);
    downloadTextFile(`geoatlas-peers-${safe}.csv`, peersLinesToCsv(peerLines));
    toast('CSV скачан', 'success');
  }

  async function onSaveTemplate() {
    if (!item) return;
    const query = anomalyEventsQuery(item);
    if (!query) {
      toast('Нет query для шаблона', 'error');
      return;
    }
    setSavingTpl(true);
    try {
      await createSearchTemplate({
        name: investigationTemplateName(item),
        query,
      });
      toast('Шаблон поиска сохранён', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось сохранить шаблон', 'error');
    } finally {
      setSavingTpl(false);
    }
  }

  const acked = Boolean(item?.acknowledged);
  const mapHref = item ? anomalyMapHref(item) : '/';
  const workspaceHref = alertFp ? investigateHref(alertFp) : '/investigate';

  return (
    <AdminLayout title="Разбор">
      <div className="page-content-inner wide">
        <ObserveSectionNav />

        <p className="page-lead">
          Рабочее место расследования по одному алерту: контекст, связи из карты, ack/assign и
          выгрузка. Карта открывается deep-link’ом без встраивания.
        </p>

        <p className="investigate-back">
          <Link to="/anomalies">← К списку аномалий</Link>
        </p>

        {!alertFp ? (
          <p className="hint warn-banner">
            Не указан параметр <code>alert</code>. Откройте алерт со страницы{' '}
            <Link to="/anomalies">Аномалии</Link>.
          </p>
        ) : null}

        {loading ? <p className="hint">Загрузка алерта…</p> : null}

        {!loading && missing ? (
          <p className="hint warn-banner">
            Алерт <code>{alertFp}</code> не найден за последние 7 суток (или уже недоступен в
            журнале). Вернитесь к <Link to="/anomalies">списку</Link>.
          </p>
        ) : null}

        {item ? (
          <section className="investigate-card">
            <div className="investigate-card-head">
              <span className={`anomaly-sev-badge sev-${item.severity || 'warn'}`}>
                {severityLabel(item.severity)}
              </span>
              <span className="investigate-code">{eventCodeLabel(item)}</span>
              <span className="hint" title={item.detected_at ? fmtDate(item.detected_at) : undefined}>
                {item.detected_at ? fmtDate(item.detected_at) : '—'}
              </span>
            </div>
            <h2 className="investigate-title">{item.title || 'Без заголовка'}</h2>
            <dl className="investigate-meta">
              <div>
                <dt>Источник</dt>
                <dd className="mono">{item.src_ip || '—'}</dd>
              </div>
              <div>
                <dt>Цель</dt>
                <dd className="mono">{item.dst_ip || '—'}</dd>
              </div>
              <div>
                <dt>Событий</dt>
                <dd>{fmtNumber(item.event_count || 0)}</dd>
              </div>
              <div>
                <dt>Окно</dt>
                <dd className="hint">
                  {item.window_start ? fmtDate(item.window_start) : '—'}
                  {' → '}
                  {item.window_end ? fmtDate(item.window_end) : '—'}
                </dd>
              </div>
              <div>
                <dt>Статус</dt>
                <dd>
                  {acked
                    ? item.ack_by
                      ? `Закрыт (${personLabel(item.ack_by) || item.ack_by})`
                      : 'Закрыт'
                    : 'Открыт'}
                </dd>
              </div>
              <div>
                <dt>Исполнитель</dt>
                <dd>
                  {acked ? (
                    personLabel(item.assigned_to || item.ack_by) || '—'
                  ) : (
                    <select
                      className="anomaly-assignee-select"
                      aria-label="ФИО исполнителя"
                      value={item.assigned_to || ''}
                      disabled={assigning}
                      onChange={(e) => {
                        const v = e.target.value;
                        if (v) void onAssign(v);
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
                </dd>
              </div>
            </dl>

            <div className="investigate-actions">
              <Link
                to={mapHref}
                className="btn"
                onClick={() => rememberMapAlert(item)}
              >
                На карте
              </Link>
              <button
                type="button"
                className="btn"
                onClick={() => void copyText('Ссылка на разбор', absoluteAppHref(workspaceHref))}
              >
                Копировать ссылку на разбор
              </button>
              <button
                type="button"
                className="btn"
                onClick={() => void copyText('Ссылка на карту', absoluteAppHref(mapHref))}
              >
                Копировать ссылку на карту
              </button>
              <button
                type="button"
                className="btn"
                disabled={savingTpl || !anomalyEventsQuery(item)}
                onClick={() => void onSaveTemplate()}
              >
                Сохранить как шаблон поиска
              </button>
              <button
                type="button"
                className="btn"
                onClick={() => {
                  const m = item.map;
                  void promptSaveHuntFromMap(
                    mapViewToHuntState({
                      period: m?.period || '1d',
                      periodFrom: '',
                      periodTo: '',
                      groupBy: m?.group || 'city',
                      filter: (m?.filter as 'all' | 'allowed' | 'blocked') || 'all',
                      search: m?.q || anomalyEventsQuery(item) || '',
                      focusedCountry: m?.country || null,
                    }),
                    toast,
                  );
                }}
              >
                Сохранить как hunt
              </button>
              {!acked ? (
                <button type="button" className="btn" disabled={acking} onClick={() => void hideAlert()}>
                  Закрыть алерт
                </button>
              ) : null}
            </div>

            {related.length ? (
              <section className="investigate-related">
                <h3 className="card-title">Связанные алерты эпизода</h3>
                <ul>
                  {related.map((r) => (
                    <li key={r.fingerprint || r.code}>
                      {r.fingerprint ? (
                        <Link to={investigateHref(r.fingerprint)}>{eventCodeLabel(r)}</Link>
                      ) : (
                        eventCodeLabel(r)
                      )}
                      {' · '}
                      {severityLabel(r.severity)}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}

            <AnomalyPeersPanel
              item={item}
              onLinesLoaded={setPeerLines}
              toolbar={
                <button
                  type="button"
                  className="btn sm"
                  disabled={peerLines.length === 0}
                  onClick={onDownloadCsv}
                >
                  Скачать CSV
                </button>
              }
            />
          </section>
        ) : null}
      </div>
    </AdminLayout>
  );
}
