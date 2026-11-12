import type { ReactNode } from 'react';
import { AdminLayout } from '@/components/AdminLayout';
import './settings-layout.css';

/** Shared settings chrome: prefixed title, content padding (nav is the app sidebar). */
export function SettingsLayout({
  title,
  children,
  plainTitle = false,
}: {
  title: string;
  children: ReactNode;
  /** When true, use title as-is (sidebar already shows Settings). */
  plainTitle?: boolean;
}) {
  const heading =
    plainTitle || title.startsWith('Настройки') ? title : `Настройки · ${title}`;

  return (
    <AdminLayout title={heading}>
      <div className="settings-content">{children}</div>
    </AdminLayout>
  );
}
