import { Suspense } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { AuthProvider } from '@/auth/AuthContext';
import { RequireAuth } from '@/auth/RequireAuth';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ToastProvider } from '@/components/Toast';
import { CommandPaletteHost } from '@/components/CommandPalette';
import { PageLoading } from '@/components/Skeleton';
import { lazyRetry } from '@/lib/lazyRetry';
import LoginPage from '@/pages/Login/LoginPage';
import ChangePasswordPage from '@/pages/ChangePassword/ChangePasswordPage';

const ProfilePage = lazyRetry(() => import('@/pages/Profile/ProfilePage'), { label: 'профиль' });
const MapPage = lazyRetry(() => import('@/pages/Map/MapPage'), { label: 'карта' });

const SystemPage = lazyRetry(() => import('@/pages/System/SystemPage'), { label: 'система' });
const UsersPage = lazyRetry(() => import('@/pages/Users/UsersPage'), { label: 'пользователи' });
const ApiTokensPage = lazyRetry(() => import('@/pages/ApiTokens/ApiTokensPage'), {
  label: 'API-токены',
});
const TlsPage = lazyRetry(() => import('@/pages/Tls/TlsPage'), { label: 'HTTPS-сертификаты' });
const ParseErrorsPage = lazyRetry(() => import('@/pages/ParseErrors/ParseErrorsPage'), {
  label: 'ошибки парсинга',
});
const ParserTestPage = lazyRetry(() => import('@/pages/ParserTest/ParserTestPage'), {
  label: 'тест парсеров',
});
const GeoMissingPage = lazyRetry(() => import('@/pages/GeoMissing/GeoMissingPage'), {
  label: 'IP без координат',
});
const GeoRangesPage = lazyRetry(() => import('@/pages/GeoRanges/GeoRangesPage'), {
  label: 'база GeoIP',
});
const ReputationPage = lazyRetry(() => import('@/pages/Reputation/ReputationPage'), {
  label: 'репутация',
});
const ConnectorsPage = lazyRetry(() => import('@/pages/Connectors/ConnectorsPage'), {
  label: 'коннекторы',
});
const AnomaliesPage = lazyRetry(() => import('@/pages/Anomalies/AnomaliesPage'), {
  label: 'аномалии',
});
const AnomalyEnginePage = lazyRetry(() => import('@/pages/Anomalies/AnomalyEnginePage'), {
  label: 'движок аномалий',
});
const HuntsPage = lazyRetry(() => import('@/pages/Hunts/HuntsPage'), { label: 'охоты' });
const InvestigatePage = lazyRetry(() => import('@/pages/Investigate/InvestigatePage'), {
  label: 'разбор',
});
const NotFoundPage = lazyRetry(() => import('@/pages/NotFound/NotFoundPage'), { label: '404' });

function Lazy({ children, label }: { children: React.ReactNode; label?: string }) {
  return (
    <ErrorBoundary label={label}>
      <Suspense fallback={<PageLoading />}>{children}</Suspense>
    </ErrorBoundary>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <ErrorBoundary label="приложение">
        <AuthProvider>
          <ToastProvider>
            <CommandPaletteHost />
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
                path="/profile"
                element={
                  <RequireAuth>
                    <Lazy label="профиль">
                      <ProfilePage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/"
                element={
                  <RequireAuth>
                    <Lazy label="карта">
                      <MapPage />
                    </Lazy>
                  </RequireAuth>
                }
              />

              <Route
                path="/system"
                element={
                  <RequireAuth admin>
                    <Lazy label="система">
                      <SystemPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/users"
                element={
                  <RequireAuth admin>
                    <Lazy label="пользователи">
                      <UsersPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/api-tokens"
                element={
                  <RequireAuth admin>
                    <Lazy label="API-токены">
                      <ApiTokensPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/tls"
                element={
                  <RequireAuth admin>
                    <Lazy label="HTTPS-сертификаты">
                      <TlsPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/parse-errors"
                element={
                  <RequireAuth admin>
                    <Lazy label="ошибки парсинга">
                      <ParseErrorsPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/parser-test"
                element={
                  <RequireAuth admin>
                    <Lazy label="тест парсеров">
                      <ParserTestPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/geo-missing"
                element={
                  <RequireAuth admin>
                    <Lazy label="IP без координат">
                      <GeoMissingPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/geo-ranges"
                element={
                  <RequireAuth admin>
                    <Lazy label="база GeoIP">
                      <GeoRangesPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/anomalies"
                element={
                  <RequireAuth>
                    <Lazy label="анomalies">
                      <AnomaliesPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/anomalies/engine"
                element={
                  <RequireAuth admin>
                    <Lazy label="движок аномалий">
                      <AnomalyEnginePage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/hunts"
                element={
                  <RequireAuth>
                    <Lazy label="охоты">
                      <HuntsPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/investigate"
                element={
                  <RequireAuth>
                    <Lazy label="разбор">
                      <InvestigatePage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/reputation"
                element={
                  <RequireAuth admin>
                    <Lazy label="репутация">
                      <ReputationPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route
                path="/connectors"
                element={
                  <RequireAuth admin>
                    <Lazy label="коннекторы">
                      <ConnectorsPage />
                    </Lazy>
                  </RequireAuth>
                }
              />
              <Route path="/login.html" element={<Navigate to="/login" replace />} />
              <Route path="/index.html" element={<Navigate to="/" replace />} />
              <Route path="/system.html" element={<Navigate to="/system" replace />} />
              <Route path="/users.html" element={<Navigate to="/users" replace />} />
              <Route path="/api-tokens.html" element={<Navigate to="/api-tokens" replace />} />
              <Route path="/tls.html" element={<Navigate to="/tls" replace />} />
              <Route path="/parse-errors.html" element={<Navigate to="/parse-errors" replace />} />
              <Route path="/parser-test.html" element={<Navigate to="/parser-test" replace />} />
              <Route path="/geo-missing.html" element={<Navigate to="/geo-missing" replace />} />
              <Route path="/geo-ranges.html" element={<Navigate to="/geo-ranges" replace />} />
              <Route path="/reputation.html" element={<Navigate to="/reputation" replace />} />
              <Route path="/anomalies.html" element={<Navigate to="/anomalies" replace />} />
              <Route path="/investigate.html" element={<Navigate to="/investigate" replace />} />
              <Route path="/change-password.html" element={<Navigate to="/change-password" replace />} />
              <Route
                path="*"
                element={
                  <Lazy label="404">
                    <NotFoundPage />
                  </Lazy>
                }
              />
            </Routes>
          </ToastProvider>
        </AuthProvider>
      </ErrorBoundary>
    </BrowserRouter>
  );
}
