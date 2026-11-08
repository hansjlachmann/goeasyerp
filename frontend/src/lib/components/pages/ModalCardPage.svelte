<script lang="ts">
	import Modal from '$lib/components/Modal.svelte';
	import CardPage from './CardPage.svelte';
	import NavigationButtons from '$lib/components/NavigationButtons.svelte';
	import type { PageDefinition } from '$lib/types/pages';
	import type { LookupData } from '$lib/types/api';
	import { api } from '$lib/services/api';
	import { onMount, untrack } from 'svelte';
	import { getRecordId, getPrimaryKeyField } from '$lib/utils/recordHelpers';
	import { cardTitle } from '$lib/utils/fieldHelpers';
	import { createNavigationActions, canNavigatePrevious, canNavigateNext } from '$lib/utils/navigationHelpers';
	import { withNavigationQuery, type NavigationQuery } from '$lib/utils/recordNavigation';
	import { t, MODAL } from '$lib/services/i18n.svelte';

	interface Props {
		open?: boolean;
		page: PageDefinition;
		record?: Record<string, any>;
		captions?: Record<string, string>;
		fieldTypes?: Record<string, string>; // Field type metadata (e.g., "bool", "code", "text")
		options?: Record<string, Record<string, string>>; // Option field values (enum lookups)
		lookups?: Record<string, LookupData>; // Table relation lookup values
		initialEditMode?: boolean;
		saveBlocked?: boolean;
		saveBlockedMessage?: string;
		onclose?: () => void;
		onaction?: (actionName: string) => void;
		onsave?: (record: Record<string, any>, opts?: { leavingKey?: boolean }) => Promise<boolean> | boolean | void;
		onclearerror?: () => void;
		// The list's filters, search and sort: record navigation follows the list
		navigationQuery?: NavigationQuery;
		// Moved to another record (Previous/Next): it is the saved state to compare edits with
		onnavigate?: (record: Record<string, any>) => void;
		// The list's FlowFilters (ISO): the card's FlowField drilldowns apply them
		flowFilters?: import('$lib/types/api').TableFilter[];
	}

	let {
		open = false,
		page,
		record = $bindable({}),
		captions = {},
		fieldTypes = {},
		options = {},
		lookups = {},
		initialEditMode,
		saveBlocked = false,
		saveBlockedMessage = '',
		onclose,
		onaction,
		onsave,
		onclearerror,
		navigationQuery,
		onnavigate,
		flowFilters = []
	}: Props = $props();

	// Get primary key field name from page definition
	const primaryKeyField = $derived(getPrimaryKeyField(page));

	// Modal size state: normal, expanded, fullscreen
	let modalSize = $state<'normal' | 'expanded' | 'fullscreen'>('expanded');

	// Navigation state
	let recordIds: string[] = $state([]);
	let currentRecordIndex = $state(-1);
	let recordIdsLoaded = $state(false);

	// Computed navigation button states using helper functions
	const canGoPrevious = $derived(canNavigatePrevious({ recordIds, currentRecordIndex }, recordIdsLoaded));
	const canGoNext = $derived(canNavigateNext({ recordIds, currentRecordIndex }, recordIdsLoaded));

	// Load record IDs for navigation when modal opens
	$effect(() => {
		if (open && page.page.enable_navigation && !recordIdsLoaded) {
			loadRecordIds();
		} else if (!open) {
			// Reset when modal closes
			recordIdsLoaded = false;
			recordIds = [];
			currentRecordIndex = -1;
		}
	});

	// Update current record index when record changes. lastRecordId is the key the form showed
	// before: a key changed directly from the record's own (rename, ) keeps its place in
	// the navigation; a key typed into a new, blank record does not.
	let lastRecordId = '';
	$effect(() => {
		if (recordIdsLoaded && record) {
			const currentRecordId = getRecordId(record, primaryKeyField) ?? '';
			const ids = untrack(() => recordIds);
			const previous = untrack(() => currentRecordIndex);
			const index = currentRecordId ? ids.indexOf(currentRecordId) : -1;
			if (currentRecordId && index === -1 && previous >= 0 && ids[previous] === lastRecordId) {
				recordIds = ids.map((id, i) => (i === previous ? currentRecordId : id));
			} else if (currentRecordId) {
				currentRecordIndex = index;
			}
			lastRecordId = currentRecordId;
		}
	});

	// Keyboard shortcuts for navigation
	$effect(() => {
		if (!open || !page.page.enable_navigation) return;

		function handleKeyDown(e: KeyboardEvent) {
			// Ctrl+ArrowUp or Ctrl+Up - Previous
			if (e.ctrlKey && (e.key === 'ArrowUp' || e.key === 'Up')) {
				e.preventDefault();
				if (canGoPrevious) navigationActions.navigatePrevious();
			}
			// Ctrl+ArrowDown or Ctrl+Down - Next
			else if (e.ctrlKey && (e.key === 'ArrowDown' || e.key === 'Down')) {
				e.preventDefault();
				if (canGoNext) navigationActions.navigateNext();
			}
			// Ctrl+Home - First
			else if (e.ctrlKey && e.key === 'Home') {
				e.preventDefault();
				if (recordIdsLoaded && recordIds.length > 0) navigationActions.navigateFirst();
			}
			// Ctrl+End - Last
			else if (e.ctrlKey && e.key === 'End') {
				e.preventDefault();
				if (recordIdsLoaded && recordIds.length > 0) navigationActions.navigateLast();
			}
		}

		window.addEventListener('keydown', handleKeyDown);
		return () => window.removeEventListener('keydown', handleKeyDown);
	});

	async function loadRecordIds() {
		try {
			// Use lightweight IDs-only endpoint
			recordIds = await api.getRecordIDs(page.page.source_table, navigationQuery);

			// Find current record index
			const currentRecordId = getRecordId(record, primaryKeyField);
			currentRecordIndex = currentRecordId ? recordIds.indexOf(currentRecordId) : -1;

			recordIdsLoaded = true;
		} catch (err) {
			console.error('Error loading record IDs for navigation:', err);
			recordIdsLoaded = true; // Set to true even on error to prevent retries
		}
	}

	// Handle pop-out to new window
	function handlePopOut() {
		const recordId = getRecordId(record, primaryKeyField);
		const url = withNavigationQuery(`/pages/${page.page.id}${recordId ? `/${recordId}` : ''}`, navigationQuery);
		window.open(url, '_blank', 'width=1200,height=800');
		onclose?.();
	}

	// Toggle fullscreen
	function toggleFullscreen() {
		modalSize = modalSize === 'fullscreen' ? 'expanded' : 'fullscreen';
	}

	// Navigation functions - load record without changing URL
	async function navigateToRecord(recordId: string) {
		try {
			const newRecord = await api.getRecord(page.page.source_table, recordId);
			// Before the form shows it: an auto-save must compare with this record, not the previous
			onnavigate?.(newRecord);
			record = newRecord;
			currentRecordIndex = recordIds.indexOf(recordId);
		} catch (err) {
			console.error('Error loading record:', err);
		}
	}

	// Create navigation actions using shared helper
	const navigationActions = createNavigationActions(
		() => ({ recordIds, currentRecordIndex }),
		navigateToRecord
	);
</script>

<Modal {open} onclose={onclose} size={modalSize} ontogglefullscreen={toggleFullscreen}>
	<!-- Edge Navigation Buttons (classic ERP style) -->
	{#if page.page.enable_navigation && recordIdsLoaded}
		<NavigationButtons
			onPrevious={navigationActions.navigatePrevious}
			onNext={navigationActions.navigateNext}
			canNavigatePrevious={canGoPrevious}
			canNavigateNext={canGoNext}
		/>
	{/if}

	<div class="modal-header">
		<h2 class="modal-title">{cardTitle(page.page.caption, page.page.data_caption_fields, record, options)}</h2>

		<div class="modal-controls">
			<!-- Fullscreen toggle -->
			<button
				onclick={toggleFullscreen}
				class="control-btn"
				tabindex="-1"
				title={`${modalSize === 'fullscreen' ? t(MODAL.EXIT_FULLSCREEN) : t(MODAL.FULLSCREEN)} (Ctrl+Shift+F12)`}
				aria-label={modalSize === 'fullscreen' ? t(MODAL.EXIT_FULLSCREEN) : t(MODAL.FULLSCREEN)}
			>
				{#if modalSize === 'fullscreen'}
					<!-- Exit fullscreen icon -->
					<svg
						xmlns="http://www.w3.org/2000/svg"
						class="h-5 w-5"
						fill="none"
						viewBox="0 0 24 24"
						stroke="currentColor"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				{:else}
					<!-- Fullscreen icon -->
					<svg
						xmlns="http://www.w3.org/2000/svg"
						class="h-5 w-5"
						fill="none"
						viewBox="0 0 24 24"
						stroke="currentColor"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
						/>
					</svg>
				{/if}
			</button>

			<!-- Pop-out to new window -->
			<button
				onclick={handlePopOut}
				class="control-btn"
				tabindex="-1"
				title={t(MODAL.OPEN_NEW_WINDOW)}
				aria-label={t(MODAL.OPEN_NEW_WINDOW)}
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					class="h-5 w-5"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
					/>
				</svg>
			</button>

			<!-- Close button -->
			<button onclick={onclose} class="control-btn" tabindex="-1" title={t(MODAL.CLOSE)} aria-label={t(MODAL.CLOSE)}>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					class="h-5 w-5"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M6 18L18 6M6 6l12 12"
					/>
				</svg>
			</button>
		</div>
	</div>

	<div class="modal-body">

		<CardPage
			{page}
			bind:record
			{captions}
			{fieldTypes}
			{options}
			{lookups}
			{initialEditMode}
			{saveBlocked}
			{saveBlockedMessage}
			{onaction}
			{onsave}
			{onclearerror}
			onbacktolist={onclose}
			navigationEnabled={false}
			{flowFilters}
		/>
	</div>
</Modal>

<style>
	.modal-header {
		@apply flex items-center justify-between px-6 py-4;
		@apply border-b border-gray-200;
		@apply bg-surface;
		@apply shrink-0;
	}

	:global(.dark) .modal-header {
		border-color: rgb(var(--gray-700)); /* gray-700 */
		background-color: rgb(var(--gray-800)); /* gray-800 */
	}

	.modal-title {
		@apply text-xl font-bold text-heading;
	}

	:global(.dark) .modal-title {
		color: rgb(var(--accent-400)); /* blue-400 */
	}

	.modal-controls {
		@apply flex items-center gap-2;
	}

	.control-btn {
		@apply p-2 rounded;
		@apply text-gray-600;
		@apply transition-colors;
	}

	.control-btn:hover {
		@apply bg-gray-100;
		@apply text-gray-900;
	}

	:global(.dark) .control-btn {
		color: rgb(var(--gray-400)); /* gray-400 */
	}

	:global(.dark) .control-btn:hover {
		background-color: rgb(var(--gray-700)); /* gray-700 */
		color: rgb(var(--gray-100)); /* gray-100 */
	}

	.modal-body {
		@apply flex-1 flex flex-col p-6;
		@apply bg-gray-50;
		@apply overflow-hidden; /* Prevent modal-body from scrolling */
		@apply relative;
	}

	:global(.dark) .modal-body {
		background-color: rgb(var(--gray-900)); /* gray-900 */
	}






	/* Hover effect */
	:global(.edge-nav-btn:not(:disabled):hover) {
		@apply shadow-xl;
	}
</style>
