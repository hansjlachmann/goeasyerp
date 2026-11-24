// A new version on the server: the browser loads it by itself. SvelteKit checks the server's
// version every minute (vite.config.js, version.pollInterval) and sets updated.current; the
// page is then loaded again where that loses nothing — the login page at once, any other page
// on its next navigation (a card or list being edited keeps its input, a report or routine
// started from the main menu its dialog, until the user moves on). Without it a tab open
// during an upgrade, or a page from the browser's cache, ran the old scripts against the new
// server (raw text keys, the first-user setup on the login page) until the user pressed
// Ctrl+Shift+R.

import { beforeNavigate } from '$app/navigation';
import { updated } from '$app/state';

import { mayReload } from './appUpdateRules';

export { reloadsAtOnce } from './appUpdateRules';

const RELOADED_AT = 'app-update-reloaded-at';

function lastReload(): number | null {
	try {
		const value = Number(sessionStorage.getItem(RELOADED_AT));
		return value > 0 ? value : null;
	} catch {
		return null;
	}
}

function markReload() {
	try {
		sessionStorage.setItem(RELOADED_AT, String(Date.now()));
	} catch {
		// no session storage (private mode): the reload still happens
	}
}

/** Loads url as a whole page — the new version's scripts — when allowed; true when it does */
export function reloadForUpdate(url?: string): boolean {
	if (!updated.current || !mayReload(lastReload(), Date.now())) return false;
	markReload();
	if (url) window.location.href = url;
	else window.location.reload();
	return true;
}

/**
 * Installs the version watch (call while the root layout is created): checks at once — a page
 * from the browser's cache is seen without waiting a minute — and when the tab is shown again,
 * and turns the next navigation after an update into a full page load
 */
export function installAppUpdateReload() {
	beforeNavigate((navigation) => {
		if (navigation.willUnload || !navigation.to || navigation.type === 'leave') return;
		if (!updated.current) return;
		const target = navigation.to.url.href;
		if (!mayReload(lastReload(), Date.now())) return;
		navigation.cancel();
		reloadForUpdate(target);
	});

	if (typeof document === 'undefined') return;
	void updated.check();
	document.addEventListener('visibilitychange', () => {
		if (document.visibilityState === 'visible') void updated.check();
	});
}
