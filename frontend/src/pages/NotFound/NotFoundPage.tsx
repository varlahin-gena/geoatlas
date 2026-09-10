import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';
import '@/styles/auth-form.css';

/** Soft branded 404 — Wave 3. */
export default function NotFoundPage() {
  const { user, loading } = useAuth();

  useEffect(() => {
    document.title = 'ГеоАтлас — 404';
    document.body.classList.add('page-auth');
    return () => document.body.classList.remove('page-auth');
  }, []);

  const loggedIn = Boolean(user && !user.authDisabled);

  return (
    <div className="not-found">
      <div className="not-found-card">
        <div className="auth-brand">
          <img className="logo" src="/logo.png" alt="" width={40} height={40} />
          <div>
            <h1>ГеоАтлас</h1>
            <p>Страница не найдена</p>
          </div>
        </div>
        <p className="not-found-text">
          Проверьте адрес или вернитесь на карту. Если вы ожидали другой раздел — откройте его из
          меню слева после входа.
        </p>
        <div className="not-found-actions">
          {loading ? null : loggedIn ? (
            <Link className="btn primary" to="/">
              На карту
            </Link>
          ) : (
            <Link className="btn primary" to="/login">
              Войти
            </Link>
          )}
          {loggedIn ? (
            <Link className="btn" to="/anomalies">
              Аномалии
            </Link>
          ) : null}
        </div>
      </div>
    </div>
  );
}
