import { FormEvent, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  createGeoRange,
  exportGeoRanges,
  fetchGeoMissing,
  type GeoMissingRow as MissingRow,
  type GeoMissingSummary as Summary,
} from '@/api/geo';
import { SettingsLayout } from '@/components/SettingsLayout';
import { useToast } from '@/components/Toast';
import { TERM } from '@/lib/glossary';
import { fmtNumber } from '@/lib/format';
import { buildPeriodQuery } from '@/pages/Map/mapConstants';
import { geoMissingKindLabel, geoMissingLogPlace } from './geoMissingDisplay';
import './geoMissing.css';

function ipv4ToUint(ip: string): number | null {
  const parts = String(ip || '').trim().split('.');
  if (parts.length !== 4) return null;
  let v = 0;
  for (let i = 0; i < 4; i++) {
    if (!/^\d+$/.test(parts[i])) return null;
    const n = Number(parts[i]);
    if (n < 0 || n > 255) return null;
    v = ((v << 8) >>> 0) + n;
  }
  return v >>> 0;
}

function parseNetworkBounds(network: string): { start: number; end: number } | null {
  const s = String(network || '').trim();
  if (!s) return null;
  if (s.includes('/')) {
    const slash = s.indexOf('/');
    const startBase = ipv4ToUint(s.slice(0, slash));
    const bits = parseInt(s.slice(slash + 1), 10);
    if (startBase === null || !Number.isInteger(bits) || bits < 0 || bits > 32) return null;
    const hostBits = 32 - bits;
    const mask = bits === 0 ? 0 : (0xffffffff << hostBits) >>> 0;
    const start = (startBase & mask) >>> 0;
    const end = (start | (~mask >>> 0)) >>> 0;
    return { start, end };
  }
  if (s.includes('-')) {
    const dash = s.indexOf('-');
    let a = ipv4ToUint(s.slice(0, dash));
    let b = ipv4ToUint(s.slice(dash + 1));
    if (a === null || b === null) return null;
    if (a > b) {
      const t = a;
      a = b;
      b = t;
    }
    return { start: a, end: b };
  }
  const v = ipv4ToUint(s);
  if (v === null) return null;
  return { start: v, end: v };
}

function summarizeLocal(list: MissingRow[]): Summary {
  const by: Record<string, number> = {};
  let events = 0;
  for (const r of list) {
    const k = r.kind || 'unknown';
    by[k] = (by[k] || 0) + 1;
    events += Number(r.count) || 0;
  }
  return {
    unique_ips: list.length,
    events,
    by_kind: by,
    public_focus: by.public_unknown || 0,
  };
}

function useDialogA11y<T extends HTMLElement>(open: boolean, onDismiss: () => void) {
  const ref = useRef<T | null>(null);
  useEffect(() => {
    if (!open) return;
    const node = ref.current;
    const preferred = node?.querySelector<HTMLElement>('input, button');
    (preferred ?? node)?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onDismiss();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onDismiss]);
  return ref;
}

export default function GeoMissingPage() {
  const { toast } = useToast();
  const titleId = useId();
  const [period, setPeriod] = useState('12h');
  const [limit, setLimit] = useState('500');
  const [kind, setKind] = useState('all');
  const [search, setSearch] = useState('');
  const [rows, setRows] = useState<MissingRow[]>([]);
  const [summary, setSummary] = useState<Summary>({});
  const [loading, setLoading] = useState(true);
  const [editIp, setEditIp] = useState<string | null>(null);
  const [form, setForm] = useState({
    network: '',
    country: '',
    region: '',
    city: '',
    lat: '',
    lon: '',
  });
  const dialogRef = useDialogA11y<HTMLFormElement>(Boolean(editIp), () => setEditIp(null));

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const periodQs = buildPeriodQuery(period, '', '').replace(/^&/, '');
      const data = await fetchGeoMissing(`${periodQs}&limit=${encodeURIComponent(limit)}`);
      setRows(data.items || []);
      setSummary(data.summary || {});
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить IP без координат', 'error');
    } finally {
      setLoading(false);
    }
  }, [period, limit, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — IP без координат';
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return rows.filter((r) => {
      if (kind !== 'all' && r.kind !== kind) return false;
      if (!q) return true;
      return (
        String(r.ip || '')
          .toLowerCase()
          .includes(q) ||
        String(r.sample_peer || '')
          .toLowerCase()
          .includes(q)
      );
    });
  }, [rows, kind, search]);

  const displaySummary = useMemo(() => {
    if (kind === 'all' && !search.trim()) return summary;
    return summarizeLocal(filtered);
  }, [summary, filtered, kind, search]);

  const publicFocus = displaySummary.public_focus || displaySummary.by_kind?.public_unknown || 0;
  const showPublicBanner = publicFocus > 0;

  async function exportCsv() {
    try {
      const res = await exportGeoRanges();
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const blob = await res.blob();
      const cd = res.headers.get('Content-Disposition') || '';
      const m = cd.match(/filename="?([^";]+)"?/i);
      const name = (m && m[1]) || `geoip-${new Date().toISOString().slice(0, 10)}.csv`;
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = name;
      a.click();
      URL.revokeObjectURL(url);
      toast('GeoIP CSV скачан', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось выгрузить CSV', 'error');
    }
  }

  async function copyPublic() {
    const ips = filtered.filter((r) => r.kind === 'public_unknown').map((r) => r.ip);
    if (!ips.length) {
      toast('Нет публичных IP в текущем фильтре', 'error');
      return;
    }
    try {
      await navigator.clipboard.writeText(ips.join('\n'));
      toast(`Скопировано IP: ${fmtNumber(ips.length)}`, 'success');
    } catch {
      toast('Не удалось скопировать', 'error');
    }
  }

  async function copyIp(ip: string) {
    try {
      await navigator.clipboard.writeText(ip);
      toast('IP скопирован', 'success');
    } catch {
      toast('Не удалось скопировать', 'error');
    }
  }

  async function onSave(e: FormEvent) {
    e.preventDefault();
    const body = {
      network: form.network.trim(),
      country: form.country.trim(),
      region: form.region.trim(),
      city: form.city.trim(),
      lat: parseFloat(form.lat),
      lon: parseFloat(form.lon),
    };
    if (!body.network || !body.country || !body.region || !body.city) {
      toast('Заполните все текстовые поля', 'error');
      return;
    }
    if (!Number.isFinite(body.lat) || !Number.isFinite(body.lon)) {
      toast('Укажите широту и долготу', 'error');
      return;
    }
    try {
      const data = await createGeoRange(body);
      const net = data.entry?.network || data.added || body.network;
      const bounds = parseNetworkBounds(net);
      let removed = 0;
      if (bounds) {
        const before = rows.length;
        const next = rows.filter((r) => {
          const v = ipv4ToUint(r.ip);
          if (v === null) return true;
          return !(v >= bounds.start && v <= bounds.end);
        });
        removed = before - next.length;
        setRows(next);
        setSummary(summarizeLocal(next));
      }
      toast(
        `Добавлено: ${net}${data.ranges != null ? ` (всего диапазонов: ${fmtNumber(data.ranges)})` : ''}${
          removed ? `; убрано из списка: ${fmtNumber(removed)}` : ''
        }`,
        'success',
      );
      setEditIp(null);
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить диапазон', 'error');
    }
  }

  const by = displaySummary.by_kind || {};

  return (
    <SettingsLayout
      title="IP без координат"
      plainTitle
      actions={
        <>
          <button type="button" className="btn" onClick={() => void copyPublic()}>
            Копировать публичные IP
          </button>
          <button type="button" className="btn" onClick={() => void exportCsv()}>
            Выгрузить GeoIP CSV
          </button>
        </>
      }
    >
      <div className="page-content-inner">
        <p className="page-lead">
          Уникальные IP без координат. Публичные — в базу GeoIP; приватные на карте обычно не нужны.
        </p>

        <div className="geo-missing-summary">
          <div className="geo-missing-stat">
            <div className="hint">Уникальных IP</div>
            <b>{fmtNumber(displaySummary.unique_ips)}</b>
          </div>
          <div className="geo-missing-stat">
            <div className="hint">Событий</div>
            <b>{fmtNumber(displaySummary.events)}</b>
          </div>
          <div className="geo-missing-stat is-focus">
            <div className="hint">Публичных без geo</div>
            <b>{fmtNumber(publicFocus)}</b>
          </div>
          <div className="geo-missing-stat">
            <div className="hint">Приватных</div>
            <b>{fmtNumber(by.private || 0)}</b>
          </div>
        </div>

        {showPublicBanner ? (
          <p className="geo-missing-banner" role="status">
            Публичные IP без GeoIP — добавьте диапазон в базу (кнопка «В базу» в строке). Страна в
            логе может быть подсказкой, но не заменяет запись в базе.
          </p>
        ) : null}

        <div className="geo-missing-toolbar">
          <div className="field geo-missing-search">
            <label htmlFor="geo-missing-search">Поиск</label>
            <input
              id="geo-missing-search"
              type="search"
              placeholder={`IP или ${TERM.peer.toLowerCase()}…`}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className="field">
            <label htmlFor="geo-missing-period">Период</label>
            <select
              id="geo-missing-period"
              value={period}
              onChange={(e) => setPeriod(e.target.value)}
            >
              <option value="15m">15 мин</option>
              <option value="1h">1 час</option>
              <option value="6h">6 часов</option>
              <option value="12h">12 часов</option>
              <option value="1d">1 день</option>
              <option value="3d">3 дня</option>
              <option value="7d">7 дней</option>
            </select>
          </div>
          <div className="field">
            <label htmlFor="geo-missing-kind">Тип</label>
            <select id="geo-missing-kind" value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="all">Все типы</option>
              <option value="public_unknown">Только публичные</option>
              <option value="private">Приватные</option>
              <option value="loopback">Loopback</option>
              <option value="link_local">Link-local</option>
              <option value="multicast">Multicast</option>
              <option value="invalid">Некорректные</option>
            </select>
          </div>
          <div className="field">
            <label htmlFor="geo-missing-limit">Лимит</label>
            <select
              id="geo-missing-limit"
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
            >
              {[200, 500, 1000, 2000].map((n) => (
                <option key={n} value={String(n)}>
                  {n}
                </option>
              ))}
            </select>
          </div>
          <button type="button" className="btn" disabled={loading} onClick={() => void load()}>
            {loading ? 'Обновление…' : 'Обновить'}
          </button>
        </div>

        <div className="card table-wrap">
          <table className="geo-missing-table">
            <thead>
              <tr>
                <th scope="col">IP</th>
                <th scope="col">Тип</th>
                <th scope="col" className="num">
                  Событий
                </th>
                <th scope="col" className="num">
                  Направление
                </th>
                <th scope="col" title={TERM.peerHint}>
                  {TERM.peer}
                </th>
                <th scope="col">В логе</th>
                <th scope="col">
                  <span className="visually-hidden">Действия</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {loading && !rows.length ? (
                <tr>
                  <td colSpan={7} className="empty">
                    Загрузка…
                  </td>
                </tr>
              ) : !filtered.length ? (
                <tr>
                  <td colSpan={7} className="empty">
                    <div className="geo-missing-empty">
                      <p>Нет IP без координат за выбранный период и фильтр.</p>
                      <p className="hint">
                        Смените период или тип — либо откройте{' '}
                        <Link to="/geo-ranges">базу GeoIP</Link>.
                      </p>
                    </div>
                  </td>
                </tr>
              ) : (
                filtered.map((r) => (
                  <tr key={r.ip}>
                    <td>
                      <div className="geo-missing-ip mono">
                        <span>{r.ip}</span>
                        <button
                          type="button"
                          className="btn sm ghost"
                          aria-label={`Копировать ${r.ip}`}
                          title="Копировать IP"
                          onClick={() => void copyIp(r.ip)}
                        >
                          Копировать
                        </button>
                      </div>
                    </td>
                    <td>{geoMissingKindLabel(r.kind)}</td>
                    <td className="num">
                      <b>{fmtNumber(r.count)}</b>
                    </td>
                    <td className="num" title="источник / назначение">
                      {fmtNumber(r.as_src)} / {fmtNumber(r.as_dst)}
                    </td>
                    <td className="mono">{r.sample_peer || '—'}</td>
                    <td>{geoMissingLogPlace(r.log_city, r.log_country)}</td>
                    <td className="actions">
                      <button
                        type="button"
                        className="btn sm"
                        title={r.action_hint || 'Добавить диапазон в базу GeoIP'}
                        onClick={() => {
                          setEditIp(r.ip);
                          setForm({
                            network: r.ip,
                            country: r.log_country || '',
                            region: '',
                            city: r.log_city || '',
                            lat: '',
                            lon: '',
                          });
                        }}
                      >
                        В базу
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {editIp ? (
        <div
          className="modal-backdrop show"
          role="presentation"
          onClick={(e) => e.target === e.currentTarget && setEditIp(null)}
        >
          <form
            ref={dialogRef}
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby={titleId}
            tabIndex={-1}
            onSubmit={(e) => void onSave(e)}
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id={titleId}>Добавить в GeoIP: {editIp}</h3>
            {(
              [
                ['network', 'Сеть'],
                ['country', 'Страна'],
                ['region', 'Регион'],
                ['city', 'Город'],
                ['lat', 'Широта'],
                ['lon', 'Долгота'],
              ] as const
            ).map(([k, label]) => (
              <div className="field" key={k}>
                <label htmlFor={`geo-add-${k}`}>{label}</label>
                <input
                  id={`geo-add-${k}`}
                  required
                  value={form[k]}
                  onChange={(e) => setForm({ ...form, [k]: e.target.value })}
                />
              </div>
            ))}
            <div className="modal-actions">
              <button type="button" className="btn" onClick={() => setEditIp(null)}>
                Отмена
              </button>
              <button type="submit" className="btn primary">
                Сохранить
              </button>
            </div>
          </form>
        </div>
      ) : null}
    </SettingsLayout>
  );
}
