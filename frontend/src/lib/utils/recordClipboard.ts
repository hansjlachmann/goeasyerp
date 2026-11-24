// Copy/paste of records on list pages (page property copy_paste_allowed): Ctrl+C copies the
// selected records (their stored fields, from the server), Ctrl+V inserts them into the same
// table — in the same company with new keys from the list's filters, or in another company.
// The copied records are kept in the browser tab (sessionStorage): they survive a company
// switch (a full page load) and are gone when the tab is closed.
import type { TableFilter } from '#lib/types/api.js';

export const CLIPBOARD_KEY = 'openerp-record-clipboard';

const hasStorage = () => typeof sessionStorage !== 'undefined';

export interface RecordClipboard {
	table: string; // source table of the records
	company: string; // company they were copied in
	records: Record<string, unknown>[];
}

export function saveClipboard(clip: RecordClipboard): void {
	if (!hasStorage()) return;
	sessionStorage.setItem(CLIPBOARD_KEY, JSON.stringify(clip));
}

/** Forgets the copied records — at log out, so the next user of the browser cannot paste them (S27) */
export function clearClipboard(): void {
	if (!hasStorage()) return;
	try {
		sessionStorage.removeItem(CLIPBOARD_KEY);
	} catch {
		// storage not available: nothing kept
	}
}

export function loadClipboard(): RecordClipboard | null {
	if (!hasStorage()) return null;
	try {
		const clip = JSON.parse(sessionStorage.getItem(CLIPBOARD_KEY) ?? 'null');
		return clip && typeof clip.table === 'string' && Array.isArray(clip.records) ? clip : null;
	} catch {
		return null;
	}
}

/**
 * Key values the list's filters set: each primary key field filtered on one exact value (no
 * range, alternatives or wildcards) — e.g. role_id = CLERK on User Members. Pasted records get
 * these values, so records copied from role READER are pasted for role CLERK.
 */
export function keyValuesFromFilters(filters: TableFilter[], primaryKeyFields: string[]): Record<string, string> {
	return singleValueFilters(filters, (field) => primaryKeyFields.includes(field));
}

/**
 * The values of the filters that select one exact value (no range, alternatives or wildcards)
 * for the fields `accept` takes. A new record gets them (classic: a field filtered to one value
 * is set to it on insert) — e.g. Exchange Rates opened for currency EUR.
 */
export function singleValueFilters(filters: TableFilter[], accept: (field: string) => boolean = () => true): Record<string, string> {
	const out: Record<string, string> = {};
	for (const f of filters) {
		const value = f.expression.trim();
		if (!accept(f.field) || value === '') continue;
		if (/[|*<>&=@()']|\.\./.test(value)) continue;
		out[f.field] = value;
	}
	return out;
}

/** The records to paste: copies with the key values from the filters applied */
export function recordsToPaste(
	records: Record<string, unknown>[],
	keyValues: Record<string, string>
): Record<string, unknown>[] {
	return records.map((r) => ({ ...r, ...keyValues }));
}
