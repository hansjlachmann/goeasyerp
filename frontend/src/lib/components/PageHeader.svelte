<script lang="ts">
	import Button from './Button.svelte';
	import { t, HELP, MODAL } from '$lib/services/i18n.svelte';
	import type { Snippet } from 'svelte';

	interface Action {
		label: string;
		onClick: () => void;
		variant?: 'primary' | 'secondary' | 'danger';
		shortcut?: string;
	}

	interface Props {
		title: string;
		subtitle?: string;
		onclose?: () => void;
		actions?: Action[];
		leftActions?: Snippet;
		rightActions?: Snippet;
	}

	let {
		title,
		subtitle,
		onclose,
		actions = [],
		leftActions,
		rightActions
	}: Props = $props();
</script>

<div class="page-header" class:has-close={!!onclose}>
	<!-- With a title: the title and the page's tools (search, customize, filter) on the first row,
	     the functions on rows of their own below it, starting at the left edge. Without a title
	     (cards, subpages) the functions come first. Everything wraps: nothing leaves the screen. -->
	<div class="header-bar" class:has-title={!!(title || subtitle)}>
		{#if title || subtitle}
			<div class="header-heading">
				{#if title}<h1 class="header-title">{title}</h1>{/if}
				{#if subtitle}
					<p class="header-subtitle">{subtitle}</p>
				{/if}
			</div>
		{/if}

		{#if leftActions}
			<div class="header-left">
				{@render leftActions()}
			</div>
		{/if}

		{#if rightActions}
			<div class="header-right">
				{@render rightActions()}
			</div>
		{:else if actions.length > 0}
			<div class="header-right">
				{#each actions as action}
					<Button variant={action.variant || 'secondary'} onclick={action.onClick}>
						{action.label}
						{#if action.shortcut}
							<span class="ml-2 text-xs opacity-70">({action.shortcut})</span>
						{/if}
					</Button>
				{/each}
			</div>
		{/if}
	</div>

	{#if onclose}
		<!-- not a Tab stop: Esc closes -->
		<button
			class="close-button"
			tabindex="-1"
			onclick={onclose}
			title={t(HELP.CLOSE)}
			aria-label={t(MODAL.CLOSE)}
		>
			<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
			</svg>
		</button>
	{/if}
</div>

<style>
	.page-header {
		@apply bg-surface border-b border-gray-200 px-6 py-4;
		position: relative;
		min-width: 0;
	}

	/* The close button sits in the top right corner: keep the rows clear of it */
	.page-header.has-close {
		padding-right: 3.5rem;
	}

	.header-bar {
		@apply flex flex-wrap items-center gap-x-4 gap-y-2;
		min-width: 0;
	}

	.header-heading {
		flex: none;
		max-width: 100%;
	}

	.header-left {
		@apply flex flex-wrap items-center gap-x-1 gap-y-1;
		flex: 1 1 auto;
		min-width: 0;
		max-width: 100%;
	}

	.header-left:empty {
		display: none;
	}

	/* No title (cards, subpages): the tools follow the functions on the same rows */
	.header-bar:not(.has-title) {
		@apply gap-x-1 gap-y-1;
	}

	.header-bar:not(.has-title) .header-left {
		display: contents;
	}

	/* Below the title and the tools, across the whole width */
	.has-title .header-left {
		order: 1;
		flex-basis: 100%;
	}

	.header-right {
		@apply flex flex-wrap items-center gap-x-1 gap-y-1;
		margin-left: auto;
		max-width: 100%;
	}

	:global(.dark) .page-header {
		background-color: rgb(var(--gray-800)); /* gray-800 */
		border-color: rgb(var(--gray-700)); /* gray-700 */
	}

	.header-title {
		@apply text-2xl font-bold text-heading;
		white-space: nowrap;
	}

	:global(.dark) .header-title {
		color: rgb(var(--accent-400)); /* blue-400 */
	}

	.header-subtitle {
		@apply mt-1 text-sm text-gray-600;
	}

	:global(.dark) .header-subtitle {
		color: rgb(var(--gray-400)); /* gray-400 */
	}

	.close-button {
		position: absolute;
		top: 0.75rem;
		right: 0.75rem;
		@apply p-2 rounded-md text-gray-500 hover:text-gray-700 hover:bg-gray-100 transition-colors;
	}

	:global(.dark) .close-button {
		color: rgb(var(--gray-400));
	}

	:global(.dark) .close-button:hover {
		color: rgb(var(--gray-100));
		background-color: rgb(var(--gray-700));
	}
</style>
