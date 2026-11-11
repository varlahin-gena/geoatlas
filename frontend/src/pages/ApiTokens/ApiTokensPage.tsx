import { FormEvent, useCallback, useEffect, useState } from 'react';
import { createToken, deleteToken, listTokens, rotateToken, type TokenRow, type TokenScope } from '@/api/tokens';
import { SettingsLayout } from '@/components/SettingsLayout';
import { EmptyState, TableSkeleton } from '@/components/Skeleton';
import { ReauthField, ReauthModal } from '@/components/ReauthModal';
import { useToast } from '@/components/Toast';
import { scopeLabel, scopeOptionLabel, TERM } from '@/lib/glossary';
import { fmtDate } from '@/lib/format';
import './apiTokens.css';

type ExpiryPreset = 'never' | '30d' | '90d' | '365d';

function expiryISO(preset: ExpiryPreset): string | undefined {
  if (preset === 'never') return undefined;
  const days = preset === '30d' ? 30 : preset === '90d' ? 90 : 365;
  return new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString();
}

function scopeClass(scope: string): string {
  if (scope === 'admin') return 'api-tokens-scope is-admin';
  if (scope === 'ops') return 'api-tokens-scope is-ops';
  return 'api-tokens-scope';
}

export default function ApiTokensPage() {
  const { toast } = useToast();
  const [tokens, setTokens] = useState<TokenRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState('');
  const [scope, setScope] = useState<TokenScope>('ops');
  const [expiry, setExpiry] = useState<ExpiryPreset>('never');
  const [secret, setSecret] = useState('');
  const [secretTitle, setSecretTitle] = useState('Токен создан');
  const [error, setError] = useState('');
  const [reauthPassword, setReauthPassword] = useState('');
  const [pendingAction, setPendingAction] = useState<
    | { type: 'rotate'; id: string; name: string }
    | { type: 'revoke'; id: string; name: string }
    | null
  >(null);
  const [actionBusy, setActionBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listTokens();
      setTokens(data.tokens || []);
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    document.title = 'ГеоАтлас — API-токены';
    void load();
  }, [load]);

  function resetCreateForm() {
    setName('');
    setScope('ops');
    setExpiry('never');
    setReauthPassword('');
  }

  function closeCreateModal() {
    setCreateOpen(false);
    resetCreateForm();
    setSecret('');
    setSecretTitle('Токен создан');
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      const body: { name: string; scope: TokenScope; expires_at?: string; current_password: string } = {
        name: name.trim(),
        scope,
        current_password: reauthPassword,
      };
      const exp = expiryISO(expiry);
      if (exp) body.expires_at = exp;
      const data = await createToken(body);
      setSecretTitle('Токен создан');
      setSecret(data.secret || '');
      resetCreateForm();
      toast('Токен создан', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось создать', 'error');
    }
  }

  return (
    <SettingsLayout
      title="API-токены"
      plainTitle
      actions={
        <button type="button" className="btn primary" onClick={() => setCreateOpen(true)}>
          Создать токен
        </button>
      }
    >
      <div className="page-content-inner narrow">
        <p className="page-lead">
          Именованные токены для автоматизации. Секрет показывается один раз; отзыв сразу закрывает
          доступ.
        </p>
        <div className="card api-tokens-card">
          <div className="table-wrap">
            <table className="api-tokens-table">
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">{TERM.scope}</th>
                  <th scope="col">Создан</th>
                  <th scope="col">Истекает</th>
                  <th scope="col">
                    <span className="visually-hidden">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {error ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      Ошибка: {error}
                    </td>
                  </tr>
                ) : loading ? (
                  <TableSkeleton cols={5} rows={4} />
                ) : tokens.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      <EmptyState
                        compact
                        title="Нет API-токенов"
                        description="Создайте именованный токен для автоматизации и интеграций."
                        action={
                          <button
                            type="button"
                            className="btn primary"
                            onClick={() => setCreateOpen(true)}
                          >
                            Создать токен
                          </button>
                        }
                      />
                    </td>
                  </tr>
                ) : (
                  tokens.map((t) => (
                    <tr key={t.id}>
                      <td>
                        <span className="api-tokens-name" title={t.name}>
                          {t.name}
                        </span>
                      </td>
                      <td>
                        <span className={scopeClass(t.scope)} title={t.scope}>
                          {scopeLabel(t.scope)}
                        </span>
                      </td>
                      <td className="api-tokens-date">{fmtDate(t.created_at)}</td>
                      <td className="api-tokens-date">
                        {t.expires_at ? fmtDate(t.expires_at) : '—'}
                      </td>
                      <td className="actions">
                        <div className="api-tokens-row-actions">
                          <button
                            type="button"
                            className="btn sm"
                            onClick={() => {
                              setPendingAction({ type: 'rotate', id: t.id, name: t.name });
                              setReauthPassword('');
                            }}
                          >
                            Обновить секрет…
                          </button>
                          <button
                            type="button"
                            className="btn sm danger ghost"
                            onClick={() => {
                              setPendingAction({ type: 'revoke', id: t.id, name: t.name });
                              setReauthPassword('');
                            }}
                          >
                            Отозвать
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
          <p className="api-tokens-env-hint">
            Переменные <code>API_AUTH_TOKEN</code> / <code>API_AUTH_PREVIOUS_TOKEN</code> дают
            админ-доступ вне этого списка.
          </p>
        </div>
      </div>

      {createOpen ? (
        <div
          className="modal-backdrop show"
          role="presentation"
          onClick={(e) => {
            if (e.target === e.currentTarget) closeCreateModal();
          }}
        >
          {secret ? (
            <div
              className="modal"
              role="dialog"
              aria-modal="true"
              aria-labelledby="token-secret-title"
              onClick={(e) => e.stopPropagation()}
            >
              <h3 id="token-secret-title">{secretTitle}</h3>
              <div className="api-tokens-secret-panel">
                <p>Секрет показывается один раз:</p>
                <code>{secret}</code>
              </div>
              <div className="modal-actions">
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
                <button type="button" className="btn primary" onClick={closeCreateModal}>
                  Готово
                </button>
              </div>
            </div>
          ) : (
            <form
              className="modal"
              role="dialog"
              aria-modal="true"
              aria-labelledby="create-token-title"
              onSubmit={onCreate}
              onClick={(e) => e.stopPropagation()}
            >
              <h3 id="create-token-title">Создать API-токен</h3>
              <p className="api-tokens-create-hint">
                Область задаёт, к чему токен получит доступ: карта, загрузки или весь API.
              </p>
              <div className="field">
                <label htmlFor="cName">Имя</label>
                <input
                  id="cName"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="ci-bot / grafana"
                  autoFocus
                />
              </div>
              <div className="field">
                <label htmlFor="cScope">{TERM.scope}</label>
                <select
                  id="cScope"
                  value={scope}
                  onChange={(e) => setScope(e.target.value as TokenScope)}
                >
                  <option value="read">{scopeOptionLabel('read')}</option>
                  <option value="ops">{scopeOptionLabel('ops')}</option>
                  <option value="admin">{scopeOptionLabel('admin')}</option>
                </select>
              </div>
              <div className="field">
                <label htmlFor="cExpiry">Срок действия</label>
                <select
                  id="cExpiry"
                  value={expiry}
                  onChange={(e) => setExpiry(e.target.value as ExpiryPreset)}
                >
                  <option value="never">Без срока</option>
                  <option value="30d">30 дней</option>
                  <option value="90d">90 дней</option>
                  <option value="365d">1 год</option>
                </select>
              </div>
              <ReauthField value={reauthPassword} onChange={setReauthPassword} />
              <div className="modal-actions">
                <button type="button" className="btn" onClick={closeCreateModal}>
                  Отмена
                </button>
                <button type="submit" className="btn primary">
                  Создать
                </button>
              </div>
            </form>
          )}
        </div>
      ) : null}

      <ReauthModal
        open={!!pendingAction}
        title={
          pendingAction?.type === 'rotate'
            ? `Обновить секрет «${pendingAction.name}»?`
            : `Отозвать токен «${pendingAction?.name ?? ''}»?`
        }
        message={
          pendingAction?.type === 'rotate'
            ? 'Будет сгенерирован новый секрет. Старый сразу перестанет работать.'
            : 'Токен будет отозван без возможности восстановления.'
        }
        confirmLabel={pendingAction?.type === 'rotate' ? 'Обновить' : 'Отозвать'}
        busy={actionBusy}
        password={reauthPassword}
        onPasswordChange={setReauthPassword}
        onCancel={() => {
          if (actionBusy) return;
          setPendingAction(null);
          setReauthPassword('');
        }}
        onConfirm={() => {
          if (!pendingAction) return;
          void (async () => {
            setActionBusy(true);
            try {
              if (pendingAction.type === 'rotate') {
                const data = await rotateToken(pendingAction.id, reauthPassword);
                setSecretTitle('Секрет обновлён');
                setSecret(data.secret || '');
                setCreateOpen(true);
                toast('Секрет обновлён', 'success');
              } else {
                await deleteToken(pendingAction.id, reauthPassword);
                toast('Отозван', 'success');
              }
              setPendingAction(null);
              setReauthPassword('');
              void load();
            } catch (err) {
              toast(err instanceof Error ? err.message : 'Ошибка', 'error');
            } finally {
              setActionBusy(false);
            }
          })();
        }}
      />
    </SettingsLayout>
  );
}
