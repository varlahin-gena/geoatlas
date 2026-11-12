import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { SESSION_EXPIRED_EVENT } from '@/api/client';
import * as authApi from '@/api/auth';
import type { AuthUser } from '@/api/types';
import { safeNext } from '@/lib/format';
import {
  getTheme,
  getThemePreference,
  setThemePreference,
  toggleTheme,
  type Theme,
  type ThemePreference,
} from './theme';
import { deriveIsAdmin, deriveReputationEnabled, deriveUiAuthEnabled } from './roles';
import { normalizeMapGroupBy, type MapGroupBy } from '@/lib/mapGrouping';
import { promiseTimeout } from '@/lib/promiseTimeout';

/** Cap auth bootstrap so RequireAuth cannot show PageSkeleton forever. */
const AUTH_BOOT_TIMEOUT_MS = 15_000;

interface AuthContextValue {
  user: AuthUser | null;
  loading: boolean;
  isAdmin: boolean;
  reputationEnabled: boolean;
  uiAuthEnabled: boolean;
  /** Resolved light/dark for map, charts, CSS. */
  theme: Theme;
  /** Stored preference (light | dark). */
  themePreference: ThemePreference;
  /** Effective default map grouping from server prefs. */
  mapGroupBy: MapGroupBy;
  refresh: () => Promise<AuthUser | null>;
  login: (username: string, password: string) => Promise<AuthUser>;
  logout: () => Promise<void>;
  /** Завершить все сессии (все устройства) и перейти на /login. */
  logoutAll: (currentPassword: string) => Promise<void>;
  /** Toggles light ↔ dark and persists to server. */
  toggleTheme: () => void;
  /** Apply theme preference locally + PATCH server. */
  setTheme: (pref: Theme) => Promise<void>;
  /** Persist map grouping default on server. */
  setMapGroupBy: (group: MapGroupBy) => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function themeFromUser(u: AuthUser | null): Theme | null {
  if (!u) return null;
  return u.theme === 'light' || u.theme === 'dark' ? u.theme : null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [theme, setThemeState] = useState<Theme>(getTheme());
  const [themePreference, setThemePreferenceState] = useState<ThemePreference>(getThemePreference());
  const themeMigratedRef = useRef(false);
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

  const applyUserTheme = useCallback((u: AuthUser | null) => {
    if (!u || u.authDisabled) return;
    const serverTheme = themeFromUser(u);
    if (serverTheme) {
      const applied = setThemePreference(serverTheme);
      setThemeState(applied);
      setThemePreferenceState(serverTheme);
      return;
    }
    // One-time migrate local nm.theme → server when unset.
    if (!themeMigratedRef.current) {
      const local = getThemePreference();
      if (local === 'light' || local === 'dark') {
        themeMigratedRef.current = true;
        setThemePreference(local);
        setThemeState(local);
        setThemePreferenceState(local);
        void authApi.updatePreferences({ theme: local }).then(
          (next) => setUser(next),
          () => {
            /* keep local; server may be unavailable */
          },
        );
        return;
      }
    }
    const applied = setThemePreference('dark');
    setThemeState(applied);
    setThemePreferenceState('dark');
  }, []);

  const refresh = useCallback(async () => {
    try {
      const me = await authApi.fetchMe();
      // 401 → null; keep prior user on transient /me failures (e.g. after password change).
      setUser(me);
      if (me) applyUserTheme(me);
      return me;
    } catch {
      return null;
    }
  }, [applyUserTheme]);

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
        const me = await promiseTimeout(
          authApi.fetchMe(),
          AUTH_BOOT_TIMEOUT_MS,
          'Таймаут проверки сессии',
        );
        if (!cancelled) {
          setUser(me);
          if (me) applyUserTheme(me);
        }
      } catch {
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [applyUserTheme]);

  const login = useCallback(
    async (username: string, password: string) => {
      const u = await authApi.login(username, password);
      setUser(u);
      applyUserTheme(u);
      return u;
    },
    [applyUserTheme],
  );

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch {
      /* ignore */
    }
    setUser(null);
    navigate('/login');
  }, [navigate]);

  const logoutAll = useCallback(
    async (currentPassword: string) => {
      await authApi.logoutAll(currentPassword);
      setUser(null);
      navigate('/login');
    },
    [navigate],
  );

  const doToggleTheme = useCallback(() => {
    const prev = getThemePreference();
    const nextTheme = toggleTheme();
    const nextPref = getThemePreference();
    setThemeState(nextTheme);
    setThemePreferenceState(nextPref);
    void authApi.updatePreferences({ theme: nextPref }).then(
      (u) => setUser(u),
      () => {
        setThemePreference(prev);
        setThemeState(prev);
        setThemePreferenceState(prev);
      },
    );
  }, []);

  const setTheme = useCallback(async (pref: Theme) => {
    const prev = getThemePreference();
    setThemePreference(pref);
    setThemeState(pref);
    setThemePreferenceState(pref);
    try {
      const u = await authApi.updatePreferences({ theme: pref });
      setUser(u);
    } catch (err) {
      setThemePreference(prev);
      setThemeState(prev);
      setThemePreferenceState(prev);
      throw err;
    }
  }, []);

  const setMapGroupBy = useCallback(async (group: MapGroupBy) => {
    const u = await authApi.updatePreferences({ map_group_by: group });
    setUser(u);
  }, []);

  const mapGroupBy = useMemo(
    () => normalizeMapGroupBy(user?.map_group_by),
    [user?.map_group_by],
  );

  const value = useMemo<AuthContextValue>(() => {
    return {
      user,
      loading,
      isAdmin: deriveIsAdmin(user),
      reputationEnabled: deriveReputationEnabled(user),
      uiAuthEnabled: deriveUiAuthEnabled(user),
      theme,
      themePreference,
      mapGroupBy,
      refresh,
      login,
      logout,
      logoutAll,
      toggleTheme: doToggleTheme,
      setTheme,
      setMapGroupBy,
    };
  }, [
    user,
    loading,
    theme,
    themePreference,
    mapGroupBy,
    refresh,
    login,
    logout,
    logoutAll,
    doToggleTheme,
    setTheme,
    setMapGroupBy,
  ]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
