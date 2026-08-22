import { type Page, type Route } from '@playwright/test';
import type { AuthUser } from '../src/api/types';

export const CSRF = 'e2e-csrf-token';

export const MAP_FIXTURE = {
  points: {
    A: { lat: 55.75, lon: 37.62, country: 'Russia', city: 'Moscow', count: 10 },
    B: { lat: 52.52, lon: 13.4, country: 'Germany', city: 'Berlin', count: 10 },
  },
  lines: [
    {
      src: 'A',
      dst: 'B',
      src_lat: 55.75,
      src_lon: 37.62,
      dst_lat: 52.52,
      dst_lon: 13.4,
      count: 10,
      status: 'allowed',
      src_country: 'Russia',
      dst_country: 'Germany',
    },
  ],
  stats: { raw_pairs: 1, skipped_no_geo: 0 },
};

async function fulfillJson(route: Route, status: number, body: unknown) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}

export async function installSessionMocks(
  page: Page,
  role: AuthUser['role'],
  opts?: { events?: unknown; eventsStatus?: number; geoCount?: number },
) {
  let session: AuthUser | null = null;
  const eventsBody = opts?.events ?? {
    points: {},
    lines: [],
    stats: { raw_pairs: 0, skipped_no_geo: 0 },
  };
  const eventsStatus = opts?.eventsStatus ?? 200;
  const geoCount = opts?.geoCount ?? 0;

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
    if (eventsStatus !== 200) {
      await fulfillJson(route, eventsStatus, { error: 'unauthorized' });
      return;
    }
    await fulfillJson(route, 200, eventsBody);
  });

  await page.route(/\/api\/geo-ranges(\/|\?|$)/, async (route) => {
    await fulfillJson(route, 200, { count: geoCount, ranges: [], index_ready: true });
  });

  await page.route('**/api/auth/geo-wizard-dismiss', async (route) => {
    if (route.request().method() !== 'POST') {
      await route.fallback();
      return;
    }
    const body = route.request().postDataJSON() as { dismissed?: boolean };
    if (session) {
      session = { ...session, geo_wizard_dismissed: Boolean(body.dismissed) };
    }
    await fulfillJson(route, 200, session || { username: 'anon', role: 'administrator', reputationEnabled: true });
  });

  await page.route(/\/api\/me\/search-templates(\/|\?|$)/, async (route) => {
    await fulfillJson(route, 200, { templates: [] });
  });

  await page.route('**/api/anomalies/summary', async (route) => {
    await fulfillJson(route, 200, { high: 0, warn: 0, total: 0, enabled: true, learning: false });
  });
  await page.route(/\/api\/anomalies(\/|\?|$)/, async (route) => {
    const url = route.request().url();
    if (url.includes('/ack')) {
      await fulfillJson(route, 200, { ok: true });
      return;
    }
    if (url.includes('/summary')) {
      await fulfillJson(route, 200, { high: 0, warn: 0, total: 0, enabled: true, learning: false });
      return;
    }
    await fulfillJson(route, 200, { items: [], summary: { high: 0, warn: 0, total: 0, enabled: true } });
  });

  await page.route('https://basemaps.cartocdn.com/**', async (route) => {
    await route.abort();
  });
  await page.route('https://*.basemaps.cartocdn.com/**', async (route) => {
    await route.abort();
  });
}

export async function seedCsrf(page: Page) {
  await page.context().addCookies([
    {
      name: 'nm_csrf',
      value: CSRF,
      domain: '127.0.0.1',
      path: '/',
    },
  ]);
}

export async function loginAs(page: Page, username: string) {
  await page.goto('/login');
  await page.locator('#username').fill(username);
  await page.locator('#password').fill('password-ok');
  await page.locator('#submitBtn').click();
}
