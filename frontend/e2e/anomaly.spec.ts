import { expect, test } from '@playwright/test';
import { MAP_FIXTURE, installSessionMocks, loginAs, seedCsrf } from './helpers';

test.describe('anomaly strip', () => {
  test('badge and На карте update query string', async ({ page }) => {
    await installSessionMocks(page, 'administrator', { events: MAP_FIXTURE });
    await page.unroute('**/api/anomalies/summary');
    await page.unroute(/\/api\/anomalies(\/|\?|$)/);
    await page.route('**/api/anomalies/summary', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ high: 1, warn: 0, total: 1, enabled: true, learning: false }),
      });
    });
    await page.route(/\/api\/anomalies(\/|\?|$)/, async (route) => {
      const url = route.request().url();
      if (url.includes('/summary')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ high: 1, warn: 0, total: 1, enabled: true, learning: false }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [
            {
              fingerprint: 'deadbeef',
              code: 'port_scan',
              severity: 'high',
              title: 'Сканирование портов с 203.0.113.5',
              detected_at: new Date().toISOString(),
              src_ip: '203.0.113.5',
              map: { period: '15m', group: 'ip', filter: 'all', q: 'src:203.0.113.5' },
            },
          ],
          summary: { high: 1, warn: 0, total: 1, enabled: true },
        }),
      });
    });
    await seedCsrf(page);
    await loginAs(page, 'admin');
    await expect(page.locator('#map-main')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('.anomaly-strip')).toContainText('критично');
    await page.locator('.anomaly-strip').click();
    await expect(page.locator('.anomaly-panel')).toBeVisible();
    await page.getByRole('button', { name: 'На карте' }).click();
    await expect(page).toHaveURL(/period=15m/);
    await expect(page).toHaveURL(/group=ip/);
    await expect(page).toHaveURL(/q=src/);
  });
});
