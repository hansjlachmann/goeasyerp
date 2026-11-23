import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	// Consult https://kit.svelte.dev/docs/integrations#preprocessors
	// for more information about preprocessors
	preprocess: vitePreprocess(),

	// Suppress specific warnings for intentional patterns
	onwarn: (warning, handler) => {
		// Ignore a11y warnings for keyboard-navigable containers
		if (warning.code === 'a11y_no_noninteractive_tabindex') return;
		if (warning.code === 'a11y_no_noninteractive_element_interactions') return;
		// Ignore CSS warnings about Tailwind's @apply directive
		if (warning.code === 'css_unused_selector') return;
		handler(warning);
	},

	kit: {
		// adapter-node for Docker deployment
		adapter: adapter({
			out: 'build',
			precompress: false,
			envPrefix: 'VITE_'
		}),
		// Content Security Policy (S25): scripts only from the app itself — SvelteKit adds a nonce
		// to its own inline start-up script and to app.html's (%sveltekit.nonce%); styles may be
		// inline (Svelte style attributes); pictures, fonts and frames from the app, data: and
		// blob: (the print preview and print frame show a PDF made in the browser); the API on
		// the same origin; no plugins but that PDF, no <base>, forms only to the app, framed by
		// the app only (clickjacking).
		csp: {
			mode: 'auto',
			directives: {
				'default-src': ['self'],
				'script-src': ['self'],
				'style-src': ['self', 'unsafe-inline'],
				'img-src': ['self', 'data:', 'blob:'],
				'font-src': ['self', 'data:'],
				'connect-src': ['self'],
				'frame-src': ['self', 'blob:'],
				'object-src': ['self', 'blob:'],
				'base-uri': ['self'],
				'form-action': ['self'],
				'frame-ancestors': ['self']
			}
		},
		// The browser asks every minute whether a new version is on the server (the build's
		// version.json); the app then loads it by itself (src/lib/utils/appUpdate.ts)
		version: {
			pollInterval: 60_000
		},
		alias: {
			$components: 'src/lib/components',
			$stores: 'src/lib/stores',
			$services: 'src/lib/services',
			$types: 'src/lib/types',
			$utils: 'src/lib/utils'
		}
	}
};

export default config;
