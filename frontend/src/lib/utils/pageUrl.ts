// A page's URL as the page's properties: /pages/<id> (a list: filter, back, select, card, new
// and the page's own parameters) or /pages/<id>/<record> (a card: nav, edit) — the routes'
// loaders and the page frames read it the same way

import { safeReturnUrl } from '$lib/utils/returnUrl';
import { parseNavigationQuery, type NavigationQuery } from '$lib/utils/recordNavigation';

export interface PageUrlProps {
	pageId: number;
	recordId?: string;
	filter: string[];
	returnUrl?: string;
	select?: string;
	openCard: boolean;
	startNew: boolean;
	openParams: Record<string, string>;
	navigationQuery?: NavigationQuery;
	edit: boolean;
}

// Query parameters of the app (the others are the page's own: a journal's template and batch)
const reserved = new Set(['record', 'filter', 'back', 'select', 'card', 'new', 'nav', 'edit']);

export function parsePageUrl(url: URL, pageId: number, pathRecordId?: string): PageUrlProps {
	const params = url.searchParams;
	const openParams: Record<string, string> = {};
	for (const [name, value] of params) {
		if (!reserved.has(name)) openParams[name] = value;
	}
	return {
		pageId,
		recordId: pathRecordId ?? (params.get('record') || undefined),
		filter: params.getAll('filter'), // field=expression, one per filter
		// Opened from a drilldown or card action: where Esc / close returns to, and the record
		// to select when this page is the one returned to
		returnUrl: safeReturnUrl(params.get('back')),
		select: params.get('select') || undefined,
		// Returning to a list whose modal card was open on the selected record: open it again
		openCard: params.get('card') === '1',
		// Opened from a lookup's New: the list starts a new record ready to type
		startNew: params.get('new') === '1',
		openParams,
		// A card opened from a list: card navigation moves through the list's records
		navigationQuery: parseNavigationQuery(params.get('nav')),
		// Opened with Edit (a list's Edit / Ctrl+E): the card starts in edit mode
		edit: params.get('edit') === '1'
	};
}

/** A path of the app (/pages/<id>[/<record>]?…) as a page's properties; null when it is not a page */
export function parsePagePath(path: string): PageUrlProps | null {
	const url = new URL(path, 'http://app');
	const match = /^\/pages\/(\d+)(?:\/([^/]+))?\/?$/.exec(url.pathname);
	if (!match) return null;
	let recordId = match[2];
	try {
		if (recordId !== undefined) recordId = decodeURIComponent(recordId);
	} catch {
		// not encoded: as it is
	}
	return parsePageUrl(url, parseInt(match[1], 10), recordId);
}
