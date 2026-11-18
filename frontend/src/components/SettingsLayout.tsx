import type { ReactNode } from 'react';
import { AdminLayout } from '@/components/AdminLayout';
import './settings-layout.css';

/** Shared settings chrome: pane title in the toolbar, content padding (nav is the app sidebar). */
export function SettingsLayout({
  title,
  children,
  actions,
  plainTitle = true,
}: {
  title: string;
  children: ReactNode;
  /** Primary page actions shown in the trailing edge of the topbar. */
  actions?: ReactNode;
  /**
   * When false, prefix with «Настройки ·». Default is plain pane title —
   * the sidebar already situates the page in Settings.
   */
  plainTitle?: boolean;
}) {
  const heading =
    plainTitle || title.startsWith('Настройки') ? title : `Настройки · ${title}`;

  return (
    <AdminLayout title={heading} actions={actions}>
      <div className="settings-content">{children}</div>
    </AdminLayout>
  );
}
