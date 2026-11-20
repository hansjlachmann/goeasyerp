import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: process.env.CI ? 'github' : 'html',
	use: {
		baseURL: 'http://localhost:4173',
		trace: 'on-first-retry'
	},
	projects: [
		{
			name: 'chromium',
			// PLAYWRIGHT_CHANNEL=chrome: an installed Google Chrome instead of Playwright's Chromium (CI)
			use: { ...devices['Desktop Chrome'], channel: process.env.PLAYWRIGHT_CHANNEL || undefined }
		}
	],
	webServer: {
		command: 'npm run preview',
		url: 'http://localhost:4173',
		reuseExistingServer: !process.env.CI
	}
});
