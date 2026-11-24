<script lang="ts">
	// An action's icon by its image name (actionIcons.ts); a bullet point without one
	import { actionIconPaths } from './actionIcons';

	let { name }: { name?: string | null } = $props();
	const paths = $derived(actionIconPaths(name));
</script>

{#if paths}
	<svg class="action-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
		{#each paths as d}
			<path stroke-linecap="round" stroke-linejoin="round" {d} />
		{/each}
	</svg>
{:else}
	<svg class="action-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true">
		<circle cx="12" cy="12" r="3" fill="currentColor" />
	</svg>
{/if}

<style>
	@reference '../../../app.css';
	.action-icon {
		@apply h-4 w-4 shrink-0;
	}
</style>
