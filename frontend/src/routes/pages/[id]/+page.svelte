<script lang="ts">
	import PageRenderer from '$lib/components/pages/PageRenderer.svelte';

	// Get page props from SvelteKit
	let { data } = $props();
</script>

<div class="page-container">
	<!-- A new PageRenderer for every navigation: pages open inside the app (goto), and
	     PageRenderer loads its page in onMount — reused, it would keep showing the old page -->
	{#key data}
		<PageRenderer pageid={data.pageId} recordid={data.recordId} initialFilter={data.filter} initialFlowFilter={data.flowFilter} returnUrl={data.returnUrl} initialSelect={data.select} openCard={data.openCard} startNew={data.startNew} openParams={data.openParams} />
	{/key}
</div>

<style>
	.page-container {
		@apply h-full w-full p-6;
	}
</style>
