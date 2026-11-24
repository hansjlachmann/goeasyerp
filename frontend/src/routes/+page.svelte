<script lang="ts">
	import { onMount } from 'svelte';
	import { breadcrumb } from '#lib/stores/breadcrumb.js';
	import { fetchMenu, clearMenuCache } from '#lib/services/pages.js';
	import { t, HOME, MSG } from '#lib/services/i18n.svelte.js';
	import type { MenuDefinition } from '#lib/types/pages.js';
	import MainMenu from '#lib/components/menu/MainMenu.svelte';
	import Tip from '#lib/components/Tip.svelte';
	import { TIPS } from '#lib/utils/tips.js';
	import { api } from '#lib/services/api.js';
	import { mainMenuTextClass, type MainMenuTextLine } from '#lib/utils/mainMenuText.js';

	let menu: MenuDefinition | null = $state(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	// the company's own text (Company Information → Main Menu Text); none: the standard text
	let ownText = $state<MainMenuTextLine[] | null>(null);

	// Clear breadcrumb on home page
	onMount(async () => {
		breadcrumb.clear();
		// Clear cache to ensure fresh menu data
		clearMenuCache();
		api.getMainMenuText()
			.then((lines) => (ownText = lines))
			.catch(() => (ownText = []));
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
			{#if ownText && ownText.length > 0}
				<!-- plain text only: never {@html} -->
				<div class="own-text">
					{#each ownText as line, i (i)}
						{#if line.style === 'heading' && i === 0}
							<h1 class={mainMenuTextClass(line)}>{line.text}</h1>
						{:else}
							<p class={mainMenuTextClass(line)}>{line.text || '\u00a0'}</p>
						{/if}
					{/each}
				</div>
			{:else if ownText}
				<h1 class="text-3xl font-bold text-heading dark:text-blue-400 mb-2">{t(HOME.TITLE)}</h1>
				<p class="text-gray-600 dark:text-gray-400">
					{t(HOME.DESCRIPTION)}
				</p>
			{/if}
		</div>

		<!-- The main menu: modules and their menus -->
		{#if loading}
			<div class="text-gray-500">{t(MSG.LOADING_MENU)}</div>
		{:else if error}
			<div class="text-red-500">{error}</div>
		{:else if menu && menu.menu && menu.menu.length > 0}
			<div class="menu-tip"><Tip tip={TIPS.mainMenu} /></div>
			<MainMenu {menu} />
		{:else}
			<div class="text-gray-500">{t(MSG.NO_MENU_ITEMS)}</div>
		{/if}
	</div>
</div>

<style>
	@reference '../app.css';
	/* the company's own lines, spaced like the standard heading and text */
	.own-text {
		@apply space-y-1 whitespace-pre-wrap;
	}

	.own-text :global(h1) {
		@apply mb-1;
	}

	.menu-tip {
		@apply mb-3 mx-auto max-w-4xl;
	}
</style>
