import { test, expect } from '@playwright/test';

test.describe('Navigation', () => {
	test('should load login page', async ({ page }) => {
		await page.goto('/login');
		await page.waitForLoadState('networkidle');

		// Should show the GoEasyERP header
		await expect(page.locator('h1')).toContainText('GoEasyERP');
	});

	test('should have responsive layout', async ({ page }) => {
		await page.goto('/login');
		await page.waitForLoadState('networkidle');

		// Check login card is visible
		const loginCard = page.locator('.login-card');
		await expect(loginCard).toBeVisible();

		// Check viewport responsiveness - mobile
		await page.setViewportSize({ width: 375, height: 667 });
		await expect(loginCard).toBeVisible();

		// Check viewport responsiveness - desktop
		await page.setViewportSize({ width: 1920, height: 1080 });
		await expect(loginCard).toBeVisible();
	});

	test('should have proper page structure', async ({ page }) => {
		await page.goto('/login');
		await page.waitForLoadState('networkidle');

		// Check basic structure exists
		await expect(page.locator('.login-container')).toBeVisible();
		await expect(page.locator('.login-card')).toBeVisible();
		await expect(page.locator('.login-header')).toBeVisible();
	});
});
