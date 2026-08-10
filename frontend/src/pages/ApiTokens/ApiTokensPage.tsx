import { FormEvent, useCallback, useEffect, useState } from 'react';
import { apiFetch } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtDate } from '@/lib/format';

interface TokenRow {
  id: string;
  name: string;
  scope: string;
  created_at?: string;
}

export default function ApiTokensPage() {
  const { toast } = useToast();
  const [tokens, setTokens] = useState<TokenRow[]>([]);
  const [name, setName] = useState('');
  const [scope, setScope] = useState('read');
  const [secret, setSecret] = useState('');
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const data = await apiFetch<{ tokens: TokenRow[] }>('/api/tokens');
      setTokens(data.tokens || []);
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка');
    }
  }, []);

  useEffect(() => {
    document.title = 'ГеоАтлас — API-токены';
    void load();
  }, [load]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      const data = await apiFetch<{ secret?: string }>('/api/tokens', {
        method: 'POST',
        body: JSON.stringify({ name: name.trim(), scope }),
      });
      setSecret(data.secret || '');
      setName('');
      toast('Токен создан', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось создать', 'error');
    }
  }

  return (
    <AdminLayout title="API-токены">
      <div className="page-content-inner narrow">
        <div className="card">
          <h2>Создать токен</h2>
          <form className="form-row" onSubmit={onCreate}>
            <div className="field">
              <label htmlFor="cName">Имя</label>
              <input id="cName" required value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="field">
              <label htmlFor="cScope">Scope</label>
              <select id="cScope" value={scope} onChange={(e) => setScope(e.target.value)}>
                <option value="read">read</option>
                <option value="ops">ops</option>
                <option value="admin">admin</option>
              </select>
            </div>
            <button type="submit" className="btn primary">
              Создать
            </button>
          </form>
          {secret ? (
            <div className="secret-panel" style={{ marginTop: 12 }}>
              <p>
                Секрет показывается один раз:
              </p>
              <code id="secretValue">{secret}</code>
              <div className="actions" style={{ marginTop: 8 }}>
                <button
                  type="button"
                  className="btn"
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(secret);
                      toast('Скопировано', 'success');
                    } catch {
                      toast('Не удалось скопировать', 'error');
                    }
                  }}
                >
                  Копировать
                </button>
                <button type="button" className="btn" onClick={() => setSecret('')}>
                  Скрыть
                </button>
              </div>
            </div>
          ) : null}
        </div>
        <div className="card">
          <h2>Токены</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Scope</th>
                  <th scope="col">Создан</th>
                  <th scope="col">
                    <span className="visually-hidden">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {error ? (
                  <tr>
                    <td colSpan={4} className="empty">
                      Ошибка: {error}
                    </td>
                  </tr>
                ) : tokens.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="empty">
                      Пока нет именованных токенов
                    </td>
                  </tr>
                ) : (
                  tokens.map((t) => (
                    <tr key={t.id}>
                      <td>{t.name}</td>
                      <td>
                        <span className="scope-badge">{t.scope}</span>
                      </td>
                      <td>{fmtDate(t.created_at)}</td>
                      <td className="actions">
                        <button
                          type="button"
                          className="btn danger"
                          onClick={async () => {
                            if (!confirm('Отозвать токен?')) return;
                            try {
                              await apiFetch(`/api/tokens/${encodeURIComponent(t.id)}`, {
                                method: 'DELETE',
                              });
                              toast('Отозван', 'success');
                              void load();
                            } catch (err) {
                              toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                            }
                          }}
                        >
                          Отозвать
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
          <p className="hint">
            Env <code>API_AUTH_TOKEN</code> / <code>API_AUTH_PREVIOUS_TOKEN</code> по-прежнему дают
            scope admin (без записи в этот список).
          </p>
        </div>
      </div>
    </AdminLayout>
  );
}
