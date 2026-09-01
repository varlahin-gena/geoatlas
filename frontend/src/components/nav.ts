type NavGroupId = 'workspace' | 'observe' | 'data' | 'threat' | 'access';

export interface NavItem {
  href: string;
  label: string;
  group: NavGroupId;
  match?: string[];
  adminOnly?: boolean;
  requiresReputation?: boolean;
  requiresUIAuth?: boolean;
  /** Full-page navigation outside the React SPA (e.g. Dozzle). */
  external?: boolean;
}

const NAV_GROUP_ORDER: NavGroupId[] = [
  'workspace',
  'observe',
  'data',
  'threat',
  'access',
];

const SETTINGS_GROUP_ORDER: NavGroupId[] = ['data', 'access'];

const NAV_GROUP_LABELS: Record<NavGroupId, string> = {
  workspace: 'Рабочее место',
  observe: 'Наблюдение',
  data: 'Данные и GeoIP',
  threat: 'Угрозы',
  access: 'Доступ',
};

export const PAGE_NAV: NavItem[] = [
  {
    href: '/',
    label: 'Карта',
    group: 'workspace',
    match: ['/', '/index.html'],
    adminOnly: false,
  },
  {
    href: '/system',
    label: 'Мониторинг системы',
    group: 'observe',
    match: ['/system', '/system.html'],
    adminOnly: true,
  },
  {
    href: '/dozzle/',
    label: 'Логи контейнеров',
    group: 'observe',
    match: ['/dozzle', '/dozzle/'],
    adminOnly: true,
    external: true,
  },
  {
    href: '/anomalies',
    label: 'Аномалии',
    group: 'observe',
    match: ['/anomalies', '/anomalies.html', '/investigate', '/investigate.html'],
    adminOnly: false,
  },
  {
    href: '/parse-errors',
    label: 'Ошибки парсинга',
    group: 'data',
    match: ['/parse-errors', '/parse-errors.html'],
    adminOnly: true,
  },
  {
    href: '/parser-test',
    label: 'Тест парсеров',
    group: 'data',
    match: ['/parser-test', '/parser-test.html'],
    adminOnly: true,
  },
  {
    href: '/geo-missing',
    label: 'IP без координат',
    group: 'data',
    match: ['/geo-missing', '/geo-missing.html'],
    adminOnly: true,
  },
  {
    href: '/geo-ranges',
    label: 'База GeoIP',
    group: 'data',
    match: ['/geo-ranges', '/geo-ranges.html'],
    adminOnly: true,
  },
  {
    href: '/reputation',
    label: 'Репутация IP',
    group: 'observe',
    match: ['/reputation', '/reputation.html'],
    adminOnly: true,
    requiresReputation: true,
  },
  {
    href: '/users',
    label: 'Пользователи',
    group: 'access',
    match: ['/users', '/users.html'],
    adminOnly: true,
    requiresUIAuth: true,
  },
  {
    href: '/api-tokens',
    label: 'API-токены',
    group: 'access',
    match: ['/api-tokens', '/api-tokens.html'],
    adminOnly: true,
  },
  {
    href: '/tls',
    label: 'HTTPS-сертификаты',
    group: 'access',
    match: ['/tls', '/tls.html'],
    adminOnly: true,
  },
];

export type NavGroupSection = {
  id: string;
  label: string;
  items: NavItem[];
};

function groupNavByOrder(items: NavItem[], order: NavGroupId[]): NavGroupSection[] {
  const byGroup = new Map<NavGroupId, NavItem[]>();
  for (const item of items) {
    const list = byGroup.get(item.group);
    if (list) list.push(item);
    else byGroup.set(item.group, [item]);
  }
  return order.filter((id) => (byGroup.get(id)?.length ?? 0) > 0).map((id) => ({
    id,
    label: NAV_GROUP_LABELS[id],
    items: byGroup.get(id)!,
  }));
}

export function groupNav(items: NavItem[]): NavGroupSection[] {
  return groupNavByOrder(items, NAV_GROUP_ORDER);
}

export function splitNavItems(items: NavItem[]): {
  workspace: NavItem[];
  observe: NavItem[];
  settings: NavGroupSection[];
} {
  const workspace = items.filter((item) => item.group === 'workspace');
  const observe = items.filter((item) => item.group === 'observe');
  const settings = groupNavByOrder(
    items.filter((item) => item.group !== 'workspace' && item.group !== 'observe'),
    SETTINGS_GROUP_ORDER,
  );
  return { workspace, observe, settings };
}

export function settingsBadgeTotal(
  sections: NavGroupSection[],
  badges: Record<string, string | null | undefined>,
): string | null {
  let sum = 0;
  for (const section of sections) {
    for (const item of section.items) {
      const raw = badges[item.href];
      if (!raw) continue;
      const n = raw.endsWith('+') ? parseInt(raw, 10) : parseInt(raw, 10);
      if (Number.isFinite(n)) sum += n;
    }
  }
  return formatNavBadge(sum, 99);
}

export function sectionBadgeTotal(
  section: NavGroupSection,
  badges: Record<string, string | null | undefined>,
): string | null {
  let sum = 0;
  for (const item of section.items) {
    const raw = badges[item.href];
    if (!raw) continue;
    const n = raw.endsWith('+') ? parseInt(raw, 10) : parseInt(raw, 10);
    if (Number.isFinite(n)) sum += n;
  }
  return formatNavBadge(sum, 99);
}

function normalizePath(pathname: string): string {
  let p = pathname || '/';
  if (p.length > 1 && p.endsWith('/')) p = p.slice(0, -1);
  return p || '/';
}

export function isNavActive(item: NavItem, pathname: string): boolean {
  const path = normalizePath(pathname);
  const matches = item.match || [item.href];
  return matches.some((m) => normalizePath(m) === path);
}

export function filterNav(
  items: NavItem[],
  opts: {
    isAdmin: boolean;
    reputationEnabled: boolean;
    uiAuthEnabled: boolean;
    adminLinksOnly?: boolean;
  },
): NavItem[] {
  return items.filter((item) => {
    if (opts.adminLinksOnly && !item.adminOnly) return false;
    if (item.adminOnly && !opts.isAdmin) return false;
    if (item.requiresReputation && !opts.reputationEnabled) return false;
    if (item.requiresUIAuth && !opts.uiAuthEnabled) return false;
    return true;
  });
}

/** Compact badge text; null when nothing to show. */
export function formatNavBadge(count: number, cap = 99): string | null {
  if (!Number.isFinite(count) || count <= 0) return null;
  if (count > cap) return `${cap}+`;
  return String(Math.floor(count));
}
