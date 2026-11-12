<script lang="ts">
	// The open list of a menu of actions — a menu button's dropdown (ActionMenu) or a list row's
	// right-click menu (the list's context menu). It floats above the page (fixed, moved to the
	// app root). An action with items is a sub-menu: a heading with its items indented under it;
	// `separator` before an action draws a line above it. Keyboard: ↑/↓/Home/End move over the
	// items (not the headings), Enter picks, Esc / Tab close; a click outside closes it.
	import { onMount, tick } from 'svelte';
	import ActionIcon from './icons/ActionIcon.svelte';
	import { portal } from '$lib/actions/portal';
	import type { Action, MenuEntry } from '$lib/types/pages';
	import { t, HOME } from '$lib/services/i18n.svelte';

	interface Props {
		actions: MenuEntry[];
		/** fixed position of the panel's top left corner */
		x: number;
		y: number;
		// whether an item can run (e.g. it needs a selected record)
		itemDisabled?: (item: Action) => boolean;
		onpick: (item: Action) => void;
		/** the menu closes (refocus: give the focus back to where it came from) */
		onclose: (refocus: boolean) => void;
		/** clicks on this element do not close the menu (the menu button toggles it itself) */
		owner?: HTMLElement | null;
		/** keep the panel inside the window (a right-click near the edge opens it up/left) */
		fit?: boolean;
	}

	let { actions, x, y, itemDisabled = () => false, onpick, onclose, owner = null, fit = false }: Props = $props();

	let active = $state(0); // index among the items that can be picked (not the headings)
	let menuElement = $state<HTMLDivElement | null>(null);
	let left = $state(0);
	let top = $state(0);

	// The menu's rows: items and sub-menu headings, with their level (indent)
	interface Row {
		item: MenuEntry;
		level: number;
		heading: boolean;
	}
	function rows(entries: MenuEntry[] | undefined, level = 0): Row[] {
		return (entries ?? []).flatMap((item) =>
			item.items
				? [{ item, level, heading: true }, ...rows(item.items, level + 1)]
				: [{ item, level, heading: false }]
		);
	}
	const menuRows = $derived(rows(actions));
	const items = $derived(menuRows.filter((r) => !r.heading).map((r) => r.item));

	function enabled(item: Action): boolean {
		return item.enabled !== false && !itemDisabled(item);
	}

	function place() {
		left = x;
		top = y;
		if (!fit || !menuElement) return;
		const { width, height } = menuElement.getBoundingClientRect();
		if (left + width > window.innerWidth - 4) left = Math.max(4, x - width);
		if (top + height > window.innerHeight - 4) top = Math.max(4, Math.min(y - height, window.innerHeight - height - 4));
	}

	$effect(() => {
		void x;
		void y;
		place();
	});

	onMount(async () => {
		active = Math.max(0, items.findIndex(enabled));
		await tick();
		place();
		menuElement?.querySelectorAll<HTMLButtonElement>('.menu-item')[active]?.focus();
	});

	function pick(item: Action) {
		if (!enabled(item)) return;
		onclose(true);
		onpick(item);
	}

	function move(step: number) {
		const buttons = menuElement?.querySelectorAll<HTMLButtonElement>('.menu-item');
		if (!buttons || buttons.length === 0) return;
		active = (active + step + buttons.length) % buttons.length;
		buttons[active].focus();
	}

	function handleKeydown(e: KeyboardEvent) {
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
				onclose(true);
				break;
			case 'Tab':
				onclose(false);
				break;
		}
	}
</script>

<svelte:window
	onresize={() => onclose(false)}
	onscrollcapture={(e) => {
		// a right-click menu belongs to where it was opened: scrolling the page closes it
		if (fit && !menuElement?.contains(e.target as Node)) onclose(false);
	}}
	onpointerdowncapture={(e) => {
		// A click outside the menu (and its button) closes it
		const target = e.target as Node;
		if (!menuElement?.contains(target) && !owner?.contains(target)) onclose(false);
	}}
/>

<!-- svelte-ignore a11y_interactive_supports_focus -->
<div
	class="menu"
	role="menu"
	data-own-keys
	data-action-menu
	use:portal
	style="top: {top}px; left: {left}px;"
	bind:this={menuElement}
	onkeydown={handleKeydown}
	oncontextmenu={(e) => e.preventDefault()}
>
	{#each menuRows as row, r (row.item.name + ':' + row.level + ':' + r)}
		{#if row.item.separator && r > 0}
			<div class="menu-separator" role="separator"></div>
		{/if}
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
				title={row.item.denied ? t(HOME.MENU_NO_PERMISSION) : undefined}
				onclick={() => pick(row.item)}
			>
				<span class="menu-label"
					><ActionIcon name={row.item.image} /><span class="truncate">{row.item.caption}</span>{#if row.item.note}<span class="menu-note">{row.item.note}</span>{/if}</span
				>
				{#if row.item.shortcut}<span class="menu-shortcut">{row.item.shortcut}</span>{/if}
			</button>
		{/if}
	{/each}
</div>

<style>
	.menu {
		position: fixed;
		z-index: 1000;
		@apply min-w-[12rem] max-h-[80vh] overflow-y-auto py-1;
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

	.menu-separator {
		@apply my-1 border-t border-gray-200;
	}

	:global(.dark) .menu-separator {
		border-color: rgb(var(--gray-700));
	}

	.menu-label {
		@apply flex items-center gap-2 min-w-0;
	}

	.menu-note {
		@apply text-xs text-gray-500 whitespace-nowrap;
	}

	.menu-shortcut {
		@apply text-xs opacity-60 whitespace-nowrap;
	}
</style>
