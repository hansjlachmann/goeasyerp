/**
 * Browser-safe localStorage utilities with type safety and error handling
 */

import { browser } from '$app/env';

/**
 * Get a JSON value from localStorage
 * @param key - Storage key
 * @param defaultValue - Default value if key doesn't exist or parse fails
 * @returns Parsed value or default
 */
export function getJson<T>(key: string, defaultValue: T): T {
	if (!browser) return defaultValue;

	try {
		const stored = localStorage.getItem(key);
		if (stored === null) return defaultValue;
		return JSON.parse(stored) as T;
	} catch (e) {
		console.error(`Failed to load ${key} from localStorage:`, e);
		return defaultValue;
	}
}

/**
 * Save a JSON value to localStorage
 * @param key - Storage key
 * @param value - Value to stringify and save
 */
export function setJson<T>(key: string, value: T): void {
	if (!browser) return;

	try {
		localStorage.setItem(key, JSON.stringify(value));
	} catch (e) {
		console.error(`Failed to save ${key} to localStorage:`, e);
	}
}

/**
 * Get a string value from localStorage
 * @param key - Storage key
 * @param defaultValue - Default value if key doesn't exist
 * @returns Stored string or default
 */
export function getString(key: string, defaultValue: string | null = null): string | null {
	if (!browser) return defaultValue;
	return localStorage.getItem(key) ?? defaultValue;
}

/**
 * Save a string value to localStorage
 * @param key - Storage key
 * @param value - String value to save
 */
export function setString(key: string, value: string): void {
	if (!browser) return;
	localStorage.setItem(key, value);
}

/**
 * Get a boolean value from localStorage
 * @param key - Storage key
 * @param defaultValue - Default value if key doesn't exist
 * @returns Boolean value
 */
export function getBoolean(key: string, defaultValue: boolean = false): boolean {
	if (!browser) return defaultValue;
	const stored = localStorage.getItem(key);
	if (stored === null) return defaultValue;
	return stored === 'true';
}

/**
 * Save a boolean value to localStorage
 * @param key - Storage key
 * @param value - Boolean value to save
 */
export function setBoolean(key: string, value: boolean): void {
	if (!browser) return;
	localStorage.setItem(key, value ? 'true' : 'false');
}

/**
 * Remove an item from localStorage
 * @param key - Storage key to remove
 */
export function remove(key: string): void {
	if (!browser) return;
	localStorage.removeItem(key);
}
