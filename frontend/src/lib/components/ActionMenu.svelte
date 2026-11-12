<script lang="ts">
	// A menu button of a page's action bar: a dropdown with the action's menu items (actions of
	// their own; MenuPanel). Keyboard: Enter, Space or ↓ open it, ↑/↓ move (over the items, not
	// the headings), Enter picks, Esc / Tab close.
	import ActionButton from './ActionButton.svelte';
	import MenuPanel from './MenuPanel.svelte';
	import type { Action } from '$lib/types/pages';
	import { t, HOME } from '$lib/services/i18n.svelte';

	interface Props {
		action: Action;
		disabled?: boolean;
		// whether an item can run (e.g. it needs a selected record)
		itemDisabled?: (item: Action) => boolean;
		onselect: (item: Action) => void;
	}

	let { action, disabled = false, itemDisabled = () => false, onselect }: Props = $props();

	let open = $state(false);
	let x = $state(0);
	let y = $state(0);

	const hasItems = $derived((action.items ?? []).length > 0);

	// The menu floats above the page at the button (fixed): no list header, card section or
	// scrolling part can cover or cut it
	let triggerElement = $state<HTMLDivElement | null>(null);
	function placeMenu() {
		const rect = triggerElement?.getBoundingClientRect();
		if (!rect) return;
		x = rect.left;
		y = rect.bottom + 4;
	}

	function openMenu() {
		if (disabled || !hasItems) return;
		placeMenu();
		open = true;
	}

	function close(refocus = true) {
		open = false;
		if (refocus) triggerElement?.querySelector<HTMLButtonElement>('button')?.focus();
	}

	// On the button only the keys that open the menu are its own (taken before the page sees them:
	// Enter would open the record, ↓ move the row); the page's shortcuts and Esc work from it
	function handleTriggerKeydown(e: KeyboardEvent) {
		if (!open && (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ')) {
			e.preventDefault();
			e.stopPropagation();
			openMenu();
		}
	}
</script>

<svelte:window onscrollcapture={() => open && placeMenu()} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="action-menu" bind:this={triggerElement} onkeydowncapture={handleTriggerKeydown}>
	<!-- a menu button whose items the user may not run at all is greyed out (denied) -->
	<ActionButton
		caption={action.caption}
		image={action.image}
		disabled={disabled || action.enabled === false}
		title={action.denied ? t(HOME.MENU_NO_PERMISSION) : undefined}
		onclick={() => (open ? close() : openMenu())}
	>
		{#snippet after()}
			<svg class="menu-arrow" class:open xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
			</svg>
		{/snippet}
	</ActionButton>

	{#if open}
		<MenuPanel actions={action.items ?? []} {x} {y} {itemDisabled} owner={triggerElement} onpick={onselect} onclose={close} />
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
</style>
