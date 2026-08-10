export interface NavItem {
  href: string;
  label: string;
  match?: string[];
  adminOnly?: boolean;
  requiresReputation?: boolean;
  requiresUIAuth?: boolean;
}

export const PAGE_NAV: NavItem[] = [
  { href: '/', label: 'Карта', match: ['/', '/index.html'], adminOnly: false },
  { href: '/system', label: 'Мониторинг', match: ['/system', '/system.html'], adminOnly: true },
  {
    href: '/parser-test',
    label: 'Тест парсеров',
    match: ['/parser-test', '/parser-test.html'],
    adminOnly: true,
  },
  {
    href: '/parse-errors',
    label: 'Ошибки парсинга',
    match: ['/parse-errors', '/parse-errors.html'],
    adminOnly: true,
  },
  {
    href: '/geo-missing',
    label: 'IP без GeoIP',
    match: ['/geo-missing', '/geo-missing.html'],
    adminOnly: true,
  },
  {
    href: '/geo-ranges',
    label: 'База GeoIP',
    match: ['/geo-ranges', '/geo-ranges.html'],
    adminOnly: true,
  },
  {
    href: '/reputation',
    label: 'Репутация IP',
    match: ['/reputation', '/reputation.html'],
    adminOnly: true,
    requiresReputation: true,
  },
  {
    href: '/users',
    label: 'Пользователи',
    match: ['/users', '/users.html'],
    adminOnly: true,
    requiresUIAuth: true,
  },
  {
    href: '/api-tokens',
    label: 'API-токены',
    match: ['/api-tokens', '/api-tokens.html'],
    adminOnly: true,
  },
];

export function normalizePath(pathname: string): string {
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
