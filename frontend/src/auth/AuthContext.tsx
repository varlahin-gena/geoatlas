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
import { getTheme, getThemePreference, toggleTheme, type Theme, type ThemePreference } from './theme';
import { deriveIsAdmin, deriveReputationEnabled, deriveUiAuthEnabled } from './roles';

interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  isAdmin: boolean;
  reputationEnabled: boolean;
  uiAuthEnabled: boolean;
  /** Resolved light/dark for map, charts, CSS. */
  theme: Theme;
  /** Stored preference (may be system). */
  themePreference: ThemePreference;
  refresh: () => Promise<AuthUser | null>;
  login: (username: string, password: string) => Promise<AuthUser>;
  logout: () => Promise<void>;
  /** Завершить все сессии (все устройства) и перейти на /login. */
  logoutAll: (currentPassword: string) => Promise<void>;
  /** Cycles system → light → dark → system. */
  toggleTheme: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [theme, setThemeState] = useState<Theme>(getTheme());
  const [themePreference, setThemePreferenceState] = useState<ThemePreference>(getThemePreference());
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    const onTheme = (e: Event) => {
      const detail = (e as CustomEvent<{ theme?: string }>).detail;
      if (detail?.theme === 'light' || detail?.theme === 'dark') {
        setThemeState(detail.theme);
      }
      setThemePreferenceState(getThemePreference());
    };
    document.addEventListener('ga-theme-change', onTheme);
    return () => document.removeEventListener('ga-theme-change', onTheme);
  }, []);

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

  const logoutAll = useCallback(async (currentPassword: string) => {
    await authApi.logoutAll(currentPassword);
    setUser(null);
    navigate('/login');
  }, [navigate]);

  const doToggleTheme = useCallback(() => {
    setThemeState(toggleTheme());
    setThemePreferenceState(getThemePreference());
  }, []);

  const value = useMemo<AuthContextValue>(() => {
    return {
      user,
      loading,
      isAdmin: deriveIsAdmin(user),
      reputationEnabled: deriveReputationEnabled(user),
      uiAuthEnabled: deriveUiAuthEnabled(user),
      theme,
      themePreference,
      refresh,
      login,
      logout,
      logoutAll,
      toggleTheme: doToggleTheme,
    };
  }, [user, loading, theme, themePreference, refresh, login, logout, logoutAll, doToggleTheme]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
