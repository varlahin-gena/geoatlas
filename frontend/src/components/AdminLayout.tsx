import { useEffect, type ReactNode } from 'react';
import { AdminSidebar, UserMenu } from './Shell';

export function AdminLayout({
  title,
  children,
  actions,
}: {
  title: string;
  children: ReactNode;
  actions?: ReactNode;
}) {
  useEffect(() => {
    document.body.classList.add('page-admin');
    return () => {
      document.body.classList.remove('page-admin');
    };
  }, []);

  return (
    <div id="adminApp" className="app">
      <AdminSidebar />
      <div className="admin-main">
        <header id="adminTopbar" className="topbar">
          <div className="topbar-title title">{title}</div>
          <div className="topbar-spacer" />
          <div className="topbar-actions" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {actions}
            <div id="userBarHost">
              <UserMenu />
            </div>
          </div>
        </header>
        <main className="page-content">{children}</main>
      </div>
    </div>
  );
}

export function PageLoading({ label = 'Загрузка…' }: { label?: string }) {
  return <div className="page-loading">{label}</div>;
}
