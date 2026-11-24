<script lang="ts">
	import Modal from '#lib/components/Modal.svelte';
	import Button from '#lib/components/Button.svelte';
	import type { PageDefinition, Field } from '#lib/types/pages.js';
	import { createDragAndDrop, type DragItem } from '#lib/utils/dragAndDrop.svelte.js';
	import { t, BTN, CUSTOMIZE } from '#lib/services/i18n.svelte.js';
	import { getFieldCaption } from '#lib/utils/fieldHelpers.js';

	export interface ItemCustomization {
		visible: boolean;
		section?: string;
		order?: number;
	}

	interface Props {
		open?: boolean;
		page: PageDefinition;
		customizations: Record<string, ItemCustomization>;
		mode: 'card' | 'list';
		captions?: Record<string, string>; // the fields' captions as the page shows them (translated)
		onclose?: () => void;
		onsave?: (customizations: Record<string, ItemCustomization>) => void;
	}

	let { open = false, page, customizations, mode, captions = {}, onclose, onsave }: Props = $props();

	// Local copy of customizations for editing
	// svelte-ignore state_referenced_locally - synced via $effect below
	let localCustomizations = $state<Record<string, ItemCustomization>>({ ...customizations });

	// Sync when customizations prop changes
	$effect(() => {
		localCustomizations = { ...customizations };
	});

	// Drag and drop utility
	const drag = createDragAndDrop<ItemCustomization>();

	// Get all items (fields for card, columns for list) with applied order
	const allItems = $derived(() => {
		const items: Array<DragItem<{ section?: string; index: number }>> = [];
		let globalIndex = 0;

		if (mode === 'card') {
			// Card mode: iterate through sections
			page.page.layout.sections?.forEach((section) => {
				section.fields.forEach((field) => {
					items.push({
						field,
						data: { section: section.caption, index: globalIndex++ }
					});
				});
			});
		} else {
			// List mode: iterate through repeater fields
			(page.page.layout.repeater?.fields || []).forEach((field, index) => {
				items.push({
					field,
					data: { index }
				});
			});
		}

		// Sort by custom order if available
		return items.sort((a, b) => {
			const orderA = localCustomizations[a.field.source]?.order ?? a.data.index;
			const orderB = localCustomizations[b.field.source]?.order ?? b.data.index;
			return orderA - orderB;
		});
	});

	// Get all sections (card mode only)
	const allSections = $derived(() => {
		if (mode !== 'card') return [];
		// Fields can move between field sections, not into a subpage
		return page.page.layout.sections?.filter((s) => !s.subpage).map((s) => s.name) || [];
	});

	// Toggle item visibility
	function toggleItem(fieldSource: string, currentVisible: boolean) {
		localCustomizations = {
			...localCustomizations,
			[fieldSource]: {
				...(localCustomizations[fieldSource] || {}),
				visible: !currentVisible
			}
		};
	}

	// Change field section (card mode only)
	function changeFieldSection(fieldSource: string, newSection: string) {
		localCustomizations = {
			...localCustomizations,
			[fieldSource]: {
				...(localCustomizations[fieldSource] || { visible: true }),
				section: newSection
			}
		};
	}

	// Get effective visibility for an item
	function getEffectiveVisibility(field: Field): boolean {
		if (field.source in localCustomizations) {
			return localCustomizations[field.source].visible;
		}
		return field.visible !== false;
	}

	// Get effective section for a field (card mode only)
	function getEffectiveSection(field: Field, originalSection?: string): string {
		if (field.source in localCustomizations && localCustomizations[field.source].section) {
			return localCustomizations[field.source].section!;
		}
		return originalSection || '';
	}

	// Save customizations
	function handleSave() {
		onsave?.(localCustomizations);
		onclose?.();
	}

	// Reset to defaults
	function handleReset() {
		localCustomizations = {};
	}

	// Drag and drop handler wrapper
	function handleDrop(e: DragEvent, targetIndex: number) {
		localCustomizations = drag.handleDrop(e, targetIndex, allItems(), localCustomizations);
	}

	// UI labels based on mode
	const title = $derived(t(mode === 'card' ? CUSTOMIZE.PAGE_TITLE : CUSTOMIZE.COLUMNS_TITLE, page.page.caption));
	const subtitle = $derived(t(mode === 'card' ? CUSTOMIZE.PAGE_HINT : CUSTOMIZE.COLUMNS_HINT));
</script>

<Modal {open} {onclose} size="expanded">
	<div class="customize-modal">
		<div class="customize-header">
			<h2 class="customize-title">{title}</h2>
			<p class="customize-subtitle">{subtitle}</p>
		</div>

		<div class="customize-body">
			<div class="fields-list" role="list">
				{#each allItems() as item, idx}
					{@const isVisible = getEffectiveVisibility(item.field)}
					{@const currentSection = mode === 'card' ? getEffectiveSection(item.field, item.data.section) : ''}
					{@const isDragging = drag.isDragging(item.field.source)}
					{@const isDragOver = drag.isDragOver(idx)}
					<div
						class="field-item"
						class:dragging={isDragging}
						class:drag-over={isDragOver}
						draggable="true"
						role="listitem"
						ondragstart={(e) => drag.handleDragStart(e, item)}
						ondragover={(e) => drag.handleDragOver(e, idx)}
						ondragleave={drag.handleDragLeave}
						ondrop={(e) => handleDrop(e, idx)}
						ondragend={drag.handleDragEnd}
					>
						<!-- Drag handle -->
						<div class="drag-handle" title={t(CUSTOMIZE.DRAG)}>
							<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
								<path
									d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z"
								/>
							</svg>
						</div>

						<label class="field-checkbox" class:pointer-events-none={isDragging}>
							<input
								type="checkbox"
								checked={isVisible}
								onchange={(e) => {
									e.stopPropagation();
									toggleItem(item.field.source, isVisible);
								}}
								onclick={(e) => e.stopPropagation()}
							/>
							<div class="field-info">
								<span class="field-name">{getFieldCaption(item.field.source, captions, item.field.caption)}</span>
								{#if mode === 'card'}
									<!-- Card mode: show section dropdown -->
									<div class="field-meta-row">
										<span class="field-section-label">{t(CUSTOMIZE.SECTION)}</span>
										<select
											class="section-select"
											value={currentSection}
											onchange={(e) => changeFieldSection(item.field.source, e.currentTarget.value)}
											onclick={(e) => e.stopPropagation()}
										>
											{#each allSections() as sectionName}
												<option value={sectionName}>{sectionName}</option>
											{/each}
										</select>
									</div>
								{:else if item.field.width}
									<!-- List mode: show width info -->
									<span class="field-meta">{t(CUSTOMIZE.WIDTH, String(item.field.width))}</span>
								{/if}
							</div>
							{#if item.field.visible === false && !localCustomizations[item.field.source]}
								<span class="field-badge">{t(CUSTOMIZE.HIDDEN_BY_DEFAULT)}</span>
							{/if}
						</label>
					</div>
				{/each}
			</div>
		</div>

		<div class="customize-footer">
			<Button variant="secondary" size="sm" onclick={handleReset}>{t(CUSTOMIZE.RESET)}</Button>
			<div class="footer-actions">
				<Button variant="secondary" size="sm" onclick={onclose}>{t(BTN.CANCEL)}</Button>
				<Button variant="primary" size="sm" onclick={handleSave}>{t(CUSTOMIZE.SAVE)}</Button>
			</div>
		</div>
	</div>
</Modal>

<style>
	@reference '../../../app.css';
	.customize-modal {
		@apply flex flex-col h-full max-h-[80vh];
	}

	.customize-header {
		@apply px-6 py-4 border-b border-gray-200;
		@apply bg-surface;
	}

	:global(.dark) .customize-header {
		@apply border-gray-700;
		@apply bg-gray-800;
	}

	.customize-title {
		@apply text-xl font-bold text-heading;
	}

	:global(.dark) .customize-title {
		@apply text-blue-400;
	}

	.customize-subtitle {
		@apply mt-1 text-sm text-gray-600;
	}

	:global(.dark) .customize-subtitle {
		@apply text-gray-400;
	}

	.customize-body {
		@apply flex-1 overflow-y-auto p-6;
		@apply bg-gray-50;
	}

	:global(.dark) .customize-body {
		@apply bg-gray-900;
	}

	.fields-list {
		@apply flex flex-col gap-2;
	}

	.field-item {
		@apply bg-surface rounded-lg border border-gray-200;
		@apply hover:border-blue-300 transition-colors;
		@apply flex items-center gap-2;
		cursor: grab;
	}

	:global(.dark) .field-item {
		@apply bg-gray-800 border-gray-700;
		@apply hover:border-blue-700;
	}

	.field-item.dragging {
		@apply opacity-50;
		cursor: grabbing;
	}

	.field-item.drag-over {
		@apply border-blue-500 border-2;
		@apply bg-blue-50;
	}

	:global(.dark) .field-item.drag-over {
		@apply border-blue-400;
		@apply bg-blue-900/20;
	}

	.drag-handle {
		@apply p-2 text-gray-400 hover:text-gray-600;
		@apply cursor-grab;
		touch-action: none;
	}

	:global(.dark) .drag-handle {
		@apply hover:text-gray-300;
	}

	.field-item.dragging .drag-handle {
		@apply cursor-grabbing;
	}

	.field-checkbox {
		@apply flex items-center gap-3 p-4 cursor-pointer flex-1;
	}

	.field-checkbox.pointer-events-none {
		pointer-events: none;
	}

	.field-checkbox input[type='checkbox'] {
		@apply w-5 h-5 rounded-sm border-gray-300 text-blue-600;
		@apply focus:ring-2 focus:ring-blue-500 focus:ring-offset-0;
		@apply cursor-pointer;
	}

	.field-info {
		@apply flex flex-col flex-1 gap-2;
	}

	.field-name {
		@apply font-medium text-gray-900;
	}

	:global(.dark) .field-name {
		@apply text-gray-100;
	}

	.field-meta {
		@apply text-sm text-gray-500;
	}

	:global(.dark) .field-meta {
		@apply text-gray-400;
	}

	.field-meta-row {
		@apply flex items-center gap-2;
	}

	.field-section-label {
		@apply text-sm text-gray-500;
	}

	:global(.dark) .field-section-label {
		@apply text-gray-400;
	}

	.section-select {
		@apply text-sm px-2 py-1 rounded-sm border border-gray-300;
		@apply bg-surface text-gray-900;
		@apply focus:ring-2 focus:ring-blue-500 focus:border-blue-500;
		@apply cursor-pointer;
	}

	:global(.dark) .section-select {
		@apply border-gray-600;
		@apply bg-gray-700 text-gray-100;
	}

	.field-badge {
		@apply px-2 py-1 text-xs rounded-full;
		@apply bg-orange-100 text-orange-700;
	}

	:global(.dark) .field-badge {
		@apply bg-orange-900 text-orange-300;
	}

	.customize-footer {
		@apply flex items-center justify-between px-6 py-4;
		@apply border-t border-gray-200;
		@apply bg-surface;
	}

	:global(.dark) .customize-footer {
		@apply border-gray-700;
		@apply bg-gray-800;
	}

	.footer-actions {
		@apply flex gap-3;
	}
</style>
