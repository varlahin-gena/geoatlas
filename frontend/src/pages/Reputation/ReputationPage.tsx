import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { authHeaders } from '@/api/client';
import {
  createReputationFeed,
  deleteReputationList,
  listReputationCatalog,
  listReputationFeeds,
  listReputationLists,
  refreshReputation,
  type ReputationFeed as Feed,
  type ReputationList as RepList,
} from '@/api/reputation';
import { useAuth } from '@/auth/AuthContext';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';

export default function ReputationPage() {
  const { toast } = useToast();
  const { reputationEnabled } = useAuth();
  const navigate = useNavigate();
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [lists, setLists] = useState<RepList[]>([]);
  const [catalog, setCatalog] = useState<Feed[]>([]);
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [category, setCategory] = useState('attacks');
  const [format, setFormat] = useState('netset');
  const [refreshBusy, setRefreshBusy] = useState(false);

  const activeNames = useMemo(
    () => new Set(feeds.map((f) => f.name).filter(Boolean)),
    [feeds],
  );

  const catalogAvailable = useMemo(
    () => catalog.filter((f) => f?.name && !activeNames.has(f.name)),
    [catalog, activeNames],
  );

  const activeRows = useMemo(() => {
    const byFeed: Record<string, Feed> = {};
    const byList: Record<string, RepList> = {};
    feeds.forEach((f) => {
      if (f?.name) byFeed[f.name] = f;
    });
    lists.forEach((l) => {
      if (l?.name) byList[l.name] = l;
    });
    return Object.keys({ ...byFeed, ...byList })
      .sort()
      .map((n) => {
        const f = byFeed[n];
        const l = byList[n];
        return {
          name: n,
          category: f?.category || l?.category || '',
          format: f ? f.format || 'netset' : '',
          url: f?.url || '',
          count: l?.count || 0,
          source: l?.source || (f ? 'url' : ''),
          updated_at: l?.updated_at,
          last_error: l?.last_error || '',
        };
      });
  }, [feeds, lists]);

  const totalRanges = useMemo(
    () => lists.reduce((s, x) => s + (x.count || 0), 0),
    [lists],
  );

  const load = useCallback(async () => {
    try {
      const [f, l, c] = await Promise.all([
        listReputationFeeds(),
        listReputationLists(),
        listReputationCatalog(),
      ]);
      setFeeds(f.feeds || []);
      setLists(l.lists || []);
      setCatalog(c.feeds || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Репутация IP';
    if (!reputationEnabled) {
      navigate('/', { replace: true });
      return;
    }
    void load();
  }, [load, reputationEnabled, navigate]);

  async function addFeedBody(body: {
    name: string;
    url: string;
    category: string;
    format: string;
  }) {
    await createReputationFeed(body);
  }

  async function onAddFeed(e: FormEvent) {
    e.preventDefault();
    try {
      await addFeedBody({
        name: name.trim(),
        url: url.trim(),
        category: category.trim() || 'unknown',
        format: format || 'netset',
      });
      toast(`Фид добавлен: ${name.trim()}. Нажмите «Обновить фиды».`, 'success');
      setName('');
      setUrl('');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  async function refreshFeeds() {
    if (refreshBusy) return;
    setRefreshBusy(true);
    toast('Обновление фидов… это может занять несколько минут', 'info');
    try {
      const data = await refreshReputation(true);
      const updated = Array.isArray(data.updated) ? data.updated : [];
      const skipped = Array.isArray(data.skipped) ? data.skipped : [];
      const failNames = Array.isArray(data.failed) ? data.failed : [];
      const errMap = data.errors && typeof data.errors === 'object' ? data.errors : {};
      const counts = data.counts && typeof data.counts === 'object' ? data.counts : {};

      const lines: string[] = [];
      if (updated.length) {
        let total = 0;
        const perFeed = updated.map((n) => {
          if (String(n).startsWith('removed:')) {
            return `${String(n).slice('removed:'.length)} (удалён устаревший)`;
          }
          const c = Number(counts[n]) || 0;
          total += c;
          return `${n} — ${fmtNumber(c)}`;
        });
        lines.push(`Обновлено ${updated.length} (диапазонов: ${fmtNumber(total)})`);
        lines.push(perFeed.join('\n'));
      } else {
        lines.push('Обновлено: 0');
      }
      if (skipped.length) {
        lines.push(`Без изменений: ${skipped.length} (${skipped.join(', ')})`);
      }
      if (failNames.length) {
        lines.push(`Ошибок: ${failNames.length}`);
        lines.push(
          failNames
            .map((n) => `${n} — ${errMap[n] ? String(errMap[n]) : 'неизвестная ошибка'}`)
            .join('\n'),
        );
      }
      await load();
      toast(lines.join('\n'), failNames.length ? 'error' : 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    } finally {
      setRefreshBusy(false);
    }
  }

  return (
    <AdminLayout
      title="Репутация IP"
      actions={
        <>
          <button type="button" className="btn" onClick={() => void load()}>
            Обновить список
          </button>
          <button
            type="button"
            className="btn primary"
            disabled={refreshBusy}
            onClick={() => void refreshFeeds()}
          >
            {refreshBusy ? 'Обновление…' : 'Обновить фиды'}
          </button>
        </>
      }
    >
      <div className="page-content-inner">
        <h1>Репутация IP</h1>
        <p className="page-lead">
          URL-фиды хранятся в <code>reputation_feeds.json</code> и обновляются по расписанию. Форматы:{' '}
          <code>netset/plain</code>, <code>spamhaus_json</code>, <code>csv_ip</code>.
        </p>
        <p className="hint">
          {activeRows.length
            ? `Списков: ${activeRows.length}, диапазонов: ${fmtNumber(totalRanges)}`
            : 'Списки ещё не загружены — добавьте фид, нажмите «Обновить фиды» или загрузите CSV.'}
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
                <option value="csv_ip">csv_ip</option>
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
                accept=".csv,text/csv,text/plain"
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  e.target.value = '';
                  if (!file) return;
                  try {
                    const res = await fetch('/upload-reputation', {
                      method: 'POST',
                      credentials: 'same-origin',
                      headers: authHeaders({ 'Content-Type': 'text/csv' }),
                      body: await file.text(),
                    });
                    const data = (await res.json().catch(() => ({}))) as {
                      error?: string;
                      ranges?: number;
                    };
                    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
                    toast(`Загружено диапазонов: ${data.ranges || 0}`, 'success');
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
          <h2>Активные списки</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Category</th>
                  <th scope="col">Format</th>
                  <th scope="col">URL</th>
                  <th scope="col">Ranges</th>
                  <th scope="col">Source</th>
                  <th scope="col">Обновлён</th>
                  <th scope="col">Ошибка</th>
                  <th scope="col"> </th>
                </tr>
              </thead>
              <tbody>
                {!activeRows.length ? (
                  <tr>
                    <td colSpan={9} className="empty">
                      Нет активных списков
                    </td>
                  </tr>
                ) : (
                  activeRows.map((r) => (
                    <tr key={r.name}>
                      <td>{r.name}</td>
                      <td>{r.category}</td>
                      <td>{r.format}</td>
                      <td className="raw" title={r.url}>
                        {r.url}
                      </td>
                      <td>{fmtNumber(r.count)}</td>
                      <td>{r.source}</td>
                      <td>{fmtDate(r.updated_at)}</td>
                      <td className="hint" style={{ color: r.last_error ? 'var(--red)' : undefined }}>
                        {r.last_error}
                      </td>
                      <td>
                        <button
                          type="button"
                          className="btn"
                          onClick={async () => {
                            if (
                              !confirm(
                                `Удалить список «${r.name}»? URL-фид с тем же именем тоже будет снят.`,
                              )
                            ) {
                              return;
                            }
                            try {
                              await deleteReputationList(r.name);
                              toast(`Удалено: ${r.name}`, 'success');
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
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>

        <div className="card" style={{ marginTop: 16 }}>
          <h2>Каталог</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Category</th>
                  <th scope="col">Format</th>
                  <th scope="col">URL</th>
                  <th scope="col"> </th>
                </tr>
              </thead>
              <tbody>
                {!catalog.length ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      Каталог пуст
                    </td>
                  </tr>
                ) : !catalogAvailable.length ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      Все пресеты уже в активных фидах
                    </td>
                  </tr>
                ) : (
                  catalogAvailable.map((c) => (
                    <tr key={c.name}>
                      <td>{c.name}</td>
                      <td>{c.category}</td>
                      <td>{c.format || 'netset'}</td>
                      <td className="raw" title={c.url}>
                        {c.url}
                      </td>
                      <td>
                        <button
                          type="button"
                          className="btn primary"
                          onClick={async () => {
                            try {
                              await addFeedBody({
                                name: c.name,
                                url: c.url || '',
                                category: c.category || 'unknown',
                                format: c.format || 'netset',
                              });
                              toast(
                                `Добавлено из каталога: ${c.name}. Нажмите «Обновить фиды».`,
                                'success',
                              );
                              void load();
                            } catch (err) {
                              toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                            }
                          }}
                        >
                          Добавить
                        </button>
                      </td>
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
