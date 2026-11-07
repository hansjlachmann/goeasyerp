<script lang="ts">
	// A menu button of a page's action bar: a dropdown with the action's menu items (actions of
	// their own). An item with items of its own is a sub-menu: a heading with its items indented
	// under it. Keyboard: Enter, Space or ↓ open it, ↑/↓ move (over the items, not the headings),
	// Enter picks, Esc / Tab close.
	import { tick } from 'svelte';
	import ActionButton from './ActionButton.svelte';
	import ActionIcon from './icons/ActionIcon.svelte';
	import { portal } from '$lib/actions/portal';
	import type { Action } from '$lib/types/pages';

	interface Props {
		action: Action;
		disabled?: boolean;
		// whether an item can run (e.g. it needs a selected record)
		itemDisabled?: (item: Action) => boolean;
		onselect: (item: Action) => void;
	}

	let { action, disabled = false, itemDisabled = () => false, onselect }: Props = $props();

	let open = $state(false);
	let active = $state(0); // index among the items that can be picked (not the headings)
	let menuElement = $state<HTMLDivElement | null>(null);

	// The menu's rows: items and sub-menu headings, with their level (indent)
	interface Row {
		item: Action;
		level: number;
		heading: boolean;
	}
	function rows(actions: Action[] | undefined, level = 0): Row[] {
		return (actions ?? []).flatMap((item) =>
			item.items
				? [{ item, level, heading: true }, ...rows(item.items, level + 1)]
				: [{ item, level, heading: false }]
		);
	}
	const menuRows = $derived(rows(action.items));
	const items = $derived(menuRows.filter((r) => !r.heading).map((r) => r.item));

	function enabled(item: Action): boolean {
		return item.enabled !== false && !itemDisabled(item);
	}

	// The menu floats above the page at the button (fixed): no list header, card section or
	// scrolling part can cover or cut it
	let triggerElement = $state<HTMLDivElement | null>(null);
	let menuStyle = $state('');
	function placeMenu() {
		const rect = triggerElement?.getBoundingClientRect();
		if (!rect) return;
		menuStyle = `top: ${rect.bottom + 4}px; left: ${rect.left}px;`;
	}

	async function openMenu() {
		if (disabled || items.length === 0) return;
		placeMenu();
		open = true;
		active = Math.max(0, items.findIndex(enabled));
		await tick();
		menuElement?.querySelectorAll<HTMLButtonElement>('.menu-item')[active]?.focus();
	}

	function close(refocus = true) {
		open = false;
		if (refocus) triggerElement?.querySelector<HTMLButtonElement>('button')?.focus();
	}

	function pick(item: Action) {
		if (!enabled(item)) return;
		close();
		onselect(item);
	}

	function move(step: number) {
		const buttons = menuElement?.querySelectorAll<HTMLButtonElement>('.menu-item');
		if (!buttons || buttons.length === 0) return;
		active = (active + step + buttons.length) % buttons.length;
		buttons[active].focus();
	}

	function handleTriggerKeydown(e: KeyboardEvent) {
		if (!open && (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ')) {
			e.preventDefault();
			e.stopPropagation();
			openMenu();
		}
	}

	function handleMenuKeydown(e: KeyboardEvent) {
		// The menu's keys are its own (not the page's shortcuts)
		e.stopPropagation();
		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				move(1);
				break;
			case 'ArrowUp':
				e.preventDefault();
				move(-1);
				break;
			case 'Home':
				e.preventDefault();
				move(-active);
				break;
			case 'End':
				e.preventDefault();
				move(items.length - 1 - active);
				break;
			case 'Escape':
				e.preventDefault();
				close();
				break;
			case 'Tab':
				close(false);
				break;
		}
	}
</script>

<svelte:window
	onresize={() => open && close(false)}
	onscrollcapture={() => open && placeMenu()}
	onpointerdowncapture={(e) => {
		// A click outside the button and the menu closes it
		const target = e.target as Node;
		if (open && !triggerElement?.contains(target) && !menuElement?.contains(target)) close(false);
	}}
/>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="action-menu"
	bind:this={triggerElement}
	data-own-keys
	onkeydown={(e) => {
		if (!open && (e.target as HTMLElement).closest('.menu') === null) handleTriggerKeydown(e);
	}}
>
	<ActionButton caption={action.caption} image={action.image} {disabled} onclick={() => (open ? close() : openMenu())}>
		{#snippet after()}
			<svg class="menu-arrow" class:open xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
			</svg>
		{/snippet}
	</ActionButton>

	{#if open}
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<div class="menu" role="menu" data-own-keys data-action-menu use:portal style={menuStyle} bind:this={menuElement} onkeydown={handleMenuKeydown}>
			{#each menuRows as row (row.item.name + row.level)}
				{#if row.heading}
					<div class="menu-heading" role="presentation" style="padding-left: {0.75 + row.level * 1.25}rem">
						<ActionIcon name={row.item.image} /><span class="truncate">{row.item.caption}</span>
					</div>
				{:else}
					{@const i = items.indexOf(row.item)}
					<button
						type="button"
						class="menu-item"
						role="menuitem"
						style="padding-left: {0.75 + row.level * 1.25}rem"
						tabindex={i === active ? 0 : -1}
						disabled={!enabled(row.item)}
						onclick={() => pick(row.item)}
					>
						<span class="menu-label"><ActionIcon name={row.item.image} /><span class="truncate">{row.item.caption}</span></span>
						{#if row.item.shortcut}<span class="menu-shortcut">{row.item.shortcut}</span>{/if}
					</button>
				{/if}
			{/each}
		</div>
	{/if}
</div>

<style>
	.action-menu {
		@apply relative inline-block;
	}

	.menu-arrow {
		@apply h-3.5 w-3.5 shrink-0 transition-transform duration-150;
	}

	.menu-arrow.open {
		@apply rotate-180;
	}

	.menu {
		position: fixed;
		z-index: 1000;
		@apply min-w-[12rem] py-1;
		@apply bg-surface border border-gray-200 rounded-md shadow-lg;
	}

	:global(.dark) .menu {
		background-color: rgb(var(--gray-800));
		border-color: rgb(var(--gray-700));
	}

	.menu-item {
		@apply w-full flex items-center justify-between gap-4 px-3 py-1.5 text-sm text-left;
		@apply text-gray-800 hover:bg-gray-100 focus:bg-gray-100 focus:outline-none;
	}

	:global(.dark) .menu-item {
		color: rgb(var(--gray-100));
	}

	:global(.dark) .menu-item:hover,
	:global(.dark) .menu-item:focus {
		background-color: rgb(var(--gray-700));
	}

	.menu-item:disabled {
		@apply opacity-50 cursor-not-allowed;
	}

	/* A sub-menu's heading: not clickable, its items are indented under it */
	.menu-heading {
		@apply flex items-center gap-2 pr-3 py-1.5 text-sm font-medium text-gray-500 cursor-default select-none;
	}

	:global(.dark) .menu-heading {
		color: rgb(var(--gray-400));
	}

	.menu-label {
		@apply flex items-center gap-2 min-w-0;
	}

	.menu-shortcut {
		@apply text-xs opacity-60;
	}
</style>
