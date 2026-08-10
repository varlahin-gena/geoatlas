import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from './AuthContext';

export function RequireAuth({
  children,
  admin = false,
  allowMustReset = false,
}: {
  children: React.ReactNode;
  admin?: boolean;
  allowMustReset?: boolean;
}) {
  const { user, loading, isAdmin } = useAuth();
  const location = useLocation();
  const next = location.pathname + location.search;

  if (loading) {
    return <div className="page-loading">Загрузка…</div>;
  }

  if (!user) {
    return <Navigate to={`/login?next=${encodeURIComponent(next)}`} replace />;
  }

  if (user.must_reset_password && !user.authDisabled && !allowMustReset) {
    return (
      <Navigate
        to={`/change-password?next=${encodeURIComponent(next)}`}
        replace
      />
    );
  }

  if (admin && !isAdmin) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}
