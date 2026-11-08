/**
 * A page's layout per user: the fields' customizations (visible, order — lists and cards),
 * the column widths and the row numbers. Stored on the server (User Preferences, type
 * `LAYOUT`, name `DEFAULT`), so it follows the user to every browser; the browser keeps a
 * copy (localStorage) that the pages read at once. Opening a page takes the server's layout
 * (`syncPageLayout`); every change is written to the copy and sent to the server.
 */

import { writable } from 'svelte/store';
import { api } from '$lib/services/api';
import { getJson, setJson, getBoolean, setBoolean, remove } from './storage';

/**
 * Generate storage key for page customizations
 */
function getStorageKey(userId: string, pageId: string | number): string {
	return `page-customization-${userId}-${pageId}`;
}

/**
 * Generate storage key for column widths
 */
function getColumnWidthsKey(userId: string, pageId: number): string {
	return `column-widths-${userId}-${pageId}`;
}

/**
 * Generate storage key for row numbers preference
 */
function getRowNumbersKey(userId: string, pageId: number): string {
	return `row-numbers-${userId}-${pageId}`;
}

// ---- The server's copy ----

/** The preference type and name a page's layout is stored under */
// (Code fields: upper case, as stored)
export const LAYOUT_PREFERENCE_TYPE = 'LAYOUT';
export const LAYOUT_PREFERENCE_NAME = 'DEFAULT';

/** A page's layout as the server stores it */
export interface PageLayout {
	customizations?: Record<string, unknown>;
	widths?: Record<string, number>;
	row_numbers?: boolean;
}

/** Bumped when a page's layout came from the server: pages read their layout again */
export const layoutVersion = writable(0);

// The layout as the browser has it
function localLayout(userId: string, pageId: number): PageLayout {
	return {
		customizations: getJson<Record<string, unknown>>(getStorageKey(userId, pageId), {}),
		widths: getJson<Record<string, number>>(getColumnWidthsKey(userId, pageId), {}),
		row_numbers: getBoolean(getRowNumbersKey(userId, pageId), false)
	};
}

function isEmptyLayout(layout: PageLayout): boolean {
	return Object.keys(layout.customizations ?? {}).length === 0 && Object.keys(layout.widths ?? {}).length === 0 && !layout.row_numbers;
}

// Pages whose layout was read from the server in this browser session (per user)
const synced = new Set<string>();

/**
 * Takes the server's layout of the page into the browser's copy (once per page and session).
 * A page the server has no layout of while the browser has one sends it (layouts kept in the
 * browser before they were stored on the server). Never throws: without the server the
 * browser's copy stays.
 */
export async function syncPageLayout(userId: string | undefined, pageId: number): Promise<void> {
	if (!userId || userId === 'anonymous' || !pageId) return;
	const id = `${userId}:${pageId}`;
	if (synced.has(id)) return;
	synced.add(id);
	try {
		const rows = await api.getPreferences(pageId, LAYOUT_PREFERENCE_TYPE);
		const row = rows.find((r) => r.preference_name.toUpperCase() === LAYOUT_PREFERENCE_NAME);
		if (!row) {
			const local = localLayout(userId, pageId);
			if (!isEmptyLayout(local)) uploadNow(userId, pageId);
			return;
		}
		const layout = JSON.parse(row.preference_data || '{}') as PageLayout;
		const before = JSON.stringify(localLayout(userId, pageId));
		setJson(getStorageKey(userId, pageId), layout.customizations ?? {});
		setJson(getColumnWidthsKey(userId, pageId), layout.widths ?? {});
		setBoolean(getRowNumbersKey(userId, pageId), !!layout.row_numbers);
		if (JSON.stringify(localLayout(userId, pageId)) !== before) layoutVersion.update((v) => v + 1);
	} catch {
		// the browser's copy stays; the next session tries again
		synced.delete(id);
	}
}

// Changes go to the server shortly after the last one (dragging a column width saves often)
const pendingUploads = new Map<string, ReturnType<typeof setTimeout>>();

function queueUpload(userId: string, pageId: string | number) {
	const page = Number(pageId);
	if (!userId || userId === 'anonymous' || !page) return;
	const id = `${userId}:${page}`;
	clearTimeout(pendingUploads.get(id));
	pendingUploads.set(
		id,
		setTimeout(() => {
			pendingUploads.delete(id);
			uploadNow(userId, page);
		}, 500)
	);
}

// The tab closes (or goes away): the changes still waiting are sent at once
if (typeof window !== 'undefined') {
	window.addEventListener('pagehide', () => {
		for (const [id, timer] of pendingUploads) {
			clearTimeout(timer);
			const [userId, page] = [id.slice(0, id.lastIndexOf(':')), Number(id.slice(id.lastIndexOf(':') + 1))];
			uploadNow(userId, page, true);
		}
		pendingUploads.clear();
	});
}

function uploadNow(userId: string, pageId: number, keepalive = false) {
	api.savePreference(pageId, LAYOUT_PREFERENCE_TYPE, LAYOUT_PREFERENCE_NAME, { ...localLayout(userId, pageId) }, keepalive).catch((err) => {
		console.error('Failed to save the page layout:', err);
	});
}

// ---- The browser's copy ----

/**
 * Load page customizations
 * @param userId - The user ID (or 'anonymous' if not logged in)
 * @param pageId - The page ID
 * @returns Parsed customizations object, or empty object if none found
 */
export function loadPageCustomizations<T = Record<string, any>>(
	userId: string,
	pageId: string | number
): T {
	return getJson<T>(getStorageKey(userId, pageId), {} as T);
}

/**
 * Save page customizations (the browser's copy and the server)
 * @param userId - The user ID (or 'anonymous' if not logged in)
 * @param pageId - The page ID
 * @param customizations - The customizations object to save
 */
export function savePageCustomizations<T = Record<string, any>>(
	userId: string,
	pageId: string | number,
	customizations: T
): void {
	setJson(getStorageKey(userId, pageId), customizations);
	queueUpload(userId, pageId);
}

/**
 * Clear page customizations
 * @param userId - The user ID (or 'anonymous' if not logged in)
 * @param pageId - The page ID
 */
export function clearPageCustomizations(userId: string, pageId: string | number): void {
	remove(getStorageKey(userId, pageId));
	queueUpload(userId, pageId);
}

/**
 * Load column widths
 */
export function loadColumnWidths(userId: string, pageId: number): Record<string, number> {
	return getJson<Record<string, number>>(getColumnWidthsKey(userId, pageId), {});
}

/**
 * Save column widths (the browser's copy and the server)
 */
export function saveColumnWidths(
	userId: string,
	pageId: number,
	widths: Record<string, number>
): void {
	setJson(getColumnWidthsKey(userId, pageId), widths);
	queueUpload(userId, pageId);
}

/**
 * Load row numbers preference
 */
export function loadRowNumbersPreference(userId: string, pageId: number): boolean {
	return getBoolean(getRowNumbersKey(userId, pageId), false);
}

/**
 * Save row numbers preference (the browser's copy and the server)
 */
export function saveRowNumbersPreference(userId: string, pageId: number, show: boolean): void {
	setBoolean(getRowNumbersKey(userId, pageId), show);
	queueUpload(userId, pageId);
}
