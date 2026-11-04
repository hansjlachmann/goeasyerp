/**
 * Color themes: the backend sends a theme's colors as tokens (backend/foundation/themes);
 * they become CSS variables on <html> (RGB triplets, see src/app.css), which the Tailwind
 * colors and component styles use.
 */

export type Mode = 'light' | 'dark';

export const STEPS = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950] as const;

export interface ThemeTokens {
	version: number;
	mode: Mode;
	gray: string[];
	accent: string[];
	vars: Record<string, string>;
}

export interface Theme {
	code: string;
	name: string;
	mode: Mode;
	source: 'builtin' | 'github';
	author?: string;
	repo_url?: string;
	/** Commit an installed theme is pinned to (asset URLs carry it) */
	version?: string;
	/** Number of wallpapers (assets bg-1 … bg-n) */
	wallpapers: number;
	tokens: ThemeTokens;
}

export interface ThemeList {
	themes: Theme[];
	can_install: boolean;
	can_refresh: boolean;
	can_delete: boolean;
}

export interface ThemeInstallResult {
	code: string;
	name: string;
	mode: Mode;
	commit: string;
	unchanged: boolean;
}

/** URL of an image of an installed theme (preview, thumb, bg-1 …), cache-busted by commit */
export function themeAssetUrl(theme: Pick<Theme, 'code' | 'version'>, name: string): string {
	return `/api/themes/${encodeURIComponent(theme.code)}/assets/${encodeURIComponent(name)}?v=${encodeURIComponent(theme.version ?? '')}`;
}

export interface AppearanceSettings {
	light_theme: string;
	dark_theme: string;
	mode: Mode;
	wallpaper: string;
	wallpaper_opacity: number;
	wallpaper_blur: number;
	panel_opacity: number;
	font: string;
	font_size: number;
	density: 'comfortable' | 'compact';
}

export interface Appearance {
	settings: AppearanceSettings;
	light: Theme;
	dark: Theme;
}

/** The semantic variables a theme may set (names without "--"), as in app.css */
export const THEME_VARS = [
	'page-bg',
	'surface',
	'fg',
	'heading',
	'appbar',
	'appbar-fg',
	'row-selected',
	'row-selected-hover',
	'text-sel-bg',
	'text-sel-fg',
	'scroll-track',
	'scroll-thumb',
	'scroll-thumb-hover'
] as const;

const HEX = /^#([0-9a-f]{6})$/i;

/** "#1a1b26" → "26 27 38"; anything that is not #rrggbb → null (never passed on) */
export function hexToTriplet(hex: string): string | null {
	const m = HEX.exec(hex);
	if (!m) return null;
	const n = parseInt(m[1], 16);
	return `${(n >> 16) & 255} ${(n >> 8) & 255} ${n & 255}`;
}

/** The CSS variables of a theme: { '--gray-50': '247 247 247', …, '--fg': '33 33 33' } */
export function tokensToVars(tokens: ThemeTokens): Record<string, string> {
	const out: Record<string, string> = {};
	STEPS.forEach((step, i) => {
		const g = hexToTriplet(tokens.gray?.[i] ?? '');
		const a = hexToTriplet(tokens.accent?.[i] ?? '');
		if (g) out[`--gray-${step}`] = g;
		if (a) out[`--accent-${step}`] = a;
	});
	for (const name of THEME_VARS) {
		const v = hexToTriplet(tokens.vars?.[name] ?? '');
		if (v) out[`--${name}`] = v;
	}
	return out;
}

/** Every variable a theme can set — cleared before another theme is applied */
export const ALL_THEME_VAR_NAMES = [
	...STEPS.map((s) => `--gray-${s}`),
	...STEPS.map((s) => `--accent-${s}`),
	...THEME_VARS.map((n) => `--${n}`)
];

/** Sets the theme's variables on el (default <html>); variables it lacks fall back to app.css */
export function applyVars(vars: Record<string, string>, el: HTMLElement = document.documentElement) {
	for (const name of ALL_THEME_VAR_NAMES) {
		if (vars[name]) el.style.setProperty(name, vars[name]);
		else el.style.removeProperty(name);
	}
}

/** Switches <html> to mode and applies the variables */
export function applyMode(mode: Mode, vars: Record<string, string>, el: HTMLElement = document.documentElement) {
	el.classList.toggle('dark', mode === 'dark');
	applyVars(vars, el);
}

/**
 * localStorage cache read by the inline script in app.html, so a page load paints in the
 * user's theme before the app has started: { mode, light: vars, dark: vars }.
 */
export const THEME_CACHE_KEY = 'theme-cache';

export interface ThemeCache {
	mode: Mode;
	light: Record<string, string>;
	dark: Record<string, string>;
	/** Classes on <html>: font-*, density-compact (app.css) */
	classes?: string[];
	/** Font size factor (1 = default) */
	scale?: number;
}

/** The fonts of the Appearance dialog; "default" is --font-sans of app.css (Inter, the brand font) */
export const FONTS = ['default', 'system', 'inter', 'ibm_plex_sans', 'serif', 'monospace'] as const;

/** Font size choices in percent (0 = default, 100 %) */
export const FONT_SIZES = [0, 90, 110, 125] as const;

/** Classes and font scale of the settings (font, density) */
export function layoutOf(settings: AppearanceSettings): { classes: string[]; scale: number } {
	const classes: string[] = [];
	if (settings.font && settings.font !== 'default' && (FONTS as readonly string[]).includes(settings.font)) {
		classes.push(`font-${settings.font.replace(/_/g, '-')}`);
	}
	if (settings.density === 'compact') classes.push('density-compact');
	const pct = settings.font_size >= 80 && settings.font_size <= 150 ? settings.font_size : 100;
	return { classes, scale: pct / 100 };
}

/** Classes on <html> the appearance owns (removed before new ones are added) */
export const LAYOUT_CLASS = /^(font-[a-z-]+|density-compact)$/;

export function applyLayout(classes: string[], scale: number, el: HTMLElement = document.documentElement) {
	for (const c of [...el.classList]) if (LAYOUT_CLASS.test(c)) el.classList.remove(c);
	for (const c of classes) if (LAYOUT_CLASS.test(c)) el.classList.add(c);
	if (scale !== 1) el.style.setProperty('--font-scale', String(scale));
	else el.style.removeProperty('--font-scale');
}

export function cacheFor(appearance: Appearance): ThemeCache {
	const { classes, scale } = layoutOf(appearance.settings);
	return {
		mode: appearance.settings.mode,
		light: tokensToVars(appearance.light.tokens),
		dark: tokensToVars(appearance.dark.tokens),
		classes,
		scale
	};
}
