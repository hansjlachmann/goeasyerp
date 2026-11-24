import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import {
	ALL_THEME_VAR_NAMES,
	FONTS,
	STEPS,
	applyLayout,
	cacheFor,
	layoutOf,
	hexToTriplet,
	tokensToVars,
	type Appearance,
	type Theme,
	type ThemeTokens
} from '../apply';
import { APPEARANCE, MENU } from '#lib/services/i18n.svelte.js';

const tokens = (over: Partial<ThemeTokens> = {}): ThemeTokens => ({
	version: 1,
	mode: 'dark',
	gray: STEPS.map(() => '#1a1b26'),
	accent: STEPS.map(() => '#7aa2f7'),
	vars: { 'page-bg': '#1a1b26', fg: '#c0caf5' },
	...over
});

describe('hexToTriplet', () => {
	it('converts #rrggbb', () => {
		expect(hexToTriplet('#1a1b26')).toBe('26 27 38');
		expect(hexToTriplet('#FFFFFF')).toBe('255 255 255');
	});
	it('rejects anything else', () => {
		for (const bad of ['', 'red', '#fff', '1a1b26', '#1a1b26; color: red', 'url(x)', '#1a1b2g']) {
			expect(hexToTriplet(bad)).toBeNull();
		}
	});
});

describe('tokensToVars', () => {
	it('maps ramps and semantic colors to CSS variables', () => {
		const v = tokensToVars(tokens());
		expect(v['--gray-50']).toBe('26 27 38');
		expect(v['--accent-950']).toBe('122 162 247');
		expect(v['--page-bg']).toBe('26 27 38');
		expect(v['--fg']).toBe('192 202 245');
	});
	it('drops invalid values and unknown names', () => {
		const v = tokensToVars(
			tokens({
				vars: { fg: 'red', 'page-bg': '#000000; display:none', 'evil-var': '#000000', surface: '#ffffff' }
			})
		);
		expect(v['--fg']).toBeUndefined();
		expect(v['--page-bg']).toBeUndefined();
		expect(v['--evil-var']).toBeUndefined();
		expect(v['--surface']).toBe('255 255 255');
	});
});

describe('theme variables and app.css', () => {
	const css = readFileSync(resolve(__dirname, '../../../app.css'), 'utf8');
	const root = css.slice(css.indexOf(':root {'), css.indexOf('}', css.indexOf(':root {')));

	it('every variable a theme sets has a default in app.css', () => {
		const missing = ALL_THEME_VAR_NAMES.filter((n) => !new RegExp(`^\\s+${n}:`, 'm').test(root));
		expect(missing).toEqual([]);
	});

	it('the cache passes the checks of the inline script in app.html', () => {
		const html = readFileSync(resolve(__dirname, '../../../app.html'), 'utf8');
		expect(html).toContain("localStorage.getItem('theme-cache')");
		const name = /^--[a-z0-9-]{1,40}$/;
		const value = /^\d{1,3} \d{1,3} \d{1,3}$/;
		const theme = (mode: 'light' | 'dark'): Theme => ({
			code: mode,
			name: mode,
			mode,
			source: 'builtin',
			wallpapers: 0,
			tokens: tokens({ mode, vars: Object.fromEntries(ALL_THEME_VAR_NAMES.filter((n) => !/gray|accent/.test(n)).map((n) => [n.slice(2), '#123456'])) })
		});
		const a: Appearance = {
			settings: {
				light_theme: 'light', dark_theme: 'dark', mode: 'light', wallpaper: '', wallpaper_opacity: 25,
				wallpaper_blur: 0, panel_opacity: 92, font: 'default', font_size: 0, density: 'comfortable'
			},
			light: theme('light'),
			dark: theme('dark')
		};
		const cache = cacheFor(a);
		expect(cache.mode).toBe('light');
		for (const vars of [cache.light, cache.dark]) {
			expect(Object.keys(vars).length).toBe(ALL_THEME_VAR_NAMES.length);
			for (const [n, v] of Object.entries(vars)) {
				expect(name.test(n)).toBe(true);
				expect(value.test(v)).toBe(true);
			}
		}
	});
});

describe('appearance translations', () => {
	for (const lang of ['en-US', 'nb-NO', 'da-DK']) {
		it(`has every key in ${lang}`, () => {
			const file = readFileSync(resolve(__dirname, `../../../../../translations/${lang}/messages.yaml`), 'utf8');
			const keys = [...Object.values(APPEARANCE), MENU.APPEARANCE];
			expect(keys.filter((k) => !new RegExp(`^\\s+${k}:`, 'm').test(file))).toEqual([]);
		});
	}
});

describe('font, size and density', () => {
	const base = {
		light_theme: 'CLASSIC-LIGHT', dark_theme: 'CLASSIC-DARK', mode: 'dark' as const, wallpaper: '', wallpaper_opacity: 25,
		wallpaper_blur: 0, panel_opacity: 92, font: 'default', font_size: 0, density: 'comfortable' as const
	};
	it('defaults change nothing', () => {
		expect(layoutOf(base)).toEqual({ classes: [], scale: 1 });
	});
	it('maps the settings to classes and a scale', () => {
		expect(layoutOf({ ...base, font: 'ibm_plex_sans', font_size: 125, density: 'compact' })).toEqual({
			classes: ['font-ibm-plex-sans', 'density-compact'],
			scale: 1.25
		});
	});
	it('ignores unknown fonts and sizes out of range', () => {
		expect(layoutOf({ ...base, font: 'comic"; x', font_size: 900 })).toEqual({ classes: [], scale: 1 });
	});
	it('every font has a rule in app.css', () => {
		const css = readFileSync(resolve(__dirname, '../../../app.css'), 'utf8');
		for (const f of FONTS.filter((f) => f !== 'default')) {
			expect(css).toContain(`html.font-${f.replace(/_/g, '-')} {`);
		}
	});
	it('applyLayout replaces only its own classes', () => {
		const el = document.createElement('html');
		el.classList.add('dark', 'font-serif', 'has-wallpaper');
		applyLayout(['font-inter', 'density-compact', 'evil class'], 1.1, el);
		expect([...el.classList].sort()).toEqual(['dark', 'density-compact', 'font-inter', 'has-wallpaper']);
		expect(el.style.getPropertyValue('--font-scale')).toBe('1.1');
		applyLayout([], 1, el);
		expect([...el.classList].sort()).toEqual(['dark', 'has-wallpaper']);
		expect(el.style.getPropertyValue('--font-scale')).toBe('');
	});
});
