import { FormEvent, useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';
import { ApiError } from '@/api/client';
import { safeNext } from '@/lib/format';
import '@/styles/auth-form.css';

export default function LoginPage() {
  const { user, loading, login } = useAuth();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const next = safeNext(params.get('next'), '/');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    document.body.classList.add('page-auth');
    document.title = 'ГеоАтлас — Вход';
    return () => document.body.classList.remove('page-auth');
  }, []);

  useEffect(() => {
    if (loading || !user) return;
    if (user.must_reset_password) {
      navigate(`/change-password?next=${encodeURIComponent(next)}`, { replace: true });
      return;
    }
    navigate(next, { replace: true });
  }, [user, loading, navigate, next]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      const u = await login(username.trim(), password);
      if (u.must_reset_password) {
        navigate(`/change-password?next=${encodeURIComponent(next)}`, { replace: true });
        return;
      }
      navigate(next, { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.message === 'invalid credentials') {
          setError('Неверный логин или пароль');
        } else if (/too many attempts/i.test(err.message)) {
          setError('Слишком много попыток — подождите и попробуйте снова');
        } else {
          setError(err.message || 'Ошибка входа');
        }
      } else {
        setError('Сервер недоступен');
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <a className="skip-link" href="#loginForm">
        К форме входа
      </a>
      <form className="auth-card" id="loginForm" autoComplete="on" onSubmit={onSubmit}>
        <div className="auth-brand">
          <img className="logo" src="/logo.png" alt="" width={40} height={40} />
          <div>
            <h1>ГеоАтлас</h1>
            <p>Локальный вход</p>
          </div>
        </div>
        <div className={`auth-error${error ? ' show' : ''}`} id="error" role="alert">
          {error}
        </div>
        <label htmlFor="username">Логин</label>
        <input
          id="username"
          name="username"
          type="text"
          autoComplete="username"
          required
          autoFocus
          value={username}
          onChange={(e) => setUsername(e.target.value)}
        />
        <label htmlFor="password">Пароль</label>
        <input
          id="password"
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        <button type="submit" id="submitBtn" disabled={busy}>
          Войти
        </button>
      </form>
    </>
  );
}
