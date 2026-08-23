import { useEffect, type ReactNode } from 'react';
import { AdminSidebar, SystemHealthPill, UserMenu } from './Shell';

export function AdminLayout({
  title,
  subtitle,
  children,
  actions,
  showSystemHealth = true,
  mainClassName = 'page-content',
  className,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  actions?: ReactNode;
  showSystemHealth?: boolean;
  /** Scrollable main region class (`page-content` for most admin pages, `content` for /system). */
  mainClassName?: string;
  /** Extra class on #adminApp (e.g. ga-system for scoped system.css). */
  className?: string;
}) {
  useEffect(() => {
    document.body.classList.add('page-admin');
    return () => {
      document.body.classList.remove('page-admin');
    };
  }, []);

  const appClass = ['app', className].filter(Boolean).join(' ');

  return (
    <div id="adminApp" className={appClass}>
      <AdminSidebar />
      <div className="admin-main">
        <header className="topbar">
          <div className="topbar-title title">
            {title}
            {subtitle ? <span className="sub">{subtitle}</span> : null}
          </div>
          <div className="topbar-spacer" />
          <div className="topbar-actions" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {actions}
            <UserMenu />
            {showSystemHealth ? <SystemHealthPill /> : null}
          </div>
        </header>
        <main className={mainClassName}>
          {children}
        </main>
      </div>
    </div>
  );
}

export function PageLoading({ label = 'Загрузка…' }: { label?: string }) {
  return <div className="page-loading">{label}</div>;
}
