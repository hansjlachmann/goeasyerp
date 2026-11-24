<script lang="ts">
	import type { PageDefinition, Field } from '#lib/types/pages.js';
	import type { TableFilter } from '#lib/types/api.js';
	import Button from '#lib/components/Button.svelte';
	import Card from '#lib/components/Card.svelte';
	import ConfirmModal from '../ConfirmModal.svelte';
	import { currentUser } from '#lib/stores/user.js';
	import { getFieldCaption as getFieldCaptionUtil } from '#lib/utils/fieldHelpers.js';
	import { getJson, setJson } from '#lib/utils/storage.js';
	import { t, BTN, FILTER as FLT } from '#lib/services/i18n.svelte.js';
	import { toast } from '#lib/stores/toast.js';
	import { currentLanguage } from '#lib/stores/session.js';
	import { get } from 'svelte/store';
	import { onMount, tick } from 'svelte';
	import { toApiFlowFilter, dateFilterHint } from '#lib/utils/flowFilter.js';
	import { datePattern } from '#lib/utils/regional.js';

	interface Props {
		page: PageDefinition;
		captions?: Record<string, string>;
		currentFilters?: TableFilter[];
		// FlowFilters (e.g. Date Filter) as typed — "Filter totals by"
		currentFlowFilters?: TableFilter[];
		onApply: (filters: TableFilter[], flowFilters: TableFilter[]) => void;
		onClose: () => void;
	}

	let { page, captions = {}, currentFilters = [], currentFlowFilters = [], onApply, onClose }: Props = $props();

	// FlowFilter fields of the source table and their typed expressions
	const flowFilterFields = $derived(page.page.flow_filter_fields ?? []);
	let flowValues = $state<Record<string, string>>({});
	$effect(() => {
		flowValues = Object.fromEntries(currentFlowFilters.map((f) => [f.field, f.expression]));
	});

	// The typed FlowFilters, or null after telling the user which one cannot be read
	function readFlowFilters(): TableFilter[] | null {
		const locale = get(currentLanguage);
		const out: TableFilter[] = [];
		for (const field of flowFilterFields) {
			const expression = (flowValues[field.name] ?? '').trim();
			if (!expression) continue;
			const result = toApiFlowFilter(expression, field.kind, locale);
			if (result.error !== undefined) {
				toast.error(t(FLT.INVALID, field.caption, result.error));
				return null;
			}
			out.push({ field: field.name, expression });
		}
		return out;
	}

	// Saved filter presets (views)
	interface FilterPreset {
		name: string;
		filters: Array<{ field: string; expression: string }>;
	}

	let savedPresets = $state<Record<string, FilterPreset>>({});
	let activePresetName = $state<string | null>(null);
	let showPresetMenu = $state<string | null>(null);
	let editingPresetName = $state<string | null>(null);
	let newPresetNameInput = $state('');

	// Confirm modal state
	let confirmModalOpen = $state(false);
	let presetToDelete = $state<string | null>(null);

	// Active filters - array of { field, expression }
	let activeFilters = $state<Array<{ field: string; expression: string }>>([]);

	// Initialize active filters from current filters
	$effect(() => {
		activeFilters = currentFilters.map((f) => ({ field: f.field, expression: f.expression }));
	});

	// Load saved presets on mount
	$effect(() => {
		const userId = $currentUser?.user_id || 'anonymous';
		const key = `filter-preset-${userId}-${page.page.id}`;
		savedPresets = getJson(key, {});
	});

	// Get all fields from page definition
	const allFields = $derived(() => {
		if (page.page.type === 'List' && page.page.layout.repeater?.fields) {
			return page.page.layout.repeater.fields;
		}
		return [];
	});

	// Get fields that haven't been added yet
	const availableFields = $derived(() => {
		const activeFieldSet = new Set(activeFilters.map((f) => f.field));
		return allFields().filter((field) => !activeFieldSet.has(field.source));
	});

	// Field picker for adding a filter: open when the pane opens (Ctrl+F7 or the Filter
	// button), so the keyboard flow is Alt+↓, arrows, Enter → the new filter's value box
	let fieldPickerOpen = $state(false);
	let selectedFieldToAdd = $state('');
	// ↑/↓ on a closed select fires change in Chrome/Firefox: browsing only moves the
	// highlighted field, Enter (or a pick in the open dropdown / with the mouse) adds it
	let browsingFields = false;
	let paneElement: HTMLDivElement | undefined = $state();

	onMount(async () => {
		if (availableFields().length === 0) return;
		fieldPickerOpen = true;
		await tick();
		paneElement?.querySelector<HTMLSelectElement>('.field-selector-inline')?.focus();
	});

	// Get field caption by source
	function getFieldCaptionBySource(fieldSource: string): string {
		const field = allFields().find((f) => f.source === fieldSource);
		if (field) {
			return getFieldCaptionUtil(field.source, captions, field.caption);
		}
		return fieldSource;
	}

	// Get field caption
	function getFieldCaption(field: Field): string {
		return getFieldCaptionUtil(field.source, captions, field.caption);
	}

	// Add a new filter field
	async function handleAddField() {
		if (!selectedFieldToAdd) return;

		activeFilters = [...activeFilters, { field: selectedFieldToAdd, expression: '' }];
		selectedFieldToAdd = '';
		fieldPickerOpen = false;
		// The cursor goes to the new filter's value box
		await tick();
		paneElement
			?.querySelector<HTMLInputElement>(`[data-filter-index="${activeFilters.length - 1}"]`)
			?.focus();
	}

	function handleFieldPickerKeyDown(e: KeyboardEvent) {
		if (['ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown'].includes(e.key) && !e.altKey) {
			browsingFields = true;
		} else if (e.key === 'Enter') {
			e.preventDefault();
			browsingFields = false;
			handleAddField();
		}
	}

	function handleFieldPickerChange() {
		if (browsingFields) {
			browsingFields = false;
			return;
		}
		handleAddField();
	}

	// Remove a filter field
	function handleRemoveFilter(index: number) {
		activeFilters = activeFilters.filter((_, i) => i !== index);
		activePresetName = null; // Clear active preset when manually changing filters
		applyFilters();
	}

	// Handle filter expression change
	function handleFilterChange(index: number, value: string) {
		activeFilters[index].expression = value;
		activePresetName = null; // Clear active preset when manually changing filters
	}

	// Apply filters (list filters and FlowFilters together)
	function applyFilters() {
		const flowFilters = readFlowFilters();
		if (flowFilters === null) return;
		const filters: TableFilter[] = activeFilters
			.filter((f) => f.expression && f.expression.trim() !== '')
			.map((f) => ({
				field: f.field,
				expression: f.expression.trim()
			}));

		onApply(filters, flowFilters);
	}

	// Clear all filters
	function handleClearAll() {
		activeFilters = [];
		flowValues = {};
		activePresetName = null;
		onApply([], []);
	}

	// Apply a saved preset
	function handleApplyPreset(presetName: string) {
		const preset = savedPresets[presetName];
		if (preset) {
			activeFilters = [...preset.filters];
			activePresetName = presetName;
			showPresetMenu = null;
			applyFilters();
		}
	}

	// Save current filters as new preset
	function handleSaveAsNewPreset() {
		const presetName = prompt('Enter a name for this view:');
		if (!presetName || presetName.trim() === '') return;

		const userId = $currentUser?.user_id || 'anonymous';
		const key = `filter-preset-${userId}-${page.page.id}`;

		savedPresets[presetName] = {
			name: presetName,
			filters: [...activeFilters]
		};

		setJson(key, savedPresets);
		activePresetName = presetName;
	}

	// Start renaming a preset
	function handleStartRename(presetName: string) {
		editingPresetName = presetName;
		newPresetNameInput = presetName;
		showPresetMenu = null;
	}

	// Save renamed preset
	function handleSaveRename(oldName: string) {
		if (!newPresetNameInput || newPresetNameInput.trim() === '') {
			editingPresetName = null;
			return;
		}

		const newName = newPresetNameInput.trim();
		if (newName === oldName) {
			editingPresetName = null;
			return;
		}

		const userId = $currentUser?.user_id || 'anonymous';
		const key = `filter-preset-${userId}-${page.page.id}`;

		// Copy preset with new name
		savedPresets[newName] = {
			name: newName,
			filters: savedPresets[oldName].filters
		};

		// Delete old preset
		delete savedPresets[oldName];

		// Update active preset name if this was the active one
		if (activePresetName === oldName) {
			activePresetName = newName;
		}

		setJson(key, savedPresets);
		editingPresetName = null;
	}

	// Cancel rename
	function handleCancelRename() {
		editingPresetName = null;
		newPresetNameInput = '';
	}

	// Delete a preset - show confirm modal
	function handleDeletePreset(presetName: string) {
		presetToDelete = presetName;
		confirmModalOpen = true;
		showPresetMenu = null;
	}

	// Confirm deletion
	function confirmDeletePreset() {
		if (!presetToDelete) return;

		const userId = $currentUser?.user_id || 'anonymous';
		const key = `filter-preset-${userId}-${page.page.id}`;

		delete savedPresets[presetToDelete];

		if (activePresetName === presetToDelete) {
			activePresetName = null;
		}

		setJson(key, savedPresets);
		confirmModalOpen = false;
		presetToDelete = null;
	}

	// Cancel deletion
	function cancelDeletePreset() {
		confirmModalOpen = false;
		presetToDelete = null;
	}

	// Toggle preset menu
	function handleTogglePresetMenu(presetName: string, event: MouseEvent) {
		event.stopPropagation();
		showPresetMenu = showPresetMenu === presetName ? null : presetName;
	}

	// Handle Enter key to apply filters
	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			applyFilters();
		}
	}

	// Handle Enter key for rename
	function handleRenameKeyDown(e: KeyboardEvent, oldName: string) {
		if (e.key === 'Enter') {
			e.preventDefault();
			handleSaveRename(oldName);
		} else if (e.key === 'Escape') {
			e.preventDefault();
			handleCancelRename();
		}
	}

	// Close preset menu when clicking outside
	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.preset-menu-container')) {
			showPresetMenu = null;
		}
	}

	$effect(() => {
		document.addEventListener('click', handleClickOutside);
		return () => {
			document.removeEventListener('click', handleClickOutside);
		};
	});
</script>

<!-- data-own-keys: keys typed in the pane are not list shortcuts (Enter in a filter field
     also opened the selected record) -->
<div class="filter-pane" data-own-keys bind:this={paneElement}>
	<Card>
		{#snippet header()}
			<div class="filter-header">
				<h3 class="filter-title">{t(FLT.TITLE, page.page.caption)}</h3>
				<button onclick={onClose} class="close-btn" title="Close filter pane">
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
		{/snippet}

		<!-- Views Section -->
		<div class="views-section">
			<h4 class="section-title">{t(FLT.VIEWS)}</h4>

			<!-- All (default view) -->
			<button
				class="view-item"
				class:active={activePresetName === null}
				onclick={() => handleClearAll()}
			>
				<span class="view-name">{t(FLT.ALL)}</span>
			</button>

			<!-- Saved presets -->
			{#each Object.keys(savedPresets) as presetName}
				{#if editingPresetName === presetName}
					<div class="view-item-editing">
						<!-- svelte-ignore a11y_autofocus -->
						<input
							type="text"
							class="rename-input"
							bind:value={newPresetNameInput}
							onkeydown={(e) => handleRenameKeyDown(e, presetName)}
							onblur={() => handleSaveRename(presetName)}
							autofocus
						/>
					</div>
				{:else}
					<div class="view-item-container preset-menu-container">
						<button
							class="view-item"
							class:active={activePresetName === presetName}
							onclick={() => handleApplyPreset(presetName)}
						>
							<span class="view-name">{presetName}</span>
						</button>
						<button
							class="preset-menu-btn"
							onclick={(e) => handleTogglePresetMenu(presetName, e)}
							title="View options"
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								class="h-4 w-4"
								fill="none"
								viewBox="0 0 24 24"
								stroke="currentColor"
							>
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z"
								/>
							</svg>
						</button>

						{#if showPresetMenu === presetName}
							<div class="preset-menu">
								<button
									class="preset-menu-item"
									onclick={() => handleStartRename(presetName)}
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										class="h-4 w-4"
										fill="none"
										viewBox="0 0 24 24"
										stroke="currentColor"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
										/>
									</svg>
									{t(FLT.RENAME)}
								</button>
								<button
									class="preset-menu-item delete"
									onclick={() => handleDeletePreset(presetName)}
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										class="h-4 w-4"
										fill="none"
										viewBox="0 0 24 24"
										stroke="currentColor"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
										/>
									</svg>
									{t(FLT.DELETE)}
								</button>
							</div>
						{/if}
					</div>
				{/if}
			{/each}

			<!-- Save current as new view -->
			{#if activeFilters.length > 0}
				<button class="view-item new-view" onclick={handleSaveAsNewPreset}>
					<svg
						xmlns="http://www.w3.org/2000/svg"
						class="h-4 w-4"
						fill="none"
						viewBox="0 0 24 24"
						stroke="currentColor"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M12 4v16m8-8H4"
						/>
					</svg>
					<span class="view-name">{t(FLT.SAVE_VIEW)}</span>
				</button>
			{/if}
		</div>

		<!-- Filter list by Section -->
		<div class="filter-section">
			<h4 class="section-title">{t(FLT.LIST_BY)}</h4>

			<!-- Active filters -->
			{#if activeFilters.length > 0}
				<div class="active-filters">
					{#each activeFilters as filter, index}
						<div class="filter-row">
							<div class="filter-field-tag">
								<span class="filter-field-name"
									>{getFieldCaptionBySource(filter.field)}</span
								>
								<button
									class="remove-filter-icon"
									onclick={() => handleRemoveFilter(index)}
									title="Remove filter"
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										class="h-3 w-3"
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
							<input
								type="text"
								class="filter-expression-input"
								data-filter-index={index}
								value={filter.expression}
								oninput={(e) => handleFilterChange(index, e.currentTarget.value)}
								onkeydown={handleKeyDown}
								onblur={applyFilters}
								placeholder={t(FLT.ENTER_EXPRESSION)}
							/>
						</div>
					{/each}
				</div>
			{/if}

			<!-- Add filter button -->
			{#if availableFields().length > 0}
				<div class="add-filter-row">
					<button class="add-filter-btn" onclick={() => (fieldPickerOpen = true)}>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							class="h-4 w-4"
							fill="none"
							viewBox="0 0 24 24"
							stroke="currentColor"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M12 4v16m8-8H4"
							/>
						</svg>
						<span>{t(FLT.ADD)}</span>
					</button>

					{#if fieldPickerOpen}
						<!-- svelte-ignore a11y_autofocus -->
						<select
							class="field-selector-inline"
							bind:value={selectedFieldToAdd}
							onkeydown={handleFieldPickerKeyDown}
							onchange={handleFieldPickerChange}
							autofocus
						>
							<option value="">{t(FLT.SELECT_FIELD)}</option>
							{#each availableFields() as field}
								<option value={field.source}>{getFieldCaption(field)}</option>
							{/each}
						</select>
					{/if}
				</div>
			{/if}

			<!-- FlowFilters ("Filter totals by"): limit the FlowFields, e.g. to a Date Filter period -->
			{#if flowFilterFields.length > 0}
				<h4 class="section-title totals-title">{t(FLT.TOTALS_BY)}</h4>
				<div class="active-filters">
					{#each flowFilterFields as field (field.name)}
						<div class="filter-row">
							<div class="filter-field-tag">
								<span class="filter-field-name">{field.caption}</span>
							</div>
							<input
								type="text"
								class="filter-expression-input"
								value={flowValues[field.name] ?? ''}
								oninput={(e) => (flowValues[field.name] = e.currentTarget.value)}
								onkeydown={handleKeyDown}
								onblur={applyFilters}
								placeholder={field.kind === 'date' ? dateFilterHint(datePattern(get(currentLanguage))) : t(FLT.ENTER_EXPRESSION)}
							/>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Filter help -->
		<div class="filter-help">
			<p class="text-xs text-gray-500 dark:text-gray-400">
				<strong>Syntax:</strong> <code>*</code> wildcard, <code>|</code> OR, <code>..</code> range,
				<code>&lt;&gt;=</code> comparison
			</p>
		</div>
	</Card>
</div>

<!-- Confirm Delete View Modal -->
<ConfirmModal
	open={confirmModalOpen}
	title={t(FLT.DELETE_VIEW)}
	message={t(FLT.DELETE_VIEW_CONFIRM, presetToDelete || '')}
	confirmText={t(BTN.DELETE)}
	variant="danger"
	onconfirm={confirmDeletePreset}
	oncancel={cancelDeletePreset}
/>

<style>
	@reference '../../../app.css';
	.totals-title {
		margin-top: 1rem;
	}

	.filter-pane {
		@apply w-80 shrink-0;
	}

	.filter-header {
		@apply flex items-center justify-between;
	}

	.filter-title {
		@apply text-lg font-semibold text-heading;
	}

	:global(.dark) .filter-title {
		@apply text-blue-400;
	}

	.close-btn {
		@apply p-1 rounded-sm hover:bg-gray-100;
		@apply text-gray-600;
		@apply transition-colors;
	}

	:global(.dark) .close-btn {
		@apply hover:bg-gray-700;
		@apply text-gray-400;
	}

	/* Views Section */
	.views-section {
		@apply mb-6 pb-4 border-b border-gray-200;
	}

	:global(.dark) .views-section {
		@apply border-gray-700;
	}

	.section-title {
		@apply text-xs font-semibold text-gray-500 uppercase tracking-wide mb-2;
	}

	:global(.dark) .section-title {
		@apply text-gray-400;
	}

	.view-item-container {
		@apply relative flex items-center gap-1;
	}

	.view-item {
		@apply w-full flex items-center gap-2 px-3 py-2 text-left rounded-sm;
		@apply text-sm text-blue-600;
		@apply hover:bg-blue-50;
		@apply transition-colors;
	}

	:global(.dark) .view-item {
		@apply text-blue-400;
		@apply hover:bg-blue-900/20;
	}

	.view-item.active {
		@apply bg-blue-100 font-medium;
	}

	:global(.dark) .view-item.active {
		@apply bg-blue-900/30;
	}

	.view-item.new-view {
		@apply text-gray-600;
		@apply border border-dashed border-gray-300;
		@apply hover:border-blue-400 hover:text-blue-600;
	}

	:global(.dark) .view-item.new-view {
		@apply text-gray-400;
		@apply border-gray-600;
	}

	.view-name {
		@apply flex-1;
	}

	.view-item-editing {
		@apply px-3 py-2;
	}

	.rename-input {
		@apply w-full px-2 py-1 text-sm;
		@apply border border-blue-500 rounded-sm;
		@apply bg-surface;
		@apply text-gray-900;
		@apply focus:outline-hidden focus:ring-2 focus:ring-blue-500;
	}

	:global(.dark) .rename-input {
		@apply bg-gray-800;
		@apply text-gray-100;
	}

	.preset-menu-btn {
		@apply p-1 rounded-sm opacity-0;
		@apply hover:bg-gray-200;
		@apply text-gray-500;
		@apply transition-opacity;
	}

	:global(.dark) .preset-menu-btn {
		@apply hover:bg-gray-600;
		@apply text-gray-400;
	}

	.view-item-container:hover .preset-menu-btn {
		@apply opacity-100;
	}

	.preset-menu {
		@apply absolute right-0 top-full mt-1 z-50;
		@apply bg-surface rounded-md shadow-lg;
		@apply border border-gray-200;
		@apply py-1 min-w-[150px];
	}

	:global(.dark) .preset-menu {
		@apply bg-gray-800;
		@apply border-gray-700;
	}

	.preset-menu-item {
		@apply w-full flex items-center gap-2 px-3 py-2 text-left;
		@apply text-sm text-gray-700;
		@apply hover:bg-gray-100;
		@apply transition-colors;
	}

	:global(.dark) .preset-menu-item {
		@apply text-gray-300;
		@apply hover:bg-gray-700;
	}

	.preset-menu-item.delete {
		@apply text-red-600;
		@apply hover:bg-red-50;
	}

	:global(.dark) .preset-menu-item.delete {
		@apply text-red-400;
		@apply hover:bg-red-900/20;
	}

	/* Filter Section */
	.filter-section {
		@apply mb-4;
	}

	.active-filters {
		@apply space-y-2 mb-3;
	}

	.filter-row {
		@apply space-y-1;
	}

	.filter-field-tag {
		@apply flex items-center gap-2 text-sm;
		@apply text-gray-700;
	}

	:global(.dark) .filter-field-tag {
		@apply text-gray-300;
	}

	.filter-field-name {
		@apply flex items-center gap-1;
	}

	.filter-field-name::before {
		content: '×';
		@apply text-gray-400 text-lg;
	}

	:global(.dark) .filter-field-name::before {
		@apply text-gray-500;
	}

	.remove-filter-icon {
		@apply p-0.5 rounded-sm hover:bg-gray-200;
		@apply text-gray-400;
		@apply opacity-0 hover:opacity-100;
	}

	:global(.dark) .remove-filter-icon {
		@apply hover:bg-gray-600;
		@apply text-gray-500;
	}

	.filter-row:hover .remove-filter-icon {
		@apply opacity-100;
	}

	.filter-expression-input {
		@apply w-full px-3 py-2 text-sm;
		@apply border border-gray-300 rounded-sm;
		@apply bg-surface;
		@apply text-gray-900;
		@apply focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:border-transparent;
	}

	:global(.dark) .filter-expression-input {
		@apply border-gray-600;
		@apply bg-gray-800;
		@apply text-gray-100;
	}

	.add-filter-row {
		@apply flex flex-col gap-2;
	}

	.add-filter-btn {
		@apply flex items-center gap-2 px-3 py-2 text-left;
		@apply text-sm text-blue-600;
		@apply hover:bg-blue-50;
		@apply rounded-sm transition-colors;
	}

	:global(.dark) .add-filter-btn {
		@apply text-blue-400;
		@apply hover:bg-blue-900/20;
	}

	.field-selector-inline {
		@apply w-full px-3 py-2 text-sm;
		@apply border border-blue-500 rounded-sm;
		@apply bg-surface;
		@apply text-gray-900;
		@apply focus:outline-hidden focus:ring-2 focus:ring-blue-500;
	}

	:global(.dark) .field-selector-inline {
		@apply bg-gray-800;
		@apply text-gray-100;
	}

	/* Filter Help */
	.filter-help {
		@apply pt-3 border-t border-gray-200;
	}

	:global(.dark) .filter-help {
		@apply border-gray-700;
	}

	.filter-help code {
		@apply px-1 py-0.5 bg-gray-100 rounded-sm;
		@apply text-xs font-mono;
	}

	:global(.dark) .filter-help code {
		@apply bg-gray-700;
	}
</style>
