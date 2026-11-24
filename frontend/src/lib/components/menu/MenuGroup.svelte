<script lang="ts">
	import { goto } from '$app/navigation';
	import type { MenuGroup as MenuGroupType } from '#lib/types/pages.js';
	import { clickOutside } from '#lib/actions/clickOutside.js';

	interface Props {
		group: MenuGroupType;
	}

	let { group }: Props = $props();

	let isOpen = $state(false);

	// Check if this is a flat menu item (has page_id directly) or a group (has items)
	const isFlatItem = $derived(group.page_id && group.page_id > 0);

	function toggleMenu() {
		isOpen = !isOpen;
	}

	function closeMenu() {
		isOpen = false;
	}

	function handleMenuItemClick(pageId: number) {
		goto(`/pages/${pageId}`); // inside the app: no reload, no flash
		closeMenu();
	}

	function handleFlatItemClick() {
		if (group.page_id) {
			goto(`/pages/${group.page_id}`);
		}
	}
</script>

{#if isFlatItem}
	<!-- Flat menu item - direct link, no dropdown -->
	<button
		class="menu-button px-4 py-2 hover:bg-appbar-fg/10 rounded-sm transition-colors flex items-center gap-2"
		onclick={handleFlatItemClick}
	>
		{#if group.icon}
			<span class="icon">{group.icon}</span>
		{/if}
		<span class="font-medium">{group.name}</span>
	</button>
{:else}
	<!-- Grouped menu item - has dropdown -->
	<div class="menu-group relative" use:clickOutside={{ callback: closeMenu, enabled: isOpen }}>
		<button
			class="menu-button px-4 py-2 hover:bg-appbar-fg/10 rounded-sm transition-colors flex items-center gap-2"
			onclick={toggleMenu}
		>
			{#if group.icon}
				<span class="icon">{group.icon}</span>
			{/if}
			<span class="font-medium">{group.name}</span>
			<span class="text-xs">▼</span>
		</button>

		{#if isOpen && group.items}
			<div class="menu-dropdown absolute top-full left-0 mt-1 bg-surface text-gray-800 rounded-lg shadow-xl border border-gray-200 min-w-64 z-50">
				{#each group.items as item}
					{#if item.separator}
						<div class="menu-separator border-t border-gray-200 my-1"></div>
					{:else if item.enabled !== false}
						<button
							class="menu-item w-full px-4 py-3 hover:bg-blue-50 transition-colors flex items-start gap-3 text-left"
							onclick={() => item.page_id && handleMenuItemClick(item.page_id)}
							disabled={!item.page_id}
						>
							{#if item.icon}
								<span class="icon text-heading mt-0.5">{item.icon}</span>
							{/if}
							<div class="flex-1">
								<div class="font-medium text-gray-900">{item.name}</div>
								{#if item.description}
									<div class="text-sm text-gray-500 mt-0.5">{item.description}</div>
								{/if}
							</div>
						</button>
					{:else}
						<div class="menu-item-disabled w-full px-4 py-3 flex items-start gap-3 opacity-50 cursor-not-allowed">
							{#if item.icon}
								<span class="icon text-gray-400 mt-0.5">{item.icon}</span>
							{/if}
							<div class="flex-1">
								<div class="font-medium text-gray-500">{item.name}</div>
								{#if item.description}
									<div class="text-sm text-gray-400 mt-0.5">{item.description}</div>
								{/if}
							</div>
						</div>
					{/if}
				{/each}
			</div>
		{/if}
	</div>
{/if}

<style>
	@reference '../../../app.css';
	.menu-group {
		@apply relative;
	}

	.menu-button {
		@apply text-sm font-medium cursor-pointer select-none;
	}

	.menu-dropdown {
		animation: slideDown 0.15s ease-out;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.menu-item {
		@apply border-none outline-hidden;
	}

	.menu-item:first-child {
		@apply rounded-t-lg;
	}

	.menu-item:last-child {
		@apply rounded-b-lg;
	}
</style>
