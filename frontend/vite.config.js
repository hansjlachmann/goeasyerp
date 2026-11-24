import adapter from '@sveltejs/adapter-node';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		// SvelteKit's configuration (SvelteKit 3: here, no svelte.config.js)
		sveltekit({
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

			// adapter-node for Docker deployment
			adapter: adapter({
				out: 'build',
				precompress: false,
				envPrefix: 'VITE_'
			}),
			// Content Security Policy (S25): scripts only from the app itself — SvelteKit adds a
			// nonce to its own inline start-up script and to app.html's (%sveltekit.nonce%); styles
			// may be inline (Svelte style attributes); pictures, fonts and frames from the app,
			// data: and blob: (the print preview and print frame show a PDF made in the browser);
			// the API on the same origin; no plugins but that PDF, no <base>, forms only to the
			// app, framed by the app only (clickjacking).
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
			}
		})
	],
	server: {
		host: '0.0.0.0', // Required for Docker
		port: 5173,
		proxy: {
			// Proxy API requests to Go backend
			'/api': {
				target: process.env.VITE_API_URL || 'http://localhost:8080',
				changeOrigin: true
			},
			'/ws': {
				target: process.env.VITE_WS_URL || 'ws://localhost:8080',
				ws: true
			}
		}
	}
});
