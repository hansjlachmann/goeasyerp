import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { resolve } from 'path';

export default defineConfig({
	plugins: [svelte()],
	test: {
		globals: true,
		environment: 'jsdom',
		include: ['src/**/*.{test,spec}.{js,ts}'],
		coverage: {
			provider: 'v8',
			reporter: ['text', 'lcov'],
			reportsDirectory: './coverage'
		}
	},
	resolve: {
		alias: {
			$lib: resolve('./src/lib'),
			$components: resolve('./src/lib/components'),
			$stores: resolve('./src/lib/stores'),
			$services: resolve('./src/lib/services'),
			$types: resolve('./src/lib/types'),
			$utils: resolve('./src/lib/utils')
		}
	}
});
