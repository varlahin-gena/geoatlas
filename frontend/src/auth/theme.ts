const THEME_KEY = 'nm.theme';

export type Theme = 'light' | 'dark';

export function getTheme(): Theme {
  try {
    return localStorage.getItem(THEME_KEY) === 'light' ? 'light' : 'dark';
  } catch {
    return 'dark';
  }
}

export function themeLabel(theme: Theme): string {
  return theme === 'light' ? 'Светлая' : 'Тёмная';
}

function applyTheme(theme: Theme): Theme {
  const t = theme === 'light' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', t);
  try {
    localStorage.setItem(THEME_KEY, t);
  } catch {
    /* ignore */
  }
  document.dispatchEvent(new CustomEvent('nm-theme-change', { detail: { theme: t } }));
  return t;
}

export function toggleTheme(): Theme {
  return applyTheme(getTheme() === 'light' ? 'dark' : 'light');
}

// Apply early to avoid FOUC when module loads.
applyTheme(getTheme());
