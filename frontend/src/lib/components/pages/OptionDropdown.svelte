<script lang="ts">
	import { dropdownPageSize, pageIndex } from '#lib/utils/dropdownPaging.js';
	import { cn } from '#lib/utils/cn.js';
	import { clickOutside } from '#lib/actions/clickOutside.js';
	import { floating } from '#lib/actions/floating.js';

	interface Props {
		// On a card: ↑/↓ on the closed field move to the field above/below (Alt+↓ / F4 open it)
		arrowsMoveFields?: boolean;
		options: Record<string, string>; // key (stored integer as string) → display label
		value: number | string | null; // Current stored value (integer)
		tabindex?: number;
		disabled?: boolean;
		compact?: boolean; // Compact mode for list page edit cells
		onselect?: (value: number) => void;
		onblur?: () => void;
	}

	let {
		arrowsMoveFields = false,
		options,
		value = null,
		tabindex = 0,
		disabled = false,
		compact = false,
		onselect,
		onblur
	}: Props = $props();

	let isOpen = $state(false);
	// the open list (moved to <body> by floating, so nothing clips it)
	let panelRef = $state<HTMLDivElement | null>(null);
	let containerRef = $state<HTMLDivElement | null>(null);
	let inputRef = $state<HTMLDivElement | null>(null);
	let bodyRef = $state<HTMLDivElement | null>(null);
	let selectedIndex = $state(-1);

	// Build entries array from options
	const entries = $derived(Object.entries(options));

	// Get display text for current value
	const displayText = $derived(() => {
		const key = String(value ?? 0);
		return options[key] || '';
	});

	// Scroll selected row into view
	$effect(() => {
		if (isOpen && selectedIndex >= 0 && bodyRef) {
			const rowElements = bodyRef.querySelectorAll('.option-row');
			if (rowElements[selectedIndex]) {
				rowElements[selectedIndex].scrollIntoView({ block: 'nearest' });
			}
		}
	});

	// Track whether handleSelect already fired (prevents re-open on focus)
	let selectHandled = false;

	function openDropdown() {
		if (disabled) return;
		if (selectHandled) return;
		isOpen = true;
		// Find current selection index
		const currentKey = String(value ?? 0);
		selectedIndex = entries.findIndex(([k]) => k === currentKey);
		if (selectedIndex < 0 && entries.length > 0) selectedIndex = 0;
	}

	function handleToggle() {
		if (disabled) return;
		selectHandled = false;
		if (isOpen) {
			isOpen = false;
		} else {
			openDropdown();
		}
	}

	function handleSelect(optKey: string) {
		const newValue = parseInt(optKey, 10);
		isOpen = false;
		selectHandled = true;
		onselect?.(newValue);
		// Re-focus the trigger so user can Tab to next field
		inputRef?.focus();
	}

	function handleBlur() {
		setTimeout(() => {
			if (containerRef?.contains(document.activeElement) || panelRef?.contains(document.activeElement)) {
				return;
			}
			isOpen = false;
			if (selectHandled) {
				selectHandled = false;
			}
			onblur?.();
		}, 150);
	}

	function handleKeydown(e: KeyboardEvent) {
		switch (e.key) {
			case 'Escape':
				if (isOpen) {
					e.preventDefault();
					isOpen = false;
				}
				// If closed, let Escape bubble up
				break;
			case 'ArrowDown':
				if (!isOpen && arrowsMoveFields && !e.altKey) break;
				e.preventDefault();
				if (!isOpen) {
					selectHandled = false;
					openDropdown();
				} else {
					selectedIndex = Math.min(selectedIndex + 1, entries.length - 1);
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
					selectedIndex = pageIndex(selectedIndex, entries.length, dropdownPageSize(bodyRef, '.option-row'), e.key === 'PageDown' ? 1 : -1);
				}
				break;
			case 'Enter':
				if (isOpen && selectedIndex >= 0 && selectedIndex < entries.length) {
					e.preventDefault();
					handleSelect(entries[selectedIndex][0]);
				}
				// When closed, let Enter bubble up for cell navigation (move to next row)
				break;
			case 'Tab':
				isOpen = false;
				break;
			case 'F4':
				e.preventDefault();
				handleToggle();
				break;
			case ' ':
				// Space opens dropdown when closed (like native select)
				if (!isOpen) {
					e.preventDefault();
					selectHandled = false;
					openDropdown();
				}
				break;
		}
	}

	function handleClickOutside() {
		isOpen = false;
	}
</script>

<div
	class={cn('option-dropdown', compact && 'option-dropdown-compact')}
	bind:this={containerRef}
	use:clickOutside={{ callback: handleClickOutside, enabled: isOpen, also: () => [panelRef] }}
>
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<!-- svelte-ignore a11y_role_has_required_aria_props -->
	<div
		class={cn('option-trigger', compact && 'option-trigger-compact')}
		bind:this={inputRef}
		{tabindex}
		role="combobox"
		aria-haspopup="listbox"
		aria-expanded={isOpen}
		aria-controls={isOpen ? 'option-listbox' : undefined}
		onkeydown={handleKeydown}
		onblur={handleBlur}
		onfocus={() => {
			// List cells (compact) open on focus; on cards the list opens with a click, F4,
			// Space or ↓ — moving through the fields does not open it
			if (compact && !selectHandled) openDropdown();
		}}
		onclick={handleToggle}
	>
		<span class="option-value">{displayText()}</span>
		<span class={cn('option-arrow', compact && 'option-arrow-compact')}>{isOpen ? '▲' : '▼'}</span>
	</div>

	{#if isOpen}
		<!-- the list keeps the focus in the field (mousedown), so it is never focused itself -->
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<div class="option-panel" role="listbox" id="option-listbox" bind:this={panelRef} use:floating={inputRef} onmousedown={(e) => e.preventDefault()}>
			<div class="option-body" bind:this={bodyRef}>
				{#each entries as [optKey, optLabel], i}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<div
						class={cn('option-row', String(value ?? 0) === optKey && 'selected', i === selectedIndex && 'focused')}
						role="option"
						tabindex="-1"
						aria-selected={String(value ?? 0) === optKey}
						onclick={() => handleSelect(optKey)}
						onmouseenter={() => selectedIndex = i}
					>
						<!-- a blank option (" ") keeps the height of a row with text -->
						{String(optLabel ?? '').trim() === '' ? '\u00a0' : optLabel}
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>

<style>
	@reference '../../../app.css';
	.option-dropdown {
		position: relative;
		width: 100%;
	}

	.option-trigger {
		@apply w-full px-3 py-2 pr-8 text-left bg-surface border border-gray-300 rounded-md;
		@apply text-gray-900 text-base cursor-pointer;
		@apply transition-all duration-150;
		@apply outline-hidden;
		@apply flex items-center justify-between;
		min-height: 2.5rem;
		position: relative;
	}

	.option-trigger:hover {
		@apply border-gray-400;
	}

	.option-trigger:focus {
		@apply border-blue-500 ring-2 ring-blue-500/20;
	}

	:global(.dark) .option-trigger {
		background-color: var(--color-bg-input, rgb(var(--gray-800)));
		border-color: var(--color-border-secondary, rgb(var(--gray-700)));
		color: var(--color-text-primary, rgb(var(--gray-50)));
	}

	:global(.dark) .option-trigger:hover {
		border-color: var(--color-text-muted);
	}

	:global(.dark) .option-trigger:focus {
		border-color: rgb(var(--accent-500));
		box-shadow: 0 0 0 2px rgba(0, 131, 143, 0.3);
	}

	/* Compact mode for list page cells */
	.option-dropdown-compact {
		height: 100%;
	}

	.option-trigger-compact {
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
		ring: 0 !important;
	}

	.option-trigger-compact:focus {
		outline: none !important;
		box-shadow: none !important;
		ring: 0 !important;
		border: 0 !important;
		background: transparent !important;
	}

	:global(.dark) .option-trigger-compact,
	:global(.dark) .option-trigger-compact:focus {
		background: transparent !important;
		color: white;
	}

	.option-value {
		@apply truncate;
		flex: 1;
	}

	.option-arrow {
		@apply text-xs text-gray-400;
		position: absolute;
		right: 0.5rem;
		top: 50%;
		transform: translateY(-50%);
	}

	.option-arrow-compact {
		font-size: 0.5rem;
		right: 4px;
	}

	.option-panel {
		position: absolute;
		top: 100%;
		left: 0;
		z-index: 50;
		min-width: 100%;
		width: max-content; /* the longest option on one line */
		max-height: 300px;
		margin-top: 2px;
		@apply bg-surface border border-gray-300 rounded-sm shadow-lg;
		overflow: hidden;
	}

	/* Dark mode: rows on the blank page background, highlighted row grey */
	:global(.dark) .option-panel {
		background-color: rgb(var(--gray-900));
		border-color: rgb(var(--gray-700));
	}

	.option-body {
		max-height: 250px;
		overflow-y: auto;
	}

	.option-row {
		@apply px-3 py-1.5 text-sm cursor-pointer whitespace-nowrap;
		@apply hover:bg-gray-100;
	}

	:global(.dark) .option-row {
		color: var(--color-text-primary, rgb(var(--gray-50)));
	}

	:global(.dark) .option-row:hover {
		background-color: rgb(var(--gray-700));
	}

	.option-row.selected {
		@apply bg-blue-50;
	}

	:global(.dark) .option-row.selected {
		background-color: rgba(0, 131, 143, 0.1);
	}

	.option-row.focused {
		@apply bg-gray-100;
	}

	:global(.dark) .option-row.focused {
		background-color: rgb(var(--gray-700));
	}

	.option-row.selected.focused {
		@apply bg-blue-100;
	}

	:global(.dark) .option-row.selected.focused {
		background-color: rgb(var(--gray-700));
	}
</style>
