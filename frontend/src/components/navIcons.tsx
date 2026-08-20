export function NavIcon({ kind }: { kind: string }) {
  const common = {
    className: 'icon',
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 2,
  } as const;
  switch (kind) {
    case 'map':
      return (
        <svg {...common}>
          <path d="M1 6v15l7-3 8 3 7-3V3l-7 3-8-3-7 3z" />
          <path d="M8 3v15M16 6v15" />
        </svg>
      );
    case 'system':
      return (
        <svg {...common}>
          <path d="M3 3v18h18" />
          <path d="M7 14l3-3 3 3 5-5" />
        </svg>
      );
    case 'parser':
      return (
        <svg {...common}>
          <polyline points="16 18 22 12 16 6" />
          <polyline points="8 6 2 12 8 18" />
        </svg>
      );
    case 'errors':
      return (
        <svg {...common}>
          <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
          <line x1="12" y1="9" x2="12" y2="13" />
          <line x1="12" y1="17" x2="12.01" y2="17" />
        </svg>
      );
    case 'geo-missing':
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="10" />
          <path d="M12 8v4M12 16h.01" />
        </svg>
      );
    case 'geo':
      return (
        <svg {...common}>
          <ellipse cx="12" cy="5" rx="9" ry="3" />
          <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" />
          <path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3" />
        </svg>
      );
    case 'reputation':
      return (
        <svg {...common}>
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        </svg>
      );
    case 'users':
      return (
        <svg {...common}>
          <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
        </svg>
      );
    case 'tokens':
      return (
        <svg {...common}>
          <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
          <path d="M7 11V7a5 5 0 0 1 10 0v4" />
        </svg>
      );
    default:
      return null;
  }
}

export const NAV_ICONS: Record<string, string> = {
  '/': 'map',
  '/system': 'system',
  '/parser-test': 'parser',
  '/parse-errors': 'errors',
  '/geo-missing': 'geo-missing',
  '/geo-ranges': 'geo',
  '/reputation': 'reputation',
  '/users': 'users',
  '/api-tokens': 'tokens',
};
