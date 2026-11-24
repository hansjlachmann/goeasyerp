<script lang="ts">
	// The button of a page action (list pages, cards, subpages): one size everywhere, no border,
	// an icon (the action's `image:`, a bullet point by default) and the caption — cut off with
	// "…" when it is too long (the whole caption shows as a tooltip)
	import type { Snippet } from 'svelte';
	import ActionIcon from './icons/ActionIcon.svelte';

	interface Props {
		caption: string;
		image?: string | null;
		shortcut?: string;
		title?: string;
		disabled?: boolean;
		active?: boolean; // a toggle that is on (e.g. row numbers)
		tabindex?: number;
		onclick?: (event: MouseEvent) => void;
		after?: Snippet; // e.g. a menu button's arrow
	}

	let { caption, image, shortcut, title, disabled = false, active = false, tabindex, onclick, after }: Props = $props();
	const tooltip = $derived(title ?? (shortcut ? `${caption} (${shortcut})` : caption));
</script>

<button
	type="button"
	class="btn btn-secondary action-btn"
	class:active
	{disabled}
	{tabindex}
	title={tooltip}
	onclick={(e) => !disabled && onclick?.(e)}
>
	<ActionIcon name={image} />
	{#if caption}<span class="action-caption">{caption}</span>{/if}
	{#if shortcut}<span class="action-shortcut">{shortcut}</span>{/if}
	{@render after?.()}
</button>

<style>
	@reference '../../app.css';
	/* One size for every action button; no border (the hover stays as on all buttons) */
	.action-btn {
		@apply h-8 px-2.5 py-0 gap-1.5 text-sm;
		@apply max-w-[14rem];
		border-color: transparent !important;
	}

	.action-caption {
		@apply truncate;
		min-width: 0;
	}

	.action-shortcut {
		@apply text-xs opacity-70 shrink-0;
	}

	.action-btn.active {
		@apply bg-primary-600 text-white hover:bg-primary-700;
	}
</style>
