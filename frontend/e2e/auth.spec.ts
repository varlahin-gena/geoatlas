import { expect, test, type Page, type Route } from '@playwright/test';
import type { AuthUser } from '../src/api/types';

const CSRF = 'e2e-csrf-token';

async function fulfillJson(route: Route, status: number, body: unknown) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}

async function installSessionMocks(page: Page, role: AuthUser['role']) {
  let session: AuthUser | null = null;

  await page.route('**/config.js', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/javascript',
      body: 'window.NM_CONFIG={};',
    });
  });

  await page.route('**/api/auth/me', async (route) => {
    if (!session) {
      await route.fulfill({ status: 401, body: '' });
      return;
    }
    await fulfillJson(route, 200, session);
  });

  await page.route('**/api/auth/login', async (route) => {
    const req = route.request();
    const csrf = req.headers()['x-csrf-token'];
    if (csrf !== CSRF) {
      await fulfillJson(route, 403, { error: 'csrf required' });
      return;
    }
    const body = req.postDataJSON() as { username?: string; password?: string };
    session = {
      username: body.username || 'user',
      role,
      reputationEnabled: true,
    };
    await fulfillJson(route, 200, session);
  });

  await page.route('**/api/system/version', async (route) => {
    await fulfillJson(route, 200, { display: 'e2e', source: 'main', commit: 'deadbeef' });
  });

  await page.route('**/api/system/status', async (route) => {
    await fulfillJson(route, 200, { level: 'ok', alerts: [] });
  });

  await page.route('**/api/system/stats', async (route) => {
    await fulfillJson(route, 200, { alerts: [], uptime_sec: 1 });
  });

  await page.route('**/api/system/retention', async (route) => {
    await fulfillJson(route, 200, {
      traffic_logs_days: 30,
      edges_days: 30,
      parse_errors_days: 7,
      system_metrics_days: 7,
    });
  });

  await page.route(/\/api\/events(\/|\?|$)/, async (route) => {
    await fulfillJson(route, 200, { points: {}, lines: [], stats: { raw_pairs: 0, skipped_no_geo: 0 } });
  });

  await page.route(/\/api\/geo-ranges(\/|\?|$)/, async (route) => {
    await fulfillJson(route, 200, { count: 0, ranges: [], index_ready: true });
  });

  await page.route(/\/api\/me\/search-templates(\/|\?|$)/, async (route) => {
    await fulfillJson(route, 200, { templates: [] });
  });

  // Soft-fail remote basemap noise
  await page.route('https://basemaps.cartocdn.com/**', async (route) => {
    await route.abort();
  });
  await page.route('https://*.basemaps.cartocdn.com/**', async (route) => {
    await route.abort();
  });
}

async function seedCsrf(page: Page) {
  await page.context().addCookies([
    {
      name: 'nm_csrf',
      value: CSRF,
      url: 'http://127.0.0.1:5173',
    },
  ]);
}

async function loginAs(page: Page, username: string) {
  await page.goto('/login');
  await page.locator('#username').fill(username);
  await page.locator('#password').fill('password-ok');
  await page.locator('#submitBtn').click();
}

test.describe('auth / CSRF / roles', () => {
  test('login sends CSRF and reaches map', async ({ page }) => {
    await installSessionMocks(page, 'administrator');
    await seedCsrf(page);

    let sawCsrf = false;
    page.on('request', (req) => {
      if (req.url().includes('/api/auth/login') && req.method() === 'POST') {
        sawCsrf = req.headers()['x-csrf-token'] === CSRF;
      }
    });

    await loginAs(page, 'admin');
    await expect(page).toHaveURL(/\/$/);
    expect(sawCsrf).toBe(true);
    await expect(page.locator('#map-main')).toBeVisible({ timeout: 15_000 });
  });

  test('operator is blocked from /system', async ({ page }) => {
    await installSessionMocks(page, 'operator');
    await seedCsrf(page);
    await loginAs(page, 'operator');
    await expect(page).toHaveURL(/\/$/);

    await page.goto('/system');
    await expect(page).toHaveURL(/\/$/);
    await expect(page.locator('#map-main')).toBeVisible({ timeout: 15_000 });
  });

  test('admin can open /system', async ({ page }) => {
    await installSessionMocks(page, 'administrator');
    await seedCsrf(page);
    await loginAs(page, 'admin');
    await expect(page).toHaveURL(/\/$/);

    await page.goto('/system');
    await expect(page).toHaveURL(/\/system$/);
    await expect(page.getByText('Мониторинг системы')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('Обзор')).toBeVisible();
  });
});
