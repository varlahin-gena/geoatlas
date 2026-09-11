const THEME_KEY = 'nm.theme';

/** Resolved appearance applied to `data-theme` (map, charts, CSS). */
export type Theme = 'light' | 'dark';

/** Stored preference; `system` follows `prefers-color-scheme`. */
export type ThemePreference = Theme | 'system';

function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return true;
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function getThemePreference(): ThemePreference {
  try {
    const raw = localStorage.getItem(THEME_KEY);
    if (raw === 'light' || raw === 'dark' || raw === 'system') return raw;
    // Missing key → follow OS (new default). Explicit light/dark from older builds stay.
    return 'system';
  } catch {
    return 'system';
  }
}

export function resolveTheme(pref: ThemePreference = getThemePreference()): Theme {
  if (pref === 'light' || pref === 'dark') return pref;
  return systemPrefersDark() ? 'dark' : 'light';
}

/** Resolved theme currently shown (and on `<html data-theme>`). */
export function getTheme(): Theme {
  return resolveTheme(getThemePreference());
}

export function themeLabel(pref: ThemePreference): string {
  if (pref === 'system') return 'Системная';
  return pref === 'light' ? 'Светлая' : 'Тёмная';
}

function applyResolved(theme: Theme): Theme {
  const t = theme === 'light' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', t);
  document.dispatchEvent(new CustomEvent('ga-theme-change', { detail: { theme: t } }));
  return t;
}

let mediaCleanup: (() => void) | null = null;

function syncSystemListener(): void {
  mediaCleanup?.();
  mediaCleanup = null;
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
  if (getThemePreference() !== 'system') return;
  const mql = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    if (getThemePreference() === 'system') {
      applyResolved(resolveTheme('system'));
    }
  };
  mql.addEventListener('change', onChange);
  mediaCleanup = () => mql.removeEventListener('change', onChange);
}

function setThemePreference(pref: ThemePreference): Theme {
  try {
    localStorage.setItem(THEME_KEY, pref);
  } catch {
    /* ignore */
  }
  const resolved = applyResolved(resolveTheme(pref));
  syncSystemListener();
  return resolved;
}

/** Cycle: system → light → dark → system. */
export function cycleThemePreference(): { preference: ThemePreference; theme: Theme } {
  const cur = getThemePreference();
  const next: ThemePreference =
    cur === 'system' ? 'light' : cur === 'light' ? 'dark' : 'system';
  const theme = setThemePreference(next);
  return { preference: next, theme };
}

/** Cycles preference; returns the new resolved theme (map/charts). */
export function toggleTheme(): Theme {
  return cycleThemePreference().theme;
}

// Apply early to avoid FOUC when module loads (does not write storage if unset).
applyResolved(resolveTheme(getThemePreference()));
syncSystemListener();
