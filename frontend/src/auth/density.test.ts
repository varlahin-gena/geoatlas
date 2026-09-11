import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

describe('ui density', () => {
  const store = new Map<string, string>();

  beforeEach(() => {
    store.clear();
    document.documentElement.removeAttribute('data-density');
    vi.resetModules();
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => {
        store.set(k, v);
      },
      removeItem: (k: string) => {
        store.delete(k);
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.documentElement.removeAttribute('data-density');
  });

  it('defaults to comfortable and applies attribute on import', async () => {
    const density = await import('./density');
    expect(density.getDensity()).toBe('comfortable');
    expect(document.documentElement.getAttribute('data-density')).toBe('comfortable');
  });

  it('toggles comfortable ↔ compact and persists', async () => {
    const density = await import('./density');
    expect(density.toggleDensity()).toBe('compact');
    expect(store.get('nm.uiDensity')).toBe('compact');
    expect(document.documentElement.getAttribute('data-density')).toBe('compact');
    expect(density.toggleDensity()).toBe('comfortable');
    expect(store.get('nm.uiDensity')).toBe('comfortable');
  });

  it('densityLabel covers both modes', async () => {
    const density = await import('./density');
    expect(density.densityLabel('comfortable')).toBe('Комфорт');
    expect(density.densityLabel('compact')).toBe('Компакт');
  });
});
