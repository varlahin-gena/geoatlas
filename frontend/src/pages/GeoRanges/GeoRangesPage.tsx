import { FormEvent, useCallback, useEffect, useState } from 'react';
import { apiFetch, apiFetchRaw } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';

interface GeoRange {
  network: string;
  country?: string;
  region?: string;
  city?: string;
  latitude?: number;
  longitude?: number;
}

export default function GeoRangesPage() {
  const { toast } = useToast();
  const [q, setQ] = useState('');
  const [rows, setRows] = useState<GeoRange[]>([]);
  const [form, setForm] = useState({
    network: '',
    country: '',
    region: '',
    city: '',
    latitude: '',
    longitude: '',
  });

  const load = useCallback(async () => {
    try {
      const qs = q.trim() ? `?search=${encodeURIComponent(q.trim())}` : '';
      const data = await apiFetch<{ ranges?: GeoRange[]; items?: GeoRange[] }>(`/api/geo-ranges${qs}`);
      setRows(data.ranges || data.items || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    }
  }, [q, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — База GeoIP';
    const t = window.setTimeout(() => void load(), 250);
    return () => window.clearTimeout(t);
  }, [load]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      await apiFetch('/api/geo-ranges', {
        method: 'POST',
        body: JSON.stringify({
          network: form.network,
          country: form.country,
          region: form.region,
          city: form.city,
          latitude: Number(form.latitude),
          longitude: Number(form.longitude),
        }),
      });
      toast('Добавлено', 'success');
      setForm({ network: '', country: '', region: '', city: '', latitude: '', longitude: '' });
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  return (
    <AdminLayout
      title="База GeoIP"
      actions={
        <button
          type="button"
          className="btn"
          onClick={async () => {
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
              toast(e instanceof Error ? e.message : 'Ошибка', 'error');
            }
          }}
        >
          CSV
        </button>
      }
    >
      <div className="page-content-inner">
        <div className="card">
          <h2>Добавить диапазон</h2>
          <form className="form-row" onSubmit={onCreate}>
            {(['network', 'country', 'region', 'city', 'latitude', 'longitude'] as const).map((k) => (
              <div className="field" key={k}>
                <label htmlFor={k}>{k}</label>
                <input
                  id={k}
                  required={k === 'network' || k === 'country' || k === 'latitude' || k === 'longitude'}
                  value={form[k]}
                  onChange={(e) => setForm({ ...form, [k]: e.target.value })}
                />
              </div>
            ))}
            <button type="submit" className="btn primary">
              Добавить
            </button>
          </form>
        </div>
        <div className="card" style={{ marginTop: 16 }}>
          <input
            placeholder="Поиск…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            style={{ marginBottom: 12 }}
          />
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th scope="col">Network</th>
                  <th scope="col">Country</th>
                  <th scope="col">City</th>
                  <th scope="col">Lat</th>
                  <th scope="col">Lon</th>
                </tr>
              </thead>
              <tbody>
                {!rows.length ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      Пусто
                    </td>
                  </tr>
                ) : (
                  rows.map((r) => (
                    <tr key={r.network}>
                      <td>{r.network}</td>
                      <td>{r.country}</td>
                      <td>{r.city}</td>
                      <td>{r.latitude}</td>
                      <td>{r.longitude}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </AdminLayout>
  );
}
