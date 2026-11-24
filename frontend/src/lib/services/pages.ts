// Page and menu API service

import type { PageDefinition, MenuDefinition } from '#lib/types/pages.js';
import { handleApiResponse } from '#lib/utils/apiHelpers.js';

const API_BASE = '/api';

// Cache for page definitions
const pageCache = new Map<number, PageDefinition>();
let menuCache: MenuDefinition | null = null;

/**
 * Fetch a page definition by ID
 */
export async function fetchPage(pageId: number): Promise<PageDefinition> {
	// Check cache first
	if (pageCache.has(pageId)) {
		return pageCache.get(pageId)!;
	}

	const response = await fetch(`${API_BASE}/pages/${pageId}`);
	const data = await handleApiResponse<PageDefinition>(response, `fetch page ${pageId}`);

	// Cache the page definition
	pageCache.set(pageId, data);
	return data;
}

/**
 * Fetch all page definitions
 */
export async function fetchAllPages(): Promise<PageDefinition[]> {
	const response = await fetch(`${API_BASE}/pages`);
	const pages = await handleApiResponse<PageDefinition[]>(response, 'fetch all pages');

	// Cache all pages
	pages.forEach((page) => {
		pageCache.set(page.page.id, page);
	});

	return pages;
}

/**
 * Fetch the menu definition
 */
export async function fetchMenu(): Promise<MenuDefinition> {
	// Check cache first
	if (menuCache) {
		return menuCache;
	}

	const response = await fetch(`${API_BASE}/menu`);
	const data = await handleApiResponse<MenuDefinition>(response, 'fetch menu');

	// Cache the menu
	menuCache = data;
	return data;
}

/**
 * Clear page cache
 */
export function clearPageCache() {
	pageCache.clear();
}

/**
 * Clear menu cache
 */
export function clearMenuCache() {
	menuCache = null;
}
