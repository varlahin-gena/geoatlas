import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';
import { isAbortError } from '@/api/client';
import { fetchSystemStatus, fetchSystemVersion } from '@/api/system';
import { roleLabelRu } from '@/auth/roles';
import { themeLabel } from '@/auth/theme';
import { usePolling } from '@/lib/usePolling';
import { filterNav, PAGE_NAV } from './nav';
import { NavSections } from './NavSections';
import { SidebarCollapseButton, SidebarShell } from './SidebarShell';
import { useNavBadges } from './useNavBadges';
import { useSidebarCollapsed } from './useSidebarCollapsed';

export function AdminSidebar() {
  const { isAdmin, reputationEnabled, uiAuthEnabled } = useAuth();
  const badges = useNavBadges();
  const { collapsed, toggle } = useSidebarCollapsed();

  useEffect(() => {
    const app = document.getElementById('adminApp');
    if (!app) return;
    app.classList.toggle('sidebar-collapsed', collapsed);
  }, [collapsed]);

  const items = filterNav(PAGE_NAV, {
    isAdmin,
    reputationEnabled,
    uiAuthEnabled,
  });

  return (
    <SidebarShell>
      <NavSections items={items} badges={badges} />
      <SidebarCollapseButton onToggle={toggle} />
    </SidebarShell>
  );
}

export function UserMenu() {
  const { user, theme, toggleTheme, logout, logoutAll } = useAuth();
  const [open, setOpen] = useState(false);
  const [versionText, setVersionText] = useState('');
  const [versionTitle, setVersionTitle] = useState('');
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!user || user.authDisabled) return;
    fetchSystemVersion()
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
  const roleRu = roleLabelRu(user.role);

  return (
    <div className={`ga-user-menu${open ? ' open' : ''}`} ref={ref}>
      <button
        type="button"
        className="ga-user-menu-trigger"
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
        <span className="ga-user-name">{displayName}</span>
        <svg className="ga-user-caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>
      <div className="ga-user-menu-dropdown" role="menu">
        <div className="ga-user-menu-meta">
          <div className="ga-meta-name">{displayName}</div>
          <div className="ga-meta-role">
            {fio ? `@${user.username} · ` : ''}
            {roleRu}
          </div>
          {versionText ? (
            <div className="ga-meta-version" title={versionTitle}>
              {versionText}
            </div>
          ) : null}
        </div>
        <button
          type="button"
          className="ga-user-menu-item"
          role="menuitem"
          onClick={(e) => {
            e.stopPropagation();
            toggleTheme();
          }}
        >
          <span>Тема</span>
          <span className="ga-theme-value">{themeLabel(theme)}</span>
        </button>
        <div className="ga-user-menu-sep" />
        <button
          type="button"
          className="ga-user-menu-item danger"
          role="menuitem"
          title="Инвалидирует сессии на всех устройствах"
          onClick={(e) => {
            e.stopPropagation();
            setOpen(false);
            if (
              !window.confirm(
                'Завершить все сессии на всех устройствах?\nПотребуется войти снова на каждом из них.',
              )
            ) {
              return;
            }
            void logoutAll();
          }}
        >
          Выйти везде
        </button>
        <button
          type="button"
          className="ga-user-menu-item danger"
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
  const [title, setTitle] = useState('Состояние системы');

  usePolling(
    async (signal) => {
      try {
        const data = await fetchSystemStatus({ signal });
        if (signal.aborted) return;
        const alerts = data.alerts || [];
        const count = alerts.length;
        const lvl = (data.level || 'ok').toLowerCase();
        const alertTitle = alerts
          .map((a) => `[${a.level || ''}] ${a.target || ''}: ${a.message || ''}`)
          .join('\n');
        if (lvl === 'error') {
          setLevel('bad');
          setText(`⚠ ${count} проблем`);
          setTitle(alertTitle || 'Есть проблемы');
        } else if (lvl === 'warn') {
          setLevel('warn');
          setText(`${count} предупр.`);
          setTitle(alertTitle || 'Есть предупреждения');
        } else {
          setLevel('ok');
          setText('Система ОК');
          setTitle(isAdmin ? 'Кликни, чтобы открыть мониторинг' : 'Состояние системы');
        }
      } catch (e) {
        if (isAbortError(e) || signal.aborted) return;
        setLevel('bad');
        setText('API недоступен');
        setTitle('Не удалось получить статус системы');
      }
    },
    5000,
    true,
  );

  const cls = `status-pill${level === 'bad' ? ' bad' : level === 'warn' ? ' warn' : level === 'ok' ? ' ok' : ''}`;
  const content = (
    <>
      <span className="dot" />
      <span>{text}</span>
      <svg className="status-pill-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
        <path d="M14 3h7v7M10 14L21 3M21 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5" />
      </svg>
    </>
  );

  if (isAdmin) {
    return (
      <Link
        to="/system"
        className={cls}
        style={{ textDecoration: 'none', cursor: 'pointer' }}
        title={title}
      >
        {content}
      </Link>
    );
  }
  return (
    <span className={cls} title={title}>
      {content}
    </span>
  );
}
