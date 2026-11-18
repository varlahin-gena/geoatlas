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
  it('uses plain pane title by default and renders children', () => {
    render(
      <MemoryRouter initialEntries={['/users']}>
        <SettingsLayout title="Пользователи">
          <p>тело</p>
        </SettingsLayout>
      </MemoryRouter>,
    );
    expect(screen.getByText('Пользователи')).toBeTruthy();
    expect(screen.queryByText('Настройки · Пользователи')).toBeNull();
    expect(screen.queryByLabelText('Разделы настроек')).toBeNull();
    expect(screen.getByText('тело')).toBeTruthy();
  });

  it('can prefix Настройки when plainTitle is false', () => {
    render(
      <MemoryRouter initialEntries={['/parsers']}>
        <SettingsLayout title="Парсеры" plainTitle={false}>
          <p>тело</p>
        </SettingsLayout>
      </MemoryRouter>,
    );
    expect(screen.getByText('Настройки · Парсеры')).toBeTruthy();
  });

  it('renders topbar actions when provided', () => {
    render(
      <MemoryRouter initialEntries={['/parsers']}>
        <SettingsLayout title="Парсеры" actions={<button type="button">Добавить правило</button>}>
          <p>тело</p>
        </SettingsLayout>
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: 'Добавить правило' })).toBeTruthy();
  });
});
