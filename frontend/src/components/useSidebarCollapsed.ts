import { useCallback, useState } from 'react';

/** Shared across Map and Admin shells. */
export const SIDEBAR_COLLAPSED_KEY = 'nm.sidebarCollapsed';

const LEGACY_KEYS = ['nm.adminSidebarCollapsed', 'nm.mapSidebarCollapsed'] as const;

/** Pure read for tests and boot. Migrates legacy keys once. */
export function readSidebarCollapsed(storage: Storage = localStorage): boolean {
  try {
    const cur = storage.getItem(SIDEBAR_COLLAPSED_KEY);
    if (cur === '1') return true;
    if (cur === '0') return false;
    for (const key of LEGACY_KEYS) {
      if (storage.getItem(key) === '1') {
        storage.setItem(SIDEBAR_COLLAPSED_KEY, '1');
        return true;
      }
    }
    return false;
  } catch {
    return false;
  }
}

export function persistSidebarCollapsed(next: boolean, storage: Storage = localStorage) {
  try {
    storage.setItem(SIDEBAR_COLLAPSED_KEY, next ? '1' : '0');
    for (const key of LEGACY_KEYS) {
      storage.removeItem(key);
    }
  } catch {
    /* ignore */
  }
}

/** One collapse preference for map + admin sidebars. */
export function useSidebarCollapsed() {
  const [collapsed, setCollapsed] = useState(() => readSidebarCollapsed());

  const toggle = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      persistSidebarCollapsed(next);
      return next;
    });
  }, []);

  return { collapsed, toggle };
}
