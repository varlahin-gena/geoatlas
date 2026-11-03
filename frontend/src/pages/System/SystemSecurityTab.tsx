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

const ROLE_LABELS: Record<string, string> = {
  administrator: 'администратор',
  admin: 'администратор',
  operator: 'оператор',
  viewer: 'просмотр',
  readonly: 'только чтение',
};

function truncUA(ua?: string): string {
  const s = (ua || '').trim();
  if (!s) return '—';
  return s.length > 48 ? `${s.slice(0, 45)}…` : s;
}

function roleLabel(role: string): string {
  return ROLE_LABELS[role] || role;
}

function roleLine(byRole?: Record<string, number>): string {
  if (!byRole) return '—';
  const parts = Object.entries(byRole)
    .filter(([, n]) => n > 0)
    .map(([role, n]) => `${roleLabel(role)}: ${n}`);
  return parts.length ? parts.join(' · ') : '—';
}

function resultClass(result?: string): string {
  const r = (result || '').toLowerCase();
  if (r === 'succeeded' || r === 'success' || r === 'ok') return 'result-ok';
  if (r === 'failed' || r === 'error' || r === 'denied') return 'result-bad';
  return '';
}

function resultLabel(result?: string): string {
  const r = (result || '').toLowerCase();
  if (r === 'succeeded' || r === 'success' || r === 'ok') return 'Успех';
  if (r === 'failed' || r === 'error') return 'Ошибка';
  if (r === 'denied') return 'Отклонён';
  return result || '—';
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
      toast(e instanceof Error ? e.message : 'Не удалось загрузить раздел', 'error');
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
      toast('Настройки сохранены', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить настройки', 'error');
    } finally {
      setSavingPolicy(false);
    }
  }

  async function onRevoke(sess: AuthSessionRecord) {
    const where = sess.ip ? ` с адреса ${sess.ip}` : '';
    if (!window.confirm(`Завершить вход «${sess.username}»${where}?`)) return;
    setBusyId(sess.id);
    try {
      await revokeSession(sess.id);
      toast('Вход завершён', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось завершить вход', 'error');
    } finally {
      setBusyId(null);
    }
  }

  async function onRevokeUser(username: string) {
    if (!window.confirm(`Завершить все входы для «${username}»?`)) return;
    setBusyId(`user:${username}`);
    try {
      await revokeUserSessions(username);
      toast('Все входы завершены', 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось завершить входы', 'error');
    } finally {
      setBusyId(null);
    }
  }

  // API отдаёт недавние неудачи входа; в этом блоке — только активный lockout.
  const locked = (data?.failed_logins || []).filter((f) => f.locked);
  const sessions = data?.sessions || [];
  const recent = data?.auth_recent || [];
  const threat24 = data?.threat_counts?.last_24h;
  const threat7 = data?.threat_counts?.last_7d;
  const posture = data?.posture;

  if (loading && !data) {
    return (
      <div className="tab-panel active" role="tabpanel">
        <ContentSkeleton label="Загрузка…" />
      </div>
    );
  }

  const tlsValue = posture?.tls?.configured
    ? [
        posture.tls.self_signed ? 'свой сертификат' : 'сертификат установлен',
        posture.tls.days_left != null ? `осталось ${posture.tls.days_left} дн.` : null,
        posture.tls.not_after ? `действует до ${posture.tls.not_after}` : null,
      ]
        .filter(Boolean)
        .join(' · ')
    : 'не установлен';

  return (
    <div className="tab-panel active security-tab" role="tabpanel">
      <section className="card">
        <h3 className={`card-title${locked.length ? ' is-alert' : ''}`}>
          Кто сейчас заблокирован
          <span className="card-title-count">{locked.length}</span>
        </h3>
        {!locked.length ? (
          <EmptyState
            compact
            title="Блокировок нет"
            description="Если кто-то много раз ошибётся с паролем, запись появится здесь."
          />
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Учётная запись</th>
                  <th scope="col">Адрес</th>
                  <th scope="col" className="num">
                    Попыток
                  </th>
                  <th scope="col">Первая попытка</th>
                  <th scope="col">Последняя</th>
                  <th scope="col">До разблокировки</th>
                </tr>
              </thead>
              <tbody>
                {locked.map((f, i) => (
                  <tr key={`${f.username}-${f.ip}-${i}`}>
                    <td>{f.username}</td>
                    <td className="mono">{f.ip}</td>
                    <td className="num">{fmtNumber(f.count)}</td>
                    <td>{fmtDate(f.first_at)}</td>
                    <td>{fmtDate(f.last_at)}</td>
                    <td>{f.locked_until ? fmtDate(f.locked_until) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>


      <section className="card">
        <h3 className="card-title">Политика блокировки УЗ</h3>
        <p className="hint">Сколько раз можно ошибиться с паролем и на сколько потом блокировать вход.</p>
        <form className="form-row policy-form" onSubmit={onSavePolicy}>
          <div className="field">
            <label htmlFor="lockMaxFails">Попыток до блокировки</label>
            <input
              id="lockMaxFails"
              type="number"
              min={3}
              max={100}
              value={policy.max_fails}
              onChange={(e) => setPolicy((p) => ({ ...p, max_fails: Number(e.target.value) || 0 }))}
            />
          </div>
          <div className="field">
            <label htmlFor="lockWindow">За сколько секунд</label>
            <input
              id="lockWindow"
              type="number"
              min={30}
              max={86400}
              value={policy.window_sec}
              onChange={(e) => setPolicy((p) => ({ ...p, window_sec: Number(e.target.value) || 0 }))}
            />
          </div>
          <div className="field">
            <label htmlFor="lockDuration">Блокировать на, сек</label>
            <input
              id="lockDuration"
              type="number"
              min={30}
              max={86400}
              value={policy.lockout_sec}
              onChange={(e) => setPolicy((p) => ({ ...p, lockout_sec: Number(e.target.value) || 0 }))}
            />
          </div>
          <button type="submit" className="btn primary" disabled={savingPolicy}>
            Сохранить
          </button>
        </form>
      </section>

      <section className="card">
        <div className="card-head">
          <h3 className="card-title">
            Кто сейчас в системе
            <span className="card-title-count">{sessions.length}</span>
          </h3>
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            Обновить
          </button>
        </div>
        {!sessions.length ? (
          <EmptyState
            compact
            title="Никто не вошёл"
            description="Список появится, когда кто-то войдёт в систему."
          />
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Пользователь</th>
                  <th scope="col">Адрес</th>
                  <th scope="col">Устройство</th>
                  <th scope="col">Вошёл</th>
                  <th scope="col">Был активен</th>
                  <th scope="col" className="actions">
                    Действия
                  </th>
                </tr>
              </thead>
              <tbody>
                {sessions.map((s) => (
                  <tr key={s.id}>
                    <td>
                      <div className="cell-stack">
                        <span className="cell-primary">{s.username}</span>
                        <span className="cell-secondary">{roleLabel(s.role)}</span>
                      </div>
                    </td>
                    <td className="mono">{s.ip || '—'}</td>
                    <td className="ua" title={s.user_agent || ''}>
                      {truncUA(s.user_agent)}
                    </td>
                    <td>{fmtDate(s.created_at)}</td>
                    <td>{fmtDate(s.last_seen_at)}</td>
                    <td className="actions">
                      <div className="row-actions">
                        <button
                          type="button"
                          className="btn"
                          disabled={busyId === s.id}
                          onClick={() => void onRevoke(s)}
                        >
                          Завершить
                        </button>
                        <button
                          type="button"
                          className="btn"
                          disabled={busyId === `user:${s.username}`}
                          onClick={() => void onRevokeUser(s.username)}
                        >
                          Завершить все
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="card">
        <h3 className="card-title">Последние входы</h3>
        {!recent.length ? (
          <EmptyState
            compact
            title="Пока пусто"
            description="Здесь будут удачные и неудачные попытки входа."
          />
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Когда</th>
                  <th scope="col">Кто</th>
                  <th scope="col">Что сделал</th>
                  <th scope="col">Итог</th>
                  <th scope="col">Адрес</th>
                  <th scope="col">Подробности</th>
                </tr>
              </thead>
              <tbody>
                {recent.map((item, idx) => (
                  <tr key={`${item.timestamp || 't'}-${item.action || 'a'}-${idx}`}>
                    <td>{fmtDate(item.timestamp)}</td>
                    <td>{item.actor || '—'}</td>
                    <td title={item.action}>{auditActionLabel(item.action)}</td>
                    <td>
                      <span className={`result-chip ${resultClass(item.result)}`}>
                        {resultLabel(item.result)}
                      </span>
                    </td>
                    <td className="mono">{item.ip || '—'}</td>
                    <td className="details">{formatAuditRow(item)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="card">
        <h3 className="card-title">Доступ и сертификат</h3>
        <div className="kv-list posture-list">
          <div className="kv-row">
            <span className="k">
              <Link to="/users">Пользователи</Link>
            </span>
            <span className="v">
              {fmtNumber(posture?.users?.total)}
              <span className="v-meta"> ({roleLine(posture?.users?.by_role)})</span>
              {posture?.users?.must_reset_password ? (
                <span className="v-meta">
                  {' '}
                  · нужно сменить пароль: {posture.users.must_reset_password}
                </span>
              ) : null}
            </span>
          </div>
          <div className="kv-row">
            <span className="k">
              <Link to="/api-tokens">Ключи API</Link>
            </span>
            <span className="v">
              {fmtNumber(posture?.tokens?.total)}
              {posture?.tokens?.expiring_soon ? (
                <span className="v-meta">
                  {' '}
                  · скоро истекают (≤30 дн.): {posture.tokens.expiring_soon}
                </span>
              ) : null}
            </span>
          </div>
          <div className="kv-row">
            <span className="k">
              <Link to="/tls">Шифрование HTTPS</Link>
            </span>
            <span className="v">{tlsValue}</span>
          </div>
        </div>
      </section>

      <section className="card">
        <h3 className="card-title">Отклонённые запросы</h3>
        <p className="hint">Сколько раз система остановила подозрительные или слишком частые обращения.</p>
        <div className="table-wrap">
          <table className="auth-fails-table perimeter-table">
            <thead>
              <tr>
                <th scope="col">Причина</th>
                <th scope="col" className="num">
                  За сутки
                </th>
                <th scope="col" className="num">
                  За неделю
                </th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  <div className="cell-stack">
                    <span className="cell-primary">Поддельный запрос</span>
                    <span className="cell-secondary">CSRF</span>
                  </div>
                </td>
                <td className="num">{fmtNumber(threat24?.csrf_reject)}</td>
                <td className="num">{fmtNumber(threat7?.csrf_reject)}</td>
              </tr>
              <tr>
                <td>
                  <div className="cell-stack">
                    <span className="cell-primary">Доступ через прокси</span>
                    <span className="cell-secondary">proxy-gate</span>
                  </div>
                </td>
                <td className="num">{fmtNumber(threat24?.proxy_gate)}</td>
                <td className="num">{fmtNumber(threat7?.proxy_gate)}</td>
              </tr>
              <tr>
                <td>
                  <div className="cell-stack">
                    <span className="cell-primary">Слишком частые запросы</span>
                    <span className="cell-secondary">лимит API</span>
                  </div>
                </td>
                <td className="num">{fmtNumber(threat24?.api_rate_limit)}</td>
                <td className="num">{fmtNumber(threat7?.api_rate_limit)}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
