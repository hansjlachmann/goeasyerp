<script lang="ts">
	import { dropdownPageSize, pageIndex } from '$utils/dropdownPaging';
	import { cn } from '$lib/utils/cn';
	import { toast } from '$lib/stores/toast';
	import { t, ERR, MSG } from '$lib/services/i18n.svelte';
	import type { LookupColumn } from '$lib/types/api';
	import { clickOutside } from '$lib/actions/clickOutside';
	import { floating } from '$lib/actions/floating';
	import { api } from '$lib/services/api';
	import { pageNavigator } from '$lib/utils/pageNavigator';
	import { withReturn } from '$lib/utils/returnUrl';
	import type { TableFilter } from '$lib/types/api';
	import Modal from '../Modal.svelte';
	import PageRenderer from './PageRenderer.svelte';
	import PageFrames from './PageFrames.svelte';

	interface LookupRow {
		_key: string;
		[key: string]: any;
	}

	interface Props {
		// On a card: ↑/↓ on the closed field move to the field above/below (Alt+↓ / F4 open it)
		arrowsMoveFields?: boolean;
		columns: LookupColumn[];
		rows: LookupRow[];
		value: string;
		fieldName?: string; // Field name for error messages
		captions?: Record<string, string>; // Field captions for column headers
		searchTimeout?: number; // Auto-clear search timeout in ms (default 1500)
		tabindex?: number;
		disabled?: boolean;
		error?: boolean;
		compact?: boolean; // Compact mode for list page edit cells (no border, tight padding)
		// Large related table: rows are not passed in but loaded from this URL when the
		// dropdown opens and while the user types (server search), at most a page at a time
		lazyUrl?: string;
		freeText?: boolean; // lookup only (validate: false): typed text is kept as it is
		// Open when the input gets the focus — only when asked for (a list cell entered with
		// Alt+↓ or its ▼): moving the cursor into the field never opens the lookup
		openOnFocus?: boolean;
		// The related table's list page: the lookup shows New, which opens it with a new record
		newPageId?: number;
		// New with the relation's filters (an item's units of measure): the list opens filtered
		newPath?: string;
		// The related table's field the relation matches: New opens the list over the page (a
		// new record ready to type) and Enter/OK there picks the record into this field
		newKeyField?: string;
		onselect?: (key: string) => void;
		onblur?: () => void;
	}

	let {
		arrowsMoveFields = false,
		columns,
		rows,
		value = '',
		fieldName = '',
		captions = {},
		searchTimeout = 1500,
		tabindex,
		disabled = false,
		error = false,
		compact = false,
		lazyUrl,
		freeText = false,
		openOnFocus = false,
		newPageId,
		newPath,
		newKeyField,
		onselect,
		onblur
	}: Props = $props();

	// Where the pages it opens go: the browser, or the page frame it is in
	const pageNav = pageNavigator();

	let isOpen = $state(false);
	// the open list (moved to <body> by floating, so nothing clips it)
	let panelRef = $state<HTMLDivElement | null>(null);
	let containerRef = $state<HTMLDivElement | null>(null);
	let inputRef = $state<HTMLInputElement | null>(null);
	let bodyRef = $state<HTMLDivElement | null>(null);
	let selectedIndex = $state(-1);

	// Track input value separately from actual value (for typing)
	// svelte-ignore state_referenced_locally - synced via $effect below
	let inputValue = $state(value || '');

	// Sync inputValue when value changes externally
	$effect(() => {
		inputValue = value || '';
	});

	// On-demand rows (lazyUrl): the rows of the last search and how many match in total
	let loadedRows = $state<LookupRow[]>([]);
	let loadedTotal = $state(0);
	let loading = $state(false);
	let loadSeq = 0;
	let searchTimer: ReturnType<typeof setTimeout> | undefined;

	// The rows the dropdown works with: passed in, or loaded on demand
	const allRows = $derived(lazyUrl ? loadedRows : rows);

	// Load rows matching search (only the latest request may set them). After loading,
	// highlight the current value, else the first row.
	async function loadRows(search: string) {
		if (!lazyUrl) return;
		const seq = ++loadSeq;
		loading = true;
		try {
			const result = await api.lookupRows(lazyUrl, { search });
			if (seq !== loadSeq) return;
			loadedRows = result.rows ?? [];
			loadedTotal = result.total ?? loadedRows.length;
			selectedIndex = Math.max(0, loadedRows.findIndex((r) => r._key === value));
			if (loadedRows.length === 0) selectedIndex = -1;
		} catch {
			if (seq === loadSeq) loadedRows = [];
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	// The search term: what the user typed, unless it is still the current value
	function searchTerm(): string {
		return inputValue && inputValue !== value ? inputValue : '';
	}

	// Filter rows based on input value (type-ahead filtering)
	// Don't filter if input matches the current selected value (user hasn't started searching)
	const filteredRows = $derived(() => {
		// On-demand rows are already the server's search result
		if (lazyUrl) return loadedRows;
		if (!inputValue) return rows;
		// If input is the current, existing value, show all rows (user opened dropdown without
		// typing). A value that is not an existing key (e.g. the first character typed into a
		// list cell) is a search term, so it filters.
		if (inputValue === value && rows.some(r => r._key === value)) return rows;
		const term = inputValue.toLowerCase();
		return rows.filter(row => rowMatches(row, term));
	});

	// Type-ahead match: code (key) starts with the term, or any column contains it
	function rowMatches(row: LookupRow, term: string): boolean {
		if (row._key.toLowerCase().startsWith(term)) return true;
		return columns.some(col => {
			const val = row[col.source];
			if (val === null || val === undefined) return false;
			return String(val).toLowerCase().includes(term);
		});
	}

	// Scroll selected row into view when navigating with keyboard
	$effect(() => {
		if (isOpen && selectedIndex >= 0 && bodyRef) {
			const rowElements = bodyRef.querySelectorAll('.lookup-row');
			if (rowElements[selectedIndex]) {
				rowElements[selectedIndex].scrollIntoView({ block: 'nearest' });
			}
		}
	});

	// Get display value for current selection (show key/code)
	const displayValue = $derived(() => {
		if (!value) return '';
		return value; // Just show the key/code value
	});

	// Get column header text
	function getColumnHeader(source: string): string {
		return captions[source] || source.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
	}

	// Format cell value for display
	function formatCellValue(val: any): string {
		if (val === null || val === undefined) return '';
		if (typeof val === 'boolean') return val ? 'Yes' : 'No';
		return String(val);
	}

	function openDropdown() {
		if (disabled) return;
		// Don't re-open if a selection was just made (handleSelect re-focuses input)
		if (selectHandled) return;
		isOpen = true;
		if (lazyUrl) {
			loadRows(searchTerm());
			return;
		}
		// Find current selection index in filtered rows
		selectedIndex = filteredRows().findIndex(r => r._key === value);
		if (selectedIndex < 0 && filteredRows().length > 0) selectedIndex = 0;
	}

	function handleToggle() {
		if (disabled) return;
		selectHandled = false; // Clear so toggle always works
		if (isOpen) {
			isOpen = false;
		} else {
			openDropdown();
		}
	}

	// Track whether handleSelect already set the value (prevents handleBlur from re-validating)
	let selectHandled = false;

	function handleSelect(row: LookupRow) {
		value = row._key;
		inputValue = row._key;
		isOpen = false;
		selectHandled = true;
		onselect?.(row._key);
		// Re-focus the input — user can Tab to next field naturally.
		// Do NOT call onblur here; the save should only fire when focus leaves the component.
		inputRef?.focus();
	}

	// Handle input change (user typing)
	function handleInput(e: Event) {
		const target = e.target as HTMLInputElement;
		inputValue = target.value;

		// User typed after a selection — need to re-validate on blur
		selectHandled = false;

		// Open dropdown when typing
		if (!isOpen && inputValue) {
			openDropdown();
		} else if (lazyUrl && isOpen) {
			// Search on the server shortly after the user stops typing
			clearTimeout(searchTimer);
			searchTimer = setTimeout(() => loadRows(searchTerm()), 150);
		}

		// Reset selection to first match
		selectedIndex = 0;
	}

	// Handle blur - validate and set value
	function handleBlur() {
		// Small delay to allow click on dropdown to register
		setTimeout(() => {
			// Focus still inside the component (e.g. input re-focused after handleSelect) — do nothing
			if (containerRef?.contains(document.activeElement) || panelRef?.contains(document.activeElement)) {
				return;
			}

			isOpen = false;

			// If handleSelect already set the value, skip re-validation but still fire onblur
			if (selectHandled) {
				selectHandled = false;
				onblur?.();
				return;
			}

			// Validate input - find matching row
			const trimmedInput = inputValue.trim().toUpperCase();
			if (trimmedInput) {
				// Find exact match by key (case-insensitive)
				const matchingRow = allRows.find(r => r._key.toUpperCase() === trimmedInput);
				if (matchingRow) {
					value = matchingRow._key;
					inputValue = matchingRow._key;
					onselect?.(matchingRow._key);
				} else if (freeText) {
					// The lookup only (validate: false): the text as typed (a filter)
					value = inputValue.trim();
					inputValue = value;
					onselect?.(value);
				} else if (lazyUrl) {
					// Not among the loaded rows: take the typed key; saving the record checks
					// that it exists (the table relation is validated on the server)
					value = trimmedInput;
					inputValue = trimmedInput;
					onselect?.(trimmedInput);
				} else {
					// No match - show error and revert to previous value
					const fieldLabel = fieldName || 'Value';
					toast.error(t(ERR.FIELD_NOT_EXIST, fieldLabel, inputValue.trim()));
					inputValue = value || '';
				}
			} else {
				// Empty input - clear value
				value = '';
				inputValue = '';
				onselect?.('');
			}

			onblur?.();
		}, 150);
	}

	// New: the related table's list with a new record ready to type (filtered like the relation)
	function openNew() {
		if (!newPageId) return;
		isOpen = false;
		const path = newPath ? `${newPath}&new=1` : `/pages/${newPageId}?new=1`;
		if (newKeyField) {
			// over the page: the record being entered stays as it is
			const filters: TableFilter[] = new URL(path, 'http://x').searchParams.getAll('filter').map((f) => {
				const i = f.indexOf('=');
				return { field: f.slice(0, i), expression: f.slice(i + 1) };
			});
			newPick = { pageId: newPageId, filters };
			return;
		}
		pageNav.open(withReturn(path));
	}

	// New over the page (lookup mode, a new record ready to type): Enter or OK picks the record
	// — the one just entered — into the field; Cancel/Esc leaves the field as it was
	let newPick = $state<{ pageId: number; filters: TableFilter[] } | null>(null);
	function closeNew(action: string, picked?: Record<string, any>) {
		newPick = null;
		if (action === 'LookupOK' && picked && newKeyField) {
			const key = String(picked[newKeyField] ?? '');
			if (key) {
				handleSelect({ _key: key } as LookupRow);
				return;
			}
		}
		inputRef?.focus();
	}

	function handleKeydown(e: KeyboardEvent) {
		// Alt+N in the open list: its New (the list's own Alt+N must not start a new row)
		if (isOpen && newPageId && e.altKey && (e.key === 'n' || e.key === 'N' || e.code === 'KeyN')) {
			e.preventDefault();
			e.stopPropagation();
			openNew();
			return;
		}
		switch (e.key) {
			case 'Escape':
				if (isOpen) {
					e.preventDefault();
					isOpen = false;
					// Revert to current value
					inputValue = value || '';
				}
				// If dropdown is closed, let Escape bubble up to close the card page
				break;
			case 'ArrowDown':
				if (!isOpen && arrowsMoveFields && !e.altKey) break;
				e.preventDefault();
				if (!isOpen) {
					selectHandled = false; // Clear so ArrowDown always opens
					openDropdown();
				} else {
					selectedIndex = Math.min(selectedIndex + 1, filteredRows().length - 1);
				}
				break;
			case 'ArrowUp':
				if (!isOpen && arrowsMoveFields && !e.altKey) break;
				e.preventDefault();
				if (isOpen) {
					selectedIndex = Math.max(selectedIndex - 1, 0);
				}
				break;
			case 'PageDown':
			case 'PageUp':
				// A page of rows down / up in the open list (long lists: G/L accounts)
				if (isOpen) {
					e.preventDefault();
					selectedIndex = pageIndex(selectedIndex, filteredRows().length, dropdownPageSize(bodyRef, '.lookup-row'), e.key === 'PageDown' ? 1 : -1);
				}
				break;
			case 'Enter':
				if (isOpen) {
					// Open: Enter selects the highlighted row
					e.preventDefault();
					if (selectedIndex >= 0 && selectedIndex < filteredRows().length) {
						handleSelect(filteredRows()[selectedIndex]);
					}
				} else {
					// Closed: commit what was typed, then let Enter bubble — in a list cell to
					// move to the next row / new row, on a card page to move to the next field
					//. Never swallow it, or the user is stuck in the field. Open the
					// dropdown with ArrowDown/Alt+ArrowDown/F4 instead.
					if (!commitTypedInput()) {
						e.preventDefault();
						e.stopPropagation();
					}
				}
				break;
			case 'Tab':
				// Commit what the user typed now, synchronously: the list page saves the cell
				// as soon as Tab bubbles up, before the delayed blur validation would run.
				if (!commitTypedInput()) {
					// No matching record: stay in the field, don't save a bad value
					e.preventDefault();
					e.stopPropagation();
					return;
				}
				isOpen = false;
				break;
			case 'F4':
				// F4 toggles dropdown (classic ERP style)
				e.preventDefault();
				handleToggle();
				break;
		}
	}

	// Resolve the typed text to a record and select it (leaving a relation field with a
	// partial value picks the matching record). Exact key match first, case-insensitive, so
	// "hans" becomes "HANS" (Code fields are uppercase); otherwise the row highlighted in the
	// open dropdown (the first type-ahead match). Returns false if nothing matches.
	function commitTypedInput(): boolean {
		if (selectHandled) return true; // already selected via click/Enter
		const typed = inputValue.trim();
		if (!typed) {
			value = '';
			inputValue = '';
			selectHandled = true;
			onselect?.('');
			return true;
		}
		const term = typed.toLowerCase();
		const exact = allRows.find((r) => r._key.toLowerCase() === term);
		if (freeText) {
			// The lookup only (a filter like 1000..1999): an exact key or the text as typed
			value = exact?._key ?? typed;
			inputValue = value;
			selectHandled = true;
			onselect?.(value);
			return true;
		}
		// The row highlighted in the open dropdown, if it matches what was typed
		const highlighted = isOpen && selectedIndex >= 0 ? filteredRows()[selectedIndex] : undefined;
		const highlightedMatch = highlighted && rowMatches(highlighted, term) ? highlighted : undefined;
		const row =
			exact ??
			highlightedMatch ??
			allRows.find((r) => r._key.toLowerCase().startsWith(term)) ??
			allRows.find((r) => rowMatches(r, term));
		if (!row && lazyUrl) {
			// On-demand rows may not include it (or not be loaded yet): take the typed key
			// synchronously; saving the record checks that it exists on the server
			value = typed.toUpperCase();
			inputValue = value;
			selectHandled = true;
			onselect?.(value);
			return true;
		}
		if (!row) {
			toast.error(t(ERR.FIELD_NOT_EXIST, fieldName || 'Value', typed));
			return false;
		}
		value = row._key;
		inputValue = row._key;
		selectHandled = true;
		onselect?.(row._key);
		return true;
	}

	// Handle click outside - close dropdown and revert input
	function handleClickOutside() {
		isOpen = false;
		// Revert input to actual value
		inputValue = value || '';
	}

	// Calculate total width
	const totalWidth = $derived(columns.reduce((sum, col) => sum + (col.width || 100), 0));
</script>

<!-- While its list is open the dropdown owns the keys (data-own-keys): the page's shortcuts
     (Alt+N New, …) must not act on them -->
<div
	class="lookup-dropdown"
	data-own-keys={isOpen ? 'lookup' : undefined}
	bind:this={containerRef}
	style="--dropdown-width: {Math.max(totalWidth + 20, 200)}px"
	use:clickOutside={{ callback: handleClickOutside, enabled: isOpen, also: () => [panelRef] }}
>
	<div class="lookup-input-wrapper">
		<input
			type="text"
			class={cn('lookup-input', compact && 'lookup-input-compact', error && 'input-error')}
			bind:this={inputRef}
			value={inputValue}
			{tabindex}
			{disabled}
			oninput={handleInput}
			onkeydown={handleKeydown}
			onblur={handleBlur}
			onfocus={() => {
				if (openOnFocus) openDropdown();
			}}
			role="combobox"
			aria-haspopup="listbox"
			aria-expanded={isOpen}
			aria-controls={isOpen ? 'lookup-listbox' : undefined}
			autocomplete="off"
		/>
		<button
			type="button"
			class={cn('lookup-arrow-btn', compact && 'lookup-arrow-btn-compact')}
			tabindex={-1}
			{disabled}
			onclick={handleToggle}
			aria-label="Toggle dropdown"
		>
			<span class={cn('lookup-arrow', compact && 'lookup-arrow-compact')}>{isOpen ? '▲' : '▼'}</span>
		</button>
	</div>

	{#if isOpen}
		<!-- the list keeps the focus in the field (mousedown), so it is never focused itself -->
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<div
			class="lookup-panel"
			role="listbox"
			id="lookup-listbox"
			bind:this={panelRef}
			use:floating={containerRef}
			data-own-keys="lookup"
			onmousedown={(e) => e.preventDefault()}
			style="--dropdown-width: {Math.max(totalWidth + 20, 200)}px"
		>
			<!-- Column headers -->
			<div class="lookup-header">
				{#each columns as col}
					<div
						class="lookup-header-cell"
						style="width: {col.width || 100}px"
					>
						{getColumnHeader(col.source)}
					</div>
				{/each}
			</div>

			<!-- Rows -->
			<div class="lookup-body" bind:this={bodyRef}>
				{#each filteredRows() as row, i}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<div
						class={cn('lookup-row', row._key === value && 'selected', i === selectedIndex && 'focused')}
						role="option"
						tabindex="-1"
						aria-selected={row._key === value}
						onclick={() => handleSelect(row)}
						onmouseenter={() => selectedIndex = i}
					>
						{#each columns as col}
							<div
								class="lookup-cell"
								style="width: {col.width || 100}px"
							>
								{formatCellValue(row[col.source])}
							</div>
						{/each}
					</div>
				{/each}
				{#if filteredRows().length === 0}
					<div class="lookup-empty">
						{#if loading}{t(MSG.LOOKUP_LOADING)}{:else}{inputValue ? t(MSG.NO_MATCHES) : t(MSG.NO_RECORDS)}{/if}
					</div>
				{/if}
				{#if lazyUrl && loadedTotal > loadedRows.length}
					<div class="lookup-empty">{t(MSG.LOOKUP_MORE, String(loadedRows.length), String(loadedTotal))}</div>
				{/if}
			</div>
			{#if newPageId}
				<!-- New: the related table's list page with a new record ready to type; Esc there
				     comes back here -->
				<div class="lookup-footer">
					<button
						type="button"
						class="lookup-new-btn"
						tabindex={-1}
						onmousedown={(e) => e.preventDefault()}
						onclick={openNew}
					>
						<span aria-hidden="true">+</span>
						{t(MSG.LOOKUP_NEW)}
						<span class="lookup-new-key">Alt+N</span>
					</button>
				</div>
			{/if}
		</div>
	{/if}
</div>

{#if newPick}
	{@const request = newPick}
	<Modal open={true} size="expanded" onclose={() => closeNew('Cancel')}>
		<div class="new-pick-page" data-own-keys>
			<PageFrames>
				<PageRenderer
					pageid={request.pageId}
					embedded={true}
					initialFilters={request.filters}
					lookupMode={true}
					editableLookup={true}
					startNew={true}
					onclose={(action, _key, picked) => closeNew(action, picked)}
				/>
			</PageFrames>
		</div>
	</Modal>
{/if}

<style>
	.new-pick-page {
		height: 70vh;
		display: flex;
		flex-direction: column;
	}

	.lookup-dropdown {
		position: relative;
		width: 100%;
	}

	.lookup-footer {
		@apply flex border-t border-gray-200 px-2 py-1;
	}

	:global(.dark) .lookup-footer {
		border-color: rgb(var(--gray-700));
	}

	.lookup-new-key {
		margin-left: auto;
		font-size: 0.75rem;
		opacity: 0.6;
	}
	.lookup-new-btn {
		@apply flex items-center gap-1 px-2 py-1 text-sm rounded text-blue-600 hover:bg-blue-50;
	}

	:global(.dark) .lookup-new-btn {
		@apply text-blue-400 hover:bg-blue-900/20;
	}

	.lookup-input-wrapper {
		@apply relative flex items-center;
	}

	.lookup-input {
		@apply w-full px-2 py-1 pr-8 text-left bg-surface border border-gray-300 rounded;
		@apply focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500;
		@apply disabled:opacity-50 disabled:cursor-not-allowed;
		min-height: 2rem;
		font-size: 0.875rem;
	}

	:global(.dark) .lookup-input {
		background-color: var(--color-bg-input, rgb(var(--gray-800)));
		border-color: var(--color-border-secondary, rgb(var(--gray-700)));
		color: var(--color-text-primary, rgb(var(--gray-50)));
	}

	.lookup-input.input-error {
		@apply border-red-500;
	}

	.lookup-arrow-btn {
		@apply absolute right-0 top-0 bottom-0 px-2;
		@apply flex items-center justify-center;
		@apply bg-transparent border-none cursor-pointer;
		@apply hover:bg-gray-100 rounded-r;
		@apply disabled:opacity-50 disabled:cursor-not-allowed;
	}

	:global(.dark) .lookup-arrow-btn:hover {
		background-color: rgb(var(--gray-700));
	}

	.lookup-arrow {
		@apply text-xs text-gray-400;
	}

	/* Compact mode styles (for list page edit cells) */
	.lookup-input-compact {
		border: 0 !important;
		background: transparent !important;
		border-radius: 0 !important;
		min-height: 0 !important;
		height: 1.3em !important;
		max-height: 1.3em !important;
		padding: 2px 18px 2px 6px !important;
		line-height: 1.3 !important;
		font-size: 0.875rem;
		box-shadow: none !important;
		outline: none !important;
	}

	.lookup-input-compact:focus {
		outline: none !important;
		box-shadow: none !important;
		ring: 0 !important;
		border: 0 !important;
		background: transparent !important;
	}

	:global(.dark) .lookup-input-compact,
	:global(.dark) .lookup-input-compact:focus {
		background: transparent !important;
		color: white;
	}

	.lookup-arrow-btn-compact {
		@apply px-1;
	}

	.lookup-arrow-compact {
		font-size: 0.5rem;
	}

	.lookup-panel {
		position: absolute;
		top: 100%;
		left: 0;
		z-index: 50;
		width: var(--dropdown-width);
		min-width: 100%;
		max-height: 300px;
		margin-top: 2px;
		@apply bg-surface border border-gray-300 rounded shadow-lg;
		overflow: hidden;
	}

	/* Dark mode: rows on the blank page background, highlighted row grey */
	:global(.dark) .lookup-panel {
		background-color: rgb(var(--gray-900));
		border-color: rgb(var(--gray-700));
	}

	.lookup-header {
		@apply flex bg-gray-100 border-b border-gray-200;
		position: sticky;
		top: 0;
	}

	:global(.dark) .lookup-header {
		background-color: rgb(var(--gray-800));
		border-color: rgb(var(--gray-700));
	}

	.lookup-header-cell {
		@apply px-2 py-1 text-xs font-semibold text-gray-600 truncate;
		flex-shrink: 0;
	}

	:global(.dark) .lookup-header-cell {
		color: var(--color-text-secondary, rgb(var(--gray-400)));
	}

	.lookup-body {
		max-height: 250px;
		overflow-y: auto;
	}

	.lookup-row {
		@apply flex cursor-pointer;
		@apply hover:bg-gray-100;
	}

	:global(.dark) .lookup-row:hover {
		background-color: rgb(var(--gray-700));
	}

	.lookup-row.selected {
		@apply bg-blue-50;
	}

	:global(.dark) .lookup-row.selected {
		background-color: rgba(0, 131, 143, 0.1);
	}

	.lookup-row.focused {
		@apply bg-gray-100;
	}

	:global(.dark) .lookup-row.focused {
		background-color: rgb(var(--gray-700));
	}

	.lookup-row.selected.focused {
		@apply bg-blue-100;
	}

	:global(.dark) .lookup-row.selected.focused {
		background-color: rgb(var(--gray-700));
	}

	.lookup-cell {
		@apply px-2 py-1 text-sm truncate border-r border-gray-100;
		flex-shrink: 0;
	}

	:global(.dark) .lookup-cell {
		border-color: var(--color-border-secondary, rgb(var(--gray-700)));
		color: var(--color-text-primary, rgb(var(--gray-50)));
	}

	.lookup-cell:last-child {
		@apply border-r-0;
	}

	.lookup-empty {
		@apply px-3 py-4 text-sm text-gray-400 text-center;
	}
</style>
