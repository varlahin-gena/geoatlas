import { filterNav, PAGE_NAV, type NavItem } from '@/components/nav';
import { PERIODS } from '@/pages/Map/mapPeriods';
import { dispatchMapCommand } from '@/lib/mapCommandBus';
import { cycleThemePreference } from '@/auth/theme';
import { toggleDensity } from '@/auth/density';

export type CommandGroup = 'navigate' | 'map' | 'triage' | 'prefs';

export type AppCommand = {
  id: string;
  label: string;
  group: CommandGroup;
  groupLabel: string;
  keywords?: string;
  /** When set, navigate after run (unless run returns false). */
  href?: string;
  run?: () => void | boolean | Promise<void | boolean>;
};

const GROUP_LABELS: Record<CommandGroup, string> = {
  navigate: 'Переход',
  map: 'Карта',
  triage: 'Разбор',
  prefs: 'Параметры',
};

const NAV_GROUP_SUBTITLE: Record<string, string> = {
  workspace: 'Рабочее место',
  triage: 'Разбор',
  system: 'Система',
  data: 'Данные',
  access: 'Доступ',
};

export type CommandCatalogContext = {
  isAdmin: boolean;
  reputationEnabled: boolean;
  uiAuthEnabled: boolean;
  pathname: string;
};

function navCommands(items: NavItem[]): AppCommand[] {
  return items.map((item) => ({
    id: `nav:${item.href}`,
    label: item.label,
    group: 'navigate' as const,
    groupLabel: NAV_GROUP_SUBTITLE[item.group] || GROUP_LABELS.navigate,
    keywords: `${item.href} ${item.group}`,
    href: item.external ? undefined : item.href,
    run: item.external
      ? () => {
          window.location.assign(item.href);
          return false;
        }
      : undefined,
  }));
}

const MAP_PERIOD_IDS = new Set(['1h', '1d', '7d', '15m', '6h']);

export function buildCommandCatalog(ctx: CommandCatalogContext): AppCommand[] {
  const items = filterNav(PAGE_NAV, {
    isAdmin: ctx.isAdmin,
    reputationEnabled: ctx.reputationEnabled,
    uiAuthEnabled: ctx.uiAuthEnabled,
  });

  const cmds: AppCommand[] = [...navCommands(items)];

  const onMap = ctx.pathname === '/' || ctx.pathname === '/index.html';
  if (onMap) {
    for (const [id, label] of PERIODS) {
      if (id === 'custom' || !MAP_PERIOD_IDS.has(id)) continue;
      cmds.push({
        id: `map:period:${id}`,
        label: `Период: ${label}`,
        group: 'map',
        groupLabel: GROUP_LABELS.map,
        keywords: `period ${id} время`,
        run: () => {
          dispatchMapCommand({ type: 'set-period', period: id });
        },
      });
    }
    cmds.push({
      id: 'map:reset-filters',
      label: 'Сбросить фильтры карты',
      group: 'map',
      groupLabel: GROUP_LABELS.map,
      keywords: 'filter clear сброс',
      run: () => {
        dispatchMapCommand({ type: 'reset-filters' });
      },
    });
    cmds.push({
      id: 'map:focus-search',
      label: 'Фокус на поиске карты',
      group: 'map',
      groupLabel: GROUP_LABELS.map,
      keywords: 'search поиск',
      run: () => {
        dispatchMapCommand({ type: 'focus-search' });
      },
    });
  }

  cmds.push({
    id: 'triage:anomalies',
    label: 'Открыть аномалии',
    group: 'triage',
    groupLabel: GROUP_LABELS.triage,
    keywords: 'alert triage',
    href: '/anomalies',
  });
  cmds.push({
    id: 'triage:hunts',
    label: 'Открыть охоты',
    group: 'triage',
    groupLabel: GROUP_LABELS.triage,
    keywords: 'hunt saved',
    href: '/hunts',
  });

  cmds.push({
    id: 'prefs:theme',
    label: 'Сменить тему',
    group: 'prefs',
    groupLabel: GROUP_LABELS.prefs,
    keywords: 'theme dark light system',
    run: () => {
      cycleThemePreference();
    },
  });
  cmds.push({
    id: 'prefs:density',
    label: 'Сменить плотность UI',
    group: 'prefs',
    groupLabel: GROUP_LABELS.prefs,
    keywords: 'density compact comfortable',
    run: () => {
      toggleDensity();
    },
  });

  return cmds;
}

/** Simple fuzzy score: all query tokens must appear in haystack. */
export function filterCommands(commands: AppCommand[], query: string): AppCommand[] {
  const q = query.trim().toLowerCase();
  if (!q) return commands;
  const tokens = q.split(/\s+/).filter(Boolean);
  const scored: { cmd: AppCommand; score: number }[] = [];
  for (const cmd of commands) {
    const hay = `${cmd.label} ${cmd.groupLabel} ${cmd.keywords || ''} ${cmd.href || ''}`.toLowerCase();
    if (!tokens.every((t) => hay.includes(t))) continue;
    let score = 0;
    if (cmd.label.toLowerCase().startsWith(tokens[0])) score += 40;
    if (cmd.label.toLowerCase().includes(tokens[0])) score += 20;
    score += Math.max(0, 30 - hay.indexOf(tokens[0]));
    scored.push({ cmd, score });
  }
  scored.sort((a, b) => b.score - a.score || a.cmd.label.localeCompare(b.cmd.label, 'ru'));
  return scored.map((s) => s.cmd);
}
