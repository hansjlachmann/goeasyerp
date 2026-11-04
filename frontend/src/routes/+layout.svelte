<script lang="ts">
	import '../app.css';
	// Self-hosted fonts of the Appearance dialog (files load only when the font is used)
	import '@fontsource-variable/inter';
	import '@fontsource/ibm-plex-sans/400.css';
	import '@fontsource/ibm-plex-sans/500.css';
	import '@fontsource/ibm-plex-sans/600.css';
	import '@fontsource/ibm-plex-sans/700.css';
	import '@fontsource/ibm-plex-mono/400.css';
	import '@fontsource/ibm-plex-mono/500.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { session } from '$stores/session';
	import { currentUser } from '$lib/stores/user';
	import { toast } from '$lib/stores/toast';
	import { theme, wallpaperUrl } from '$lib/stores/theme';
	import { loadTranslations } from '$lib/services/i18n.svelte';
	import MenuBar from '$lib/components/menu/MenuBar.svelte';
	import Breadcrumb from '$lib/components/Breadcrumb.svelte';
	import { breadcrumb } from '$lib/stores/breadcrumb';
	import ToastContainer from '$lib/components/ToastContainer.svelte';
	import PrintPreview from '$lib/components/PrintPreview.svelte';
	import { regionalVersion } from '$lib/stores/regional';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	// Help pages open in a detached window (window.open): no menu bar or breadcrumb
	const detached = $derived(page.url.pathname.startsWith('/help/'));

	// The browser tab's title: the page shown (the breadcrumb's last item) and the product name
	const pageTitle = $derived.by(() => {
		const items = $breadcrumb.items;
		return items.length > 1 ? `${items[items.length - 1].label} – GoEasyERP` : 'GoEasyERP';
	});

	// Subscribe to theme to ensure it stays in sync
	let currentTheme = $state<'light' | 'dark'>('light');
	theme.subscribe((value) => {
		currentTheme = value;
	});

	// Initialize session and user on app load
	onMount(async () => {
		session.initialize();
		currentUser.loadFromStorage();

		// Load translations from backend API
		await loadTranslations();

		// Expose toast to console for testing (development only)
		if (typeof window !== 'undefined') {
			(window as any).toast = toast;
		}
	});
</script>

<svelte:head>
	{#if !detached}
		<title>{pageTitle}</title>
	{/if}
</svelte:head>

{#if $wallpaperUrl && !detached}
	<!-- Wallpaper of the active theme (Appearance); app.css: .wallpaper-layer, has-wallpaper -->
	<div class="wallpaper-layer" aria-hidden="true" style:background-image={`url("${$wallpaperUrl}")`}></div>
{/if}

<div class="min-h-screen flex flex-col">
	{#if !detached}
		<MenuBar />
		<Breadcrumb />
	{/if}
	<main class="flex-1" class:overflow-hidden={!detached}>
		<!-- drawn again when the user's date/number formats change (My Settings) -->
		{#key $regionalVersion}
			{@render children()}
		{/key}
	</main>
</div>

<!-- Toast notifications -->
<ToastContainer />

<!-- A printed report's preview -->
<PrintPreview />
