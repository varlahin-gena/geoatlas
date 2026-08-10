import { FormEvent, useCallback, useEffect, useState } from 'react';
import { apiFetch, apiFetchRaw } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';

interface MissingRow {
  ip: string;
  count?: number;
  country?: string;
}

export default function GeoMissingPage() {
  const { toast } = useToast();
  const [period, setPeriod] = useState('1d');
  const [limit, setLimit] = useState('200');
  const [rows, setRows] = useState<MissingRow[]>([]);
  const [editIp, setEditIp] = useState<string | null>(null);
  const [form, setForm] = useState({
    network: '',
    country: 'Россия',
    region: '',
    city: '',
    lat: '',
    lon: '',
  });

  const load = useCallback(async () => {
    try {
      const q = new URLSearchParams({ period, limit });
      const data = await apiFetch<{ items?: MissingRow[]; missing?: MissingRow[] }>(
        `/api/geo-missing?${q}`,
      );
      setRows(data.items || data.missing || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    }
  }, [period, limit, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — IP без GeoIP';
    void load();
  }, [load]);

  async function exportCsv() {
    try {
      const res = await apiFetchRaw('/api/geo-ranges/export');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'geo-ranges.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка экспорта', 'error');
    }
  }

  async function onSave(e: FormEvent) {
    e.preventDefault();
    try {
      await apiFetch('/api/geo-ranges', {
        method: 'POST',
        body: JSON.stringify({
          network: form.network || editIp,
          country: form.country,
          region: form.region,
          city: form.city,
          latitude: Number(form.lat),
          longitude: Number(form.lon),
        }),
      });
      toast('Диапазон добавлен', 'success');
      setEditIp(null);
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  return (
    <AdminLayout
      title="IP без GeoIP"
      actions={
        <button type="button" className="btn" onClick={() => void exportCsv()}>
          Выгрузить GeoIP CSV
        </button>
      }
    >
      <div className="page-content-inner">
        <div className="toolbar" style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
          <select value={period} onChange={(e) => setPeriod(e.target.value)}>
            <option value="1h">1ч</option>
            <option value="6h">6ч</option>
            <option value="1d">1д</option>
            <option value="7d">7д</option>
            <option value="30d">30д</option>
          </select>
          <select value={limit} onChange={(e) => setLimit(e.target.value)}>
            {[100, 200, 500, 1000].map((n) => (
              <option key={n} value={String(n)}>
                {n}
              </option>
            ))}
          </select>
          <button type="button" className="btn" onClick={() => void load()}>
            Обновить
          </button>
        </div>
        <div className="card table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">IP</th>
                <th scope="col">Событий</th>
                <th scope="col"> </th>
              </tr>
            </thead>
            <tbody>
              {!rows.length ? (
                <tr>
                  <td colSpan={3} className="empty">
                    Нет IP без координат
                  </td>
                </tr>
              ) : (
                rows.map((r) => (
                  <tr key={r.ip}>
                    <td>{r.ip}</td>
                    <td>{fmtNumber(r.count)}</td>
                    <td>
                      <button
                        type="button"
                        className="btn"
                        onClick={() => {
                          setEditIp(r.ip);
                          setForm((f) => ({ ...f, network: `${r.ip}-${r.ip}` }));
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
        <div className="modal-backdrop show" onClick={(e) => e.target === e.currentTarget && setEditIp(null)}>
          <form className="modal" role="dialog" aria-modal="true" onSubmit={onSave}>
            <h3>Добавить GeoIP: {editIp}</h3>
            {(['network', 'country', 'region', 'city', 'lat', 'lon'] as const).map((k) => (
              <div className="field" key={k}>
                <label htmlFor={k}>{k}</label>
                <input
                  id={k}
                  required={k === 'network' || k === 'country' || k === 'lat' || k === 'lon'}
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
