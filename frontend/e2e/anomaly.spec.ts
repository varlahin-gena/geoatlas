import { expect, test } from '@playwright/test';
import { MAP_FIXTURE, installSessionMocks, loginAs, seedCsrf } from './helpers';

test.describe('anomaly strip', () => {
  test('badge and На карте update query string', async ({ page }) => {
    await installSessionMocks(page, 'administrator', { events: MAP_FIXTURE });
    // Non-empty GeoIP so the empty-geo wizard does not block the anomaly strip.
    await page.unroute(/\/api\/geo-ranges(\/|\?|$)/);
    await page.route(/\/api\/geo-ranges(\/|\?|$)/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ count: 1000, ranges: [], index_ready: true }),
      });
    });
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
    await expect(page.locator('.anomaly-strip')).toHaveCount(0);
    const panelBox = await page.locator('.anomaly-panel').boundingBox();
    const legendBox = await page.locator('.legend').boundingBox();
    expect(panelBox).toBeTruthy();
    expect(legendBox).toBeTruthy();
    if (panelBox && legendBox) {
      const overlap =
        panelBox.x < legendBox.x + legendBox.width &&
        panelBox.x + panelBox.width > legendBox.x &&
        panelBox.y < legendBox.y + legendBox.height &&
        panelBox.y + panelBox.height > legendBox.y;
      expect(overlap).toBe(false);
    }
    await page.getByRole('button', { name: 'На карте' }).click();
    await expect(page).toHaveURL(/q=src/);
    await expect(page).toHaveURL(/group=ip/);
    // Keep the current (wider) map period; default 1d is omitted from the query string.
    await expect(page).not.toHaveURL(/period=15m/);
  });
});
