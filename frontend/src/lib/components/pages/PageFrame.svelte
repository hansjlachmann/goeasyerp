<script lang="ts">
	// A page in a page frame (PageFrames): the page of its URL, as the routes show it, with the
	// frame's navigator
	import PageRenderer from './PageRenderer.svelte';
	import { setPageNavigator, type PageNavigator } from '$lib/utils/pageNavigator';
	import { parsePagePath } from '$lib/utils/pageUrl';

	let { url, pageNav }: { url: string; pageNav: PageNavigator } = $props();

	// svelte-ignore state_referenced_locally - a frame keeps its navigator
	setPageNavigator(pageNav);
	const page = $derived(parsePagePath(url));
</script>

<!-- A new PageRenderer for every URL of the frame (as the routes: it loads in onMount) -->
{#key url}
	{#if page}
		<PageRenderer
			pageid={page.pageId}
			recordid={page.recordId}
			initialFilter={page.filter}
			initialSelect={page.select}
			openCard={page.openCard}
			startNew={page.startNew}
			openParams={page.openParams}
			navigationQuery={page.navigationQuery}
			initialEdit={page.edit}
		/>
	{/if}
{/key}
