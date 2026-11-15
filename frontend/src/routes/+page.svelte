<script lang="ts">
	import { onMount } from 'svelte';
	import { breadcrumb } from '$lib/stores/breadcrumb';
	import { fetchMenu, clearMenuCache } from '$lib/services/pages';
	import { t, HOME, MSG } from '$lib/services/i18n.svelte';
	import type { MenuDefinition } from '$lib/types/pages';
	import { openShortcutHelp } from '$lib/utils/shortcutHelp';
	import MainMenu from '$lib/components/menu/MainMenu.svelte';

	let menu: MenuDefinition | null = $state(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Clear breadcrumb on home page
	onMount(async () => {
		breadcrumb.clear();
		// Clear cache to ensure fresh menu data
		clearMenuCache();
		try {
			menu = await fetchMenu();
		} catch (err) {
			console.error('Error loading menu:', err);
			error = err instanceof Error ? err.message : 'Failed to load menu';
		} finally {
			loading = false;
		}
	});
</script>

<div class="container mx-auto px-4 py-8">
	<div class="max-w-5xl mx-auto">
		<div class="text-center mb-8">
			<h1 class="text-3xl font-bold text-heading dark:text-blue-400 mb-2">{t(HOME.TITLE)}</h1>
			<p class="text-gray-600 dark:text-gray-400">
				{t(HOME.DESCRIPTION)}
			</p>
			<div class="shortcuts-tip">
				<p class="font-semibold text-fg">⌨ {t(HOME.SHORTCUTS_TITLE)}</p>
				<p class="text-gray-600 dark:text-gray-400">{t(HOME.SHORTCUTS_TEXT)}</p>
				<button type="button" class="shortcuts-link" onclick={openShortcutHelp}>{t(HOME.SHORTCUTS_LINK)} ↗</button>
			</div>
		</div>

		<!-- The main menu: modules and their menus -->
		{#if loading}
			<div class="text-gray-500">{t(MSG.LOADING_MENU)}</div>
		{:else if error}
			<div class="text-red-500">{error}</div>
		{:else if menu && menu.menu && menu.menu.length > 0}
			<MainMenu {menu} />
		{:else}
			<div class="text-gray-500">{t(MSG.NO_MENU_ITEMS)}</div>
		{/if}
	</div>
</div>

<style>
	/* the keyboard tip, close under the description, without a frame */
	.shortcuts-tip {
		@apply mt-2 mx-auto max-w-2xl text-sm space-y-1;
	}

	.shortcuts-link {
		@apply text-primary-600 hover:underline font-medium;
	}

	:global(.dark) .shortcuts-link {
		@apply text-primary-400;
	}
</style>
