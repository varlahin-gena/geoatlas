import { FormEvent, useCallback, useEffect, useState } from 'react';
import {
  createUser,
  deleteUser,
  listUsers,
  resetUserPassword,
  updateUserFullName,
  updateUserRole,
  type UserRow,
} from '@/api/users';
import { useAuth } from '@/auth/AuthContext';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtDate } from '@/lib/format';
import { MIN_PASSWORD_LEN, validatePasswordClient } from '@/lib/passwordPolicy';
export default function UsersPage() {
  const { user: me, refresh } = useAuth();
  const { toast } = useToast();
  const [users, setUsers] = useState<UserRow[]>([]);
  const [error, setError] = useState('');
  const [fio, setFio] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState('operator');
  const [mustReset, setMustReset] = useState(true);
  const [resetTarget, setResetTarget] = useState<string | null>(null);
  const [resetPass, setResetPass] = useState('');
  const [resetMust, setResetMust] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await listUsers();
      setUsers(data.users || []);
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка');
    }
  }, []);

  useEffect(() => {
    document.title = 'ГеоАтлас — Пользователи';
    void load();
  }, [load]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    const policyErr = validatePasswordClient(password, username);
    if (policyErr) {
      toast(policyErr, 'error');
      return;
    }
    try {
      await createUser({
        username,
        password,
        role,
        full_name: fio,
        must_reset_password: mustReset,
      });
      toast('Создано', 'success');
      setUsername('');
      setPassword('');
      setFio('');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  return (
    <AdminLayout title="Пользователи">
      <div className="page-content-inner narrow">
        <h1>Пользователи</h1>
        <p className="page-lead">локальные учётные записи</p>
        <div className="card">
          <h2>Создать учётную запись</h2>
          <form className="form-row" onSubmit={onCreate}>
            <div className="field" style={{ minWidth: 220, flex: 1 }}>
              <label htmlFor="cFio">ФИО</label>
              <input id="cFio" maxLength={200} value={fio} onChange={(e) => setFio(e.target.value)} placeholder="Иванов Иван Иванович" />
            </div>
            <div className="field">
              <label htmlFor="cUser">Логин</label>
              <input
                id="cUser"
                required
                pattern="[A-Za-z0-9._\-]{2,64}"
                title="2–64: латиница, цифры, . _ -"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="cPass">Пароль</label>
              <input
                id="cPass"
                type="password"
                required
                minLength={MIN_PASSWORD_LEN}
                autoComplete="new-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <span className="hint">мин. {MIN_PASSWORD_LEN}, буква и цифра</span>            </div>
            <div className="field">
              <label htmlFor="cRole">Роль</label>
              <select id="cRole" value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="operator">Оператор</option>
                <option value="administrator">Администратор</option>
              </select>
            </div>
            <label className="checkbox">
              <input type="checkbox" checked={mustReset} onChange={(e) => setMustReset(e.target.checked)} />
              <span>Сброс пароля при первом входе</span>
            </label>
            <button type="submit" className="btn primary">
              Создать
            </button>
          </form>
        </div>

        <div className="card">
          <h2>Учётные записи</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th scope="col">ФИО</th>
                  <th scope="col">Логин</th>
                  <th scope="col">Роль</th>
                  <th scope="col">Статус</th>
                  <th scope="col">Создана</th>
                  <th scope="col">
                    <span className="visually-hidden">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {error ? (
                  <tr>
                    <td colSpan={6} className="empty">
                      Ошибка: {error}
                    </td>
                  </tr>
                ) : users.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="empty">
                      Нет пользователей
                    </td>
                  </tr>
                ) : (
                  users.map((u) => {
                    const isSelf =
                      !!me?.username && me.username.toLowerCase() === u.username.toLowerCase();
                    return (
                      <tr key={u.username}>
                        <td>
                          <input
                            className="fio-input"
                            defaultValue={u.full_name || ''}
                            maxLength={200}
                            placeholder="ФИО"
                            onBlur={async (e) => {
                              const v = e.target.value;
                              if (v === (u.full_name || '')) return;
                              try {
                                await updateUserFullName(u.username, v);
                                toast('ФИО сохранено', 'success');
                                if (isSelf) void refresh();
                                void load();
                              } catch (err) {
                                toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                                e.target.value = u.full_name || '';
                              }
                            }}
                          />
                        </td>
                        <td>
                          <b>{u.username}</b>
                          {isSelf ? ' ' : null}
                          {isSelf ? <span className="badge">вы</span> : null}
                        </td>
                        <td>
                          <select
                            disabled={isSelf}
                            defaultValue={u.role}
                            onChange={async (e) => {
                              try {
                                await updateUserRole(u.username, e.target.value);
                                toast('Роль обновлена', 'success');
                                void load();
                              } catch (err) {
                                toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                                void load();
                              }
                            }}
                          >
                            <option value="operator">Оператор</option>
                            <option value="administrator">Администратор</option>
                          </select>
                        </td>
                        <td>
                          {u.must_reset_password ? (
                            <span className="badge reset">сброс при входе</span>
                          ) : (
                            <span className="badge">ок</span>
                          )}
                        </td>
                        <td>{fmtDate(u.created_at)}</td>
                        <td className="actions">
                          <button type="button" className="btn" onClick={() => setResetTarget(u.username)}>
                            Сбросить пароль
                          </button>
                          <button
                            type="button"
                            className="btn danger"
                            disabled={isSelf}
                            onClick={async () => {
                              if (!confirm(`Удалить пользователя «${u.username}»?`)) return;
                              try {
                                await deleteUser(u.username);
                                toast('Удалено', 'success');
                                void load();
                              } catch (err) {
                                toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                              }
                            }}
                          >
                            Удалить
                          </button>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {resetTarget ? (
        <div
          className="modal-backdrop show"
          onClick={(e) => {
            if (e.target === e.currentTarget) setResetTarget(null);
          }}
        >
          <form
            className="modal"
            role="dialog"
            aria-modal="true"
            onSubmit={async (e) => {
              e.preventDefault();
              const policyErr = validatePasswordClient(resetPass, resetTarget);
              if (policyErr) {
                toast(policyErr, 'error');
                return;
              }
              try {
                await resetUserPassword(resetTarget, {
                  password: resetPass,
                  must_reset_password: resetMust,
                });
                toast('Пароль сброшен', 'success');
                setResetTarget(null);
                setResetPass('');
                void load();
              } catch (err) {
                toast(err instanceof Error ? err.message : 'Ошибка', 'error');
              }
            }}
          >
            <h3>
              Сброс пароля: <span>{resetTarget}</span>
            </h3>
            <div className="field">
              <label htmlFor="rPass">Новый пароль</label>
              <input
                id="rPass"
                type="password"
                required
                minLength={MIN_PASSWORD_LEN}
                autoComplete="new-password"
                value={resetPass}
                onChange={(e) => setResetPass(e.target.value)}
                autoFocus
              />
            </div>
            <label className="checkbox">
              <input type="checkbox" checked={resetMust} onChange={(e) => setResetMust(e.target.checked)} />
              <span>Сброс пароля при первом входе</span>
            </label>
            <div className="modal-actions">
              <button type="button" className="btn" onClick={() => setResetTarget(null)}>
                Отмена
              </button>
              <button type="submit" className="btn primary">
                Сохранить
              </button>
            </div>
          </form>
        </div>
      ) : null}
      <style>{`
        .fio-input { width: 100%; min-width: 180px; background: var(--panel-2); border: 1px solid var(--border); border-radius: 6px; color: var(--text); padding: 6px 8px; }
        .actions { display: flex; flex-wrap: wrap; gap: 6px; }
      `}</style>
    </AdminLayout>
  );
}
