import { FormEvent, useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  fetchSecuritySummary,
  putLoginPolicy,
  revokeSession,
  revokeUserSessions,
  type AuthSessionRecord,
  type LoginPolicy,
  type SystemSecuritySummary,
} from '@/api/system';
import { ContentSkeleton, EmptyState } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import { auditActionLabel, formatAuditRow } from '@/lib/auditFormat';
import { fmtDate, fmtNumber } from '@/lib/format';

function truncUA(ua?: string): string {
  const s = (ua || '').trim();
  if (!s) return '—';
  return s.length > 48 ? `${s.slice(0, 45)}…` : s;
}

function roleLine(byRole?: Record<string, number>): string {
  if (!byRole) return '—';
  const parts = Object.entries(byRole)
    .filter(([, n]) => n > 0)
    .map(([role, n]) => `${role}: ${n}`);
  return parts.length ? parts.join(' · ') : '—';
}

export function SystemSecurityTab() {
  const { toast } = useToast();
  const [data, setData] = useState<SystemSecuritySummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [policy, setPolicy] = useState<LoginPolicy>({ max_fails: 10, window_sec: 60, lockout_sec: 300 });
  const [savingPolicy, setSavingPolicy] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchSecuritySummary({ cache: 'no-store' });
      setData(res);
      if (res.login_policy) setPolicy(res.login_policy);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить безопасность', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onSavePolicy(e: FormEvent) {
    e.preventDefault();
    setSavingPolicy(true);
    try {
      const next = await putLoginPolicy(policy);
      setPolicy(next);
      toast('Политика сохранена', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить политику', 'error');
    } finally {
      setSavingPolicy(false);
    }
  }

  async function onRevoke(sess: AuthSessionRecord) {
    if (!window.confirm(`Отозвать сессию ${sess.username} (${sess.ip || 'без IP'})?`)) return;
    setBusyId(sess.id);
    try {
      await revokeSession(sess.id);
      toast('Сессия отозвана', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось отозвать сессию', 'error');
    } finally {
      setBusyId(null);
    }
  }

  async function onRevokeUser(username: string) {
    if (!window.confirm(`Выйти везде для «${username}»? Все сессии будут отозваны.`)) return;
    setBusyId(`user:${username}`);
    try {
      await revokeUserSessions(username);
      toast('Все сессии отозваны', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось отозвать сессии', 'error');
    } finally {
      setBusyId(null);
    }
  }

  const failed = data?.failed_logins || [];
  const sessions = data?.sessions || [];
  const recent = data?.auth_recent || [];
  const threat24 = data?.threat_counts?.last_24h;
  const threat7 = data?.threat_counts?.last_7d;
  const posture = data?.posture;

  if (loading && !data) {
    return (
      <div className="tab-panel active" role="tabpanel">
        <ContentSkeleton label="Загрузка безопасности…" />
      </div>
    );
  }

  return (
    <div className="tab-panel active" role="tabpanel">
      <section className="card">
        <details className="section-details" open>
          <summary className="card-title" style={{ color: 'var(--red)' }}>
            ■ Блокировки входа <span>({failed.length ? failed.length : 'нет'})</span>
          </summary>
          <div>
            {!failed.length ? (
              <EmptyState
                compact
                title="Нет неуспешных попыток"
                description="Неудачные логины и блокировки появятся в этом списке."
              />
            ) : (
              <div className="table-wrap">
                <table className="auth-fails-table">
                  <thead>
                    <tr>
                      <th scope="col">Логин</th>
                      <th scope="col">IP</th>
                      <th scope="col">Попыток</th>
                      <th scope="col">Первая</th>
                      <th scope="col">Последняя</th>
                      <th scope="col">Блок</th>
                      <th scope="col">До</th>
                    </tr>
                  </thead>
                  <tbody>
                    {failed.map((f, i) => (
                      <tr key={`${f.username}-${f.ip}-${i}`}>
                        <td>{f.username}</td>
                        <td>{f.ip}</td>
                        <td>{fmtNumber(f.count)}</td>
                        <td>{fmtDate(f.first_at)}</td>
                        <td>{fmtDate(f.last_at)}</td>
                        <td>{f.locked ? <span className="badge-locked">locked</span> : '—'}</td>
                        <td>{f.locked && f.locked_until ? fmtDate(f.locked_until) : '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </details>
      </section>

      <section className="card">
        <h3 className="card-title">Политика lockout</h3>
        <p className="hint">Порог неудачных попыток, окно подсчёта и длительность блокировки.</p>
        <form className="history-filters" onSubmit={onSavePolicy}>
          <label>
            Макс. попыток
            <input
              className="input"
              type="number"
              min={3}
              max={100}
              value={policy.max_fails}
              onChange={(e) => setPolicy((p) => ({ ...p, max_fails: Number(e.target.value) || 0 }))}
            />
          </label>
          <label>
            Окно, сек
            <input
              className="input"
              type="number"
              min={30}
              max={86400}
              value={policy.window_sec}
              onChange={(e) => setPolicy((p) => ({ ...p, window_sec: Number(e.target.value) || 0 }))}
            />
          </label>
          <label>
            Блокировка, сек
            <input
              className="input"
              type="number"
              min={30}
              max={86400}
              value={policy.lockout_sec}
              onChange={(e) => setPolicy((p) => ({ ...p, lockout_sec: Number(e.target.value) || 0 }))}
            />
          </label>
          <button type="submit" className="btn primary" disabled={savingPolicy}>
            Сохранить политику
          </button>
        </form>
      </section>

      <section className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
          <h3 className="card-title" style={{ margin: 0 }}>
            Активные сессии <span>({sessions.length})</span>
          </h3>
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            Обновить
          </button>
        </div>
        {!sessions.length ? (
          <EmptyState compact title="Нет активных сессий" description="Сессии появятся после входа пользователей." />
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Пользователь</th>
                  <th scope="col">IP</th>
                  <th scope="col">Клиент</th>
                  <th scope="col">Создана</th>
                  <th scope="col">Активность</th>
                  <th scope="col">Действия</th>
                </tr>
              </thead>
              <tbody>
                {sessions.map((s) => (
                  <tr key={s.id}>
                    <td>
                      {s.username}
                      <div className="hint">{s.role}</div>
                    </td>
                    <td>{s.ip || '—'}</td>
                    <td title={s.user_agent || ''}>{truncUA(s.user_agent)}</td>
                    <td>{fmtDate(s.created_at)}</td>
                    <td>{fmtDate(s.last_seen_at)}</td>
                    <td style={{ whiteSpace: 'nowrap' }}>
                      <button
                        type="button"
                        className="btn"
                        disabled={busyId === s.id}
                        onClick={() => void onRevoke(s)}
                      >
                        Отозвать
                      </button>{' '}
                      <button
                        type="button"
                        className="btn"
                        disabled={busyId === `user:${s.username}`}
                        onClick={() => void onRevokeUser(s.username)}
                      >
                        Выйти везде
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="card">
        <h3 className="card-title">Недавние входы</h3>
        {!recent.length ? (
          <EmptyState compact title="Событий пока нет" description="Успешные и неудачные входы появятся здесь." />
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Время</th>
                  <th scope="col">Кто</th>
                  <th scope="col">Действие</th>
                  <th scope="col">Результат</th>
                  <th scope="col">IP</th>
                  <th scope="col">Детали</th>
                </tr>
              </thead>
              <tbody>
                {recent.map((item, idx) => (
                  <tr key={`${item.timestamp || 't'}-${item.action || 'a'}-${idx}`}>
                    <td>{fmtDate(item.timestamp)}</td>
                    <td>{item.actor || '—'}</td>
                    <td title={item.action}>{auditActionLabel(item.action)}</td>
                    <td>{item.result || '—'}</td>
                    <td>{item.ip || '—'}</td>
                    <td>{formatAuditRow(item)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="card">
        <h3 className="card-title">Поза</h3>
        <div className="hint" style={{ display: 'grid', gap: 8 }}>
          <div>
            <Link to="/users">Пользователи</Link>
            {' — '}
            {fmtNumber(posture?.users?.total)} ({roleLine(posture?.users?.by_role)})
            {posture?.users?.must_reset_password
              ? ` · сброс пароля: ${posture.users.must_reset_password}`
              : null}
          </div>
          <div>
            <Link to="/api-tokens">API-токены</Link>
            {' — '}
            {fmtNumber(posture?.tokens?.total)}
            {posture?.tokens?.expiring_soon
              ? ` · истекают ≤30 дн.: ${posture.tokens.expiring_soon}`
              : null}
          </div>
          <div>
            <Link to="/tls">TLS</Link>
            {' — '}
            {posture?.tls?.configured
              ? [
                  posture.tls.self_signed ? 'self-signed' : 'сертификат',
                  posture.tls.days_left != null ? `${posture.tls.days_left} дн.` : null,
                  posture.tls.not_after ? `до ${posture.tls.not_after}` : null,
                ]
                  .filter(Boolean)
                  .join(' · ')
              : 'не настроен'}
          </div>
        </div>
      </section>

      <section className="card">
        <h3 className="card-title">Периметр</h3>
        <p className="hint">Отказы CSRF, proxy-gate и API rate-limit из журнала аудита.</p>
        <div className="table-wrap">
          <table className="auth-fails-table">
            <thead>
              <tr>
                <th scope="col">Сигнал</th>
                <th scope="col">24 ч</th>
                <th scope="col">7 дн</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>CSRF</td>
                <td>{fmtNumber(threat24?.csrf_reject)}</td>
                <td>{fmtNumber(threat7?.csrf_reject)}</td>
              </tr>
              <tr>
                <td>Proxy-gate</td>
                <td>{fmtNumber(threat24?.proxy_gate)}</td>
                <td>{fmtNumber(threat7?.proxy_gate)}</td>
              </tr>
              <tr>
                <td>API rate-limit</td>
                <td>{fmtNumber(threat24?.api_rate_limit)}</td>
                <td>{fmtNumber(threat7?.api_rate_limit)}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
