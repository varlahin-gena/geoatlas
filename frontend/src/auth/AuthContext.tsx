import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { SESSION_EXPIRED_EVENT } from '@/api/client';
import * as authApi from '@/api/auth';
import type { AuthUser } from '@/api/types';
import { safeNext } from '@/lib/format';
import { getTheme, toggleTheme, type Theme } from './theme';
import { deriveIsAdmin, deriveReputationEnabled, deriveUiAuthEnabled } from './roles';

interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  isAdmin: boolean;
  reputationEnabled: boolean;
  uiAuthEnabled: boolean;
  theme: Theme;
  refresh: () => Promise<AuthUser | null>;
  login: (username: string, password: string) => Promise<AuthUser>;
  logout: () => Promise<void>;
  /** Завершить все сессии (все устройства) и перейти на /login. */
  logoutAll: () => Promise<void>;
  toggleTheme: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [theme, setThemeState] = useState<Theme>(getTheme());
  const navigate = useNavigate();
  const location = useLocation();

  const refresh = useCallback(async () => {
    try {
      const me = await authApi.fetchMe();
      // 401 → null; keep prior user on transient /me failures (e.g. after password change).
      setUser(me);
      return me;
    } catch {
      return null;
    }
  }, []);

  useEffect(() => {
    const onExpired = () => {
      setUser(null);
      if (location.pathname === '/login') return;
      const next = safeNext(location.pathname + location.search);
      navigate(`/login?next=${encodeURIComponent(next)}`, { replace: true });
    };
    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired);
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired);
  }, [location.pathname, location.search, navigate]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const me = await authApi.fetchMe();
        if (!cancelled) setUser(me);
      } catch {
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const u = await authApi.login(username, password);
    setUser(u);
    return u;
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch {
      /* ignore */
    }
    setUser(null);
    navigate('/login');
  }, [navigate]);

  const logoutAll = useCallback(async () => {
    try {
      await authApi.logoutAll();
    } catch {
      /* ignore — всё равно сбрасываем локальную сессию */
    }
    setUser(null);
    navigate('/login');
  }, [navigate]);

  const doToggleTheme = useCallback(() => {
    setThemeState(toggleTheme());
  }, []);

  const value = useMemo<AuthContextValue>(() => {
    return {
      user,
      loading,
      isAdmin: deriveIsAdmin(user),
      reputationEnabled: deriveReputationEnabled(user),
      uiAuthEnabled: deriveUiAuthEnabled(user),
      theme,
      refresh,
      login,
      logout,
      logoutAll,
      toggleTheme: doToggleTheme,
    };
  }, [user, loading, theme, refresh, login, logout, logoutAll, doToggleTheme]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
