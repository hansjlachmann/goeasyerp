<script lang="ts">
	import { pageNavigator } from '$lib/utils/pageNavigator';
	import { withNavigationQuery, type NavigationQuery } from '$lib/utils/recordNavigation';
	import { onMount } from 'svelte';
	import type { PageDefinition } from '$lib/types/pages';
	import type { LookupData, FixedFilter, TableFilter } from '$lib/types/api';
	import type { HeaderField } from '$lib/types/pages';
	import { fetchPage } from '$lib/services/pages';
	import { api } from '$lib/services/api';
	import { currentUser } from '$lib/stores/user';
	import { toast } from '$lib/stores/toast';
	import { confirm } from '$lib/stores/confirm';
	import { breadcrumb } from '$lib/stores/breadcrumb';
	import { cardTitle, formatDate, isDateType } from '$lib/utils/fieldHelpers';
	import { t, MSG, ERR, DLG, BTN, PAGE_ERR, FILTER as FLT } from '$lib/services/i18n.svelte';
	import Button from '../Button.svelte';
	import CardPage from './CardPage.svelte';
	import MatrixGrid from './MatrixGrid.svelte';
	import OptionDropdown from './OptionDropdown.svelte';
	import { withReturn } from '$lib/utils/returnUrl';
	import ListPage from './ListPage.svelte';
	import PageRenderer from './PageRenderer.svelte';
	import PageFrames from './PageFrames.svelte';
	import ListPageSkeleton from './ListPageSkeleton.svelte';
	import CardPageSkeleton from './CardPageSkeleton.svelte';
	import ConfirmModal from '../ConfirmModal.svelte';
	import Modal from '../Modal.svelte';
	import { getRecordId, getRecordLabel, getPrimaryKeyField, getPrimaryKeyFields, recordKeyText, canInsertNewRecord, deepCopy, applyServerValues } from '$lib/utils/recordHelpers';
	import { createNavigationActions } from '$lib/utils/navigationHelpers';
	import { getJson } from '$lib/utils/storage';
	import { syncPageLayout } from '$lib/utils/customizationStorage';
	import { estimateRowsPerPage, windowSize, windowOffsetFor, type ListWindowRequest } from '$lib/utils/listWindow';
	import { apiFlowFilters, apiFieldFilters, toApiFlowFilter } from '$lib/utils/flowFilter';
	import { currentLanguage } from '$lib/stores/session';
	import { get } from 'svelte/store';

	interface Props {
		pageid: number;
		recordid?: string;
		initialFilter?: string | string[]; // ?filter=field=expression (several: one per field)
		initialFlowFilter?: string[]; // ?flowfilter=field=expression: the FlowFilters as typed (Date Filter)
		returnUrl?: string; // opened from a drilldown/card action: Esc and close go back here
		initialSelect?: string; // record key to select (returning from a drilldown)
		openCard?: boolean; // and open its modal card again (returning from a card's action)
		startNew?: boolean; // a list opened from a lookup's New: start a new record
		initialEdit?: boolean; // a card opened with Edit (?edit=1): it starts in edit mode
		navigationQuery?: NavigationQuery; // card opened from a list: navigate through its records
		// Opened from a codeunit (RunModal) in a modal: no breadcrumb or navigation; the list is
		// filtered (initialFilters) and read-only, and onclose reports how it was closed — in
		// lookup mode Enter / double-click picks a record ('LookupOK'), Esc cancels
		embedded?: boolean;
		initialFilters?: import('$lib/types/api').TableFilter[];
		// the record's FlowFilters (ISO; SetTableView): a card's period grid starts with them
		initialFlowFilters?: import('$lib/types/api').TableFilter[];
		lookupMode?: boolean;
		// Embedded but editable: a page's own lookup (a journal's template or batch list), where
		// the user may also add and change records, as on the page itself
		editableLookup?: boolean;
		// key: the picked record's id; row: a card's picked period grid row (a period's start date
		// or the row's key — classic GetLastDate / GetLastLocation …)
		onclose?: (action: 'OK' | 'LookupOK' | 'Cancel', key?: string, record?: Record<string, any>, row?: string) => void;
		// A subpage in a card: a list page tied to the card's record — fixed filters (the link),
		// new records get the link values; Esc in navigation mode goes back to the card (onexit);
		// onlineschange after a line was saved or deleted (the card's totals)
		subpage?: boolean;
		// A subpage: its card's edit mode (undefined: the list decides itself). The lines are
		// editable only while the card is (no New/Edit/Delete otherwise), and the card's Edit is
		// the lines' Edit too (the subpage shows none)
		parentEditMode?: boolean;
		fixedFilters?: import('$lib/types/api').FixedFilter[];
		newRecordDefaults?: Record<string, any>;
		onexit?: () => void;
		onlineschange?: () => void;
		// A subpage: when its card bumps this (the header was saved, a function asked for it),
		// the lines reload — with the user's filters, search and position
		refreshToken?: number;
		// The page's parameters from the URL (a journal's template and batch): its OnOpenPage
		// trigger turns them into the list's filters
		openParams?: Record<string, string>;
	}

	let {
		pageid,
		recordid,
		initialFilter,
		initialFlowFilter,
		returnUrl,
		initialSelect,
		openCard = false,
		startNew = false,
		initialEdit = false,
		navigationQuery,
		embedded = false,
		initialFilters,
		initialFlowFilters = [],
		lookupMode = false,
		editableLookup = false,
		onclose,
		subpage = false,
		parentEditMode = undefined,
		fixedFilters = [],
		newRecordDefaults = {},
		onexit,
		onlineschange,
		refreshToken = 0,
		openParams = {}
	}: Props = $props();

	// Where the page goes: the browser, or the page frame it is in
	const pageNav = pageNavigator();

	// ---- OnOpenPage (a journal): the page's parameters, the filters its trigger set ----
	// svelte-ignore state_referenced_locally - the URL's parameters only start the page
	let pageParams = $state<Record<string, string>>({ ...openParams });
	let openFilters = $state<FixedFilter[]>([]);
	// The trigger asks the user to pick a record first (a template), or a header field's lookup
	let pickRequest = $state<{ pageId: number; filters: TableFilter[]; param: string; field: string } | null>(null);
	// The list's fixed filters: a subpage's link and the OnOpenPage trigger's; new records get them
	const listFixedFilters = $derived([...fixedFilters, ...openFilters]);
	const listNewRecordDefaults = $derived({
		...newRecordDefaults,
		// a filter expression (a range) gives a new record no value
		...Object.fromEntries(openFilters.filter((f) => !f.expression).map((f) => [f.field, f.value]))
	});

	// Runs the page's OnOpenPage trigger; false when the user must pick a record first
	async function runOpenTrigger(): Promise<boolean> {
		if (!page?.page.has_open_trigger) return true;
		const result = await api.openPage(page.page.id, pageParams);
		pageParams = result.params ?? {};
		openFilters = result.fixed_filters ?? [];
		if (result.select) {
			pickRequest = {
				pageId: result.select.page_id,
				param: result.select.param,
				field: result.select.field,
				filters: (result.select.filters ?? []).map((f) => ({ field: f.field, expression: String(f.value ?? '') }))
			};
			return false;
		}
		// The page keeps its parameters (reload, back)
		if (!embedded && !subpage && typeof window !== 'undefined') {
			pageNav.replaceState(pageUrlWith(pageParams));
		}
		return true;
	}

	// The cursor into the lines (a card's Tab from its last field before this subpage)
	export function enterLines() {
		listPageRef?.enterLines();
	}

	// This page's URL with other parameters (the rest of the query kept)
	function pageUrlWith(params: Record<string, string>): string {
		const url = pageNav.location();
		for (const [name, value] of Object.entries(params)) {
			if (value === '' || value === undefined || value === null) url.searchParams.delete(name);
			else url.searchParams.set(name, value);
		}
		return url.pathname + url.search;
	}

	// The list's filters and FlowFilters go into its URL, so the way back to it — Esc from a
	// drilldown or an action's page (their back= is this URL), a reload — opens it filtered again
	function keepFiltersInUrl() {
		if (embedded || subpage || lookupMode || typeof window === 'undefined') return;
		const url = pageNav.location();
		url.searchParams.delete('filter');
		url.searchParams.delete('flowfilter');
		for (const f of currentFilters) url.searchParams.append('filter', `${f.field}=${f.expression}`);
		for (const f of currentFlowFilters) url.searchParams.append('flowfilter', `${f.field}=${f.expression}`);
		pageNav.replaceState(url.pathname + url.search);
	}

	// Opens the page again with a changed parameter (a picked template, another batch)
	function reopenWith(param: string, value: string) {
		const params = { ...pageParams, [param]: value };
		if (embedded || subpage) return;
		pageNav.move(pageUrlWith(params), { replaceState: true });
	}

	// The first header field has the focus when the page opens (a journal: Batch Name, so
	// Alt+↓ opens the batches at once); the list never takes it from there
	// The list below the header fields (Tab from the last header field goes into its lines)
	let listPageRef = $state<ReturnType<typeof ListPage>>();

	function focusOnOpen(node: HTMLInputElement, first: boolean) {
		if (!first) return;
		requestAnimationFrame(() => {
			if (pickRequest) return;
			node.focus();
			node.select();
		});
	}

	// A header field's lookup: its page filtered to the list's values of lookup_filters
	function openHeaderLookup(header: HeaderField) {
		if (!header.lookup_page) return;
		const filters: TableFilter[] = [];
		for (const [lookupField, listField] of Object.entries(header.lookup_filters ?? {})) {
			const value = openFilters.find((f) => f.field === listField)?.value;
			if (value !== undefined && value !== null && value !== '') filters.push({ field: lookupField, expression: String(value) });
		}
		pickRequest = { pageId: header.lookup_page, filters, param: header.param, field: header.lookup_field ?? header.source };
	}

	// A picked record (template selection or a header lookup): its field becomes the parameter
	function handlePick(action: 'OK' | 'LookupOK' | 'Cancel', picked?: Record<string, any>) {
		const request = pickRequest;
		pickRequest = null;
		if (!request || action !== 'LookupOK' || !picked) {
			// Nothing picked when the page could not open yet: back where the user came from
			if (request && openFilters.length === 0 && !embedded && !subpage) pageNav.close(returnUrl ?? '/');
			return;
		}
		reopenWith(request.param, String(picked[request.field] ?? ''));
	}

	// A header field's value as typed, as the page parameter: Code values in upper case; a date
	// field takes a date filter in the user's format (t, 01.01..31.01), kept as ISO (null: it
	// cannot be read — the user is told)
	function headerParam(header: HeaderField, value: string): string | null {
		const next = fieldTypes[header.source] && /code/i.test(fieldTypes[header.source]) ? value.toUpperCase() : value;
		if (!isDateType(fieldTypes[header.source])) return next;
		const result = toApiFlowFilter(value, 'date', get(currentLanguage));
		if (result.error !== undefined) {
			toast.error(t(FLT.INVALID, header.caption ?? header.source, result.error));
			return null;
		}
		return result.value ?? '';
	}

	// A header field typed in (on leaving the field or Enter)
	async function handleHeaderChange(header: HeaderField, value: string) {
		const next = headerParam(header, value);
		if (next === null) return;
		if (next === (pageParams[header.param] ?? '')) {
			// The same value (a code typed in lower case): the field shows it as applied
			const input = document.querySelector<HTMLInputElement>(`[data-header-param="${header.param}"]`);
			if (input) input.value = headerValue(header);
			return;
		}
		if (await headerValueRefused(header, next)) return;
		reopenWith(header.param, next);
	}

	// A record's key typed into a header field (a journal's batch): the page's trigger decides
	// which record opens. When that is another one, the typed one does not exist: the user is
	// told and the page stays on the record it shows, with the cursor in the field.
	let headerCheck: Promise<boolean> | null = null;
	function headerValueRefused(header: HeaderField, next: string): Promise<boolean> {
		if (!page?.page.has_open_trigger || !header.lookup_page) return Promise.resolve(false);
		headerCheck ??= (async () => {
			try {
				const result = await api.openPage(page!.page.id, { ...pageParams, [header.param]: next });
				if ((result.params?.[header.param] ?? '').toUpperCase() === next.toUpperCase()) return false;
				const input = document.querySelector<HTMLInputElement>(`[data-header-param="${header.param}"]`);
				if (input) {
					input.value = headerValue(header);
					input.focus();
					input.select();
				}
				if (next !== '') toast.error(t(ERR.FIELD_NOT_EXIST, header.caption ?? header.source, next));
				return true;
			} finally {
				headerCheck = null;
			}
		})();
		return headerCheck;
	}

	// An action with apply_header_fields (Navigate's Find): every header field as typed becomes
	// the page's parameter; unchanged values search again
	function applyHeaderFields() {
		if (!page) return;
		const params = { ...pageParams };
		for (const header of page.page.header_fields ?? []) {
			const input = document.querySelector<HTMLInputElement>(`[data-header-param="${header.param}"]`);
			if (!input) continue;
			const next = headerParam(header, input.value);
			if (next === null) return;
			params[header.param] = next;
		}
		const changed = Object.keys(params).some((name) => (params[name] ?? '') !== (pageParams[name] ?? ''));
		if (changed && !embedded && !subpage) {
			pageNav.move(pageUrlWith(params), { replaceState: true });
		} else {
			void loadListData(0);
		}
	}

	// A header field's value as the user reads it (a date filter's dates in the user's format)
	function headerValue(header: HeaderField): string {
		const value = pageParams[header.param] ?? '';
		if (!isDateType(fieldTypes[header.source])) return value;
		const locale = get(currentLanguage);
		return value.replace(/\d{4}-\d{2}-\d{2}/g, (iso) => formatDate(iso, locale));
	}

	// A subpage reloads its lines when its card asks (refreshToken changes after the first load)
	let lastRefreshToken: number | undefined = undefined;
	$effect(() => {
		const token = refreshToken;
		if (lastRefreshToken !== undefined && token !== lastRefreshToken && page) void loadListData();
		lastRefreshToken = token;
	});

	// State
	let page: PageDefinition | null = $state(null);

	// A subpage follows its card's edit mode: read-only lines without New/Edit/Delete while the
	// card is read-only; while it is in edit mode the card's Edit stands for the lines' Edit
	const listPageDef = $derived(page ? withParentEditMode(page) : null);
	function withParentEditMode(def: PageDefinition): PageDefinition {
		if (parentEditMode === undefined) return def;
		const hidden = parentEditMode ? ['Edit'] : ['New', 'Edit', 'Delete'];
		return {
			...def,
			page: {
				...def.page,
				editable: parentEditMode ? def.page.editable : false,
				actions: (def.page.actions ?? []).filter((a) => !hidden.includes(a.name))
			}
		};
	}

	let captions: Record<string, string> = $state({});
	let fieldTypes: Record<string, string> = $state({}); // Field type metadata (e.g., "bool", "code", "text")
	let options: Record<string, Record<string, string>> = $state({}); // Option field values (enum lookups)
	let lookups: Record<string, LookupData> = $state({}); // Table relation lookup values
	let navigation: Record<string, string> = $state({}); // Navigation translations
	let pageLoading = $state(true);  // Loading page definition
	let dataLoading = $state(false); // Loading data after page definition is known
	let error = $state<string | null>(null);
	let sessionExpired = $state(false); // the page request found no valid session (401)
	let errorStatus = $state(0); // HTTP status of a failed page request

	// A new login: full load of the login page (login starts the app fresh)
	function logInAgain() {
		currentUser.clear();
		window.location.href = '/login';
	}

	// Data for the page
	let record: Record<string, any> = $state({});
	let records: Array<Record<string, any>> = $state([]);

	// List pages load a window of rows (the visible page plus two pages each way, see
	// listWindow.ts); search and sort run on the server over all records
	let listTotal = $state(0);
	let windowOffset = $state(0);
	let windowLimit = $state(windowSize(estimateRowsPerPage(typeof window !== 'undefined' ? window.innerHeight : 800)));
	let listSearch = $state('');
	let listSort = $state<ListWindowRequest['sort']>(null);
	let listLoadSeq = 0; // only the latest request may set the rows (fast typing in search)

	// Track if record was successfully loaded (for distinguishing new vs existing)
	let isExistingRecord = $state(false);
	// Track the current record ID (may differ from URL recordid after insert)
	let currentRecordId = $state<string | undefined>(undefined);

	// RunModal lookup: the record the list's cursor is on (OK picks it)
	let lookupCurrent = $state<Record<string, any> | undefined>(undefined);
	// a card in lookup mode: its period grid's current row (a period's start date or a row key)
	let lookupRow = $state<string | undefined>(undefined);

	// Filters for list pages - parse initialFilter if provided (format: "field=expression")
	// svelte-ignore state_referenced_locally - the initial filters only set the starting state
	let currentFilters: import('$lib/types/api').TableFilter[] = $state(initialFilters ?? parseInitialFilter(initialFilter));
	// FlowFilters as typed in the filter pane (e.g. Date Filter "01.01.26..31.03.26"); sent as ISO
	// svelte-ignore state_referenced_locally - the URL's FlowFilters only set the starting state
	let currentFlowFilters: import('$lib/types/api').TableFilter[] = $state(parseInitialFilter(initialFlowFilter));

	function parseInitialFilter(filter: string | string[] | undefined): import('$lib/types/api').TableFilter[] {
		const filters: import('$lib/types/api').TableFilter[] = [];
		for (const f of Array.isArray(filter) ? filter : filter ? [filter] : []) {
			const eqIndex = f.indexOf('=');
			if (eqIndex > 0) filters.push({ field: f.substring(0, eqIndex), expression: f.substring(eqIndex + 1) });
		}
		return filters;
	}

	// Navigation data for card pages
	let recordIds: string[] = $state([]);
	let currentRecordIndex = $state(-1);

	// Get primary key field name from page definition
	const primaryKeyField = $derived(getPrimaryKeyField(page));
	const primaryKeyFieldsList = $derived(getPrimaryKeyFields(page));

	// Load page definition and data
	onMount(async () => {
		try {
			pageLoading = true;
			error = null;

			// The user's layout of the page (columns, widths) from the server, with the definition
			const layoutSync = syncPageLayout(get(currentUser)?.user_id, Number(pageid));
			// Fetch page definition
			const response = await fetch(`/api/pages/${pageid}`);
			if (!response.ok) {
				// 401: no valid session any more (expired, or e.g. after an update) — offer a new login.
				// The texts are translated in the template: translations may still be loading here.
				sessionExpired = response.status === 401;
				errorStatus = response.status;
				let serverMessage = '';
				if (!sessionExpired) {
					try {
						serverMessage = (await response.json())?.error ?? '';
					} catch {
						// not JSON (e.g. a proxy error page while the server restarts)
					}
				}
				throw new Error(serverMessage);
			}

			const result = await response.json();
			if (!result.success) {
				throw new Error(result.error || 'Failed to load page');
			}

			await layoutSync;
			page = result.data;
			// Opened from a codeunit: a read-only list (the codeunit waits for the user)
			if (embedded && !editableLookup && page) page = { ...page, page: { ...page.page, editable: false } };
			captions = result.captions?.fields || {};
			fieldTypes = result.captions?.field_types || {};
			navigation = result.navigation || {};
			pageLoading = false;

			if (!page) {
				throw new Error('Page data is null');
			}

			// Now show skeleton while loading data
			dataLoading = true;

			// Load data based on page type
			if (page.page.type === 'Card') {
				await loadCardData();
				// Set breadcrumb for card page
				await setBreadcrumbForCardPage();
			} else if (page.page.type === 'List' && page.page.layout.matrix) {
				// A matrix page loads its matrix itself (MatrixGrid)
				if (!embedded && !subpage) breadcrumb.setListPage(page.page.id, page.page.caption, navigation.home);
			} else if (page.page.type === 'List') {
				// A page with an OnOpenPage trigger (a journal) gets its filters first
				if (!(await runOpenTrigger())) return;
				await loadListData(await initialListOffset());
				// Set breadcrumb for list page (not for a page opened from a codeunit)
				if (!embedded && !subpage) breadcrumb.setListPage(page.page.id, page.page.caption, navigation.home);
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Unknown error';
			console.error('Error loading page:', err);
		} finally {
			pageLoading = false;
			dataLoading = false;
		}
	});

	// Set breadcrumb for card page (needs to fetch list page caption if available)
	async function setBreadcrumbForCardPage() {
		if (!page) return;

		let listPageCaption: string | undefined;

		// If card page has a list_page_id, fetch the list page caption
		if (page.page.list_page_id) {
			try {
				const listPageResponse = await fetch(`/api/pages/${page.page.list_page_id}`);
				if (listPageResponse.ok) {
					const listPageResult = await listPageResponse.json();
					if (listPageResult.success) {
						listPageCaption = listPageResult.data.page.caption;
					}
				}
			} catch (err) {
				console.error('Error fetching list page for breadcrumb:', err);
			}
		}

		// Get record label (primary key value like customer number)
		const shownRecordId = recordid ?? currentRecordId; // a card opened with a filter: its first record
		const recordLabelValue = getRecordLabel(record) || shownRecordId;

		breadcrumb.setCardPage(
			page.page.id,
			page.page.caption,
			shownRecordId,
			recordLabelValue,
			page.page.list_page_id,
			listPageCaption,
			navigation.home,
			shownRecordId ? cardTitle(page.page.caption, page.page.data_caption_fields, record, options) : undefined
		);
	}

	// A new card record: blank, or — on a card with OnNewRecord triggers — the record the
	// page's triggers start it as (a Sales Order card: an order)
	async function newCardRecord(): Promise<Record<string, any>> {
		if (!page?.page.has_new_record_triggers) return {};
		try {
			return await api.initRecord(page.page.source_table, { pageId: page.page.id, belowXRec: false });
		} catch (err) {
			console.error('Failed to initialize the new record:', err);
			return {};
		}
	}

	// Load data for card page
	async function loadCardData() {
		if (!page) return;

		try {
			// A setup card opens on the table's single record (blank key); a card opened with a
			// filter (an action's run_page_filter) on the first record in the filter
			let cardRecordId = recordid || (page.page.setup_table ? '' : undefined);
			if (cardRecordId === undefined && currentFilters.length > 0) {
				const ids = await api.getRecordIDs(page.page.source_table, { filters: apiFilters(currentFilters) });
				cardRecordId = ids[0];
			}
			if (cardRecordId !== undefined) {
				// Load specific record with captions (includes option values and lookups)
				const result = await api.getRecordWithCaptions(page.page.source_table, cardRecordId);
				record = result.data;
				// Merge options from the record response (contains enum values)
				if (result.captions?.options) {
					options = result.captions.options;
				}
				// Merge lookups from the record response (table relation values)
				if (result.captions?.lookups) {
					lookups = result.captions.lookups;
				}
				isExistingRecord = true; // Successfully loaded an existing record
				currentRecordId = cardRecordId;
			} else {
				// New record - fetch options/lookups before rendering
				record = await newCardRecord();
				isExistingRecord = false;
				currentRecordId = undefined;

				// Fetch options and lookups (await to ensure they're loaded before render)
				try {
					const result = await api.getTableOptionsAndLookups(page.page.source_table);
					options = result.options;
					lookups = result.lookups;
				} catch (err) {
					console.error('Failed to load options/lookups:', err);
					options = {};
					lookups = {};
				}
			}

			// Load record IDs for navigation if enabled
			if (page.page.enable_navigation && recordid) {
				// Use lightweight IDs-only endpoint
				recordIds = await api.getRecordIDs(page.page.source_table, navigationQuery);

				// Find current record index
				currentRecordIndex = recordIds.indexOf(recordid);
			}
		} catch (err) {
			console.error('Error loading card data:', err);
			record = {};
			isExistingRecord = false; // Record didn't exist or failed to load - treat as new
			currentRecordId = undefined;
		}
	}

	// The list's filters for the API: dates as ISO (typed in the user's format)
	function apiFilters<T extends { field: string; expression: string }>(filters: T[]): T[] {
		return apiFieldFilters(filters, (field) => isDateType(fieldTypes[field]), get(currentLanguage));
	}

	// The list's query: filters, FlowFilters, search over the visible stored fields, sort.
	// Shared by the windows of the list and "delete all selected", so both mean the same records.
	function listQuery(visibleFields = getVisibleFields(), flowFields = new Set(page?.page.flow_fields ?? [])) {
		const query: import('$lib/types/api').ListOptions = {};
		if (!page) return query;
		if (currentFilters.length > 0) {
			query.filters = apiFilters(currentFilters);
		}
		if (listFixedFilters.length > 0) {
			query.fixed_filters = listFixedFilters; // a subpage's link to its card, a journal's batch
		}
		if (Object.keys(pageParams).length > 0) {
			query.page_params = pageParams; // the page's parameters, read by its triggers
		}
		const flowFilters = apiFlowFilters(currentFlowFilters, page.page.flow_filter_fields, get(currentLanguage));
		if (flowFilters.length > 0) {
			query.flow_filters = flowFilters;
		}
		if (listSearch.trim()) {
			// FlowFields are computed, not stored: the server can not search them
			query.search = listSearch.trim();
			query.search_fields = visibleFields.filter((f) => !flowFields.has(f));
		}
		if (listSort) {
			query.sort_by = listSort.field;
			query.sort_order = listSort.direction;
		}
		return query;
	}

	// Load a window of the list (offset/limit, filters, search, sort). windowOffset and
	// records change together when the response arrives, so they always match.
	async function loadListData(offset: number = windowOffset) {
		if (!page) return;
		const seq = ++listLoadSeq;

		try {
			// Determine which fields are visible based on customizations
			const visibleFields = getVisibleFields();
			const flowFields = new Set(page.page.flow_fields ?? []);

			// Load records with only visible fields to avoid expensive FlowField calculations
			const listOptions: import('$lib/types/api').ListOptions = {
				...listQuery(visibleFields, flowFields),
				offset,
				limit: windowLimit
			};
			if (visibleFields.length > 0) {
				listOptions.fields = visibleFields;
			}
			// The page's triggers fill its calculated fields (OnAfterGetRecord), or its rows (a
			// temporary source table: OnFindRecords from the page parameters)
			if (page.page.has_record_triggers || page.page.source_table_temporary) listOptions.page_id = page.page.id;

			// Fetch records, options, and lookups in a single API call
			const response = await api.listRecordsWithOptions(page.page.source_table, listOptions);
			if (seq !== listLoadSeq) return;
			const rows = response.list.records || [];
			const total = response.list.total ?? rows.length;
			// Past the end (rows deleted, search narrowed): load the last window instead
			if (rows.length === 0 && total > 0 && offset > 0) {
				return loadListData(Math.max(0, total - windowLimit));
			}
			records = rows;
			listTotal = total;
			windowOffset = offset;
			options = response.options;
			lookups = response.lookups || {};
		} catch (err) {
			if (seq !== listLoadSeq) return;
			console.error('Error loading list data:', err);
			records = [];
			listTotal = 0;
		}
	}

	// Returning from a drilldown to a list: start with the window around the record to
	// select (its position in the list's filters and order), not at the top
	async function initialListOffset(): Promise<number> {
		if (!page || !initialSelect) return 0;
		try {
			// the list's own query, so the position is the record's place in the list's windows
			const ids = await api.getRecordIDs(page.page.source_table, listQuery());
			const position = ids.indexOf(initialSelect);
			if (position < 0) return 0;
			const perPage = estimateRowsPerPage(typeof window !== 'undefined' ? window.innerHeight : 800);
			return windowOffsetFor(position, perPage, ids.length);
		} catch {
			return 0;
		}
	}

	// ListPage asks for another window: moved past the loaded rows, searched or sorted
	async function handleListWindow(request: ListWindowRequest) {
		if (request.limit !== undefined) windowLimit = request.limit;
		if (request.search !== undefined) listSearch = request.search;
		if (request.sort !== undefined) listSort = request.sort;
		await loadListData(request.offset !== undefined ? Math.max(0, request.offset) : windowOffset);
	}

	// Get visible fields from page definition and user customizations
	function getVisibleFields(): string[] {
		if (!page || !page.page.layout.repeater?.fields) return [];

		// Load user customizations from storage (user-specific)
		let userId = 'anonymous';
		currentUser.subscribe(user => {
			userId = user?.user_id || 'anonymous';
		})();

		const key = `page-customization-${userId}-${page.page.id}`;
		const customizations = getJson<Record<string, { visible: boolean }>>(key, {});

		// Filter to visible fields only
		return page.page.layout.repeater.fields
			.filter(field => {
				// Check if user has customized this field
				if (field.source in customizations) {
					return customizations[field.source].visible;
				}
				// Otherwise use the field's visible property (default true)
				return field.visible !== false;
			})
			.map(field => field.source);
	}

	// Handle actions from card page
	async function handleCardAction(actionName: string) {
		if (!page) return;

		switch (actionName) {
			case 'New':
				record = await newCardRecord();
				isExistingRecord = false; // Reset for new record
				currentRecordId = undefined;
				// Update URL to remove the recordid
				pageNav.replaceState(`/pages/${page.page.id}`);
				break;
			case 'Delete':
				// Get record ID from record object or recordid prop
				const deleteId = getRecordId(record, primaryKeyField, primaryKeyFieldsList) || recordid;
				if (deleteId) {
					confirm.show(
						t(DLG.DELETE_RECORD_TITLE),
						t(DLG.DELETE_RECORD_KEY_CONFIRM, recordKeyText(record, primaryKeyFieldsList, captions, options)),
						async () => {
							try {
								await api.deleteRecord(page!.page.source_table, deleteId);
								toast.success(t(MSG.RECORD_DELETED_SUCCESS));
								// Navigate back to the list page if available
								if (page!.page.type === 'Card') {
									// Try to find the associated list page by convention
									// Customer Card (21) -> Customer List (22)
									const listPageId = page!.page.id + 1;
									pageNav.close(`/pages/${listPageId}`);
								}
							} catch (err) {
								console.error('Delete error:', err);
								toast.error(err instanceof Error && err.message ? err.message : t(ERR.FAILED_DELETE));
							}
						}
					);
				}
				break;
			case 'Refresh':
				await loadCardData();
				break;
		}
	}

	// A subpage's lines changed: read the card's FlowFields (totals) again — only those, so
	// nothing the user is typing on the card is touched
	async function refreshCardTotals() {
		if (!page || currentRecordId === undefined || !record) return;
		try {
			const fresh = await api.getRecordWithCaptions(page.page.source_table, currentRecordId);
			for (const field of page.page.flow_fields ?? []) {
				if (fresh.data && field in fresh.data) record[field] = fresh.data[field];
			}
		} catch (err) {
			console.error('Failed to refresh the card totals:', err);
		}
	}

	// Handle save from card page
	async function handleCardSave(savedRecord: Record<string, any>, opts?: { leavingKey?: boolean }): Promise<boolean> {
		if (!page) return false;

		try {
			if (isExistingRecord && currentRecordId !== undefined) {
				// Update existing record (only if we successfully loaded an existing record)
				const sentRecord = deepCopy(savedRecord);
				const modified = await api.modifyRecord(page.page.source_table, currentRecordId, savedRecord);
				// Values a trigger changed (tidied, recalculated) show at once
				applyServerValues(savedRecord, sentRecord, modified);
				// Renamed (key changed, ): address the record by its new key from now on
				const newRecordId = getRecordId(modified, primaryKeyField, primaryKeyFieldsList);
				if (newRecordId && newRecordId !== currentRecordId) {
					recordIds = recordIds.map((id) => (id === currentRecordId ? newRecordId : id));
					currentRecordId = newRecordId;
					pageNav.replaceState(withNavigationQuery(`/pages/${page.page.id}/${newRecordId}`, navigationQuery));
				}
				// the card's own "Saving… / ✓ Saved" indicator at the top tells it
			} else {
				// Insert a new record only once its primary key is filled in (Record Entry
				// rule); until then field changes stay on the form
				if (!canInsertNewRecord(savedRecord, {}, primaryKeyFieldsList, page.page.auto_key) && !(page.page.auto_key && opts?.leavingKey)) {
					return false;
				}
				const sentRecord = deepCopy(savedRecord);
				const insertedRecord = await api.insertRecord(page.page.source_table, savedRecord, page.page.has_new_record_triggers ? page.page.id : undefined);
				applyServerValues(savedRecord, sentRecord, insertedRecord);
				// After successful insert, mark as existing for future saves
				const newRecordId = getRecordId(insertedRecord, primaryKeyField, primaryKeyFieldsList);
				isExistingRecord = true;
				currentRecordId = newRecordId;
				// Navigate to the proper URL with the new record ID
				if (newRecordId && page.page.id) {
					// Update URL without full page reload
					pageNav.replaceState(`/pages/${page.page.id}/${newRecordId}`);
				}
			}
			return true;
		} catch (err) {
			// Re-throw so CardPage can catch and revert the field value
			throw err;
		}
	}

	// Handle actions from list page
	async function handleListAction(actionName: string, selectedRecord?: Record<string, any>) {
		if (!page) return;
		if (page.page.actions?.find((a) => a.name === actionName)?.apply_header_fields) {
			applyHeaderFields();
			return;
		}

		switch (actionName) {
			case 'New':
				// Navigate to card page in new mode
				if (page.page.card_page_id) {
					pageNav.open(`/pages/${page.page.card_page_id}`);
				}
				break;
			case 'Edit':
				if (selectedRecord && page.page.card_page_id) {
					// Navigate to card page with record ID
					const editRecordId = getRecordId(selectedRecord, primaryKeyField, primaryKeyFieldsList);
					pageNav.open(`/pages/${page.page.card_page_id}/${editRecordId}?edit=1`);
				}
				break;
			case 'Delete':
				if (selectedRecord) {
					const deleteRecordId = getRecordId(selectedRecord, primaryKeyField, primaryKeyFieldsList);
					confirm.show(
						t(DLG.DELETE_RECORD_TITLE),
						t(DLG.DELETE_RECORD_KEY_CONFIRM, recordKeyText(selectedRecord, primaryKeyFieldsList, captions, options)),
						async () => {
							try {
								await api.deleteRecord(page!.page.source_table, deleteRecordId!);
								await loadListData();
								toast.success(t(MSG.RECORD_DELETED));
							} catch (err) {
								toast.error(err instanceof Error && err.message ? err.message : t(ERR.FAILED_DELETE));
							}
						}
					);
				}
				break;
			case 'Refresh':
				await loadListData();
				onlineschange?.(); // a subpage: the card's totals too
				break;
		}
	}

	// Handle row click in list page
	function handleRowClick(clickedRecord: Record<string, any>, navigation?: NavigationQuery) {
		if (embedded) {
			// Opened from a codeunit: Enter / double-click picks the record in lookup mode
			if (lookupMode) onclose?.('LookupOK', getRecordId(clickedRecord, primaryKeyField, primaryKeyFieldsList) ?? '', clickedRecord);
			return;
		}
		if (!page || !page.page.card_page_id) return;

		const recordId = getRecordId(clickedRecord, primaryKeyField, primaryKeyFieldsList);
		pageNav.open(withNavigationQuery(`/pages/${page.page.card_page_id}/${recordId}`, navigation));
	}

	// Handle save notification from list page (inline editing)
	// Note: ListPage already saves the record internally via handleCellBlur,
	// this callback is just for notification - no need to save again
	async function handleListSave(savedRecord: Record<string, any>, isNew: boolean) {
		if (!page) return;
		// Record is already saved by ListPage, just refresh the underlying data. Not awaited:
		// ListPage awaits onsave inside its save, and the next cell's save must not wait for
		// a list reload
		void loadListData();
		onlineschange?.();
	}

	// Handle delete from list page
	async function handleListDelete(deletedRecord: Record<string, any>) {
		if (!page) return;

		try {
			const recordId = getRecordId(deletedRecord, primaryKeyField, primaryKeyFieldsList);
			if (!recordId) {
				console.error('Cannot delete record: no record ID found', deletedRecord);
				toast.error('Cannot delete: record has no ID');
				return;
			}
			await api.deleteRecord(page.page.source_table, recordId);
			await loadListData();
			onlineschange?.();
			toast.success(t(MSG.RECORD_DELETED));
		} catch (err) {
			toast.error(err instanceof Error && err.message ? err.message : t(ERR.FAILED_DELETE));
			console.error('Delete error:', err);
			throw err;
		}
	}

	// Delete the records selected on the list (multi-select): one transaction on the server;
	// when a record cannot be deleted nothing is, and the message names it
	async function handleListBulkDelete(request: { keys?: string[]; all?: boolean }, count: number) {
		if (!page) return;
		try {
			const deleted = await api.deleteRecords(page.page.source_table, request, request.all ? listQuery() : undefined);
			await loadListData();
			onlineschange?.();
			toast.success(t(MSG.RECORDS_DELETED, deleted.toLocaleString(get(currentLanguage))));
		} catch (err) {
			toast.error(err instanceof Error && err.message ? err.message : t(ERR.FAILED_DELETE));
			throw err;
		}
	}

	// Paste records copied on a list (Ctrl+V): one transaction on the server; when a record
	// cannot be inserted nothing is, and the message names it. Returns the inserted keys.
	async function handleListPaste(records: Record<string, unknown>[]): Promise<string[]> {
		if (!page) return [];
		try {
			// In a subpage the pasted lines belong to the card's record: its link values
			const rows = subpage || openFilters.length > 0 ? records.map((r) => ({ ...r, ...listNewRecordDefaults })) : records;
			const keys = await api.insertRecords(page.page.source_table, rows);
			await loadListData();
			onlineschange?.();
			toast.success(t(MSG.RECORDS_PASTED, keys.length.toLocaleString(get(currentLanguage))));
			return keys;
		} catch (err) {
			toast.error(err instanceof Error && err.message ? err.message : t(ERR.FAILED_SAVE_RECORD));
			throw err;
		}
	}

	// Handle filter change from list page
	async function handleFilterChange(filters: import('$lib/types/api').TableFilter[], flowFilters: import('$lib/types/api').TableFilter[] = []) {
		currentFilters = filters;
		currentFlowFilters = flowFilters;
		keepFiltersInUrl();
		await loadListData(0);
	}

	// Navigation functions for card pages
	function navigateToRecord(targetRecordId: string) {
		if (!page) return;
		pageNav.move(withNavigationQuery(`/pages/${page.page.id}/${targetRecordId}`, navigationQuery));
	}

	// Create navigation actions using shared helper
	const navigationActions = createNavigationActions(
		() => ({ recordIds, currentRecordIndex }),
		navigateToRecord
	);

	// Handle closing the card page
	// The loading skeleton appears only when loading takes longer than this: a page that loads
	// quickly (the usual case) shows no flash of placeholder before its content
	const skeletonDelay = 1000;
	let showSkeleton = $state(false);
	// One loading period from the page definition through its data: the timer does not
	// restart between the two
	const loading = $derived(pageLoading || dataLoading);
	$effect(() => {
		if (!loading) {
			showSkeleton = false;
			return;
		}
		const timer = setTimeout(() => (showSkeleton = true), skeletonDelay);
		return () => clearTimeout(timer);
	});

	// Esc on a matrix page (not inside the matrix's cell or filters): back where it came from
	function handleMatrixPageKey(e: KeyboardEvent) {
		if (e.key !== 'Escape' || e.defaultPrevented || embedded || subpage) return;
		if (page?.page.type !== 'List' || !page.page.layout.matrix) return;
		const target = e.target as HTMLElement | null;
		if (target?.closest('input, [role="dialog"]')) return;
		pageNav.close(returnUrl ?? '/');
	}

	// Esc / close on a card page: back to the page that opened it (an action or drilldown of
	// another card — that card again, on its record), else to the card's list, else back in the
	// browser's history (in a page frame: the page under it)
	function handleCardClose() {
		pageNav.close(returnUrl ?? (page?.page.list_page_id ? `/pages/${page.page.list_page_id}` : undefined));
	}
</script>

<svelte:window onkeydown={handleMatrixPageKey} />

{#if pageLoading || (dataLoading && page)}
	<!-- Loading: the skeleton only after skeletonDelay — a quick load shows no placeholder -->
	{#if showSkeleton}
		{#if page?.page.type === 'Card'}
			<CardPageSkeleton />
		{:else}
			<ListPageSkeleton />
		{/if}
	{/if}
{:else if error !== null}
	<!-- error is "" for a failed page request: the screen translates its status -->
	<div class="flex items-center justify-center h-full">
		<div class="text-center">
			<div class="text-red-600 dark:text-red-400 text-5xl mb-4">⚠</div>
			<h2 class="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-2">{t(PAGE_ERR.TITLE)}</h2>
			<p class="text-gray-600 dark:text-gray-400 max-w-md">
				{sessionExpired
					? t(PAGE_ERR.SESSION_EXPIRED)
					: error || t(PAGE_ERR.LOAD_FAILED, String(errorStatus))}
			</p>
			<div class="mt-6 flex justify-center gap-3">
				<Button variant="primary" onclick={logInAgain}>{t(PAGE_ERR.LOG_IN_AGAIN)}</Button>
				{#if !sessionExpired}
					<Button variant="secondary" onclick={() => window.location.reload()}>{t(PAGE_ERR.TRY_AGAIN)}</Button>
				{/if}
			</div>
		</div>
	</div>
{:else if page}
	{#if page.page.type === 'Card' && embedded && lookupMode}
		<!-- Lookup mode (RunModal) on a card: OK picks the card's record and the period grid's
		     current row (Enter / double-click on a row too), Cancel or Esc closes without one -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="lookup-page"
			onkeydown={(e) => {
				if (e.key === 'Escape' && !e.defaultPrevented) {
					e.preventDefault();
					onclose?.('Cancel');
				}
			}}
		>
			<div class="lookup-list">
				<CardPage
					{page}
					existing={isExistingRecord}
					bind:record
					{captions}
					{fieldTypes}
					{options}
					{lookups}
					flowFilters={initialFlowFilters}
					onaction={handleCardAction}
					onclose={() => onclose?.('Cancel')}
					ongridrow={(row) => (lookupRow = row)}
					ongridpick={(row) => onclose?.('LookupOK', currentRecordId ?? '', record, row)}
				/>
			</div>
			<div class="lookup-buttons">
				<Button variant="secondary" onclick={() => onclose?.('Cancel')}>{t(BTN.CANCEL)}</Button>
				<Button
					variant="primary"
					disabled={lookupRow === undefined}
					onclick={() => onclose?.('LookupOK', currentRecordId ?? '', record, lookupRow)}
				>{t(BTN.OK)}</Button>
			</div>
		</div>
	{:else if page.page.type === 'Card'}
		<CardPage
			{page}
			existing={isExistingRecord}
			initialEditMode={initialEdit || undefined}
			bind:record
			{captions}
			{fieldTypes}
			{options}
			{lookups}
			onaction={handleCardAction}
			onsave={handleCardSave}
			onclose={handleCardClose}
			navigationEnabled={page.page.enable_navigation || false}
			canNavigateFirst={currentRecordIndex > 0}
			canNavigatePrevious={currentRecordIndex > 0}
			canNavigateNext={currentRecordIndex >= 0 && currentRecordIndex < recordIds.length - 1}
			canNavigateLast={currentRecordIndex >= 0 && currentRecordIndex < recordIds.length - 1}
			onNavigateFirst={navigationActions.navigateFirst}
			onNavigatePrevious={navigationActions.navigatePrevious}
			onNavigateNext={navigationActions.navigateNext}
			onNavigateLast={navigationActions.navigateLast}
			onrefresh={refreshCardTotals}
		/>
	{:else if page.page.type === 'List' && page.page.layout.matrix}
		<!-- A matrix page (Resource Capacity): the records against periods; a cell's entries open
		     as a page with the way back here; Esc goes back -->
		<div class="matrix-page">
			<h1 class="matrix-page-title">{page.page.caption}</h1>
			<MatrixGrid
				pageId={page.page.id}
				section=""
				matrix={page.page.layout.matrix}
				actions={page.page.actions}
				record={{}}
				ondrilldown={(pageId, filters) => pageNav.open(withReturn(`/pages/${pageId}?` + filters.map((f) => 'filter=' + encodeURIComponent(`${f.field}=${f.expression}`)).join('&')))}
			/>
		</div>
	{:else if page.page.type === 'List'}
		{#if embedded && lookupMode}
			<!-- Lookup mode (RunModal): OK picks the cursor's record, Cancel closes without one -->
			<div class="lookup-page">
				<div class="lookup-list">
				<ListPage
					{page}
					{records}
					{captions}
					{fieldTypes}
					{options}
					{lookups}
					{currentFilters}
					{currentFlowFilters}
					total={listTotal}
					{windowOffset}
					{returnUrl}
					initialSelectKey={initialSelect}
					initialOpenCard={openCard}
					{startNew}
					onclose={embedded ? () => onclose?.(lookupMode ? 'Cancel' : 'OK') : subpage ? onexit : undefined}
					pickOnEnter={embedded}
					{subpage}
					fixedFilters={listFixedFilters}
					newRecordDefaults={listNewRecordDefaults}
					{pageParams}
					oncurrent={embedded && lookupMode ? (r) => (lookupCurrent = r) : undefined}
					onwindow={handleListWindow}
					onaction={handleListAction}
					onrowclick={handleRowClick}
					onsave={handleListSave}
					ondelete={handleListDelete}
					onbulkdelete={handleListBulkDelete}
					onpaste={handleListPaste}
					onfilter={handleFilterChange}
				/>
				</div>
				<div class="lookup-buttons">
					<Button variant="secondary" onclick={() => onclose?.('Cancel')}>{t(BTN.CANCEL)}</Button>
					<Button
						variant="primary"
						disabled={!lookupCurrent}
						onclick={() => onclose?.('LookupOK', getRecordId(lookupCurrent, primaryKeyField, primaryKeyFieldsList) ?? '', lookupCurrent)}
					>{t(BTN.OK)}</Button>
				</div>
			</div>
		{:else}
			{#if page.page.header_fields?.length && (openFilters.length > 0 || page.page.source_table_temporary)}
				<!-- Header fields above the list (a journal's Batch Name) -->
				<div class="page-header-fields">
					{#each page.page.header_fields as header (header.param)}
						<label class="header-field">
							<span class="header-caption">{header.caption ?? header.source}</span>
							{#if header.options?.length}
								<span class="header-input-wrap header-option">
									<OptionDropdown
										options={Object.fromEntries(header.options.map((o, i) => [String(i), o.caption]))}
										value={Math.max(0, header.options.findIndex((o) => o.value === (pageParams[header.param] ?? '')))}
										onselect={(i) => {
											const next = header.options?.[i]?.value ?? '';
											if (next !== (pageParams[header.param] ?? '')) reopenWith(header.param, next);
										}}
									/>
								</span>
							{:else}
							<span class="header-input-wrap">
								<input
									class="header-input"
									class:header-input-date={isDateType(fieldTypes[header.source])}
									data-header-param={header.param}
									use:focusOnOpen={header === page.page.header_fields[0]}
									value={headerValue(header)}
									data-applied-value={headerValue(header)}
									autocomplete="off"
									onchange={(e) => handleHeaderChange(header, (e.currentTarget as HTMLInputElement).value)}
									onkeydown={(e) => {
										if (e.key === 'Enter') {
											e.preventDefault();
											handleHeaderChange(header, (e.currentTarget as HTMLInputElement).value);
										} else if (e.key === 'Tab' && !e.shiftKey && header === page?.page.header_fields?.at(-1)) {
											// Tab from the last header field: into the lines (their first column)
											e.preventDefault();
											listPageRef?.enterLines();
										} else if ((e.key === 'ArrowDown' && e.altKey) || e.key === 'F4') {
											e.preventDefault();
											openHeaderLookup(header);
										}
									}}
								/>
								{#if header.lookup_page}
									<button type="button" class="header-lookup" tabindex="-1" aria-label={header.caption} onclick={() => openHeaderLookup(header)}>▼</button>
								{/if}
							</span>
							{/if}
						</label>
					{/each}
				</div>
			{/if}
			<ListPage
				bind:this={listPageRef}
				page={listPageDef ?? page}
				editOnClick={subpage && parentEditMode === true}
				{records}
				{captions}
				{fieldTypes}
				{options}
				{lookups}
				{currentFilters}
				{currentFlowFilters}
				total={listTotal}
				{windowOffset}
				{returnUrl}
				initialSelectKey={initialSelect}
				initialOpenCard={openCard}
				{startNew}
				onclose={embedded ? () => onclose?.(lookupMode ? 'Cancel' : 'OK') : subpage ? onexit : undefined}
				pickOnEnter={embedded}
				{subpage}
				fixedFilters={listFixedFilters}
				newRecordDefaults={listNewRecordDefaults}
				{pageParams}
				oncurrent={embedded && lookupMode ? (r) => (lookupCurrent = r) : undefined}
				onwindow={handleListWindow}
				onaction={handleListAction}
				onrowclick={handleRowClick}
				onsave={handleListSave}
				ondelete={handleListDelete}
				onbulkdelete={handleListBulkDelete}
				onpaste={handleListPaste}
				onfilter={handleFilterChange}
			/>
		{/if}
	{:else}
		<div class="flex items-center justify-center h-full">
			<div class="text-center">
				<p class="text-gray-600 dark:text-gray-400">Page type "{page.page.type}" not yet supported</p>
			</div>
		</div>
	{/if}
{/if}

<!-- A record to pick before the page opens (a journal template), or a header field's lookup -->
{#if pickRequest}
	{@const request = pickRequest}
	<Modal open={true} size="expanded" onclose={() => handlePick('Cancel')}>
		<div class="pick-page">
			<PageFrames>
				<PageRenderer
					pageid={request.pageId}
					embedded={true}
					initialFilters={request.filters}
					lookupMode={true}
					editableLookup={true}
					onclose={(action, _key, picked) => handlePick(action, picked)}
				/>
			</PageFrames>
		</div>
	</Modal>
{/if}

<!-- Confirm Modal -->
<ConfirmModal
	open={$confirm.open}
	title={$confirm.title}
	message={$confirm.message}
	confirmText={$confirm.confirmText ?? t(BTN.DELETE)}
	variant={$confirm.variant ?? 'danger'}
	onconfirm={confirm.confirm}
	oncancel={confirm.cancel}
/>

<style>
	.matrix-page {
		padding: 1rem 1.5rem;
	}

	.matrix-page-title {
		font-size: 1.5rem;
		font-weight: 600;
		margin-bottom: 1rem;
		color: rgb(var(--accent-600));
	}

	:global(.dark) .matrix-page-title {
		color: rgb(var(--accent-400));
	}

	/* Header fields above a list (a journal's Batch Name) */
	.page-header-fields {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem 2rem;
		margin-bottom: 0.75rem;
	}
	.header-field {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.875rem;
	}
	.header-caption {
		color: rgb(var(--gray-600));
	}
	:global(.dark) .header-caption {
		color: rgb(var(--gray-400));
	}
	.header-input-wrap {
		display: inline-flex;
		align-items: stretch;
		border: 1px solid rgb(var(--gray-300));
		border-radius: 0.25rem;
		background-color: var(--color-bg-input);
		color: var(--color-text-primary);
	}
	:global(.dark) .header-input-wrap {
		border-color: rgb(var(--gray-600));
	}
	.header-input-wrap:focus-within {
		border-color: rgb(var(--accent-500));
	}
	/* a header field with options (Report Selection's Usage) */
	.header-option {
		min-width: 14rem;
	}
	/* a date header field holds a date filter (a range) */
	.header-input.header-input-date {
		width: 15rem;
	}
	.header-input {
		width: 9rem;
		padding: 0.125rem 0.5rem;
		background: transparent;
		border: none;
		outline: none;
		color: inherit;
	}
	.header-lookup {
		padding: 0 0.4rem;
		font-size: 0.65rem;
		color: rgb(var(--gray-500));
	}
	.pick-page {
		height: 70vh;
		display: flex;
		flex-direction: column;
	}

	/* RunModal in lookup mode: the list with OK and Cancel below it */
	.lookup-page {
		@apply flex flex-col h-full gap-3;
	}

	.lookup-list {
		@apply flex-1 min-h-0;
	}

	.lookup-buttons {
		@apply flex justify-end gap-2;
	}
</style>
