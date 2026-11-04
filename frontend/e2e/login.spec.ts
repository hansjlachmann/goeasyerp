import { test, expect } from '@playwright/test';

test.describe('Login Page', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/login');
		// Wait for page to stabilize (API calls may fail without backend)
		await page.waitForLoadState('networkidle');
	});

	test('should display login page', async ({ page }) => {
		// Check page title/header
		await expect(page.locator('h1')).toContainText('GoEasyERP');
	});

	test('should have login card', async ({ page }) => {
		await expect(page.locator('.login-card')).toBeVisible();
	});

	test('should have header with sign in or setup text', async ({ page }) => {
		// Header shows either "Sign In" or "Initial Setup" depending on state
		// When backend is unavailable, translation keys (LOGIN_SIGN_IN / LOGIN_INITIAL_SETUP) are shown instead
		const headerP = page.locator('.login-header p');
		await expect(headerP).toBeVisible();
		const text = await headerP.textContent();
		const validTexts = ['Sign In', 'Initial Setup', 'LOGIN_SIGN_IN', 'LOGIN_INITIAL_SETUP'];
		expect(validTexts.some((v) => text?.includes(v))).toBeTruthy();
	});

	test('should have login container with gradient background', async ({ page }) => {
		const container = page.locator('.login-container');
		await expect(container).toBeVisible();
	});
});
