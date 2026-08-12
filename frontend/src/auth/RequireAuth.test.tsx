import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ROLE_ADMIN, ROLE_OPERATOR, type AuthUser } from '@/api/types';
import { RequireAuth } from './RequireAuth';

const authState = vi.hoisted(() => ({
  user: null as AuthUser | null,
  loading: false,
  isAdmin: false,
}));

vi.mock('./AuthContext', () => ({
  useAuth: () => ({
    user: authState.user,
    loading: authState.loading,
    isAdmin: authState.isAdmin,
  }),
}));

function renderAt(path: string, admin = false) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<div>login-page</div>} />
        <Route path="/change-password" element={<div>reset-page</div>} />
        <Route path="/" element={<div>home</div>} />
        <Route
          path="/admin"
          element={
            <RequireAuth admin={admin}>
              <div>secret</div>
            </RequireAuth>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('RequireAuth', () => {
  beforeEach(() => {
    authState.user = null;
    authState.loading = false;
    authState.isAdmin = false;
  });

  it('shows loading state', () => {
    authState.loading = true;
    renderAt('/admin');
    expect(screen.getByText('Загрузка…')).toBeInTheDocument();
  });

  it('redirects anonymous users to login with next', () => {
    renderAt('/admin');
    expect(screen.getByText('login-page')).toBeInTheDocument();
  });

  it('redirects must_reset_password to change-password', () => {
    authState.user = {
      username: 'u',
      role: ROLE_OPERATOR,
      must_reset_password: true,
    };
    renderAt('/admin');
    expect(screen.getByText('reset-page')).toBeInTheDocument();
  });

  it('blocks non-admin from admin routes', () => {
    authState.user = { username: 'op', role: ROLE_OPERATOR };
    authState.isAdmin = false;
    renderAt('/admin', true);
    expect(screen.getByText('home')).toBeInTheDocument();
    expect(screen.queryByText('secret')).not.toBeInTheDocument();
  });

  it('allows admin', () => {
    authState.user = { username: 'a', role: ROLE_ADMIN };
    authState.isAdmin = true;
    renderAt('/admin', true);
    expect(screen.getByText('secret')).toBeInTheDocument();
  });
});
