import { writable, get } from 'svelte/store';
import { browser } from '$app/env';
import { getString, setString } from '#lib/utils/storage.js';
import { api } from '#lib/services/api.js';
import {
	applyLayout,
	applyMode,
	cacheFor,
	themeAssetUrl,
	THEME_CACHE_KEY,
	type Appearance,
	type AppearanceSettings,
	type Mode,
	type Theme,
	type ThemeCache
} from '#lib/theme/apply.js';

// Same keys and default as the inline script in app.html (dark unless "light" is saved)
const THEME_STORAGE_KEY = 'theme';

function getInitialMode(): Mode {
	if (!browser) return 'dark';
	return getString(THEME_STORAGE_KEY) === 'light' ? 'light' : 'dark';
}

function readCache(): ThemeCache | null {
	try {
		const raw = getString(THEME_CACHE_KEY);
		return raw ? (JSON.parse(raw) as ThemeCache) : null;
	} catch {
		return null;
	}
}

let cache: ThemeCache | null = browser ? readCache() : null;

/** The user's appearance from the backend (null until loaded / logged out) */
export const appearance = writable<Appearance | null>(null);

/** URL of the wallpaper shown now (the active theme's chosen wallpaper), null for none */
export const wallpaperUrl = writable<string | null>(null);

/**
 * The wallpaper of a mode: the setting names bg-1 … bg-5 of the active theme; a theme with
 * fewer wallpapers shows its first, a theme without wallpapers none.
 */
export function wallpaperFor(a: Appearance | null, mode: Mode): string | null {
	const name = a?.settings.wallpaper;
	const th = a ? (mode === 'light' ? a.light : a.dark) : null;
	if (!name || !th || th.source !== 'github' || !th.wallpapers) return null;
	const n = Number(name.slice(3));
	return themeAssetUrl(th, `bg-${n >= 1 && n <= th.wallpapers ? n : 1}`);
}

/** Wallpaper layer, its opacity/blur and the panel opacity (CSS variables on <html>) */
function applyWallpaper(mode: Mode) {
	const a = get(appearance);
	const url = wallpaperFor(a, mode);
	wallpaperUrl.set(url);
	const html = document.documentElement;
	html.classList.toggle('has-wallpaper', !!url);
	if (url && a) {
		html.style.setProperty('--wp-opacity', String(Math.min(Math.max(a.settings.wallpaper_opacity, 0), 100) / 100));
		html.style.setProperty('--wp-blur', `${Math.min(Math.max(a.settings.wallpaper_blur, 0), 20)}px`);
		html.style.setProperty('--panel-alpha', String(Math.min(Math.max(a.settings.panel_opacity, 75), 100) / 100));
	} else {
		for (const v of ['--wp-opacity', '--wp-blur', '--panel-alpha']) html.style.removeProperty(v);
	}
}

function show(mode: Mode) {
	if (!browser) return;
	applyMode(mode, cache?.[mode] ?? {});
	applyLayout(cache?.classes ?? [], cache?.scale ?? 1);
	applyWallpaper(mode);
	setString(THEME_STORAGE_KEY, mode);
	if (cache) {
		cache.mode = mode;
		setString(THEME_CACHE_KEY, JSON.stringify(cache));
	}
}

/** Takes an appearance from the backend: caches both slots' colors and shows its mode */
function adopt(a: Appearance) {
	appearance.set(a);
	cache = cacheFor(a);
	modeStore.set(a.settings.mode);
	show(a.settings.mode);
}

const modeStore = writable<Mode>(getInitialMode());

/** The current mode (light/dark); toggle() switches between the user's two theme slots */
export const theme = {
	subscribe: modeStore.subscribe,
	toggle: () => {
		const mode: Mode = get(modeStore) === 'light' ? 'dark' : 'light';
		modeStore.set(mode);
		show(mode);
		if (get(appearance)) {
			saveAppearance({ mode }).catch(() => {});
		}
	},
	set: (mode: Mode) => {
		modeStore.set(mode);
		show(mode);
	}
};

/**
 * Loads the user's appearance after login / app start. A user without a saved record
 * keeps the mode this browser had (before themes, the mode lived only here).
 */
export async function loadAppearance() {
	const a = await api.getAppearance();
	if (!a.saved && a.settings.mode !== get(modeStore)) {
		a.settings.mode = get(modeStore);
		adopt(a);
		saveAppearance({ mode: a.settings.mode }).catch(() => {});
		return;
	}
	adopt(a);
}

/** Saves settings; the backend checks them and returns the new appearance, which is shown */
export async function saveAppearance(patch: Partial<AppearanceSettings>): Promise<Appearance> {
	const a = await api.saveAppearance(patch);
	adopt(a);
	return a;
}

/** Chooses a theme: it takes the slot of its mode, and the app switches to that mode */
export function chooseTheme(t: Theme): Promise<Appearance> {
	const slot = t.mode === 'light' ? 'light_theme' : 'dark_theme';
	return saveAppearance({ [slot]: t.code, mode: t.mode });
}

/** Forgets the user's appearance (logout); the cached colors stay for the login page */
export function clearAppearance() {
	appearance.set(null);
	if (browser) applyWallpaper(get(modeStore));
}

// Initialize the mode on page load (app.html has applied the cached colors already)
if (browser) {
	applyMode(getInitialMode(), cache?.[getInitialMode()] ?? {});
}
