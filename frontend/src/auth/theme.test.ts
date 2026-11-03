import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

describe('theme preference', () => {
  const store = new Map<string, string>();

  function stubScheme(dark: boolean) {
    vi.stubGlobal(
      'matchMedia',
      vi.fn((query: string) => ({
        matches: query.includes('prefers-color-scheme: dark') ? dark : false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
        onchange: null,
      })),
    );
  }

  function stubStorage() {
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => {
        store.set(k, v);
      },
      removeItem: (k: string) => {
        store.delete(k);
      },
    });
  }

  beforeEach(() => {
    store.clear();
    vi.resetModules();
    stubStorage();
    stubScheme(true);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('defaults to dark when storage is empty', async () => {
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('dark');
    expect(theme.getTheme()).toBe('dark');
    expect(store.has('nm.theme')).toBe(false);
  });

  it('migrates legacy system to OS preference and writes storage', async () => {
    store.set('nm.theme', 'system');
    stubScheme(false);
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('light');
    expect(store.get('nm.theme')).toBe('light');
  });

  it('migrates legacy system to dark when OS prefers dark', async () => {
    store.set('nm.theme', 'system');
    stubScheme(true);
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('dark');
    expect(store.get('nm.theme')).toBe('dark');
  });

  it('keeps explicit light/dark from storage', async () => {
    store.set('nm.theme', 'light');
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('light');
    expect(theme.getTheme()).toBe('light');
  });

  it('cycles light ↔ dark', async () => {
    store.set('nm.theme', 'dark');
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('dark');

    let step = theme.cycleThemePreference();
    expect(step.preference).toBe('light');
    expect(step.theme).toBe('light');
    expect(store.get('nm.theme')).toBe('light');

    step = theme.cycleThemePreference();
    expect(step.preference).toBe('dark');
    expect(step.theme).toBe('dark');
  });

  it('themeLabel covers light and dark', async () => {
    const theme = await import('./theme');
    expect(theme.themeLabel('light')).toBe('Светлая');
    expect(theme.themeLabel('dark')).toBe('Тёмная');
  });

  it('applies server theme from /me over local FOUC cache', async () => {
    store.set('nm.theme', 'dark');
    document.documentElement.setAttribute('data-theme', 'dark');
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('dark');

    const applied = theme.setThemePreference('light');
    expect(applied).toBe('light');
    expect(theme.getThemePreference()).toBe('light');
    expect(store.get('nm.theme')).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });
});

