import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { SettingsLayout, settingsNavSections } from './SettingsLayout';
import { filterNav, PAGE_NAV } from './nav';

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

describe('settingsNavSections', () => {
  it('keeps only data and access groups', () => {
    const items = filterNav(PAGE_NAV, {
      isAdmin: true,
      reputationEnabled: true,
      uiAuthEnabled: true,
    });
    const sections = settingsNavSections(items);
    expect(sections.every((s) => s.id === 'data' || s.id === 'access')).toBe(true);
    expect(sections.length).toBeGreaterThan(0);
  });
});

describe('SettingsLayout', () => {
  it('renders panes and prefixes title', () => {
    render(
      <MemoryRouter initialEntries={['/users']}>
        <SettingsLayout title="Пользователи">
          <p>тело</p>
        </SettingsLayout>
      </MemoryRouter>,
    );
    expect(screen.getByText('Настройки · Пользователи')).toBeTruthy();
    expect(screen.getByLabelText('Разделы настроек')).toBeTruthy();
    expect(screen.getByText('тело')).toBeTruthy();
  });
});
