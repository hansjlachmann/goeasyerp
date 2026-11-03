/**
 * Jobs Service - Handles background job execution with progress tracking
 * Uses Server-Sent Events (SSE) for real-time progress updates
 */

import { t, ERR, MSG } from '$lib/services/i18n.svelte';
import { toast } from '$lib/stores/toast';
import { buildQueryString } from '$lib/services/api';
import type { ListOptions } from '$lib/types/api';
import { downloadBlob, downloadPdf, handlePrintout, pdfBlob, printoutsOf, resultFileOf } from '$lib/utils/printout';

/** An answer to a codeunit's question was refused by the server: show why */
async function answerRefused(res: Response): Promise<void> {
	let message = res.statusText;
	try {
		message = (await res.json()).error || message;
	} catch {
		// not JSON: the status text
	}
	toast.error(message);
}

/** The records selected on a list a codeunit is run on: their keys, or all of the list's query */
export interface JobSelection {
	keys?: string[];
	all?: boolean;
	query?: ListOptions;
}

export interface ProgressEvent {
	job_id: string;
	field: number;
	value: number;
	message: string;
	completed: boolean;
	error: string;
	timestamp: number;
	event_type?: string;
	confirm_id?: string;
	data?: Record<string, unknown>; // Result data (e.g., PDF base64)
	error_details?: ErrorDetails; // what lies behind the error (Show Entries)
}

/** What lies behind an error (the G/L entries of a posting that does not balance): sections of
 *  rows, every value a text formatted by the server; marked rows are pointed out */
export interface ErrorDetails {
	title: string;
	sections: ErrorDetailsSection[];
}

export interface ErrorDetailsSection {
	caption: string;
	columns: { caption: string; numeric?: boolean }[];
	rows: string[][];
	marked?: number[];
}

export interface DialogResult {
	title: string;
	message: string;
	type: 'info' | 'success' | 'warning' | 'error';
}

export interface SyncJobResult {
	success: boolean;
	message: string;
	data?: Record<string, unknown>;
	dialog?: DialogResult;
	refresh?: boolean; // the page reloads its data (a card: its record and its subpages' lines)
}

export interface JobCallbacks {
	onProgress?: (event: ProgressEvent) => void;
	onComplete?: (event: ProgressEvent) => void;
	onError?: (error: string, details?: ErrorDetails) => void;
	onSyncResult?: (result: SyncJobResult) => void;
	onConfirm?: (event: ProgressEvent, respond: (response: boolean) => void) => void;
	onInputRequest?: (event: ProgressEvent, respond: (values: Record<string, string> | null) => void) => void;
	// The codeunit opens a page (RunModal): respond with {action, key} when the user closes it
	onRunPage?: (event: ProgressEvent, respond: (values: Record<string, string> | null) => void) => void;
	onData?: (data: Record<string, unknown>) => void; // Called when job returns data (e.g., PDF)
	onJobStarted?: (jobId: string) => void; // Called when async job starts with job ID for cancellation
}

/**
 * Download a base64 PDF as a file
 */
export function downloadBase64PDF(base64Data: string, filename: string = 'document.pdf') {
	downloadPdf(pdfBlob(base64Data), filename);
}

/**
 * Cancel a running job
 */
export async function cancelJob(jobId: string): Promise<boolean> {
	try {
		const response = await fetch(`/api/jobs/${jobId}/cancel`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' }
		});
		return response.ok;
	} catch {
		return false;
	}
}

/**
 * Start a job (codeunit execution)
 * The backend decides whether to run sync or async based on UsesProgress()
 * Returns a promise that resolves when the job completes
 */
export async function startJob(
	codeunitId: number,
	record: Record<string, unknown>,
	callbacks: JobCallbacks = {},
	selection?: JobSelection,
	objectType: 'codeunit' | 'report' | 'function' = 'codeunit', // a report: its number is in codeunitId
	functionName?: string, // objectType 'function': "table:Job_Queue_Entry.ShowLineNo"
	// The list's filters: the codeunit's record carries them (a journal's Post works on the batch
	// the list shows)
	listFilters?: Pick<ListOptions, 'filters' | 'fixed_filters' | 'page_params' | 'page_id'>
): Promise<ProgressEvent | SyncJobResult | null> {
	// Start the job; a list's selection (all: with the list's query) goes along for
	// rec.SetSelectionFilter in the codeunit
	const params = selection?.all ? buildQueryString(selection.query) : '';
	const response = await fetch(`/api/jobs/start${params ? '?' + params : ''}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			...(objectType === 'function'
				? { function: functionName }
				: objectType === 'report'
					? { report_id: codeunitId }
					: { codeunit_id: codeunitId }),
			record,
			...(selection ? { selection: { keys: selection.keys, all: selection.all } } : {}),
			...(listFilters?.filters?.length ? { filters: listFilters.filters } : {}),
			...(listFilters?.fixed_filters?.length ? { fixed_filters: listFilters.fixed_filters } : {}),
			...(listFilters?.page_params && Object.keys(listFilters.page_params).length ? { page_params: listFilters.page_params } : {}),
			// a page on a temporary source table: the server fills its rows again (the function's
			// record is one of them)
			...(listFilters?.page_id ? { page_id: listFilters.page_id } : {})
		})
	});

	if (!response.ok) {
		const error = await response.json();
		callbacks.onError?.(error.error || t(ERR.FAILED_START_JOB));
		throw new Error(error.error || t(ERR.FAILED_START_JOB));
	}

	const result = await response.json();

	// Check if this is a sync result (no job_id) or async job (has job_id)
	if (result.data.job_id) {
		// Async job - connect to SSE for progress updates
		const jobId = result.data.job_id;

		// Notify caller of job ID for potential cancellation
		callbacks.onJobStarted?.(jobId);

		return new Promise((resolve, reject) => {
			const eventSource = new EventSource(`/api/jobs/${jobId}/events`);
			let lastEvent: ProgressEvent | null = null;

			eventSource.addEventListener('connected', () => {
				// Connection established
			});

			eventSource.addEventListener('progress', (e: MessageEvent) => {
				try {
					const event: ProgressEvent = JSON.parse(e.data);
					lastEvent = event;

					if (event.completed) {
						eventSource.close();
						if (event.error) {
							callbacks.onError?.(event.error, event.error_details);
							reject(new Error(event.error));
						} else {
							// Handle result data (e.g., PDF download)
							if (event.data) {
								callbacks.onData?.(event.data);

								// A printed report's printouts: preview, print or download each
								for (const printout of printoutsOf(event.data)) handlePrintout(printout);
								// A processing report's file (a declaration): downloaded
								const file = resultFileOf(event.data);
								if (file) downloadBlob(file.blob, file.filename);
							}
							callbacks.onComplete?.(event);
							resolve(event);
						}
					} else if (event.event_type === 'confirm') {
						// Handle confirm dialog
						const respond = async (response: boolean) => {
							try {
								await fetch(`/api/jobs/${jobId}/confirm`, {
									method: 'POST',
									headers: { 'Content-Type': 'application/json' },
									body: JSON.stringify({
										confirm_id: event.confirm_id,
										response
									})
								});
							} catch (err) {
								console.error('Failed to send confirm response:', err);
							}
						};
						callbacks.onConfirm?.(event, respond);
					} else if (event.event_type === 'request_input') {
						// Handle input request dialog
						const respond = async (values: Record<string, string> | null) => {
							try {
								const res = await fetch(`/api/jobs/${jobId}/confirm`, {
									method: 'POST',
									headers: { 'Content-Type': 'application/json' },
									body: JSON.stringify({
										confirm_id: event.confirm_id,
										values: values ?? {}
									})
								});
								// The codeunit waits for this answer: say so when it was refused
								if (!res.ok) await answerRefused(res);
							} catch (err) {
								console.error('Failed to send input response:', err);
							}
						};
						callbacks.onInputRequest?.(event, respond);
					} else if (event.event_type === 'run_page') {
						// The codeunit opens a page (RunModal) and waits: {action, key} when it closes
						const respond = async (values: Record<string, string> | null) => {
							try {
								const res = await fetch(`/api/jobs/${jobId}/confirm`, {
									method: 'POST',
									headers: { 'Content-Type': 'application/json' },
									body: JSON.stringify({ confirm_id: event.confirm_id, values: values ?? {} })
								});
								if (!res.ok) await answerRefused(res);
							} catch (err) {
								console.error('Failed to send page response:', err);
							}
						};
						if (callbacks.onRunPage) callbacks.onRunPage(event, respond);
						else respond(null); // nothing can show the page: cancel
					} else if (event.event_type === 'hyperlink') {
						// Hyperlink from the codeunit: open http(s)/mailto URLs in a new tab
						if (/^(https?:\/\/|mailto:)/i.test(event.message)) {
							const tab = window.open(event.message, '_blank');
							if (tab) {
								tab.opener = null;
							} else {
								toast.warning(t(MSG.POPUP_BLOCKED, event.message), 10000);
							}
						}
					} else {
						callbacks.onProgress?.(event);
					}
				} catch (err) {
					console.error('Failed to parse progress event:', err);
				}
			});

			eventSource.addEventListener('close', () => {
				eventSource.close();
				if (lastEvent) {
					resolve(lastEvent);
				} else {
					resolve(null);
				}
			});

			eventSource.onerror = () => {
				eventSource.close();
				const errMsg = t(ERR.CONNECTION_LOST);
				callbacks.onError?.(errMsg);
				reject(new Error(errMsg));
			};
		});
	} else {
		// Sync result - return immediately
		const syncResult: SyncJobResult = {
			success: result.data.success,
			message: result.data.message,
			data: result.data.data,
			dialog: result.data.dialog,
			refresh: result.data.refresh === true
		};
		callbacks.onSyncResult?.(syncResult);
		return syncResult;
	}
}

/**
 * Run a job with progress modal support
 * This is a higher-level function that manages the progress state
 */
export function createJobRunner() {
	let isRunning = $state(false);
	let progress = $state(0);
	let message = $state('');
	let error = $state('');
	let title = $state(t(MSG.PROCESSING));

	async function run(
		codeunitId: number,
		record: Record<string, unknown>,
		jobTitle: string = t(MSG.PROCESSING)
	): Promise<boolean> {
		isRunning = true;
		progress = 0;
		message = '';
		error = '';
		title = jobTitle;

		try {
			await startJob(codeunitId, record, {
				onProgress: (event) => {
					progress = event.value;
					if (event.message) {
						message = event.message;
					}
				},
				onComplete: (event) => {
					progress = 100;
					if (event.message) {
						message = event.message;
					}
				},
				onError: (err) => {
					error = err;
				}
			});

			// Keep modal open briefly to show 100%
			await new Promise((resolve) => setTimeout(resolve, 500));
			isRunning = false;
			return !error;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Unknown error';
			// Keep modal open to show error
			await new Promise((resolve) => setTimeout(resolve, 2000));
			isRunning = false;
			return false;
		}
	}

	function close() {
		isRunning = false;
	}

	return {
		get isRunning() {
			return isRunning;
		},
		get progress() {
			return progress;
		},
		get message() {
			return message;
		},
		get error() {
			return error;
		},
		get title() {
			return title;
		},
		run,
		close
	};
}
