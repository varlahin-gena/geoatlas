import { expect, test } from '@playwright/test';
import { CSRF, installSessionMocks, loginAs, seedCsrf } from './helpers';

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
