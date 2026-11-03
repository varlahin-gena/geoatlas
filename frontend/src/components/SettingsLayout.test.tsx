import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { SettingsLayout } from './SettingsLayout';

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({
    isAdmin: true,
    reputationEnabled: true,
    uiAuthEnabled: true,
    user: { username: 'admin', role: 'admin' },
  }),
}));

vi.mock('./Shell', () => ({
  AdminSidebar: () => <aside data-testid="sidebar" />,
  SystemHealthPill: () => null,
  UserMenu: () => null,
}));

describe('SettingsLayout', () => {
  it('prefixes title and renders children without settings panes', () => {
    render(
      <MemoryRouter initialEntries={['/users']}>
        <SettingsLayout title="Пользователи">
          <p>тело</p>
        </SettingsLayout>
      </MemoryRouter>,
    );
    expect(screen.getByText('Настройки · Пользователи')).toBeTruthy();
    expect(screen.queryByLabelText('Разделы настроек')).toBeNull();
    expect(screen.getByText('тело')).toBeTruthy();
  });
});
