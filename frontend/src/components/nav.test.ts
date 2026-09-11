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
  it('operator and dashboard share non-admin nav (map + triage, no system)', () => {
    const opts = { isAdmin: false, reputationEnabled: true, uiAuthEnabled: true };
    const operatorNav = filterNav(PAGE_NAV, opts);
    const dashboardNav = filterNav(PAGE_NAV, opts);
    expect(operatorNav.map((i) => i.href)).toEqual(dashboardNav.map((i) => i.href));
    expect(operatorNav.map((i) => i.href)).toEqual(['/', '/anomalies', '/investigate', '/hunts']);
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
      'triage',
      'system',
      'data',
      'access',
    ]);
    expect(sections.find((s) => s.id === 'triage')?.items.map((i) => i.href)).toEqual([
      '/anomalies',
      '/investigate',
      '/hunts',
    ]);
    expect(sections.find((s) => s.id === 'system')?.items.map((i) => i.href)).toEqual([
      '/system',
      '/dozzle/',
      '/anomalies/engine',
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
    expect(sections.map((s) => s.id)).toEqual(['system', 'data', 'access']);
    expect(sections.every((s) => s.items.every((i: NavItem) => i.adminOnly))).toBe(true);
  });
});

describe('splitNavItems', () => {
  it('separates workspace and flat sidebar sections', () => {
    const items = filterNav(PAGE_NAV, {
      isAdmin: true,
      reputationEnabled: true,
      uiAuthEnabled: true,
    });
    const { workspace, sections } = splitNavItems(items);
    expect(workspace.map((i) => i.href)).toEqual(['/']);
    expect(sections.map((s) => s.id)).toEqual(['triage', 'system', 'data', 'access']);
    expect(sections.find((s) => s.id === 'triage')?.items.map((i) => i.label)).toEqual([
      'Аномалии',
      'Разбор',
      'Охоты',
    ]);
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
