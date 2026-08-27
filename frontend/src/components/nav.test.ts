import { describe, expect, it } from 'vitest';
import {
  filterNav,
  formatNavBadge,
  groupNav,
  splitNavItems,
  PAGE_NAV,
  type NavItem,
} from './nav';

describe('filterNav role access', () => {
  it('operator and dashboard share non-admin nav (map + anomalies, no system)', () => {
    const opts = { isAdmin: false, reputationEnabled: true, uiAuthEnabled: true };
    const operatorNav = filterNav(PAGE_NAV, opts);
    const dashboardNav = filterNav(PAGE_NAV, opts);
    expect(operatorNav.map((i) => i.href)).toEqual(dashboardNav.map((i) => i.href));
    expect(operatorNav.some((i) => i.href === '/')).toBe(true);
    expect(operatorNav.some((i) => i.href === '/anomalies')).toBe(true);
    expect(operatorNav.some((i) => i.href === '/system')).toBe(false);
    expect(operatorNav.some((i) => i.href === '/users')).toBe(false);
  });
});

describe('groupNav', () => {
  it('orders sections and skips empty groups', () => {
    const items = filterNav(PAGE_NAV, {
      isAdmin: true,
      reputationEnabled: true,
      uiAuthEnabled: true,
    });
    const sections = groupNav(items);
    expect(sections.map((s) => s.id)).toEqual([
      'workspace',
      'observe',
      'data',
      'access',
    ]);
    expect(sections.find((s) => s.id === 'observe')?.items.map((i) => i.href)).toEqual([
      '/system',
      '/dozzle/',
      '/anomalies',
      '/reputation',
    ]);
    expect(sections.find((s) => s.id === 'data')?.items.map((i) => i.href)).toEqual([
      '/parse-errors',
      '/parser-test',
      '/geo-missing',
      '/geo-ranges',
    ]);
  });

  it('hides workspace when adminLinksOnly', () => {
    const items = filterNav(PAGE_NAV, {
      isAdmin: true,
      reputationEnabled: false,
      uiAuthEnabled: false,
      adminLinksOnly: true,
    });
    const sections = groupNav(items);
    expect(sections.map((s) => s.id)).toEqual(['observe', 'data', 'access']);
    expect(sections.every((s) => s.items.every((i: NavItem) => i.adminOnly))).toBe(true);
  });
});

describe('splitNavItems', () => {
  it('separates workspace, observe, and settings groups', () => {
    const items = filterNav(PAGE_NAV, {
      isAdmin: true,
      reputationEnabled: true,
      uiAuthEnabled: true,
    });
    const { workspace, observe, settings } = splitNavItems(items);
    expect(workspace.map((i) => i.href)).toEqual(['/']);
    expect(observe.map((i) => i.label)).toEqual([
      'Мониторинг системы',
      'Логи контейнеров',
      'Аномалии',
      'Репутация IP',
    ]);
    expect(settings.map((s) => s.id)).toEqual(['data', 'access']);
  });
});

describe('formatNavBadge', () => {
  it('formats counts and caps', () => {
    expect(formatNavBadge(0)).toBeNull();
    expect(formatNavBadge(-1)).toBeNull();
    expect(formatNavBadge(12)).toBe('12');
    expect(formatNavBadge(100, 99)).toBe('99+');
  });
});
