<script lang="ts">
	// The filters of one data item on a report's request page: as the list page's filter pane —
	// any field of the table can be added ("Filter..."), each filter is a field and an expression
	// in the same filter syntax (the server applies them as the list does)
	import { tick } from 'svelte';
	import { t, FILTER as FLT } from '#lib/services/i18n.svelte.js';
	import type { RequestDataItem, RequestFilter } from '#lib/types/pages.js';

	interface Props {
		item: RequestDataItem;
		filters: RequestFilter[];
	}

	let { item, filters = $bindable([]) }: Props = $props();

	let element: HTMLDivElement | undefined = $state();
	let fieldPickerOpen = $state(false);
	let selectedField = $state('');
	// ↑/↓ on a closed select fires change in Chrome/Firefox: browsing only moves the highlighted
	// field, Enter (or a pick with the mouse) adds it
	let browsingFields = false;

	const availableFields = $derived(item.fields.filter((f) => !filters.some((x) => x.field === f.name)));

	function caption(field: string): string {
		return item.fields.find((f) => f.name === field)?.caption ?? field;
	}

	async function openPicker() {
		fieldPickerOpen = true;
		await tick();
		element?.querySelector<HTMLSelectElement>('.field-selector-inline')?.focus();
	}

	async function addField() {
		if (!selectedField) return;
		filters = [...filters, { field: selectedField, expression: '' }];
		selectedField = '';
		fieldPickerOpen = false;
		// The cursor goes to the new filter's value box
		await tick();
		element?.querySelector<HTMLInputElement>(`[data-filter-index="${filters.length - 1}"]`)?.focus();
	}

	function handlePickerKeydown(e: KeyboardEvent) {
		if (['ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown'].includes(e.key) && !e.altKey) {
			browsingFields = true;
		} else if (e.key === 'Enter') {
			// Enter adds the field (not OK of the request page)
			e.preventDefault();
			e.stopPropagation();
			browsingFields = false;
			addField();
		} else if (e.key === 'Escape') {
			e.stopPropagation();
			fieldPickerOpen = false;
		}
	}

	function handlePickerChange() {
		if (browsingFields) {
			browsingFields = false;
			return;
		}
		addField();
	}

	function removeFilter(index: number) {
		filters = filters.filter((_, i) => i !== index);
	}
</script>

<div class="request-filters" style="margin-left: {item.indent * 1.25}rem" bind:this={element}>
	<h4 class="item-title">{item.caption}</h4>
	{#each filters as filter, index (filter.field)}
		<div class="filter-row">
			<div class="filter-field-tag">
				<span class="filter-field-name">{caption(filter.field)}</span>
				<button type="button" class="remove-filter-icon" onclick={() => removeFilter(index)} title={t(FLT.DELETE)}>
					<svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>
			<input
				type="text"
				class="filter-expression-input"
				data-filter-index={index}
				bind:value={filter.expression}
				placeholder={t(FLT.ENTER_EXPRESSION)}
			/>
		</div>
	{/each}

	{#if availableFields.length > 0}
		<div class="add-filter-row">
			{#if fieldPickerOpen}
				<select
					class="field-selector-inline"
					bind:value={selectedField}
					onkeydown={handlePickerKeydown}
					onchange={handlePickerChange}
					onblur={() => (fieldPickerOpen = false)}
				>
					<option value="">{t(FLT.SELECT_FIELD)}</option>
					{#each availableFields as field (field.name)}
						<option value={field.name}>{field.caption}</option>
					{/each}
				</select>
			{:else}
				<button type="button" class="add-filter-btn" onclick={openPicker}>
					<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
					</svg>
					<span>{t(FLT.ADD)}</span>
				</button>
			{/if}
		</div>
	{/if}
</div>

<style>
	.request-filters {
		@apply space-y-2;
	}

	.item-title {
		@apply text-sm font-semibold text-gray-800 pt-2;
	}

	:global(.dark) .item-title {
		@apply text-gray-200;
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

	.remove-filter-icon {
		@apply p-0.5 rounded hover:bg-gray-200;
		@apply text-gray-400;
	}

	:global(.dark) .remove-filter-icon {
		@apply hover:bg-gray-600;
		@apply text-gray-500;
	}

	.filter-expression-input {
		@apply w-full px-3 py-2 text-sm;
		@apply border border-gray-300 rounded;
		@apply bg-surface;
		@apply text-gray-900;
		@apply focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent;
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
		@apply flex items-center gap-2 px-3 py-1.5 text-left;
		@apply text-sm text-blue-600;
		@apply hover:bg-blue-50;
		@apply rounded transition-colors;
	}

	:global(.dark) .add-filter-btn {
		@apply text-blue-400;
		@apply hover:bg-blue-900/20;
	}

	.field-selector-inline {
		@apply w-full px-3 py-2 text-sm;
		@apply border border-blue-500 rounded;
		@apply bg-surface;
		@apply text-gray-900;
		@apply focus:outline-none focus:ring-2 focus:ring-blue-500;
	}

	:global(.dark) .field-selector-inline {
		@apply bg-gray-800;
		@apply text-gray-100;
	}
</style>
