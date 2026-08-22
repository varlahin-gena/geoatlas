import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import {
  createGeoRange,
  exportGeoRanges,
  fetchGeoMissing,
  type GeoMissingRow as MissingRow,
  type GeoMissingSummary as Summary,
} from '@/api/geo';
import { AdminLayout } from '@/components/AdminLayout';
import { DataSectionNav } from '@/components/DataSectionNav';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';
import { buildPeriodQuery } from '@/pages/Map/mapConstants';

const KIND_LABELS: Record<string, string> = {
  public_unknown: 'public',
  private: 'private',
  loopback: 'loopback',
  link_local: 'link-local',
  multicast: 'multicast',
  invalid: 'invalid',
};

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

export default function GeoMissingPage() {
  const { toast } = useToast();
  const [period, setPeriod] = useState('12h');
  const [limit, setLimit] = useState('500');
  const [kind, setKind] = useState('all');
  const [search, setSearch] = useState('');
  const [rows, setRows] = useState<MissingRow[]>([]);
  const [summary, setSummary] = useState<Summary>({});
  const [editIp, setEditIp] = useState<string | null>(null);
  const [form, setForm] = useState({
    network: '',
    country: '',
    region: '',
    city: '',
    lat: '',
    lon: '',
  });

  const load = useCallback(async () => {
    try {
      // SoT: minutes=/hours=/days= — not period=.
      const periodQs = buildPeriodQuery(period, '', '').replace(/^&/, '');
      const data = await fetchGeoMissing(`${periodQs}&limit=${encodeURIComponent(limit)}`);
      setRows(data.items || []);
      setSummary(data.summary || {});
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
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
      toast(e instanceof Error ? e.message : 'Ошибка экспорта', 'error');
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
      toast(`Скопировано IP: ${ips.length}`, 'success');
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
      toast('Укажите Latitude и Longitude', 'error');
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
        `Добавлено: ${net}${data.ranges != null ? ` (всего диапазонов: ${data.ranges})` : ''}${
          removed ? `; убрано из списка: ${removed}` : ''
        }`,
        'success',
      );
      setEditIp(null);
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  const by = displaySummary.by_kind || {};

  return (
    <AdminLayout title="IP без координат"
      actions={
        <>
          <button type="button" className="btn" onClick={() => void copyPublic()}>
            Копировать public IP
          </button>
          <button type="button" className="btn" onClick={() => void exportCsv()}>
            Выгрузить GeoIP CSV
          </button>
        </>
      }
    >
      <div className="page-content-inner">
        <DataSectionNav />
        <h1>Адреса, которые не удалось поставить на карту</h1>
        <p className="page-lead">
          Уникальные IP из трафика без координат. Публичные можно добавить в базу GeoIP; приватные
          обычно на карте не нужны.
        </p>
        <div className="summary" style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginBottom: 12 }}>
          <div>
            <div className="hint">Уникальных IP</div>
            <b>{fmtNumber(displaySummary.unique_ips)}</b>
          </div>
          <div>
            <div className="hint">Событий (показано)</div>
            <b>{fmtNumber(displaySummary.events)}</b>
          </div>
          <div>
            <div className="hint">Публичных без geo</div>
            <b>{fmtNumber(displaySummary.public_focus || by.public_unknown || 0)}</b>
          </div>
          <div>
            <div className="hint">Приватных</div>
            <b>{fmtNumber(by.private || 0)}</b>
          </div>
        </div>
        <div className="toolbar" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
          <input
            placeholder="Поиск по IP или peer…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <select value={period} onChange={(e) => setPeriod(e.target.value)}>
            <option value="15m">15 мин</option>
            <option value="1h">1 час</option>
            <option value="6h">6 часов</option>
            <option value="12h">12 часов</option>
            <option value="1d">1 день</option>
            <option value="3d">3 дня</option>
            <option value="7d">7 дней</option>
          </select>
          <select value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="all">Все типы</option>
            <option value="public_unknown">Только публичные</option>
            <option value="private">Приватные</option>
            <option value="loopback">Loopback</option>
            <option value="link_local">Link-local</option>
            <option value="multicast">Multicast</option>
            <option value="invalid">Некорректные</option>
          </select>
          <select value={limit} onChange={(e) => setLimit(e.target.value)}>
            {[200, 500, 1000, 2000].map((n) => (
              <option key={n} value={String(n)}>
                {n}
              </option>
            ))}
          </select>
          <button type="button" className="btn" onClick={() => void load()}>
            Обновить
          </button>
          <span className="badge">{filtered.length ? `Показано: ${filtered.length}` : ''}</span>
        </div>
        <div className="card table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">IP</th>
                <th scope="col">Тип</th>
                <th scope="col">Событий</th>
                <th scope="col">as src/dst</th>
                <th scope="col">Peer</th>
                <th scope="col">Log geo</th>
                <th scope="col">Подсказка</th>
                <th scope="col"> </th>
              </tr>
            </thead>
            <tbody>
              {!filtered.length ? (
                <tr>
                  <td colSpan={8} className="empty">
                    Нет IP без geo за выбранный период / фильтр
                  </td>
                </tr>
              ) : (
                filtered.map((r) => (
                  <tr key={r.ip}>
                    <td className="mono">
                      {r.ip}{' '}
                      <button
                        type="button"
                        className="btn"
                        style={{ padding: '2px 8px', fontSize: 11 }}
                        onClick={async () => {
                          try {
                            await navigator.clipboard.writeText(r.ip);
                            toast('IP скопирован', 'success');
                          } catch {
                            toast('Не удалось скопировать', 'error');
                          }
                        }}
                      >
                        копир.
                      </button>
                    </td>
                    <td>{KIND_LABELS[r.kind || ''] || r.kind || '—'}</td>
                    <td>
                      <b>{fmtNumber(r.count)}</b>
                    </td>
                    <td>
                      {fmtNumber(r.as_src)} / {fmtNumber(r.as_dst)}
                    </td>
                    <td className="mono">{r.sample_peer || '—'}</td>
                    <td>{[r.log_city, r.log_country].filter(Boolean).join(', ') || '—'}</td>
                    <td className="hint">{r.action_hint || ''}</td>
                    <td>
                      <button
                        type="button"
                        className="btn"
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
                        добавить в базу
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
          onClick={(e) => e.target === e.currentTarget && setEditIp(null)}
        >
          <form className="modal" role="dialog" aria-modal="true" onSubmit={onSave}>
            <h3>Добавить GeoIP: {editIp}</h3>
            {(
              [
                ['network', 'Network'],
                ['country', 'Country'],
                ['region', 'Region'],
                ['city', 'City'],
                ['lat', 'Latitude'],
                ['lon', 'Longitude'],
              ] as const
            ).map(([k, label]) => (
              <div className="field" key={k}>
                <label htmlFor={k}>{label}</label>
                <input
                  id={k}
                  required={k === 'network' || k === 'country' || k === 'region' || k === 'city' || k === 'lat' || k === 'lon'}
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
    </AdminLayout>
  );
}
