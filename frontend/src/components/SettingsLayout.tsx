import type { ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';
import { AdminLayout } from '@/components/AdminLayout';
import {
  filterNav,
  isNavActive,
  PAGE_NAV,
  splitNavItems,
  type NavGroupSection,
} from '@/components/nav';
import './settings-layout.css';

export function settingsNavSections(
  items: ReturnType<typeof filterNav>,
): NavGroupSection[] {
  return splitNavItems(items).sections.filter((s) => s.id === 'data' || s.id === 'access');
}

/** Shared settings chrome for data + access pages (HIG settings panes). */
export function SettingsLayout({
  title,
  children,
  actions,
}: {
  title: string;
  children: ReactNode;
  actions?: ReactNode;
}) {
  const location = useLocation();
  const { isAdmin, reputationEnabled, uiAuthEnabled } = useAuth();
  const items = filterNav(PAGE_NAV, { isAdmin, reputationEnabled, uiAuthEnabled });
  const sections = settingsNavSections(items);
  const heading = title.startsWith('Настройки') ? title : `Настройки · ${title}`;

  return (
    <AdminLayout title={heading} actions={actions}>
      <div className="settings-shell">
        <nav className="settings-panes" aria-label="Разделы настроек">
          {sections.map((section) => (
            <div key={section.id} className="settings-pane-group">
              <div className="settings-pane-label">{section.label}</div>
              {section.items.map((item) => {
                const active = isNavActive(item, location.pathname);
                return (
                  <Link
                    key={item.href}
                    to={item.href}
                    className={`settings-pane-link${active ? ' active' : ''}`}
                    aria-current={active ? 'page' : undefined}
                  >
                    {item.label}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>
        <div className="settings-content">{children}</div>
      </div>
    </AdminLayout>
  );
}
