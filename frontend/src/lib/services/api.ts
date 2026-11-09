import type { NavigationQuery } from '$lib/utils/recordNavigation';
import type { FieldState, MatrixColumn, MatrixRow, PeriodGridRow } from '$lib/types/pages';
import type {
	ApiResponse,
	ListResponse,
	ListOptions,
	TableRecord,
	LookupData,
	CodeunitResult,
	ValidateFieldResult,
	TableFilter,
	FixedFilter
} from '$types/api';
import type { Appearance, AppearanceSettings, ThemeInstallResult, ThemeList } from '$lib/theme/apply';
import { handleApiResponse, handleApiResponseVoid, handleApiResponseFull, handleApiResponseWithCaptions, type DataWithCaptions } from '$lib/utils/apiHelpers';
import type { CompanyInfo } from '$lib/utils/company';
import { showNotes } from '$lib/utils/notes';

const API_BASE = '/api';

// Every API call of this client: the answer's notes (a number series reaching its Warning
// No. …) are shown to the user
async function fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
	const response = await globalThis.fetch(input, init);
	showNotes(response);
	return response;
}

/** What the matrix endpoints take: the card section, its record, the user's filters and the view */
export interface MatrixRequest {
	section: string;
	record: TableRecord;
	filters: { field: string; expression: string }[];
	view_by: number;
	view_as: number;
	date: string;
	step: number;
	columns: number;
}

// Builds a record endpoint URL. An empty id (a setup table's single
// record has a blank primary key) omits the id segment so it matches the no-id
// backend routes instead of producing a trailing-slash 404.
function recordUrl(tableName: string, action: 'card' | 'modify' | 'delete', id: string): string {
	return id === ''
		? `${API_BASE}/tables/${tableName}/${action}`
		: `${API_BASE}/tables/${tableName}/${action}/${id}`;
}

// Helper to build query string from filters
export function buildQueryString(options?: ListOptions): string {
	if (!options) return '';

	const params = new URLSearchParams();

	if (options.page) params.append('page', options.page.toString());
	if (options.page_size) params.append('page_size', options.page_size.toString());
	if (options.limit) {
		params.append('offset', (options.offset ?? 0).toString());
		params.append('limit', options.limit.toString());
	}
	if (options.flow_filters && options.flow_filters.length > 0) {
		params.append('flow_filters', JSON.stringify(options.flow_filters));
	}
	if (options.search && options.search_fields && options.search_fields.length > 0) {
		params.append('search', options.search);
		params.append('search_fields', JSON.stringify(options.search_fields));
	}
	if (options.sort_by) params.append('sort_by', options.sort_by);
	if (options.sort_order) params.append('sort_order', options.sort_order);

	// Add filters as JSON
	if (options.filters && options.filters.length > 0) {
		params.append('filters', JSON.stringify(options.filters));
	}
	if (options.fixed_filters && options.fixed_filters.length > 0) {
		params.append('fixed_filters', JSON.stringify(options.fixed_filters));
	}

	// Add fields as JSON
	if (options.fields && options.fields.length > 0) {
		params.append('fields', JSON.stringify(options.fields));
	}
	if (options.page_id) params.append('page_id', options.page_id.toString());
	if (options.page_params && Object.keys(options.page_params).length > 0) {
		params.append('page_params', JSON.stringify(options.page_params));
	}

	return params.toString();
}

// Generic API client
export const api = {
	// Generic table operations
	async listRecords<T = TableRecord>(
		tableName: string,
		options?: ListOptions
	): Promise<ListResponse<T>> {
		const query = buildQueryString(options);
		const url = `${API_BASE}/tables/${tableName}/list${query ? '?' + query : ''}`;
		const response = await fetch(url);
		return handleApiResponse<ListResponse<T>>(response, `list ${tableName}`);
	},

	async listRecordsWithOptions<T = TableRecord>(
		tableName: string,
		listOptions?: ListOptions
	): Promise<{ list: ListResponse<T>; options: Record<string, Record<string, string>>; lookups: Record<string, LookupData> }> {
		const query = buildQueryString(listOptions);
		const url = `${API_BASE}/tables/${tableName}/list${query ? '?' + query : ''}`;
		const response = await fetch(url);
		const result = await handleApiResponseWithCaptions<ListResponse<T>>(response, `list ${tableName}`);
		return {
			list: result.data,
			options: result.captions?.options || {},
			lookups: result.captions?.lookups || {}
		};
	},

	// Rows for an on-demand dropdown (LookupData.lazy_url): matching search, or the row with key
	async lookupRows(
		lazyUrl: string,
		params: { search?: string; key?: string }
	): Promise<{ rows: Array<{ _key: string; [key: string]: any }>; total: number }> {
		const query = new URLSearchParams();
		if (params.search) query.append('search', params.search);
		if (params.key) query.append('key', params.key);
		const response = await fetch(`${lazyUrl}${query.toString() ? '?' + query.toString() : ''}`);
		return handleApiResponse(response, 'load lookup rows');
	},

	// Keys of the records in list order; with the list's filters, search and sort (card navigation)
	/**
	 * The records to copy (list Ctrl+C): the given keys, or with all every record the list
	 * shows (query: the list's filters and search). Stored fields only, at most 1,000.
	 */
	async copyRecords(
		tableName: string,
		request: { keys?: string[]; all?: boolean },
		query?: ListOptions
	): Promise<Record<string, unknown>[]> {
		const params = request.all ? buildQueryString(query) : '';
		const response = await fetch(`${API_BASE}/tables/${tableName}/copy${params ? '?' + params : ''}`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(request)
		});
		const data = await handleApiResponse<{ records: Record<string, unknown>[] }>(response, `copy ${tableName} records`);
		return data.records;
	},

	/**
	 * Insert several records in one transaction (list Ctrl+V): a record that cannot be
	 * inserted rolls back all; the error names it. Returns the keys of the inserted records.
	 */
	async insertRecords(tableName: string, records: Record<string, unknown>[]): Promise<string[]> {
		const response = await fetch(`${API_BASE}/tables/${tableName}/insert-many`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ records })
		});
		const data = await handleApiResponse<{ keys: string[] }>(response, `paste ${tableName} records`);
		return data.keys;
	},

	/**
	 * Delete several records in one transaction (list multi-select): the given keys, or with
	 * all every record the list shows — query carries the list's filters and search. A record
	 * that cannot be deleted rolls back all; the error names it. Returns the number deleted.
	 */
	async deleteRecords(
		tableName: string,
		request: { keys?: string[]; all?: boolean },
		query?: ListOptions
	): Promise<number> {
		const params = request.all ? buildQueryString(query) : '';
		const response = await fetch(`${API_BASE}/tables/${tableName}/delete-many${params ? '?' + params : ''}`, {
			method: 'DELETE',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(request)
		});
		const data = await handleApiResponse<{ deleted: number }>(response, `delete ${tableName} records`);
		return data.deleted;
	},

	async getRecordIDs(tableName: string, query?: NavigationQuery): Promise<string[]> {
		const params = buildQueryString(query);
		const url = `${API_BASE}/tables/${tableName}/ids${params ? '?' + params : ''}`;
		const response = await fetch(url);
		const data = await handleApiResponse<{ ids: string[] }>(response, `get ${tableName} IDs`);
		return data.ids;
	},

	async getRecord<T = TableRecord>(tableName: string, id: string): Promise<T> {
		const response = await fetch(recordUrl(tableName, 'card', id));
		return handleApiResponse<T>(response, `get ${tableName} ${id}`);
	},

	async getRecordWithCaptions<T = TableRecord>(tableName: string, id: string, flowFilters?: TableFilter[]): Promise<DataWithCaptions<T>> {
		const query = flowFilters && flowFilters.length > 0 ? `?flow_filters=${encodeURIComponent(JSON.stringify(flowFilters))}` : '';
		const response = await fetch(recordUrl(tableName, 'card', id) + query);
		return handleApiResponseWithCaptions<T>(response, `get ${tableName} ${id}`);
	},

	async getTableOptions(tableName: string): Promise<Record<string, Record<string, string>>> {
		// Fast endpoint that only returns option metadata (no records)
		if (!tableName) {
			console.warn('getTableOptions called with empty tableName');
			return {};
		}
		const response = await fetch(`${API_BASE}/tables/${tableName}/options`);
		if (!response.ok) {
			console.error(`getTableOptions failed for ${tableName}: ${response.statusText}`);
			return {};
		}
		const result = await response.json();
		return result.data?.options || {};
	},

	async getTableOptionsAndLookups(tableName: string): Promise<{ options: Record<string, Record<string, string>>; lookups: Record<string, LookupData> }> {
		// Fast endpoint that returns both options and lookup data (includes columns, rows for advanced lookups)
		if (!tableName) {
			console.warn('getTableOptionsAndLookups called with empty tableName');
			return { options: {}, lookups: {} };
		}
		const response = await fetch(`${API_BASE}/tables/${tableName}/options`);
		if (!response.ok) {
			console.error(`getTableOptionsAndLookups failed for ${tableName}: ${response.statusText}`);
			return { options: {}, lookups: {} };
		}
		const result = await response.json();
		return {
			options: result.data?.options || {},
			lookups: result.data?.lookups || {}
		};
	},

	// pageId: a list saving its row — the response also carries the page's calculated fields of
	// the row (its footer)
	async insertRecord<T = TableRecord>(tableName: string, data: Partial<T>, pageId?: number): Promise<T> {
		const response = await fetch(`${API_BASE}/tables/${tableName}/insert${pageId ? `?page_id=${pageId}` : ''}`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(data)
		});
		return handleApiResponse<T>(response, `insert ${tableName}`);
	},

	// A page's OnOpenPage trigger (a journal): its parameters as the trigger left them, the
	// filters of its list (filter group 2) and, when the user must pick a record first, the page
	// to pick from
	async openPage(pageId: number, params: Record<string, string>): Promise<OpenPageResult> {
		const response = await fetch(`${API_BASE}/pages/${pageId}/open`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ params })
		});
		return handleApiResponse<OpenPageResult>(response, `open page ${pageId}`);
	},

	// A user's preferences of one type for a page (the user is the session's): name → data
	async getPreferences(pageId: number, type: string): Promise<{ preference_name: string; preference_data: string }[]> {
		const response = await fetch(`${API_BASE}/preferences/${pageId}/${encodeURIComponent(type)}`);
		return (await handleApiResponse<{ preference_name: string; preference_data: string }[] | null>(response, `preferences ${pageId}`)) ?? [];
	},

	// keepalive: the request outlives the page (sent while the tab closes)
	async savePreference(pageId: number, type: string, name: string, data: Record<string, unknown>, keepalive = false): Promise<void> {
		const response = await fetch(`${API_BASE}/preferences/${pageId}/${encodeURIComponent(type)}`, {
			method: 'POST',
			keepalive,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ preference_name: name, preference_data: data })
		});
		await handleApiResponse(response, `save preference ${pageId}`);
	},

	// A FlowField's drilldown (field.drilldown_flow_filters): the page it opens and the filters,
	// as the FlowField calculates — the record's values, a total account's Totaling, the
	// FlowFilters set (ISO, as the list sends them)
	async drilldown(
		pageId: number,
		field: string,
		record: Record<string, unknown>,
		flowFilters: TableFilter[] = []
	): Promise<{ page_id: number; filters: TableFilter[] }> {
		const response = await fetch(`${API_BASE}/pages/${pageId}/drilldown`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ field, record, flow_filters: flowFilters })
		});
		return handleApiResponse<{ page_id: number; filters: TableFilter[] }>(response, `drilldown ${pageId} ${field}`);
	},

	async modifyRecord<T = TableRecord>(
		tableName: string,
		id: string,
		data: Partial<T>,
		pageId?: number
	): Promise<T> {
		const response = await fetch(recordUrl(tableName, 'modify', id) + (pageId ? `?page_id=${pageId}` : ''), {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(data)
		});
		return handleApiResponse<T>(response, `modify ${tableName} ${id}`);
	},

	async deleteRecord(tableName: string, id: string): Promise<void> {
		const response = await fetch(recordUrl(tableName, 'delete', id), {
			method: 'DELETE'
		});
		return handleApiResponseVoid(response, `delete ${tableName} ${id}`);
	},

	// Validate a field (VALIDATE). Pass the in-progress record so the field's
	// OnValidate trigger can see and fill in sibling fields; the result carries the
	// resulting record.
	/**
	 * Run a page's OnAfterGetRecord triggers for the record it shows: the fields they made
	 * editable / read-only or visible / hidden, and the values of its calculated fields
	 */
	async getFieldStates(
		pageId: number,
		record: TableRecord
	): Promise<{ states: Record<string, FieldState>; values: Record<string, any> }> {
		const response = await fetch(`${API_BASE}/pages/${pageId}/field-states`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(record)
		});
		const data = await handleApiResponse<{ field_states: Record<string, FieldState>; values?: Record<string, any> }>(
			response,
			`field states of page ${pageId}`
		);
		return { states: data.field_states ?? {}, values: data.values ?? {} };
	},

	// A card's period grid section: the record's FlowFields per period (or per record of the
	// grid's rows table) — POST /api/pages/:id/period-grid
	async periodGrid(
		pageId: number,
		request: {
			section: string;
			record: TableRecord;
			filters: { field: string; expression: string }[];
			view_by: number;
			view_as: number;
			date: string;
			step: number;
			rows: number;
		}
	): Promise<{ rows: PeriodGridRow[]; date: string }> {
		const response = await fetch(`${API_BASE}/pages/${pageId}/period-grid`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(request)
		});
		const data = await handleApiResponse<{ rows: PeriodGridRow[]; date: string }>(response, `period grid of page ${pageId}`);
		return { rows: data.rows ?? [], date: data.date ?? '' };
	},

	async matrix(
		pageId: number,
		request: MatrixRequest
	): Promise<{ columns: MatrixColumn[]; rows: MatrixRow[]; date: string; period?: MatrixColumn }> {
		const response = await fetch(`${API_BASE}/pages/${pageId}/matrix`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(request)
		});
		const data = await handleApiResponse<{ columns: MatrixColumn[]; rows: MatrixRow[]; date: string; period?: MatrixColumn }>(response, `matrix of page ${pageId}`);
		return { columns: data.columns ?? [], rows: data.rows ?? [], date: data.date ?? '', period: data.period };
	},

	// A value typed into a matrix cell (the row's key, the column's period)
	async matrixValidate(
		pageId: number,
		request: MatrixRequest & { key: string; start: string; end: string; value: string; rounding_factor: number; column_filters?: Record<string, string> }
	): Promise<void> {
		const response = await fetch(`${API_BASE}/pages/${pageId}/matrix/validate`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(request)
		});
		await handleApiResponse<unknown>(response, `matrix cell of page ${pageId}`);
	},

	async validateField(
		tableName: string,
		fieldName: string,
		value: any,
		record?: TableRecord
	): Promise<ValidateFieldResult> {
		const response = await fetch(`${API_BASE}/tables/${tableName}/validate`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ field: fieldName, value, record })
		});

		if (!response.ok) {
			throw new Error(`Failed to validate field: ${response.statusText}`);
		}

		const result: ApiResponse<TableRecord> = await response.json();
		return {
			valid: result.success,
			error: result.error,
			record: result.data
		};
	},

	// Get a new, not yet inserted record with its defaults applied (OnNewRecord)
	// The initial values of a new record; with a page that has OnNewRecord triggers, the record
	// the new one is inserted next to (xRec, below it when belowXRec) gives more of them
	async initRecord<T = TableRecord>(
		tableName: string,
		newRecord?: { pageId: number; xRec?: Record<string, any>; belowXRec: boolean; values?: Record<string, any> }
	): Promise<T> {
		const response = await fetch(`${API_BASE}/tables/${tableName}/init`, {
			method: 'POST',
			...(newRecord && {
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					page_id: newRecord.pageId,
					x_rec: newRecord.xRec ?? null,
					below_x_rec: newRecord.belowXRec,
					values: newRecord.values ?? null
				})
			})
		});
		return handleApiResponse<T>(response, `init ${tableName}`);
	},

	// Run codeunit by ID with record data
	async runCodeunit(codeunitId: number, record: Record<string, any>): Promise<CodeunitResult> {
		const response = await fetch(`${API_BASE}/codeunits/run`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ codeunit_id: codeunitId, record })
		});
		return handleApiResponse<CodeunitResult>(response, `run codeunit ${codeunitId}`);
	},

	// Authentication
	async login(userID: string, password: string, company?: string): Promise<ApiResponse> {
		const response = await fetch(`${API_BASE}/auth/login`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ user_id: userID, password, company })
		});
		return handleApiResponseFull(response, 'login');
	},

	async logout(): Promise<ApiResponse> {
		const response = await fetch(`${API_BASE}/auth/logout`, {
			method: 'POST'
		});
		return handleApiResponseFull(response, 'logout');
	},

	async getCurrentUser(): Promise<ApiResponse> {
		const response = await fetch(`${API_BASE}/auth/user`);
		return handleApiResponseFull(response, 'get current user');
	},

	async createInitialUser(data: {
		user_id: string;
		user_name: string;
		email: string;
		password: string;
	}): Promise<ApiResponse> {
		const response = await fetch(`${API_BASE}/auth/init`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(data)
		});
		return handleApiResponseFull(response, 'create initial user');
	},

	async setLanguage(language: string, persist: boolean = true): Promise<ApiResponse> {
		const response = await fetch(`${API_BASE}/auth/language`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ language, persist })
		});
		return handleApiResponseFull(response, 'set language');
	},

	async getLanguages(): Promise<{ code: string; name: string }[]> {
		const response = await fetch(`${API_BASE}/auth/languages`);
		const result = await handleApiResponseFull<{ code: string; name: string }[]>(response, 'get languages');
		return result.data || [];
	},

	async listCompanies(): Promise<ApiResponse<CompanyInfo[]>> {
		const response = await fetch(`${API_BASE}/auth/companies`);
		return handleApiResponseFull<CompanyInfo[]>(response, 'list companies');
	},

	async getCompanies(): Promise<CompanyInfo[]> {
		const response = await fetch(`${API_BASE}/auth/companies`);
		const result = await handleApiResponseFull<CompanyInfo[]>(response, 'get companies');
		return result.data || [];
	},

	async setCompany(company: string): Promise<ApiResponse> {
		const response = await fetch(`${API_BASE}/auth/company`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ company })
		});
		return handleApiResponseFull(response, 'set company');
	},

	async createCompany(name: string): Promise<ApiResponse<{ name: string; message: string }>> {
		const response = await fetch(`${API_BASE}/auth/companies`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		});
		return handleApiResponseFull<{ name: string; message: string }>(response, 'create company');
	},

	// Appearance: color themes and the user's personalization
	/** The theme gallery and what the user may do with installed themes */
	async getThemes(): Promise<ThemeList> {
		const response = await fetch(`${API_BASE}/themes`);
		const result = await handleApiResponseFull<ThemeList>(response, 'get themes');
		return result.data ?? { themes: [], can_install: false, can_refresh: false, can_delete: false };
	},

	/** Installs the theme of a GitHub repository (the backend downloads and checks it) */
	async installTheme(url: string): Promise<ThemeInstallResult> {
		const response = await fetch(`${API_BASE}/themes/install`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ url })
		});
		const result = await handleApiResponseFull<ThemeInstallResult>(response, 'install theme');
		return result.data!;
	},

	async refreshTheme(code: string): Promise<ThemeInstallResult> {
		const response = await fetch(`${API_BASE}/themes/${encodeURIComponent(code)}/refresh`, { method: 'POST' });
		const result = await handleApiResponseFull<ThemeInstallResult>(response, 'refresh theme');
		return result.data!;
	},

	async deleteTheme(code: string): Promise<void> {
		const response = await fetch(`${API_BASE}/themes/${encodeURIComponent(code)}`, { method: 'DELETE' });
		await handleApiResponseFull(response, 'delete theme');
	},

	/** The user's regional settings (My Settings): date and number format, work date */
	async getMySettings(): Promise<MySettings> {
		const response = await fetch(`${API_BASE}/my-settings`);
		const result = await handleApiResponseFull<MySettings>(response, 'get my settings');
		return result.data!;
	},

	/** Changes only the fields given (work_date '' = today) */
	async saveMySettings(patch: Partial<MySettings>): Promise<MySettings> {
		const response = await fetch(`${API_BASE}/my-settings`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(patch)
		});
		const result = await handleApiResponseFull<MySettings>(response, 'save my settings');
		return result.data!;
	},

	async getAppearance(): Promise<Appearance & { saved: boolean }> {
		const response = await fetch(`${API_BASE}/appearance`);
		const result = await handleApiResponseFull<Appearance & { saved: boolean }>(response, 'get appearance');
		return result.data!;
	},

	/** Changes only the fields given; the backend checks them (User_Personalization) */
	async saveAppearance(patch: Partial<AppearanceSettings>): Promise<Appearance> {
		const response = await fetch(`${API_BASE}/appearance`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(patch)
		});
		const result = await handleApiResponseFull<Appearance>(response, 'save appearance');
		return result.data!;
	},

	// Version
	async getVersion(): Promise<string> {
		try {
			const response = await fetch(`${API_BASE}/version`);
			if (!response.ok) return 'unknown';
			const result = await response.json();
			return result.version || 'unknown';
		} catch {
			return 'unknown';
		}
	}
};

export interface OpenPageResult {
	params: Record<string, string>;
	fixed_filters: FixedFilter[];
	select: { page_id: number; param: string; field: string; filters?: FixedFilter[] } | null;
}

/** My Settings: '' formats follow the language, '' work date is today */
export interface MySettings {
	date_format: string;
	number_format: string;
	work_date: string;
	today?: string;
}
