import { expect, test } from '@playwright/test';
import { installSessionMocks, loginAs, seedCsrf } from './helpers';

test.describe('geo wizard', () => {
  test('admin with empty geo sees wizard; Escape dismisses', async ({ page }) => {
    await installSessionMocks(page, 'administrator');
    await seedCsrf(page);
    await loginAs(page, 'admin');
    const dialog = page.getByRole('dialog', { name: 'Мастер GeoIP' });
    await expect(dialog).toBeVisible({ timeout: 15_000 });
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(page.locator('#map-main')).toBeVisible();
  });

  test('operator does not see geo wizard', async ({ page }) => {
    await installSessionMocks(page, 'operator');
    await seedCsrf(page);
    await loginAs(page, 'operator');
    await expect(page.locator('#map-main')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole('dialog', { name: 'Мастер GeoIP' })).toHaveCount(0);
  });
});
