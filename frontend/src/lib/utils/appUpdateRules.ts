// When the app loads a new version by itself (appUpdate.ts); apart from it so it can be
// tested without SvelteKit's $app modules

// at most one automatic reload in this time: a server answering an old version.json after
// the reload (a proxy's cache) must not reload the page again and again
const RELOAD_GUARD_MS = 30_000;

/** Pages that hold nothing the user could lose: reloaded as soon as the new version is seen */
export function reloadsAtOnce(pathname: string): boolean {
	return pathname === '/login' || pathname.startsWith('/login/');
}

/** Whether an automatic reload may happen now (none in the last RELOAD_GUARD_MS) */
export function mayReload(lastReload: number | null, now: number): boolean {
	return lastReload === null || !(now - lastReload >= 0 && now - lastReload < RELOAD_GUARD_MS);
}
