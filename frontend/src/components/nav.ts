type NavGroupId = 'workspace' | 'observe' | 'data' | 'threat' | 'access';

export interface NavItem {
  href: string;
  label: string;
  group: NavGroupId;
  match?: string[];
  adminOnly?: boolean;
  requiresReputation?: boolean;
  requiresUIAuth?: boolean;
}

const NAV_GROUP_ORDER: NavGroupId[] = [
  'workspace',
  'observe',
  'data',
  'threat',
  'access',
];

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
    group: 'threat',
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
];

export type NavGroupSection = {
  id: string;
  label: string;
  items: NavItem[];
};

export function groupNav(items: NavItem[]): NavGroupSection[] {
  const byGroup = new Map<NavGroupId, NavItem[]>();
  for (const item of items) {
    const list = byGroup.get(item.group);
    if (list) list.push(item);
    else byGroup.set(item.group, [item]);
  }
  return NAV_GROUP_ORDER.filter((id) => (byGroup.get(id)?.length ?? 0) > 0).map((id) => ({
    id,
    label: NAV_GROUP_LABELS[id],
    items: byGroup.get(id)!,
  }));
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
