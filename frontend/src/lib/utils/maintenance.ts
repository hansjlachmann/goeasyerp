import { writable } from 'svelte/store';

/**
 * Maintenance: while the server waits for its extension apps to be built for a new version, it
 * answers every API call with 503 and a JSON body marked `maintenance: true` that brings the
 * notice's texts in the browser's language (the page cannot load its translations then). The
 * app shows the notice instead of the error of whatever call met it, checks every
 * `CHECK_INTERVAL_MS` whether the server is back and then loads the page again — a full load:
 * the server now runs another program (new pages, texts, apps).
 */
export interface Maintenance {
	title: string;
	message: string;
	retry: string;
	/** the build of the apps failed: an administrator has to act, the check goes on */
	failed: boolean;
	/** the server is starting a newer build of its apps: back in moments, checked more often */
	starting: boolean;
}

/** The maintenance notice shown, null while the server answers normally */
export const maintenance = writable<Maintenance | null>(null);

/** How often the notice asks whether the server is back */
export const CHECK_INTERVAL_MS = 15_000;

/** … while the server is starting a newer build (seconds, not minutes) */
export const STARTING_CHECK_INTERVAL_MS = 2_000;

/** The check interval for a notice */
export function checkInterval(m: Maintenance): number {
	return m.starting ? STARTING_CHECK_INTERVAL_MS : CHECK_INTERVAL_MS;
}

/** What the notice asks: a small API call that every server version answers */
export const CHECK_URL = '/api/version';

/**
 * The maintenance notice of an answer: a 503 whose body is marked maintenance — recognised by
 * the marker, never by the message text. Any other answer (also another 503, e.g. the proxy's
 * "Backend unavailable") is none. The response is read from a clone: the caller still has it.
 */
export async function maintenanceOf(response: Response): Promise<Maintenance | null> {
	if (response.status !== 503) return null;
	try {
		const body = await response.clone().json();
		if (!body || body.maintenance !== true) return null;
		return {
			title: typeof body.title === 'string' ? body.title : '',
			message: typeof body.error === 'string' ? body.error : '',
			retry: typeof body.retry === 'string' ? body.retry : '',
			failed: body.failed === true,
			starting: body.starting === true
		};
	} catch {
		return null;
	}
}

function isApiCall(input: RequestInfo | URL): boolean {
	const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
	try {
		return new URL(url, 'http://app.invalid').pathname.startsWith('/api/');
	} catch {
		return false;
	}
}

/**
 * A fetch that reports maintenance: every API answer goes through `maintenanceOf`, a maintenance
 * one sets the store (the caller gets the answer unchanged and handles it as before; the notice
 * covers the page). Installed once over the browser's fetch (`installMaintenanceWatch`), so it
 * sees every call of the app — the API client, translations, pages, jobs.
 */
export function watchingFetch(base: typeof fetch, report: (m: Maintenance) => void): typeof fetch {
	return async (input: RequestInfo | URL, init?: RequestInit) => {
		const response = await base(input, init);
		if (response.status === 503 && isApiCall(input)) {
			const m = await maintenanceOf(response);
			if (m) report(m);
		}
		return response;
	};
}

let originalFetch: typeof fetch | null = null;

/** Puts the maintenance watch over the browser's fetch (once; the app's root layout) */
export function installMaintenanceWatch(): void {
	if (originalFetch || typeof globalThis.fetch !== 'function') return;
	originalFetch = globalThis.fetch.bind(globalThis);
	globalThis.fetch = watchingFetch(originalFetch, (m) => maintenance.set(m));
}

/**
 * Asks the server once whether maintenance is over: true when it answers normally (anything but
 * a maintenance answer — also an error of another kind, which the page then shows itself); a
 * maintenance answer updates the notice (a build that failed meanwhile). A network error is
 * not the end of maintenance: the server is restarting into the new program.
 */
export async function maintenanceOver(fetchFn: typeof fetch = originalFetch ?? globalThis.fetch): Promise<boolean> {
	try {
		const response = await fetchFn(CHECK_URL, { cache: 'no-store' });
		const m = await maintenanceOf(response);
		if (m) {
			maintenance.set(m);
			return false;
		}
		return response.status !== 502 && response.status !== 503 && response.status !== 504;
	} catch {
		return false;
	}
}
