import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';
import { apiFetch } from '@/api/client';
import type { SystemVersion } from '@/api/types';
import { ROLE_ADMIN } from '@/api/types';
import { themeLabel } from '@/auth/theme';
import { filterNav, isNavActive, PAGE_NAV } from './nav';

const SIDEBAR_KEY = 'nm.adminSidebarCollapsed';

function NavIcon({ kind }: { kind: string }) {
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

const NAV_ICONS: Record<string, string> = {
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

export function AdminSidebar({ adminLinksOnly = false }: { adminLinksOnly?: boolean }) {
  const { isAdmin, reputationEnabled, uiAuthEnabled } = useAuth();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(SIDEBAR_KEY) === '1';
    } catch {
      return false;
    }
  });

  useEffect(() => {
    const app = document.getElementById('adminApp');
    if (!app) return;
    app.classList.toggle('sidebar-collapsed', collapsed);
  }, [collapsed]);

  const items = filterNav(PAGE_NAV, {
    isAdmin,
    reputationEnabled,
    uiAuthEnabled,
    adminLinksOnly,
  });

  return (
    <aside id="adminSidebar" className="sidebar" aria-label="Навигация" data-nm-dynamic-nav="1">
      <div className="sidebar-header">
        <img className="logo" src="/logo.png" alt="" width={28} height={28} aria-hidden="true" />
        <div className="title">ГеоАтлас</div>
      </div>
      <div className="sidebar-section">
        <div className="sidebar-section-title">Разделы</div>
        {items.map((item) => {
          const active = isNavActive(item, location.pathname);
          return (
            <Link
              key={item.href}
              to={item.href}
              className={`side-btn${active ? ' active' : ''}`}
              aria-current={active ? 'page' : undefined}
              title={item.label}
            >
              <NavIcon kind={NAV_ICONS[item.href] || 'map'} />
              <span className="label">{item.label}</span>
            </Link>
          );
        })}
      </div>
      <div className="sidebar-collapse-btn">
        <button
          type="button"
          className="side-btn"
          title="Развернуть / свернуть меню"
          onClick={() => {
            const next = !collapsed;
            setCollapsed(next);
            try {
              localStorage.setItem(SIDEBAR_KEY, next ? '1' : '0');
            } catch {
              /* ignore */
            }
          }}
        >
          <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M15 18l-6-6 6-6" />
          </svg>
          <span className="label">Свернуть меню</span>
        </button>
      </div>
    </aside>
  );
}

export function UserMenu() {
  const { user, theme, toggleTheme, logout } = useAuth();
  const [open, setOpen] = useState(false);
  const [versionText, setVersionText] = useState('');
  const [versionTitle, setVersionTitle] = useState('');
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!user || user.authDisabled) return;
    apiFetch<SystemVersion>('/api/system/version')
      .then((data) => {
        const label = (data.display || data.ref || data.version || '').trim();
        if (!label) return;
        let text = `Версия: ${label}`;
        if (data.source === 'main' && data.commit) {
          text = `Версия: main · ${data.commit}`;
        } else if (data.source === 'release' && data.version && !label.includes(data.version)) {
          text = `Версия: ${label} (${data.version})`;
        }
        setVersionText(text);
        setVersionTitle(
          [data.ref && `ref: ${data.ref}`, data.commit && `commit: ${data.commit}`, data.version && `product: ${data.version}`]
            .filter(Boolean)
            .join('\n'),
        );
      })
      .catch(() => {});
  }, [user]);

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('click', onDoc);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('click', onDoc);
      document.removeEventListener('keydown', onKey);
    };
  }, []);

  if (!user || user.authDisabled) return null;

  const fio = (user.full_name || '').trim();
  const displayName = fio || user.username;
  const roleRu = user.role === ROLE_ADMIN ? 'Администратор' : 'Оператор';

  return (
    <div className={`nm-user-menu${open ? ' open' : ''}`} ref={ref}>
      <button
        type="button"
        className="nm-user-menu-trigger"
        aria-haspopup="menu"
        aria-expanded={open}
        title={
          fio
            ? `ФИО: ${fio}\nЛогин: ${user.username}\nРоль: ${roleRu}`
            : `Логин: ${user.username}\nРоль: ${roleRu}`
        }
        onClick={(e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        }}
      >
        <span className="nm-user-name">{displayName}</span>
        <svg className="nm-user-caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>
      <div className="nm-user-menu-dropdown" role="menu">
        <div className="nm-user-menu-meta">
          <div className="nm-meta-name">{displayName}</div>
          <div className="nm-meta-role">
            {fio ? `@${user.username} · ` : ''}
            {roleRu}
          </div>
          {versionText ? (
            <div className="nm-meta-version" title={versionTitle}>
              {versionText}
            </div>
          ) : null}
        </div>
        <button
          type="button"
          className="nm-user-menu-item"
          role="menuitem"
          onClick={(e) => {
            e.stopPropagation();
            toggleTheme();
          }}
        >
          <span>Тема</span>
          <span className="nm-theme-value">{themeLabel(theme)}</span>
        </button>
        <div className="nm-user-menu-sep" />
        <button
          type="button"
          className="nm-user-menu-item danger"
          role="menuitem"
          onClick={(e) => {
            e.stopPropagation();
            setOpen(false);
            void logout();
          }}
        >
          Выйти
        </button>
      </div>
    </div>
  );
}

export function SystemHealthPill() {
  const { isAdmin } = useAuth();
  const [text, setText] = useState('— загрузка —');
  const [level, setLevel] = useState<'ok' | 'warn' | 'bad' | null>(null);

  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      try {
        const res = await fetch('/api/system/status', { credentials: 'same-origin' });
        if (!res.ok) throw new Error(String(res.status));
        const data = (await res.json()) as {
          level?: string;
          alert_count?: number;
          alerts?: unknown[];
        };
        if (cancelled) return;
        const count = data.alert_count ?? data.alerts?.length ?? 0;
        const lvl = (data.level || 'ok').toLowerCase();
        if (lvl === 'error') {
          setLevel('bad');
          setText(`⚠ ${count} проблем`);
        } else if (lvl === 'warn') {
          setLevel('warn');
          setText(`${count} предупр.`);
        } else {
          setLevel('ok');
          setText('Система ОК');
        }
      } catch {
        if (!cancelled) {
          setLevel('bad');
          setText('API недоступен');
        }
      }
    };
    void tick();
    const id = window.setInterval(tick, 5000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  const cls = `status-pill${level === 'bad' ? ' bad' : level === 'warn' ? ' warn' : level === 'ok' ? ' ok' : ''}`;
  const content = (
    <>
      <span className="dot" />
      <span id="systemHealthText">{text}</span>
      <svg className="status-pill-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
        <path d="M14 3h7v7M10 14L21 3M21 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5" />
      </svg>
    </>
  );

  if (isAdmin) {
    return (
      <Link
        to="/system"
        id="systemHealthPill"
        className={cls}
        style={{ textDecoration: 'none', cursor: 'pointer' }}
        title="Кликни, чтобы открыть мониторинг"
      >
        {content}
      </Link>
    );
  }
  return (
    <span id="systemHealthPill" className={cls} title="Состояние системы">
      {content}
    </span>
  );
}
