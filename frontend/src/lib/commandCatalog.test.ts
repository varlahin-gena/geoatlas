import { describe, expect, it } from 'vitest';
import { buildCommandCatalog, filterCommands, type AppCommand } from './commandCatalog';

const baseCtx = {
  isAdmin: true,
  reputationEnabled: true,
  uiAuthEnabled: true,
  pathname: '/anomalies',
};

describe('filterCommands', () => {
  const sample: AppCommand[] = [
    {
      id: 'a',
      label: 'Карта',
      group: 'navigate',
      groupLabel: 'Рабочее место',
      keywords: 'map geo',
      href: '/',
    },
    {
      id: 'b',
      label: 'Аномалии',
      group: 'triage',
      groupLabel: 'Разбор',
      keywords: 'alert',
      href: '/anomalies',
    },
    {
      id: 'c',
      label: 'Сменить тему',
      group: 'prefs',
      groupLabel: 'Параметры',
      keywords: 'theme dark light',
    },
  ];

  it('returns all when query empty', () => {
    expect(filterCommands(sample, '  ')).toEqual(sample);
  });

  it('matches label tokens', () => {
    expect(filterCommands(sample, 'аном').map((c) => c.id)).toEqual(['b']);
  });

  it('matches keywords and requires all tokens', () => {
    expect(filterCommands(sample, 'theme dark').map((c) => c.id)).toEqual(['c']);
    expect(filterCommands(sample, 'theme xyz')).toEqual([]);
  });

  it('ranks prefix matches higher', () => {
    const cmds: AppCommand[] = [
      { id: '1', label: 'Тема системная', group: 'prefs', groupLabel: 'Параметры' },
      { id: '2', label: 'Тема', group: 'prefs', groupLabel: 'Параметры' },
    ];
    expect(filterCommands(cmds, 'тема').map((c) => c.id)).toEqual(['2', '1']);
  });
});

describe('buildCommandCatalog', () => {
  it('includes nav and prefs off the map', () => {
    const cmds = buildCommandCatalog(baseCtx);
    expect(cmds.some((c) => c.id === 'nav:/anomalies')).toBe(true);
    expect(cmds.some((c) => c.id === 'prefs:theme')).toBe(true);
    expect(cmds.some((c) => c.id === 'prefs:density')).toBe(true);
    expect(cmds.some((c) => c.id.startsWith('map:'))).toBe(false);
  });

  it('adds map commands on map route', () => {
    const cmds = buildCommandCatalog({ ...baseCtx, pathname: '/' });
    expect(cmds.some((c) => c.id === 'map:reset-filters')).toBe(true);
    expect(cmds.some((c) => c.id === 'map:focus-search')).toBe(true);
    expect(cmds.some((c) => c.id === 'map:period:1h')).toBe(true);
  });
});
