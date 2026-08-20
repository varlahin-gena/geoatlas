import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { apiFetchRaw } from '@/api/client';
import {
  addEnterpriseNet,
  clearGeoRanges,
  deleteEnterpriseNet,
  exportGeoRanges,
  fetchEnterpriseNets,
  fetchGeoRanges,
  updateGeoRange,
  type EnterpriseNet,
  type GeoRange,
} from '@/api/geo';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';

function isIPv4(s: string): boolean {
  const m = String(s || '')
    .trim()
    .match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (!m) return false;
  for (let i = 1; i <= 4; i++) {
    if (Number(m[i]) > 255) return false;
  }
  return true;
}

export default function GeoRangesPage() {
  const { toast } = useToast();
  const [params] = useSearchParams();
  const [tab, setTab] = useState<'base' | 'enterprise'>('base');
  const [q, setQ] = useState('');
  const [ipSearch, setIpSearch] = useState(() => (params.get('ip') || '').trim());
  const [ipStatus, setIpStatus] = useState<{ mode: '' | 'hit' | 'miss'; text: string }>({
    mode: '',
    text: '',
  });
  const [rows, setRows] = useState<GeoRange[]>([]);
  const [totalInDb, setTotalInDb] = useState(0);
  const [uploadMaxBytes, setUploadMaxBytes] = useState(0);
  const [uploadMaxRanges, setUploadMaxRanges] = useState(0);
  const [ipLookupActive, setIpLookupActive] = useState(false);
  const [edit, setEdit] = useState<GeoRange | null>(null);
  const [form, setForm] = useState({
    original_network: '',
    network: '',
    country: '',
    region: '',
    city: '',
    lat: '',
    lon: '',
  });
  const [busy, setBusy] = useState(false);
  const geoFileRef = useRef<HTMLInputElement>(null);
  const [enterprise, setEnterprise] = useState<EnterpriseNet[]>([]);
  const [entSearch, setEntSearch] = useState('');
  const [entIP, setEntIP] = useState('');
  const [entHits, setEntHits] = useState<GeoRange[]>([]);
  const [entBusy, setEntBusy] = useState(false);
  const [manualNet, setManualNet] = useState('');
  const [manualLabel, setManualLabel] = useState('');

  const load = useCallback(async () => {
    const ip = ipSearch.trim();
    let lookup = false;
    if (ip && isIPv4(ip)) {
      lookup = true;
    } else if (ip) {
      setIpStatus({ mode: 'miss', text: 'Введите корректный IPv4' });
      setIpLookupActive(false);
      return;
    } else {
      setIpStatus({ mode: '', text: '' });
    }
    setIpLookupActive(lookup);
    try {
      const data = await fetchGeoRanges(
        lookup ? { ip } : undefined,
      );
      setRows(data.ranges || []);
      setTotalInDb(Number(data.count) || (data.ranges || []).length);
      if (data.limits) {
        setUploadMaxBytes(Number(data.limits.upload_max_bytes) || 0);
        setUploadMaxRanges(Number(data.limits.upload_max_ranges) || 0);
      }
      if (lookup) {
        setIpStatus(
          data.ip_hit
            ? { mode: 'hit', text: `Найден диапазон для ${ip}` }
            : { mode: 'miss', text: `Нет диапазона для ${ip}` },
        );
      }
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    }
  }, [ipSearch, toast]);

  const loadEnterprise = useCallback(async () => {
    try {
      const data = await fetchEnterpriseNets();
      setEnterprise(data.items || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка загрузки сетей предприятия', 'error');
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — База GeoIP';
    const t = window.setTimeout(() => void load(), 300);
    return () => window.clearTimeout(t);
  }, [load]);

  useEffect(() => {
    void loadEnterprise();
  }, [loadEnterprise]);

  useEffect(() => {
    if (tab !== 'enterprise') return;
    const q = entSearch.trim();
    const ip = entIP.trim();
    const t = window.setTimeout(() => {
      void (async () => {
        if (!q && !ip) {
          setEntHits([]);
          return;
        }
        try {
          const data = await fetchGeoRanges(
            ip && isIPv4(ip) ? { ip } : q ? { q, limit: 200 } : undefined,
          );
          setEntHits(data.ranges || []);
        } catch (e) {
          toast(e instanceof Error ? e.message : 'Ошибка поиска', 'error');
        }
      })();
    }, 300);
    return () => window.clearTimeout(t);
  }, [tab, entSearch, entIP, toast]);

  // SoT: text search is client-side; IP lookup is server-side ?ip=.
  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    if (!needle) return rows;
    return rows.filter(
      (r) =>
        String(r.network || '')
          .toLowerCase()
          .includes(needle) ||
        String(r.country || '')
          .toLowerCase()
          .includes(needle) ||
        String(r.region || '')
          .toLowerCase()
          .includes(needle) ||
        String(r.city || '')
          .toLowerCase()
          .includes(needle),
    );
  }, [rows, q]);

  const markedKeys = useMemo(() => {
    const s = new Set<string>();
    for (const n of enterprise) {
      if (n.start_ip != null && n.end_ip != null) s.add(`${n.start_ip}-${n.end_ip}`);
      if (n.network) s.add(n.network);
    }
    return s;
  }, [enterprise]);

  function isMarked(r: GeoRange): boolean {
    if (r.start_ip != null && r.end_ip != null && markedKeys.has(`${r.start_ip}-${r.end_ip}`)) return true;
    return Boolean(r.network && markedKeys.has(r.network));
  }

  async function markRange(r: GeoRange) {
    const network = (r.network || '').trim();
    if (!network) {
      toast('Нет поля Network', 'error');
      return;
    }
    setEntBusy(true);
    try {
      await addEnterpriseNet({
        network,
        country: r.country,
        region: r.region,
        city: r.city,
        label: [r.city, r.region, r.country].filter(Boolean).join(', '),
      });
      toast(`Отмечено: ${network}`, 'success');
      await loadEnterprise();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    } finally {
      setEntBusy(false);
    }
  }

  async function unmarkNet(n: EnterpriseNet) {
    if (n.start_ip == null || n.end_ip == null) return;
    setEntBusy(true);
    try {
      await deleteEnterpriseNet(n.start_ip, n.end_ip);
      toast('Снята отметка', 'success');
      await loadEnterprise();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    } finally {
      setEntBusy(false);
    }
  }

  async function addManual(e: FormEvent) {
    e.preventDefault();
    const network = manualNet.trim();
    if (!network) {
      toast('Укажите CIDR или IP', 'error');
      return;
    }
    setEntBusy(true);
    try {
      await addEnterpriseNet({ network, label: manualLabel.trim() || undefined });
      setManualNet('');
      setManualLabel('');
      toast(`Отмечено: ${network}`, 'success');
      await loadEnterprise();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    } finally {
      setEntBusy(false);
    }
  }

  function openEdit(r: GeoRange) {
    setEdit(r);
    setForm({
      original_network: r.network || '',
      network: r.network || '',
      country: r.country || '',
      region: r.region || '',
      city: r.city || '',
      lat: r.lat != null ? String(r.lat) : '',
      lon: r.lon != null ? String(r.lon) : '',
    });
  }

  async function onSave(e: FormEvent) {
    e.preventDefault();
    const body = {
      original_network: form.original_network.trim(),
      network: form.network.trim(),
      country: form.country.trim(),
      region: form.region.trim(),
      city: form.city.trim(),
      lat: parseFloat(form.lat),
      lon: parseFloat(form.lon),
    };
    if (!body.original_network || !body.network || !body.country || !body.region || !body.city) {
      toast('Заполните все текстовые поля', 'error');
      return;
    }
    if (!Number.isFinite(body.lat) || !Number.isFinite(body.lon)) {
      toast('Укажите Latitude и Longitude', 'error');
      return;
    }
    try {
      const data = await updateGeoRange(body);
      setEdit(null);
      toast(`Сохранено: ${data.updated || body.network}`, 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? `Не удалось сохранить: ${err.message}` : 'Ошибка', 'error');
    }
  }

  async function clearDatabase() {
    if (
      !window.confirm(
        'Удалить ВСЮ базу GeoIP (geo_ranges) и обнулить индекс в памяти?\nКарта временно останется без координат, пока не зальёте CSV снова.',
      )
    ) {
      return;
    }
    if (!window.confirm('Точно очистить? Это необратимо без повторной загрузки CSV.')) {
      return;
    }
    setBusy(true);
    try {
      const data = await clearGeoRanges();
      toast(
        `База очищена (было в индексе: ${fmtNumber(Number(data.index_before) || 0)}). Можно загрузить CSV.`,
        'success',
      );
      void load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка очистки', 'error');
    } finally {
      setBusy(false);
    }
  }

  async function uploadCsv(file: File) {
    if (uploadMaxBytes > 0 && file.size > uploadMaxBytes) {
      toast(
        `Файл ${fmtNumber(file.size)} байт больше лимита ${fmtNumber(uploadMaxBytes)}`,
        'error',
      );
      return;
    }
    if (totalInDb >= 400_000) {
      const ok = window.confirm(
        `В базе уже ${fmtNumber(totalInDb)} диапазонов. Полная заливка без очистки всё ещё часто упирается в защиту upload и может вернуть HTTP 409.\nBackend теперь расходует RAM экономнее, но безопаснее сначала нажать «Очистить базу». Продолжить?`,
      );
      if (!ok) return;
    }
    setBusy(true);
    try {
      const res = await apiFetchRaw('/upload-geo', {
        method: 'POST',
        headers: { 'Content-Type': 'text/csv' },
        body: file,
      });
      const ct = res.headers.get('content-type') || '';
      const data = ct.includes('application/json')
        ? await res.json().catch(() => ({}))
        : await res.text();
      if (!res.ok) {
        const msg =
          typeof data === 'object' && data && 'error' in data
            ? String((data as { error: unknown }).error)
            : `HTTP ${res.status}`;
        throw new Error(msg);
      }
      const ranges =
        typeof data === 'object' && data && 'ranges' in data
          ? Number((data as { ranges: unknown }).ranges)
          : 0;
      toast(`GeoIP загружен${ranges ? `: ${fmtNumber(ranges)} диапазонов` : ''}`, 'success');
      void load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка загрузки', 'error');
    } finally {
      setBusy(false);
    }
  }

  return (
    <AdminLayout
      title="База GeoIP"
      actions={
        <>
          <input
            ref={geoFileRef}
            type="file"
            accept=".csv,text/csv"
            style={{ display: 'none' }}
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void uploadCsv(f);
              e.target.value = '';
            }}
          />
          <button
            type="button"
            className="btn danger"
            disabled={busy}
            onClick={() => void clearDatabase()}
          >
            Очистить базу
          </button>
          <button
            type="button"
            className="btn"
            disabled={busy}
            onClick={() => geoFileRef.current?.click()}
          >
            Загрузить CSV
          </button>
          <button
            type="button"
            className="btn primary"
            disabled={busy}
            onClick={async () => {
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
                toast(e instanceof Error ? e.message : 'Ошибка', 'error');
              }
            }}
          >
            Выгрузить CSV
          </button>
        </>
      }
    >
      <div className="page-content-inner">
        <h1>База GeoIP</h1>
        <p className="page-lead">
          Текущие диапазоны в таблице geo_ranges. Точечные правки — через «изменить». Полная замена
          базы: «Очистить базу» (снимает early 409), затем «Загрузить CSV». Рестарт backend не нужен.
        </p>
        <nav className="toolbar" role="tablist" aria-label="Разделы базы GeoIP" style={{ gap: 8, marginBottom: 16 }}>
          <button
            type="button"
            role="tab"
            className={tab === 'base' ? 'btn primary' : 'btn'}
            aria-selected={tab === 'base'}
            onClick={() => setTab('base')}
          >
            База GeoIP
          </button>
          <button
            type="button"
            role="tab"
            className={tab === 'enterprise' ? 'btn primary' : 'btn'}
            aria-selected={tab === 'enterprise'}
            onClick={() => setTab('enterprise')}
          >
            Сети предприятия{enterprise.length ? ` (${enterprise.length})` : ''}
          </button>
        </nav>
        {tab === 'enterprise' ? (
          <>
            <p className="hint" style={{ marginTop: 0 }}>
              Отметьте диапазоны своей организации. Без отмеченных сетей детектор аномалий не работает;
              с отметками все алерты учитывают только трафик с/на эти подсети. «Скрыть» подавляет
              повтор по каждой сети отдельно. Пометки не сбрасываются при заливке GeoIP CSV.
            </p>
            <div className="summary" style={{ display: 'flex', gap: 16, marginBottom: 12 }}>
              <div>
                <div className="hint">Отмечено</div>
                <b>{enterprise.length}</b>
              </div>
            </div>
            <form
              className="toolbar"
              style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}
              onSubmit={addManual}
            >
              <input
                placeholder="CIDR вручную, например 10.20.0.0/16"
                value={manualNet}
                onChange={(e) => setManualNet(e.target.value)}
                style={{ minWidth: 240 }}
              />
              <input
                placeholder="Подпись (офис, ЦОД…)"
                value={manualLabel}
                onChange={(e) => setManualLabel(e.target.value)}
                style={{ minWidth: 160 }}
              />
              <button type="submit" className="btn primary" disabled={entBusy}>
                Отметить
              </button>
            </form>
            <div className="toolbar" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
              <input
                placeholder="Найти в GeoIP: город, организация, страна…"
                value={entSearch}
                onChange={(e) => setEntSearch(e.target.value)}
                style={{ minWidth: 260 }}
              />
              <input
                placeholder="Или IP…"
                value={entIP}
                onChange={(e) => setEntIP(e.target.value)}
                style={{ minWidth: 140 }}
              />
            </div>
            {(entSearch.trim() || entIP.trim()) ? (
              <div className="card table-wrap" style={{ marginBottom: 16 }}>
                <table>
                  <thead>
                    <tr>
                      <th scope="col">Network</th>
                      <th scope="col">Country</th>
                      <th scope="col">Region</th>
                      <th scope="col">City</th>
                      <th scope="col">
                        <span className="visually-hidden">Действия</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {!entHits.length ? (
                      <tr>
                        <td colSpan={5} className="empty">
                          Нет диапазонов по запросу
                        </td>
                      </tr>
                    ) : (
                      entHits.map((r) => (
                        <tr key={`${r.network}-${r.start_ip}-${r.end_ip}`}>
                          <td className="mono">{r.network}</td>
                          <td>{r.country}</td>
                          <td>{r.region}</td>
                          <td>{r.city}</td>
                          <td>
                            {isMarked(r) ? (
                              <span className="hint">уже отмечен</span>
                            ) : (
                              <button
                                type="button"
                                className="btn primary"
                                disabled={entBusy}
                                onClick={() => void markRange(r)}
                              >
                                Отметить
                              </button>
                            )}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            ) : null}
            <h2 style={{ fontSize: 16, margin: '8px 0' }}>Отмеченные сети</h2>
            <div className="card table-wrap">
              <table>
                <thead>
                  <tr>
                    <th scope="col">Network</th>
                    <th scope="col">Подпись</th>
                    <th scope="col">Country</th>
                    <th scope="col">City</th>
                    <th scope="col">
                      <span className="visually-hidden">Действия</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {!enterprise.length ? (
                    <tr>
                      <td colSpan={5} className="empty">
                        Пока ничего не отмечено — найдите диапазон в базе GeoIP или введите CIDR
                      </td>
                    </tr>
                  ) : (
                    enterprise.map((n) => (
                      <tr key={`${n.start_ip}-${n.end_ip}`}>
                        <td className="mono">{n.network}</td>
                        <td>{n.label}</td>
                        <td>{n.country}</td>
                        <td>{n.city}</td>
                        <td>
                          <button
                            type="button"
                            className="btn danger"
                            disabled={entBusy}
                            onClick={() => void unmarkNet(n)}
                          >
                            Снять
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </>
        ) : (
          <>
        {totalInDb >= 400_000 ? (
          <p className="hint" style={{ marginBottom: 12, color: 'var(--warn, #b45309)' }}>
            В базе уже {fmtNumber(totalInDb)} диапазонов
            {uploadMaxRanges > 0 ? ` (лимит upload ≈ ${fmtNumber(uploadMaxRanges)})` : ''}.
            Повторная заливка без очистки будет отклонена (HTTP 409).
            {uploadMaxBytes > 0
              ? ` Лимит размера файла: ${(uploadMaxBytes / (1024 * 1024)).toFixed(0)} МиБ.`
              : ''}
          </p>
        ) : null}
        <div className="summary" style={{ display: 'flex', gap: 16, marginBottom: 12 }}>
          <div>
            <div className="hint">Диапазонов в базе</div>
            <b>{fmtNumber(totalInDb || rows.length)}</b>
          </div>
          <div>
            <div className="hint">Показано</div>
            <b>{fmtNumber(filtered.length)}</b>
          </div>
        </div>
        <div className="toolbar" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
          <input
            placeholder="Поиск по Network, стране, городу…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            style={{ minWidth: 220 }}
          />
          <input
            placeholder="Найти по IP…"
            value={ipSearch}
            onChange={(e) => setIpSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                void load();
              }
            }}
            style={{ minWidth: 160 }}
          />
          {ipStatus.text ? (
            <span className={`hint${ipStatus.mode === 'hit' ? '' : ''}`}>{ipStatus.text}</span>
          ) : null}
          <button type="button" className="btn" onClick={() => void load()}>
            Обновить
          </button>
        </div>
        <div className="card table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">Network</th>
                <th scope="col">Country</th>
                <th scope="col">Region</th>
                <th scope="col">City</th>
                <th scope="col">Latitude</th>
                <th scope="col">Longitude</th>
                <th scope="col">
                  <span className="visually-hidden">Действия</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {!filtered.length ? (
                <tr>
                  <td colSpan={7} className="empty">
                    {ipLookupActive
                      ? 'IP не входит ни в один диапазон базы GeoIP'
                      : rows.length
                        ? 'Нет диапазонов по текстовому фильтру'
                        : 'Нет диапазонов — загрузите CSV или добавьте со страницы IP без GeoIP'}
                  </td>
                </tr>
              ) : (
                filtered.map((r) => (
                  <tr key={r.network} className={ipLookupActive ? 'ip-hit-row' : undefined}>
                    <td className="mono">{r.network}</td>
                    <td>{r.country}</td>
                    <td>{r.region}</td>
                    <td>{r.city}</td>
                    <td className="mono">{r.lat}</td>
                    <td className="mono">{r.lon}</td>
                    <td>
                      <button type="button" className="btn" onClick={() => openEdit(r)}>
                        изменить
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
          </>
        )}
      </div>
      {edit ? (
        <div
          className="modal-backdrop show"
          onClick={(e) => e.target === e.currentTarget && setEdit(null)}
        >
          <form className="modal" role="dialog" aria-modal="true" onSubmit={onSave}>
            <h3>Изменить диапазон GeoIP</h3>
            <p className="hint" style={{ marginTop: 0 }}>
              Поля соответствуют шаблону CSV SIEM KUMA.
            </p>
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
                  required
                  value={form[k]}
                  onChange={(e) => setForm({ ...form, [k]: e.target.value })}
                />
              </div>
            ))}
            <div className="modal-actions">
              <button type="button" className="btn" onClick={() => setEdit(null)}>
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
