<script lang="ts">
	// The GoEasyERP wordmark ("Go" amber, "EasyERP" slate or off-white), from frontend/static
	// (the source files are in goeasyerp-brand/). Variants: slate on light backgrounds, off-white
	// on slate (the app bar); auto follows the light/dark mode.
	import { base } from '$app/paths';

	interface Props {
		variant?: 'slate' | 'offwhite' | 'auto';
		height?: string; // CSS height, e.g. "1.25rem"
		alt?: string; // "" where the name is in the text next to it
	}

	let { variant = 'auto', height = '1.5rem', alt = 'GoEasyERP' }: Props = $props();
</script>

{#if variant === 'auto'}
	<img class="wordmark light-only" src="{base}/goeasyerp-wordmark-slate.svg" {alt} style:height />
	<img class="wordmark dark-only" src="{base}/goeasyerp-wordmark-offwhite.svg" {alt} style:height />
{:else}
	<img class="wordmark" src="{base}/goeasyerp-wordmark-{variant}.svg" {alt} style:height />
{/if}

<style>
	.wordmark {
		display: block;
		width: auto;
	}

	.dark-only {
		display: none;
	}

	:global(.dark) .light-only {
		display: none;
	}

	:global(.dark) .dark-only {
		display: block;
	}
</style>
