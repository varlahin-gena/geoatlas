import { lazy, Suspense } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { AuthProvider } from '@/auth/AuthContext';
import { RequireAuth } from '@/auth/RequireAuth';
import { ToastProvider } from '@/components/Toast';
import { PageLoading } from '@/components/AdminLayout';
import LoginPage from '@/pages/Login/LoginPage';
import ChangePasswordPage from '@/pages/ChangePassword/ChangePasswordPage';

const MapPage = lazy(() => import('@/pages/Map/MapPage'));
const SystemPage = lazy(() => import('@/pages/System/SystemPage'));
const UsersPage = lazy(() => import('@/pages/Users/UsersPage'));
const ApiTokensPage = lazy(() => import('@/pages/ApiTokens/ApiTokensPage'));
const ParseErrorsPage = lazy(() => import('@/pages/ParseErrors/ParseErrorsPage'));
const ParserTestPage = lazy(() => import('@/pages/ParserTest/ParserTestPage'));
const GeoMissingPage = lazy(() => import('@/pages/GeoMissing/GeoMissingPage'));
const GeoRangesPage = lazy(() => import('@/pages/GeoRanges/GeoRangesPage'));
const ReputationPage = lazy(() => import('@/pages/Reputation/ReputationPage'));

function Lazy({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<PageLoading />}>{children}</Suspense>;
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ToastProvider>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route
              path="/change-password"
              element={
                <RequireAuth allowMustReset>
                  <ChangePasswordPage />
                </RequireAuth>
              }
            />
            <Route
              path="/"
              element={
                <RequireAuth>
                  <Lazy>
                    <MapPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/system"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <SystemPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/users"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <UsersPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/api-tokens"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <ApiTokensPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/parse-errors"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <ParseErrorsPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/parser-test"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <ParserTestPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/geo-missing"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <GeoMissingPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/geo-ranges"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <GeoRangesPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route
              path="/reputation"
              element={
                <RequireAuth admin>
                  <Lazy>
                    <ReputationPage />
                  </Lazy>
                </RequireAuth>
              }
            />
            <Route path="/login.html" element={<Navigate to="/login" replace />} />
            <Route path="/index.html" element={<Navigate to="/" replace />} />
            <Route path="/system.html" element={<Navigate to="/system" replace />} />
            <Route path="/users.html" element={<Navigate to="/users" replace />} />
            <Route path="/api-tokens.html" element={<Navigate to="/api-tokens" replace />} />
            <Route path="/parse-errors.html" element={<Navigate to="/parse-errors" replace />} />
            <Route path="/parser-test.html" element={<Navigate to="/parser-test" replace />} />
            <Route path="/geo-missing.html" element={<Navigate to="/geo-missing" replace />} />
            <Route path="/geo-ranges.html" element={<Navigate to="/geo-ranges" replace />} />
            <Route path="/reputation.html" element={<Navigate to="/reputation" replace />} />
            <Route path="/change-password.html" element={<Navigate to="/change-password" replace />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </ToastProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
