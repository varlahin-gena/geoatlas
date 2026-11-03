import type { ReactNode } from 'react';
import { AdminLayout } from '@/components/AdminLayout';
import './settings-layout.css';

/** Shared settings chrome: prefixed title, content padding (nav is the app sidebar). */
export function SettingsLayout({
  title,
  children,
  actions,
}: {
  title: string;
  children: ReactNode;
  actions?: ReactNode;
}) {
  const heading = title.startsWith('Настройки') ? title : `Настройки · ${title}`;

  return (
    <AdminLayout title={heading} actions={actions}>
      <div className="settings-content">{children}</div>
    </AdminLayout>
  );
}
