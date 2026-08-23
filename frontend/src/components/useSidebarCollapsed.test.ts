import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  persistSidebarCollapsed,
  readSidebarCollapsed,
  SIDEBAR_COLLAPSED_KEY,
  useSidebarCollapsed,
} from './useSidebarCollapsed';

function memStorage(initial: Record<string, string> = {}): Storage {
  const map = new Map(Object.entries(initial));
  return {
    get length() {
      return map.size;
    },
    clear() {
      map.clear();
    },
    getItem(key: string) {
      return map.has(key) ? map.get(key)! : null;
    },
    key() {
      return null;
    },
    removeItem(key: string) {
      map.delete(key);
    },
    setItem(key: string, value: string) {
      map.set(key, value);
    },
  };
}

describe('readSidebarCollapsed', () => {
  it('prefers the shared key', () => {
    const s = memStorage({
      [SIDEBAR_COLLAPSED_KEY]: '0',
      'nm.mapSidebarCollapsed': '1',
    });
    expect(readSidebarCollapsed(s)).toBe(false);
  });

  it('migrates legacy map key', () => {
    const s = memStorage({ 'nm.mapSidebarCollapsed': '1' });
    expect(readSidebarCollapsed(s)).toBe(true);
    expect(s.getItem(SIDEBAR_COLLAPSED_KEY)).toBe('1');
  });

  it('migrates legacy admin key', () => {
    const s = memStorage({ 'nm.adminSidebarCollapsed': '1' });
    expect(readSidebarCollapsed(s)).toBe(true);
  });
});

describe('persistSidebarCollapsed', () => {
  it('writes shared key and clears legacy', () => {
    const s = memStorage({
      'nm.mapSidebarCollapsed': '1',
      'nm.adminSidebarCollapsed': '0',
    });
    persistSidebarCollapsed(true, s);
    expect(s.getItem(SIDEBAR_COLLAPSED_KEY)).toBe('1');
    expect(s.getItem('nm.mapSidebarCollapsed')).toBeNull();
    expect(s.getItem('nm.adminSidebarCollapsed')).toBeNull();
  });
});

describe('useSidebarCollapsed', () => {
  it('keeps multiple hook callers in sync after toggle', () => {
    const a = renderHook(() => useSidebarCollapsed());
    const b = renderHook(() => useSidebarCollapsed());
    const before = a.result.current.collapsed;
    expect(b.result.current.collapsed).toBe(before);

    act(() => {
      a.result.current.toggle();
    });

    expect(a.result.current.collapsed).toBe(!before);
    expect(b.result.current.collapsed).toBe(!before);
  });
});
