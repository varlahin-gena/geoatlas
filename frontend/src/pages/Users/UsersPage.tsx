import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import {
  createUser,
  deleteUser,
  listUsers,
  resetUserPassword,
  updateUserEnabled,
  updateUserFullName,
  updateUserRole,
  type UserRow,
} from '@/api/users';
import { useAuth } from '@/auth/AuthContext';
import type { UserRole } from '@/api/types';
import { ROLE_ADMIN, ROLE_OPERATOR } from '@/api/types';
import { USER_ROLE_OPTIONS } from '@/auth/roles';
import { SettingsLayout } from '@/components/SettingsLayout';
import { EmptyState, TableSkeleton } from '@/components/Skeleton';
import { ReauthField, ReauthModal } from '@/components/ReauthModal';
import { useToast } from '@/components/Toast';
import { fmtDate } from '@/lib/format';
import { MIN_PASSWORD_LEN, validatePasswordClient } from '@/lib/passwordPolicy';
import './users.css';

function isEnabled(u: UserRow): boolean {
  return u.enabled !== false;
}

export default function UsersPage() {
  const { user: me, refresh } = useAuth();
  const { toast } = useToast();
  const [users, setUsers] = useState<UserRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [fio, setFio] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState<UserRole>(ROLE_OPERATOR);
  const [mustReset, setMustReset] = useState(true);
  const [resetTarget, setResetTarget] = useState<string | null>(null);
  const [resetPass, setResetPass] = useState('');
  const [resetMust, setResetMust] = useState(true);
  const [reauthPassword, setReauthPassword] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleteReauth, setDeleteReauth] = useState('');
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [enableBusy, setEnableBusy] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listUsers();
      setUsers(data.users || []);
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    document.title = 'ГеоАтлас — Пользователи';
    void load();
  }, [load]);

  const enabledAdminCount = useMemo(
    () => users.filter((u) => u.role === ROLE_ADMIN && isEnabled(u)).length,
    [users],
  );

  function resetCreateForm() {
    setUsername('');
    setPassword('');
    setFio('');
    setRole(ROLE_OPERATOR);
    setMustReset(true);
  }

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
      resetCreateForm();
      setCreateOpen(false);
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  async function onToggleEnabled(u: UserRow, next: boolean) {
    const isSelf = !!me?.username && me.username.toLowerCase() === u.username.toLowerCase();
    if (!next && isSelf) {
      toast('Нельзя выключить собственную учётную запись', 'error');
      return;
    }
    if (!next && u.role === ROLE_ADMIN && enabledAdminCount <= 1) {
      toast('Должен остаться хотя бы один активный администратор', 'error');
      return;
    }
    setEnableBusy(u.username);
    try {
      await updateUserEnabled(u.username, next);
      toast(next ? 'Учётная запись включена' : 'Учётная запись выключена', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
      void load();
    } finally {
      setEnableBusy(null);
    }
  }

  return (
    <SettingsLayout title="Пользователи" plainTitle>
      <div className="page-content-inner narrow">
        <p className="page-lead">
          Учётные записи входа в интерфейс. Выключенная запись не входит в систему; должен остаться
          хотя бы один активный администратор.
        </p>

        <div className="page-actions">
          <button type="button" className="btn primary" onClick={() => setCreateOpen(true)}>
            Создать
          </button>
        </div>

        <div className="card users-card">
          <div className="table-wrap">
            <table className="users-table">
              <thead>
                <tr>
                  <th scope="col">ФИО</th>
                  <th scope="col">Логин</th>
                  <th scope="col">Роль</th>
                  <th scope="col">Включена</th>
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
                ) : loading ? (
                  <TableSkeleton cols={6} rows={4} />
                ) : users.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="empty">
                      <EmptyState
                        compact
                        title="Нет пользователей"
                        description="Создайте учётную запись для оператора или dashboard-просмотра."
                        action={
                          <button
                            type="button"
                            className="btn primary"
                            onClick={() => setCreateOpen(true)}
                          >
                            Создать пользователя
                          </button>
                        }
                      />
                    </td>
                  </tr>
                ) : (
                  users.map((u) => {
                    const isSelf =
                      !!me?.username && me.username.toLowerCase() === u.username.toLowerCase();
                    const enabled = isEnabled(u);
                    const lastActiveAdmin = u.role === ROLE_ADMIN && enabled && enabledAdminCount <= 1;
                    const enableBlocked = isSelf || lastActiveAdmin;
                    return (
                      <tr
                        key={u.username}
                        className={enabled ? undefined : 'users-row-disabled'}
                      >
                        <td>
                          <input
                            className="users-fio-input"
                            defaultValue={u.full_name || ''}
                            maxLength={200}
                            placeholder="ФИО"
                            aria-label={`ФИО ${u.username}`}
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
                          <span className="users-login-cell">
                            <b title={u.username}>{u.username}</b>
                            {isSelf ? <span className="badge">Вы</span> : null}
                            {u.must_reset_password ? (
                              <span className="badge reset">сброс при входе</span>
                            ) : null}
                          </span>
                        </td>
                        <td>
                          <select
                            disabled={isSelf}
                            defaultValue={u.role}
                            aria-label={`Роль ${u.username}`}
                            onChange={async (e) => {
                              const nextRole = e.target.value;
                              if (
                                u.role === ROLE_ADMIN &&
                                nextRole !== ROLE_ADMIN &&
                                enabled &&
                                enabledAdminCount <= 1
                              ) {
                                toast(
                                  'Должен остаться хотя бы один активный администратор',
                                  'error',
                                );
                                e.target.value = u.role;
                                return;
                              }
                              try {
                                await updateUserRole(u.username, nextRole);
                                toast('Роль обновлена', 'success');
                                void load();
                              } catch (err) {
                                toast(err instanceof Error ? err.message : 'Ошибка', 'error');
                                void load();
                              }
                            }}
                          >
                            {USER_ROLE_OPTIONS.map((opt) => (
                              <option key={opt.value} value={opt.value}>
                                {opt.label}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td>
                          <label
                            className={`users-enable${enabled ? '' : ' is-off'}`}
                            title={
                              isSelf
                                ? 'Нельзя выключить собственную учётную запись'
                                : lastActiveAdmin
                                  ? 'Должен остаться хотя бы один активный администратор'
                                  : enabled
                                    ? 'Включена'
                                    : 'Выключена'
                            }
                          >
                            <input
                              type="checkbox"
                              role="switch"
                              checked={enabled}
                              disabled={enableBusy === u.username || (!enabled ? false : enableBlocked)}
                              aria-label={
                                enabled
                                  ? `Выключить ${u.username}`
                                  : `Включить ${u.username}`
                              }
                              onChange={(e) => void onToggleEnabled(u, e.target.checked)}
                            />
                          </label>
                        </td>
                        <td className="users-date">{fmtDate(u.created_at)}</td>
                        <td className="actions">
                          <div className="users-row-actions">
                            <details className="users-more">
                              <summary
                                className="btn sm ghost users-more-trigger"
                                aria-label={`Действия для ${u.username}`}
                              >
                                …
                              </summary>
                              <div className="users-more-menu" role="menu">
                                <button
                                  type="button"
                                  role="menuitem"
                                  className="users-more-item"
                                  onClick={(e) => {
                                    const root = e.currentTarget.closest('details');
                                    if (root instanceof HTMLDetailsElement) root.open = false;
                                    setResetTarget(u.username);
                                  }}
                                >
                                  Сбросить пароль…
                                </button>
                                <div className="users-more-sep" role="separator" />
                                <button
                                  type="button"
                                  role="menuitem"
                                  className="users-more-item is-danger"
                                  disabled={isSelf}
                                  onClick={(e) => {
                                    const root = e.currentTarget.closest('details');
                                    if (root instanceof HTMLDetailsElement) root.open = false;
                                    setDeleteTarget(u.username);
                                    setDeleteReauth('');
                                  }}
                                >
                                  Удалить
                                </button>
                              </div>
                            </details>
                          </div>
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

      {createOpen ? (
        <div
          className="modal-backdrop show"
          role="presentation"
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              setCreateOpen(false);
              resetCreateForm();
            }
          }}
        >
          <form
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="create-user-title"
            onSubmit={onCreate}
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id="create-user-title">Создать учётную запись</h3>
            <div className="field">
              <label htmlFor="cFio">ФИО</label>
              <input
                id="cFio"
                maxLength={200}
                value={fio}
                onChange={(e) => setFio(e.target.value)}
                placeholder="Иванов Иван Иванович"
              />
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
                autoFocus
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
              <span className="field-hint">мин. {MIN_PASSWORD_LEN}, буква и цифра</span>
            </div>
            <div className="field">
              <label htmlFor="cRole">Роль</label>
              <select id="cRole" value={role} onChange={(e) => setRole(e.target.value as UserRole)}>
                {USER_ROLE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={mustReset}
                onChange={(e) => setMustReset(e.target.checked)}
              />
              <span>Сброс пароля при первом входе</span>
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="btn"
                onClick={() => {
                  setCreateOpen(false);
                  resetCreateForm();
                }}
              >
                Отмена
              </button>
              <button type="submit" className="btn primary">
                Создать
              </button>
            </div>
          </form>
        </div>
      ) : null}

      {resetTarget ? (
        <div
          className="modal-backdrop show"
          role="presentation"
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              setResetTarget(null);
              setReauthPassword('');
            }
          }}
        >
          <form
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="reset-user-title"
            onClick={(e) => e.stopPropagation()}
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
                  current_password: reauthPassword,
                });
                toast('Пароль сброшен', 'success');
                setResetTarget(null);
                setResetPass('');
                setReauthPassword('');
                void load();
              } catch (err) {
                toast(err instanceof Error ? err.message : 'Ошибка', 'error');
              }
            }}
          >
            <h3 id="reset-user-title">
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
              <input
                type="checkbox"
                checked={resetMust}
                onChange={(e) => setResetMust(e.target.checked)}
              />
              <span>Сброс пароля при первом входе</span>
            </label>
            <ReauthField value={reauthPassword} onChange={setReauthPassword} />
            <div className="modal-actions">
              <button
                type="button"
                className="btn"
                onClick={() => {
                  setResetTarget(null);
                  setReauthPassword('');
                }}
              >
                Отмена
              </button>
              <button type="submit" className="btn primary">
                Сохранить
              </button>
            </div>
          </form>
        </div>
      ) : null}

      <ReauthModal
        open={!!deleteTarget}
        title={`Удалить пользователя «${deleteTarget ?? ''}»?`}
        message="Учётная запись будет удалена без возможности восстановления."
        confirmLabel="Удалить"
        busy={deleteBusy}
        password={deleteReauth}
        onPasswordChange={setDeleteReauth}
        onCancel={() => {
          if (deleteBusy) return;
          setDeleteTarget(null);
          setDeleteReauth('');
        }}
        onConfirm={() => {
          if (!deleteTarget) return;
          void (async () => {
            setDeleteBusy(true);
            try {
              await deleteUser(deleteTarget, deleteReauth);
              toast('Удалено', 'success');
              setDeleteTarget(null);
              setDeleteReauth('');
              void load();
            } catch (err) {
              toast(err instanceof Error ? err.message : 'Ошибка', 'error');
            } finally {
              setDeleteBusy(false);
            }
          })();
        }}
      />
    </SettingsLayout>
  );
}
