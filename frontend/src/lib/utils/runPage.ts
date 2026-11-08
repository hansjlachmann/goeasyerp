// The URL of an action's run_page with its filters from the current record: run_page_filter_field
// = record[run_page_filter_value] (one field), or run_page_filters — several fields, each from
// the record (value), a constant (const) or the range of two of its fields (from..to — a
// register's entries), classic RunFormLink. The page is run_page or the number in the record's
// run_page_field (a Navigate line's entries page); run_page_params are page parameters from the
// record's fields (Navigate's Document No. and Posting Date). null when a filter needs a value
// the record does not have (no row selected); an optional filter with a blank value is left out.

import type { RunPageFilter } from '$lib/types/pages';
import { withReturn } from '$lib/utils/returnUrl';

export interface RunPageAction {
	run_page?: number;
	run_page_field?: string;
	run_page_filter_field?: string;
	run_page_filter_value?: string;
	run_page_filters?: RunPageFilter[];
	run_page_params?: Record<string, string>;
}

const blank = (value: unknown) => value === undefined || value === null || value === '';

export function runPagePath(action: RunPageAction, record: Record<string, any> | undefined | null): string | null {
	const pageId = action.run_page_field ? record?.[action.run_page_field] : action.run_page;
	if (blank(pageId) || pageId === 0) return null;
	const query: string[] = [];
	if (action.run_page_filter_field) {
		const value = action.run_page_filter_value ? record?.[action.run_page_filter_value] : undefined;
		if (blank(value)) return null;
		query.push(`filter=${encodeURIComponent(`${action.run_page_filter_field}=${value}`)}`);
	}
	for (const f of action.run_page_filters ?? []) {
		if (f.from !== undefined && f.to !== undefined) {
			const from = record?.[f.from];
			const to = record?.[f.to];
			if (blank(from) || blank(to)) {
				if (f.optional) continue;
				return null;
			}
			query.push(`filter=${encodeURIComponent(`${f.field}=${from}..${to}`)}`);
			continue;
		}
		const value = f.value !== undefined ? record?.[f.value] : f.const;
		if (blank(value)) {
			if (f.optional) continue;
			return null;
		}
		query.push(`filter=${encodeURIComponent(`${f.field}=${value}`)}`);
	}
	for (const [param, field] of Object.entries(action.run_page_params ?? {})) {
		const value = record?.[field];
		if (blank(value)) continue;
		query.push(`${encodeURIComponent(param)}=${encodeURIComponent(String(value))}`);
	}
	return `/pages/${pageId}` + (query.length ? `?${query.join('&')}` : '');
}

/**
 * The URL of the page a codeunit's result opens afterwards (open_page: {page_id, key} — e.g.
 * the order a quote became); null when it opens none
 */
export function openPagePath(openPage: unknown): string | null {
	if (!openPage || typeof openPage !== 'object') return null;
	const { page_id, key, params, return: returns } = openPage as { page_id?: unknown; key?: unknown; params?: unknown; return?: unknown };
	if (typeof page_id !== 'number' || page_id <= 0) return null;
	let path = typeof key === 'string' && key !== '' ? `/pages/${page_id}/${encodeURIComponent(key)}` : `/pages/${page_id}`;
	// the page's parameters (Posting Preview: preview)
	const query = new URLSearchParams();
	if (params && typeof params === 'object') {
		for (const [name, value] of Object.entries(params as Record<string, unknown>)) {
			if (typeof value === 'string' && value !== '') query.set(name, value);
		}
	}
	if (query.size > 0) path += '?' + query.toString();
	// back to the page the codeunit ran from (Esc / close)
	return returns === true ? withReturn(path) : path;
}

/** The URL of a page opened with these filters (a FlowField's drilldown): one filter= each */
export function filteredPagePath(pageId: number, filters: { field: string; expression: string }[]): string {
	const query = filters.map((f) => `filter=${encodeURIComponent(`${f.field}=${f.expression}`)}`);
	return `/pages/${pageId}` + (query.length ? `?${query.join('&')}` : '');
}
