import { FormEvent, useCallback, useEffect, useState } from 'react';
import { apiFetch, authHeaders } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';

interface Feed {
  name: string;
  url?: string;
  category?: string;
  format?: string;
  enabled?: boolean;
  last_refresh?: string;
  ranges?: number;
}

interface RepList {
  name: string;
  category?: string;
  ranges?: number;
  updated_at?: string;
}

export default function ReputationPage() {
  const { toast } = useToast();
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [lists, setLists] = useState<RepList[]>([]);
  const [catalog, setCatalog] = useState<Feed[]>([]);
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [category, setCategory] = useState('attacks');
  const [format, setFormat] = useState('netset');

  const load = useCallback(async () => {
    try {
      const [f, l, c] = await Promise.all([
        apiFetch<{ feeds?: Feed[] }>('/api/reputation/feeds'),
        apiFetch<{ lists?: RepList[] }>('/api/reputation/lists'),
        apiFetch<{ feeds?: Feed[]; catalog?: Feed[] }>('/api/reputation/catalog'),
      ]);
      setFeeds(f.feeds || []);
      setLists(l.lists || []);
      setCatalog(c.feeds || c.catalog || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Репутация IP';
    void load();
  }, [load]);

  async function onAddFeed(e: FormEvent) {
    e.preventDefault();
    try {
      await apiFetch('/api/reputation/feeds', {
        method: 'POST',
        body: JSON.stringify({ name, url, category, format, enabled: true }),
      });
      toast('Фид добавлен', 'success');
      setName('');
      setUrl('');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  return (
    <AdminLayout
      title="Репутация IP"
      actions={
        <button
          type="button"
          className="btn primary"
          onClick={async () => {
            try {
              await apiFetch('/api/reputation/refresh?force=1', { method: 'POST' });
              toast('Обновление запущено', 'success');
              void load();
            } catch (e) {
              toast(e instanceof Error ? e.message : 'Ошибка', 'error');
            }
          }}
        >
          Обновить фиды
        </button>
      }
    >
      <div className="page-content-inner">
        <h1>Репутация IP</h1>
        <p className="page-lead">
          URL-фиды хранятся в <code>reputation_feeds.json</code> и обновляются по расписанию. Форматы:{' '}
          <code>netset/plain</code>, <code>spamhaus_json</code>, <code>csv_ip</code>.
        </p>
        <p className="hint">
          Списков: {fmtNumber(feeds.length)}, диапазонов:{' '}
          {fmtNumber(lists.reduce((s, l) => s + (l.ranges || 0), 0))}
        </p>
        <div className="card">
          <h2>Добавить фид</h2>
          <form className="form-row" onSubmit={onAddFeed}>
            <div className="field">
              <label>Имя</label>
              <input required value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="field" style={{ flex: 1, minWidth: 240 }}>
              <label>URL</label>
              <input required value={url} onChange={(e) => setUrl(e.target.value)} />
            </div>
            <div className="field">
              <label>Category</label>
              <input value={category} onChange={(e) => setCategory(e.target.value)} />
            </div>
            <div className="field">
              <label>Format</label>
              <select value={format} onChange={(e) => setFormat(e.target.value)}>
                <option value="netset">netset</option>
                <option value="spamhaus_json">spamhaus_json</option>
              </select>
            </div>
            <button type="submit" className="btn primary">
              Добавить
            </button>
          </form>
          <div style={{ marginTop: 12 }}>
            <label>
              Ручная загрузка файла{' '}
              <input
                type="file"
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  try {
                    const res = await fetch('/upload-reputation', {
                      method: 'POST',
                      credentials: 'same-origin',
                      headers: authHeaders({ 'Content-Type': 'text/plain' }),
                      body: file,
                    });
                    if (!res.ok) throw new Error(`HTTP ${res.status}`);
                    toast('Загружено', 'success');
                    void load();
                  } catch (err) {
                    toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                  }
                }}
              />
            </label>
          </div>
        </div>

        <div className="card" style={{ marginTop: 16 }}>
          <h2>Фиды</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Category</th>
                  <th scope="col">Ranges</th>
                  <th scope="col">Обновлён</th>
                  <th scope="col"> </th>
                </tr>
              </thead>
              <tbody>
                {feeds.map((f) => (
                  <tr key={f.name}>
                    <td>{f.name}</td>
                    <td>{f.category}</td>
                    <td>{fmtNumber(f.ranges)}</td>
                    <td>{fmtDate(f.last_refresh)}</td>
                    <td>
                      <button
                        type="button"
                        className="btn danger"
                        onClick={async () => {
                          if (!confirm(`Удалить фид ${f.name}?`)) return;
                          try {
                            await apiFetch(`/api/reputation/feeds/${encodeURIComponent(f.name)}`, {
                              method: 'DELETE',
                            });
                            toast('Удалено', 'success');
                            void load();
                          } catch (e) {
                            toast(e instanceof Error ? e.message : 'Ошибка', 'error');
                          }
                        }}
                      >
                        Удалить
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        <div className="card" style={{ marginTop: 16 }}>
          <h2>Списки</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Category</th>
                  <th scope="col">Ranges</th>
                  <th scope="col"> </th>
                </tr>
              </thead>
              <tbody>
                {lists.map((l) => (
                  <tr key={l.name}>
                    <td>{l.name}</td>
                    <td>{l.category}</td>
                    <td>{fmtNumber(l.ranges)}</td>
                    <td>
                      <button
                        type="button"
                        className="btn danger"
                        onClick={async () => {
                          if (!confirm(`Удалить список ${l.name}?`)) return;
                          try {
                            await apiFetch(`/api/reputation/lists/${encodeURIComponent(l.name)}`, {
                              method: 'DELETE',
                            });
                            toast('Удалено', 'success');
                            void load();
                          } catch (e) {
                            toast(e instanceof Error ? e.message : 'Ошибка', 'error');
                          }
                        }}
                      >
                        Удалить
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        <div className="card" style={{ marginTop: 16 }}>
          <h2>Каталог</h2>
          <ul>
            {catalog.map((c) => (
              <li key={c.name}>
                <button
                  type="button"
                  className="btn"
                  onClick={() => {
                    setName(c.name);
                    setUrl(c.url || '');
                    setCategory(c.category || 'attacks');
                    setFormat(c.format || 'netset');
                  }}
                >
                  {c.name}
                </button>{' '}
                <span className="hint">{c.url}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </AdminLayout>
  );
}
