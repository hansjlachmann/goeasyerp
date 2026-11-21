/**
 * The company of this browser tab: each tab keeps the company it works in (sessionStorage —
 * per tab; a duplicated tab takes it along) and sends it with every API request (the header
 * X-Company), so switching company in one tab never moves the others. A tab without one —
 * a new tab — takes the company the server's cookie names: the one the user chose last.
 *
 * Requests that cannot carry a header (an image, a download link) take it as the query
 * parameter `company` (`withTabCompanyParam`). When the server refuses the tab's company
 * (gone or no longer allowed: 409 marked X-Tab-Company), the tab forgets it and opens the main
 * menu in the cookie's company — never runs in another company silently.
 */

const STORAGE_KEY = 'openerp-tab-company';
export const TAB_COMPANY_HEADER = 'X-Company';
export const TAB_COMPANY_UNAVAILABLE_HEADER = 'X-Tab-Company';

/** The tab's company ('' when it has none yet) */
export function getTabCompany(): string {
	try {
		return sessionStorage.getItem(STORAGE_KEY) ?? '';
	} catch {
		return '';
	}
}

/** Pins the tab to a company (login, company switch, the first session of a new tab) */
export function setTabCompany(company: string): void {
	try {
		if (company) sessionStorage.setItem(STORAGE_KEY, company);
		else sessionStorage.removeItem(STORAGE_KEY);
	} catch {
		// no storage (private mode, blocked): the tab follows the cookie, as before
	}
}

/** Forgets the tab's company (log out) */
export function clearTabCompany(): void {
	setTabCompany('');
}

function apiPath(input: RequestInfo | URL): string | null {
	const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
	try {
		const parsed = new URL(url, 'http://app.invalid');
		if (parsed.origin !== 'http://app.invalid' && typeof location !== 'undefined' && parsed.origin !== location.origin) return null;
		return parsed.pathname.startsWith('/api/') ? parsed.pathname : null;
	} catch {
		return null;
	}
}

/**
 * A fetch that sends the tab's company with every API request of this origin and reports a
 * refused tab company (`onUnavailable`, after the tab forgot it)
 */
export function withTabCompany(base: typeof fetch, onUnavailable: () => void): typeof fetch {
	return async (input: RequestInfo | URL, init?: RequestInit) => {
		const company = getTabCompany();
		if (!company || apiPath(input) === null) return base(input, init);
		const headers = new Headers(init?.headers ?? (input instanceof Request ? input.headers : undefined));
		if (!headers.has(TAB_COMPANY_HEADER)) headers.set(TAB_COMPANY_HEADER, company);
		const response = await base(input, { ...init, headers });
		if (response.status === 409 && response.headers.get(TAB_COMPANY_UNAVAILABLE_HEADER) === 'unavailable') {
			clearTabCompany();
			onUnavailable();
		}
		return response;
	};
}

let installed = false;

/** Puts the tab's company on the browser's fetch (once; the app's root layout) */
export function installTabCompany(): void {
	if (installed || typeof globalThis.fetch !== 'function') return;
	installed = true;
	globalThis.fetch = withTabCompany(globalThis.fetch.bind(globalThis), () => {
		window.location.href = '/';
	});
}

/** An API URL for an image or a link (no headers): the tab's company as a query parameter */
export function withTabCompanyParam(url: string): string {
	const company = getTabCompany();
	if (!company) return url;
	return `${url}${url.includes('?') ? '&' : '?'}company=${encodeURIComponent(company)}`;
}
