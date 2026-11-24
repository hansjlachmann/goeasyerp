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
	import { session } from '#lib/stores/session.js';
	import { currentUser } from '#lib/stores/user.js';
	import { toast } from '#lib/stores/toast.js';
	import { theme, wallpaperUrl } from '#lib/stores/theme.js';
	import { loadTranslations } from '#lib/services/i18n.svelte.js';
	import MenuBar from '#lib/components/menu/MenuBar.svelte';
	import Breadcrumb from '#lib/components/Breadcrumb.svelte';
	import { breadcrumb } from '#lib/stores/breadcrumb.js';
	import ToastContainer from '#lib/components/ToastContainer.svelte';
	import PrintPreview from '#lib/components/PrintPreview.svelte';
	import QuestionDialog from '#lib/components/QuestionDialog.svelte';
	import MaintenanceNotice from '#lib/components/MaintenanceNotice.svelte';
	import { installMaintenanceWatch } from '#lib/utils/maintenance.js';
	import { installTabCompany } from '#lib/utils/tabCompany.js';
	import { installAppUpdateReload, reloadForUpdate, reloadsAtOnce } from '#lib/utils/appUpdate.js';
	import { updated } from '$app/state';
	import { regionalVersion } from '#lib/stores/regional.js';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	// Every API answer is watched for the server's maintenance (before the first call below)
	installMaintenanceWatch();
	// … and every API request carries the company this tab works in (tabs in different companies)
	installTabCompany();
	// … and a new version on the server is loaded by itself (utils/appUpdate.ts)
	installAppUpdateReload();
	$effect(() => {
		if (updated.current && reloadsAtOnce(page.url.pathname)) reloadForUpdate();
	});

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

<!-- The server's Yes/No questions during a request (a table trigger's Confirm) -->
<QuestionDialog />

<!-- The server is being updated (its apps built for a new version): a notice over everything -->
<MaintenanceNotice />
