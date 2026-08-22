import { expect, test } from '@playwright/test';
import { MAP_FIXTURE, installSessionMocks, loginAs, seedCsrf } from './helpers';

test.describe('map fixture / URL / session', () => {
  test('login shows fixture arcs in stats', async ({ page }) => {
    await installSessionMocks(page, 'administrator', { events: MAP_FIXTURE, geoCount: 1000 });
    await seedCsrf(page);
    await loginAs(page, 'admin');
    await expect(page.locator('#map-main')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('.map-info-dock')).toBeVisible();
    await page.getByRole('tab', { name: 'Статистика' }).click();
    await expect(page.locator('#stat-edges')).toHaveText('1');
    await expect(page.locator('#stat-total')).toHaveText('10');
  });

  test('query string is sent to /api/events', async ({ page }) => {
    await installSessionMocks(page, 'administrator', { events: MAP_FIXTURE });
    await seedCsrf(page);
    await loginAs(page, 'admin');
    await expect(page.locator('#map-main')).toBeVisible({ timeout: 15_000 });

    const eventsUrl = page.waitForRequest((req) => req.url().includes('/api/events?') && req.method() === 'GET');
    await page.goto('/?period=6h&group=ip&filter=blocked&q=Moscow&country=Russia');
    const req = await eventsUrl;
    const url = req.url();
    expect(url).toContain('hours=6');
    expect(url).toContain('group_by=ip');
    expect(url).toContain('filter=blocked');
    expect(url).toContain('q=Moscow');
    expect(url).toContain('country=Russia');
  });

  test('401 on /api/events redirects to login', async ({ page }) => {
    await installSessionMocks(page, 'administrator', { events: MAP_FIXTURE });
    await seedCsrf(page);
    await loginAs(page, 'admin');
    await expect(page.locator('#map-main')).toBeVisible({ timeout: 15_000 });

    await page.unroute(/\/api\/events(\/|\?|$)/);
    await page.route(/\/api\/events(\/|\?|$)/, async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'unauthorized' }),
      });
    });
    await page.goto('/?period=1h');
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
    expect(page.url()).toMatch(/next=/);
  });
});
