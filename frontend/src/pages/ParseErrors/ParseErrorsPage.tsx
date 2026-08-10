import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { apiFetch } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtDate } from '@/lib/format';

interface ParseErrorRow {
  id: string;
  timestamp?: string;
  vendor?: string;
  reason?: string;
  raw?: string;
}

export default function ParseErrorsPage() {
  const { toast } = useToast();
  const [rows, setRows] = useState<ParseErrorRow[]>([]);
  const [limit, setLimit] = useState('100');
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const q = new URLSearchParams({ limit, search: search.trim() });
      const data = await apiFetch<{ errors: ParseErrorRow[] }>(`/api/parse-errors?${q}`);
      setRows(data.errors || []);
      setSelected(new Set());
    } catch (e) {
      toast(`Ошибка загрузки: ${e instanceof Error ? e.message : e}`, 'error');
    } finally {
      setLoading(false);
    }
  }, [limit, search, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Ошибки парсинга';
  }, []);

  useEffect(() => {
    const t = window.setTimeout(() => void load(), 300);
    return () => window.clearTimeout(t);
  }, [load]);

  async function deleteIds(ids: string[]) {
    if (!ids.length) return;
    try {
      await apiFetch('/api/parse-errors/delete', {
        method: 'POST',
        body: JSON.stringify({ ids }),
      });
      setRows((prev) => prev.filter((r) => !ids.includes(r.id)));
      setSelected(new Set());
      toast(`Удалено записей: ${ids.length}`, 'success');
    } catch (e) {
      toast(`Ошибка удаления: ${e instanceof Error ? e.message : e}`, 'error');
    }
  }

  return (
    <AdminLayout title="Ошибки парсинга">
      <div className="page-content-inner">
        <div className="toolbar" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
          <input
            placeholder="Поиск…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <select value={limit} onChange={(e) => setLimit(e.target.value)}>
            {[50, 100, 200, 500].map((n) => (
              <option key={n} value={String(n)}>
                {n}
              </option>
            ))}
          </select>
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            Обновить
          </button>
          <button
            type="button"
            className="btn"
            disabled={!selected.size}
            onClick={() => void deleteIds([...selected])}
          >
            Удалить выбранные{selected.size ? ` (${selected.size})` : ''}
          </button>
          <button
            type="button"
            className="btn danger"
            onClick={async () => {
              if (!confirm('Удалить ВСЕ нераспознанные строки? Действие необратимо.')) return;
              try {
                await apiFetch('/api/parse-errors/delete', {
                  method: 'POST',
                  body: JSON.stringify({ all: true }),
                });
                setRows([]);
                toast('Таблица очищена', 'success');
              } catch (e) {
                toast(`Ошибка очистки: ${e instanceof Error ? e.message : e}`, 'error');
              }
            }}
          >
            Удалить все
          </button>
          <span className="badge">{rows.length ? `Показано: ${rows.length}` : ''}</span>
        </div>
        <div className="table-wrap card">
          <table>
            <thead>
              <tr>
                <th scope="col">
                  <input
                    type="checkbox"
                    checked={rows.length > 0 && selected.size === rows.length}
                    onChange={(e) => {
                      setSelected(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set());
                    }}
                  />
                </th>
                <th scope="col">Время</th>
                <th scope="col">Вендор</th>
                <th scope="col">Причина</th>
                <th scope="col">Строка</th>
                <th scope="col"> </th>
              </tr>
            </thead>
            <tbody>
              {!rows.length ? (
                <tr>
                  <td colSpan={6} className="empty">
                    {loading ? 'Загрузка…' : 'Нет нераспознанных строк'}
                  </td>
                </tr>
              ) : (
                rows.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <input
                        type="checkbox"
                        checked={selected.has(r.id)}
                        onChange={(e) => {
                          setSelected((prev) => {
                            const n = new Set(prev);
                            if (e.target.checked) n.add(r.id);
                            else n.delete(r.id);
                            return n;
                          });
                        }}
                      />
                    </td>
                    <td>{fmtDate(r.timestamp)}</td>
                    <td>{r.vendor || '—'}</td>
                    <td>{r.reason}</td>
                    <td className="raw">{r.raw}</td>
                    <td>
                      <Link
                        className="btn"
                        to="/parser-test"
                        onClick={() => {
                          try {
                            localStorage.setItem('nm.parserTest.prefill', r.raw || '');
                          } catch {
                            /* ignore */
                          }
                        }}
                      >
                        → Тест
                      </Link>{' '}
                      <button type="button" className="btn" onClick={() => void deleteIds([r.id])}>
                        ✕
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </AdminLayout>
  );
}
