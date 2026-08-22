import { describe, expect, it } from 'vitest';
import {
  filterNav,
  formatNavBadge,
  groupNav,
  splitNavItems,
  PAGE_NAV,
  type NavItem,
} from './nav';

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
