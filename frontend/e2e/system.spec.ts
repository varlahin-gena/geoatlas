import { expect, test } from '@playwright/test';
import { installSessionMocks, loginAs, seedCsrf } from './helpers';

test.describe('system page', () => {
  test('admin opens overview and charts tab without crash', async ({ page }) => {
    await installSessionMocks(page, 'administrator', { geoCount: 100 });
    await page.route('**/api/system/stats', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          alerts: [],
          uptime_sec: 120,
          pipeline: {
            rate: { events_per_sec: 42, drops_per_sec: 0, buffer_drops_per_sec: 0 },
            ingest: {
              lag_sec: 1,
              queue_depth: 10,
              queue_capacity: 1000,
              buffered_lines: 0,
            },
            syslogng: { events_per_sec: 40, drops_per_sec: 0 },
          },
          health: { backend: { ok: 1 }, ingest: { ok: 1 }, syslogng: { up: 1 } },
          storage: {},
        }),
      });
    });
    await seedCsrf(page);
    await loginAs(page, 'admin');

    await page.goto('/system');
    await expect(page.getByRole('heading', { name: 'Мониторинг системы' })).toBeVisible({
      timeout: 15_000,
    });
    await expect(page.locator('.status-strip')).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Обзор' })).toHaveAttribute('aria-selected', 'true');

    await page.getByRole('tab', { name: 'Графики' }).click();
    await expect(page.getByRole('tab', { name: 'Графики' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.locator('.tab-panel')).toBeVisible();
  });
});
