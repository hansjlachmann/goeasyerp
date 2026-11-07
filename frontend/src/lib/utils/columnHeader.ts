// Column header menu of list pages: sort, "Filter to This Value", clear filter, hide,
// drag to reorder. Pure helpers, kept out of ListPage so they are unit-tested.
import type { TableFilter } from '$lib/types/api';
import type { ItemCustomization } from '$lib/utils/fieldHelpers';

/**
 * The filter expression that matches value exactly, or null when there is none: a blank
 * value, or one containing filter operators (| .. * < > — the filter syntax has no quoting,
 * so such values cannot be matched exactly). Booleans filter as 1/0, which both
 * databases compare with their boolean columns.
 */
export function filterValueFor(value: unknown): string | null {
	if (value === null || value === undefined) return null;
	if (typeof value === 'boolean') return value ? '1' : '0';
	const text = String(value).trim();
	if (text === '') return null;
	if (/[|*<>]|\.\./.test(text)) return null;
	return text;
}

/** filters with field's filter set to expression (replacing one the field had) */
export function withFilter(filters: TableFilter[], field: string, expression: string): TableFilter[] {
	return [...filters.filter((f) => f.field !== field), { field, expression }];
}

/** filters without field's filter */
export function withoutFilter(filters: TableFilter[], field: string): TableFilter[] {
	return filters.filter((f) => f.field !== field);
}

/**
 * Column customizations after dragging column source onto column target (it takes target's
 * place). order is the column order as shown, hidden columns included (they keep their
 * place); every column gets its new position as order, visibility unchanged.
 */
export function moveColumn(
	customizations: Record<string, ItemCustomization>,
	order: { source: string; visible: boolean }[],
	source: string,
	target: string
): Record<string, ItemCustomization> {
	const from = order.findIndex((c) => c.source === source);
	const to = order.findIndex((c) => c.source === target);
	if (from < 0 || to < 0 || from === to) return customizations;
	const moved = [...order];
	const [column] = moved.splice(from, 1);
	moved.splice(to, 0, column);
	const out: Record<string, ItemCustomization> = { ...customizations };
	moved.forEach((c, index) => {
		out[c.source] = { ...(customizations[c.source] ?? {}), visible: c.visible, order: index };
	});
	return out;
}

/** Customizations with column source hidden */
export function hideColumn(
	customizations: Record<string, ItemCustomization>,
	source: string
): Record<string, ItemCustomization> {
	return { ...customizations, [source]: { ...(customizations[source] ?? {}), visible: false } };
}

/**
 * A column caption that wraps after a slash ("Income/" and "Balance" on two lines) rather than
 * inside a word: browsers do not break there on their own. A zero-width space, so nothing shows.
 */
export function captionWithBreaks(caption: string): string {
	return caption.replace(/\/(?=\S)/g, '/\u200B');
}

/** The longest part of a caption that stays on one line (it wraps at spaces, after / and -) */
export function longestCaptionWord(caption: string): string {
	return caption
		.split(/\s+|(?<=[/-])/)
		.reduce((longest, word) => (word.length > longest.length ? word : longest), '');
}
