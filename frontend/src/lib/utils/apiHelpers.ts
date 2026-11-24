// API response handling utilities

import type { ApiResponse, CaptionData } from '#lib/types/api.js';
import { session } from '#lib/stores/session.js';

/**
 * The backend sends X-Session-Changed when a response re-issued the session cookie (e.g.
 * renaming the company the user works in): reload the session state, so the menu bar and
 * everything else showing the company or user follows at once.
 */
export function followSessionChange(response: Response): void {
	if (response.headers?.get('X-Session-Changed')) {
		void session.initialize();
	}
}

/**
 * Result type for API calls that need both data and captions
 */
export interface DataWithCaptions<T> {
	data: T;
	captions?: CaptionData;
}

/**
 * Parse error from API response
 */
async function parseApiError(response: Response, errorContext: string): Promise<string> {
	try {
		const result: ApiResponse = await response.json();
		return result.error || `Failed to ${errorContext}: ${response.statusText}`;
	} catch {
		return `Failed to ${errorContext}: ${response.statusText}`;
	}
}

/**
 * Handle API response - checks for HTTP errors and API-level errors
 * @param response - The fetch Response object
 * @param errorContext - Context for error messages (e.g., "list customers")
 * @returns The parsed response data
 * @throws Error if response is not ok or API returns error
 */
export async function handleApiResponse<T>(
	response: Response,
	errorContext: string
): Promise<T> {
	followSessionChange(response);
	if (!response.ok) {
		throw new Error(await parseApiError(response, errorContext));
	}

	const result: ApiResponse<T> = await response.json();
	if (!result.success) {
		throw new Error(result.error || `Failed to ${errorContext}`);
	}

	return result.data as T;
}

/**
 * Handle API response that returns void (no data expected)
 */
export async function handleApiResponseVoid(
	response: Response,
	errorContext: string
): Promise<void> {
	followSessionChange(response);
	if (!response.ok) {
		throw new Error(await parseApiError(response, errorContext));
	}

	const result: ApiResponse = await response.json();
	if (!result.success) {
		throw new Error(result.error || `Failed to ${errorContext}`);
	}
}

/**
 * Handle API response that returns the full ApiResponse (for auth endpoints)
 */
export async function handleApiResponseFull<T = any>(
	response: Response,
	errorContext: string
): Promise<ApiResponse<T>> {
	followSessionChange(response);
	if (!response.ok) {
		try {
			const result: ApiResponse<T> = await response.json();
			throw { status: response.status, message: result.error || `Failed to ${errorContext}` };
		} catch (e) {
			if (e && typeof e === 'object' && 'status' in e) {
				throw e;
			}
			throw { status: response.status, message: `Failed to ${errorContext}` };
		}
	}

	return await response.json();
}

/**
 * Handle API response returning data with captions (for table operations that need option values)
 */
export async function handleApiResponseWithCaptions<T>(
	response: Response,
	errorContext: string
): Promise<DataWithCaptions<T>> {
	followSessionChange(response);
	if (!response.ok) {
		throw new Error(await parseApiError(response, errorContext));
	}

	const result: ApiResponse<T> = await response.json();
	if (!result.success) {
		throw new Error(result.error || `Failed to ${errorContext}`);
	}

	return {
		data: result.data as T,
		captions: result.captions
	};
}
