/** @type {import('tailwindcss').Config} */

// Colors are CSS variables holding RGB triplets ("18 18 18"), set in src/app.css: the
// default values are the classic ERP palette (light and dark mode, sampled from
// the reference screenshots), and a theme
// (src/lib/theme) overrides them at run time. Components keep using Tailwind's
// gray/blue/primary classes; `<alpha-value>` keeps opacity modifiers (bg-gray-900/50) working.
//
// Neutrals (--gray-N): each step serves its light-mode role (text/borders on white) and its
// dark-mode role (surfaces/borders/text on the dark page):
//   50  light: secondary bg, dark: primary text   100 light: hover background
//   200 light: neutral tile, borders              300 light: divider
//   400 dark: secondary text                      500 muted text
//   600 light: secondary text                     700 dark: separators, inputs
//   800 dark: raised surface                      900 dark: page background
// Accent (--accent-N, teal): 400 = dark-mode links, 500 = tiles, 600 = light-mode
// links/actions, 900 = dark-mode notification bar.
const steps = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950];
const scale = (name) =>
	Object.fromEntries(steps.map((n) => [n, `rgb(var(--${name}-${n}) / <alpha-value>)`]));
const color = (name) => `rgb(var(--${name}) / <alpha-value>)`;

export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	darkMode: 'class',
	theme: {
		extend: {
			colors: {
				gray: scale('gray'),
				blue: scale('accent'),
				primary: scale('accent'),
				surface: color('surface'), // panels, cards, dropdowns (light mode: white)
				page: color('page-bg'), // page background
				fg: color('fg'), // body text
				heading: color('heading'), // page and section titles (light mode)
				appbar: { DEFAULT: color('appbar'), fg: color('appbar-fg') } // app bar
			},
			fontFamily: {
				sans: ['var(--font-sans)'],
				mono: ['var(--font-mono)']
			}
		}
	},
	plugins: []
};
