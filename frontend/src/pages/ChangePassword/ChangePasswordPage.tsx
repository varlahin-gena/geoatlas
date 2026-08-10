import { FormEvent, useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { changePassword } from '@/api/auth';
import { ApiError } from '@/api/client';
import { useAuth } from '@/auth/AuthContext';
import { safeNext } from '@/lib/format';
import '@/styles/auth-form.css';

export default function ChangePasswordPage() {
  const { user, loading, refresh } = useAuth();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const next = safeNext(params.get('next'), '/');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    document.body.classList.add('page-auth');
    document.title = 'ГеоАтлас — Смена пароля';
    return () => document.body.classList.remove('page-auth');
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!user) {
      navigate(`/login?next=${encodeURIComponent('/change-password')}`, { replace: true });
    }
  }, [user, loading, navigate]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    if (newPassword.length < 8) {
      setError('Пароль не менее 8 символов');
      return;
    }
    if (newPassword !== confirm) {
      setError('Пароли не совпадают');
      return;
    }
    setBusy(true);
    try {
      await changePassword(currentPassword, newPassword);
      await refresh();
      navigate(next, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сменить пароль');
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <a className="skip-link" href="#changeForm">
        К форме смены пароля
      </a>
      <form className="auth-card" id="changeForm" onSubmit={onSubmit}>
        <div className="auth-brand">
          <img className="logo" src="/logo.png" alt="" width={40} height={40} />
          <div>
            <h1>Смена пароля</h1>
            <p>{user?.must_reset_password ? 'Требуется сменить пароль' : 'Новый пароль'}</p>
          </div>
        </div>
        <div className={`auth-error${error ? ' show' : ''}`} role="alert">
          {error}
        </div>
        <label htmlFor="current">Текущий пароль</label>
        <input
          id="current"
          type="password"
          autoComplete="current-password"
          required
          value={currentPassword}
          onChange={(e) => setCurrentPassword(e.target.value)}
        />
        <label htmlFor="new">Новый пароль</label>
        <input
          id="new"
          type="password"
          autoComplete="new-password"
          required
          minLength={8}
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
        />
        <label htmlFor="confirm">Повтор</label>
        <input
          id="confirm"
          type="password"
          autoComplete="new-password"
          required
          minLength={8}
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
        />
        <button type="submit" disabled={busy}>
          Сохранить
        </button>
      </form>
    </>
  );
}
