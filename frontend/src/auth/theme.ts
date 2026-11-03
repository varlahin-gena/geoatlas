const THEME_KEY = 'nm.theme';

/** Appearance applied to `data-theme` (map, charts, CSS). */
export type Theme = 'light' | 'dark';

/** @deprecated Use `Theme` — preference is always an explicit light/dark. */
export type ThemePreference = Theme;

function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return true;
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

/** Migrate legacy `system` once; empty storage → dark (map-first default). */
export function getThemePreference(): Theme {
  try {
    const raw = localStorage.getItem(THEME_KEY);
    if (raw === 'light' || raw === 'dark') return raw;
    if (raw === 'system') {
      const migrated: Theme = systemPrefersDark() ? 'dark' : 'light';
      try {
        localStorage.setItem(THEME_KEY, migrated);
      } catch {
        /* ignore */
      }
      return migrated;
    }
  } catch {
    /* ignore */
  }
  return 'dark';
}

export function resolveTheme(pref: Theme = getThemePreference()): Theme {
  return pref === 'light' ? 'light' : 'dark';
}

/** Theme currently shown (and on `<html data-theme>`). */
export function getTheme(): Theme {
  return resolveTheme(getThemePreference());
}

export function themeLabel(pref: Theme): string {
  return pref === 'light' ? 'Светлая' : 'Тёмная';
}

function applyResolved(theme: Theme): Theme {
  const t = theme === 'light' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', t);
  document.dispatchEvent(new CustomEvent('ga-theme-change', { detail: { theme: t } }));
  return t;
}

/** Persist preference locally (FOUC cache) and apply to `data-theme`. */
export function setThemePreference(pref: Theme): Theme {
  try {
    localStorage.setItem(THEME_KEY, pref);
  } catch {
    /* ignore */
  }
  return applyResolved(resolveTheme(pref));
}

/** Cycle: light ↔ dark. */
export function cycleThemePreference(): { preference: Theme; theme: Theme } {
  const next: Theme = getThemePreference() === 'light' ? 'dark' : 'light';
  const theme = setThemePreference(next);
  return { preference: next, theme };
}

/** Toggles light/dark; returns the new theme (map/charts). */
export function toggleTheme(): Theme {
  return cycleThemePreference().theme;
}

// Apply early to avoid FOUC when module loads.
applyResolved(resolveTheme(getThemePreference()));
