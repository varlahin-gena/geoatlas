import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { apiFetch, apiFetchRaw, authHeaders } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';

interface GeoRange {
  network: string;
  country?: string;
  region?: string;
  city?: string;
  lat?: number;
  lon?: number;
}

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

  const load = useCallback(async () => {
    const ip = ipSearch.trim();
    const qs = new URLSearchParams();
    let lookup = false;
    if (ip && isIPv4(ip)) {
      qs.set('ip', ip);
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
      const data = await apiFetch<{
        ranges?: GeoRange[];
        count?: number;
        ip_hit?: boolean;
        limits?: { upload_max_bytes?: number; upload_max_ranges?: number };
      }>(`/api/geo-ranges${qs.toString() ? `?${qs}` : ''}`);
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

  useEffect(() => {
    document.title = 'ГеоАтлас — База GeoIP';
    const t = window.setTimeout(() => void load(), 300);
    return () => window.clearTimeout(t);
  }, [load]);

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
      const data = await apiFetch<{ updated?: string }>('/api/geo-ranges', {
        method: 'PUT',
        body: JSON.stringify(body),
      });
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
      const data = await apiFetch<{ index_before?: number }>('/api/geo-ranges/clear', {
        method: 'POST',
        body: '{}',
      });
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
      const res = await fetch('/upload-geo', {
        method: 'POST',
        credentials: 'same-origin',
        headers: authHeaders({ 'Content-Type': 'text/csv' }),
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
                const res = await apiFetchRaw('/api/geo-ranges/export');
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
