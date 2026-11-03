import { FormEvent, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { changePassword } from '@/api/auth';
import { ApiError } from '@/api/client';
import { useAuth } from '@/auth/AuthContext';
import { roleLabelRu } from '@/auth/roles';
import { themeLabel, type Theme } from '@/auth/theme';
import { AdminLayout } from '@/components/AdminLayout';
import { AuthPasswordField } from '@/components/AuthPasswordField';
import { useToast } from '@/components/Toast';
import {
  MAP_GROUP_BY_KEYS,
  MAP_GROUP_BY_LABELS,
  type MapGroupBy,
} from '@/lib/mapGrouping';
import { MIN_PASSWORD_LEN, validatePasswordClient } from '@/lib/passwordPolicy';
import '@/styles/auth-form.css';
import './profile.css';

export default function ProfilePage() {
  const { user, isAdmin, themePreference, mapGroupBy, setTheme, setMapGroupBy, refresh } =
    useAuth();
  const { toast } = useToast();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [newTouched, setNewTouched] = useState(false);
  const [confirmTouched, setConfirmTouched] = useState(false);
  const [pwdBusy, setPwdBusy] = useState(false);
  const [prefsBusy, setPrefsBusy] = useState(false);

  useEffect(() => {
    document.title = 'ГеоАтлас — Профиль';
  }, []);

  const newPasswordHint = useMemo(() => {
    if (!newTouched || !newPassword) return null;
    return validatePasswordClient(newPassword, user?.username);
  }, [newPassword, newTouched, user?.username]);

  const confirmHint = useMemo(() => {
    if (!confirmTouched || !confirm) return null;
    if (newPassword !== confirm) return 'Пароли не совпадают';
    return null;
  }, [confirm, confirmTouched, newPassword]);

  async function onSavePassword(e: FormEvent) {
    e.preventDefault();
    setNewTouched(true);
    setConfirmTouched(true);
    if (newPassword !== confirm) {
      toast('Пароли не совпадают', 'error');
      return;
    }
    const policyErr = validatePasswordClient(newPassword, user?.username);
    if (policyErr) {
      toast(policyErr, 'error');
      return;
    }
    setPwdBusy(true);
    try {
      await changePassword(currentPassword, newPassword);
      await refresh();
      setCurrentPassword('');
      setNewPassword('');
      setConfirm('');
      setNewTouched(false);
      setConfirmTouched(false);
      toast('Пароль изменён', 'success');
    } catch (err) {
      if (err instanceof ApiError && err.message === 'current password is incorrect') {
        toast('Неверный текущий пароль', 'error');
      } else {
        toast(err instanceof Error ? err.message : 'Не удалось сменить пароль', 'error');
      }
    } finally {
      setPwdBusy(false);
    }
  }

  async function onThemeChange(next: Theme) {
    if (next === themePreference) return;
    setPrefsBusy(true);
    try {
      await setTheme(next);
      toast('Тема сохранена', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить тему', 'error');
    } finally {
      setPrefsBusy(false);
    }
  }

  async function onGroupChange(next: MapGroupBy) {
    if (next === mapGroupBy) return;
    setPrefsBusy(true);
    try {
      await setMapGroupBy(next);
      toast('Группировка сохранена', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить группировку', 'error');
    } finally {
      setPrefsBusy(false);
    }
  }

  const fio = (user?.full_name || '').trim();

  return (
    <AdminLayout title="Профиль" className="ga-profile">
      <div className="profile-page">
        <section className="profile-card">
          <h2 className="profile-card-title">Учётная запись</h2>
          <dl className="profile-kv">
            <div>
              <dt>Логин</dt>
              <dd className="mono">{user?.username || '—'}</dd>
            </div>
            <div>
              <dt>ФИО</dt>
              <dd>{fio || '—'}</dd>
            </div>
            <div>
              <dt>Роль</dt>
              <dd>{user?.role ? roleLabelRu(user.role) : '—'}</dd>
            </div>
          </dl>
          <p className="hint profile-card-foot">
            {isAdmin ? (
              <>
                ФИО меняется в{' '}
                <Link to="/users">Пользователях</Link>.
              </>
            ) : (
              'ФИО меняет администратор в разделе «Пользователи».'
            )}
          </p>
        </section>

        <section className="profile-card">
          <h2 className="profile-card-title">Пароль</h2>
          <p className="hint">
            Смена пароля завершит остальные сессии. Минимум {MIN_PASSWORD_LEN} символов, буква и
            цифра.
          </p>
          <form className="profile-form" onSubmit={onSavePassword}>
            <AuthPasswordField
              id="profileCurrent"
              label="Текущий пароль"
              name="current-password"
              autoComplete="current-password"
              required
              value={currentPassword}
              onChange={setCurrentPassword}
            />
            <AuthPasswordField
              id="profileNew"
              label="Новый пароль"
              name="new-password"
              autoComplete="new-password"
              required
              value={newPassword}
              onChange={(v) => {
                setNewPassword(v);
                if (!newTouched) setNewTouched(true);
              }}
              onBlur={() => setNewTouched(true)}
            />
            {newPasswordHint ? (
              <p className="profile-field-hint" role="status">
                {newPasswordHint}
              </p>
            ) : null}
            <AuthPasswordField
              id="profileConfirm"
              label="Повторите новый пароль"
              name="confirm-password"
              autoComplete="new-password"
              required
              value={confirm}
              onChange={(v) => {
                setConfirm(v);
                if (!confirmTouched) setConfirmTouched(true);
              }}
              onBlur={() => setConfirmTouched(true)}
            />
            {confirmHint ? (
              <p className="profile-field-hint" role="status">
                {confirmHint}
              </p>
            ) : null}
            <button
              type="submit"
              className="btn primary"
              disabled={
                pwdBusy ||
                !currentPassword ||
                !newPassword ||
                !confirm ||
                Boolean(newPasswordHint) ||
                Boolean(confirmHint)
              }
              aria-busy={pwdBusy}
            >
              {pwdBusy ? 'Сохранение…' : 'Сменить пароль'}
            </button>
          </form>
        </section>

        <section className="profile-card">
          <h2 className="profile-card-title">Оформление</h2>
          <p className="hint">
            Тема интерфейса для этого пользователя на всех устройствах. Перекрывает тему
            браузера/ОС.
          </p>
          <div className="profile-choice-row" role="radiogroup" aria-label="Тема">
            {(['light', 'dark'] as const).map((t) => (
              <label key={t} className={`profile-choice${themePreference === t ? ' is-active' : ''}`}>
                <input
                  type="radio"
                  name="profile-theme"
                  value={t}
                  checked={themePreference === t}
                  disabled={prefsBusy}
                  onChange={() => void onThemeChange(t)}
                />
                <span>{themeLabel(t)}</span>
              </label>
            ))}
          </div>
        </section>

        <section className="profile-card">
          <h2 className="profile-card-title">Карта</h2>
          <p className="hint">
            Группировка дуг при открытии карты, если на карте ещё не выбран другой режим.
          </p>
          <label htmlFor="profileGroupBy">Группировка дуг</label>
          <select
            id="profileGroupBy"
            className="input"
            value={mapGroupBy}
            disabled={prefsBusy}
            onChange={(e) => void onGroupChange(e.target.value as MapGroupBy)}
          >
            {MAP_GROUP_BY_KEYS.map((k) => (
              <option key={k} value={k}>
                {MAP_GROUP_BY_LABELS[k]}
              </option>
            ))}
          </select>
        </section>
      </div>
    </AdminLayout>
  );
}
