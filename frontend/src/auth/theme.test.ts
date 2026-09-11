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

  it('defaults to system when storage is empty and resolves dark OS', async () => {
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('system');
    expect(theme.resolveTheme('system')).toBe('dark');
    expect(theme.getTheme()).toBe('dark');
    expect(store.has('nm.theme')).toBe(false);
  });

  it('resolves system to light when OS prefers light', async () => {
    stubScheme(false);
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('system');
    expect(theme.resolveTheme('system')).toBe('light');
  });

  it('keeps explicit light/dark from storage', async () => {
    store.set('nm.theme', 'light');
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('light');
    expect(theme.getTheme()).toBe('light');
  });

  it('cycles system → light → dark → system', async () => {
    const theme = await import('./theme');
    expect(theme.getThemePreference()).toBe('system');

    let step = theme.cycleThemePreference();
    expect(step.preference).toBe('light');
    expect(step.theme).toBe('light');
    expect(store.get('nm.theme')).toBe('light');

    step = theme.cycleThemePreference();
    expect(step.preference).toBe('dark');
    expect(step.theme).toBe('dark');

    step = theme.cycleThemePreference();
    expect(step.preference).toBe('system');
    expect(step.theme).toBe('dark');
  });

  it('themeLabel covers all preferences', async () => {
    const theme = await import('./theme');
    expect(theme.themeLabel('system')).toBe('Системная');
    expect(theme.themeLabel('light')).toBe('Светлая');
    expect(theme.themeLabel('dark')).toBe('Тёмная');
  });
});
