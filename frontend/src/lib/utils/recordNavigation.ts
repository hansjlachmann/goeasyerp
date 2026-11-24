// Card navigation (First/Previous/Next/Last, Ctrl+Up/Down) moves through the records the list
// showed, in its order: the list's filters, search and sort go with the card — as a prop to the
// modal card, as ?nav=<JSON> to a card page — and /ids applies them.
import type { ListOptions, TableFilter } from '#lib/types/api.js';

export type NavigationQuery = Pick<ListOptions, 'filters' | 'fixed_filters' | 'search' | 'search_fields' | 'sort_by' | 'sort_order'>;

// The query with only the parts in effect (undefined when there are none)
export function navigationQuery(q: NavigationQuery): NavigationQuery | undefined {
	const out: NavigationQuery = {};
	if (q.filters && q.filters.length > 0) out.filters = q.filters;
	if (q.fixed_filters && q.fixed_filters.length > 0) out.fixed_filters = q.fixed_filters;
	const search = q.search?.trim();
	if (search && q.search_fields && q.search_fields.length > 0) {
		out.search = search;
		out.search_fields = q.search_fields;
	}
	if (q.sort_by) {
		out.sort_by = q.sort_by;
		if (q.sort_order === 'desc') out.sort_order = 'desc';
	}
	return Object.keys(out).length > 0 ? out : undefined;
}

// url with nav=<query> (unchanged without a query)
export function withNavigationQuery(url: string, q: NavigationQuery | undefined): string {
	const effective = q && navigationQuery(q);
	if (!effective) return url;
	return url + (url.includes('?') ? '&' : '?') + 'nav=' + encodeURIComponent(JSON.stringify(effective));
}

const isString = (v: unknown): v is string => typeof v === 'string';

// The nav parameter of a card page URL; anything malformed is ignored
export function parseNavigationQuery(value: string | null | undefined): NavigationQuery | undefined {
	if (!value) return undefined;
	let raw: unknown;
	try {
		raw = JSON.parse(value);
	} catch {
		return undefined;
	}
	if (!raw || typeof raw !== 'object') return undefined;
	const r = raw as Record<string, unknown>;
	const filters = Array.isArray(r.filters)
		? r.filters.filter((f): f is TableFilter => !!f && isString(f.field) && isString(f.expression))
		: undefined;
	return navigationQuery({
		filters,
		search: isString(r.search) ? r.search : undefined,
		search_fields: Array.isArray(r.search_fields) ? r.search_fields.filter(isString) : undefined,
		sort_by: isString(r.sort_by) ? r.sort_by : undefined,
		sort_order: r.sort_order === 'desc' ? 'desc' : 'asc'
	});
}
