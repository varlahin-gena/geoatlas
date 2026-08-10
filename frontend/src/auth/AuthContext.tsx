import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { useNavigate } from 'react-router-dom';
import * as authApi from '@/api/auth';
import type { AuthUser } from '@/api/types';
import { ROLE_ADMIN } from '@/api/types';
import { applyTheme, getTheme, toggleTheme, type Theme } from './theme';

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
  setTheme: (t: Theme) => void;
  toggleTheme: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [theme, setThemeState] = useState<Theme>(getTheme());
  const navigate = useNavigate();

  const refresh = useCallback(async () => {
    try {
      const me = await authApi.fetchMe();
      setUser(me);
      return me;
    } catch {
      setUser(null);
      return null;
    }
  }, []);

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

  const setTheme = useCallback((t: Theme) => {
    setThemeState(applyTheme(t));
  }, []);

  const doToggleTheme = useCallback(() => {
    setThemeState(toggleTheme());
  }, []);

  const value = useMemo<AuthContextValue>(() => {
    const isAdmin = !user || !!user.authDisabled || user.role === ROLE_ADMIN;
    return {
      user,
      loading,
      isAdmin,
      reputationEnabled: !user || user.reputationEnabled !== false,
      uiAuthEnabled: !user || !user.authDisabled,
      theme,
      refresh,
      login,
      logout,
      setTheme,
      toggleTheme: doToggleTheme,
    };
  }, [user, loading, theme, refresh, login, logout, setTheme, doToggleTheme]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
