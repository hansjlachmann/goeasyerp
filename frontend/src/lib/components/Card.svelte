<script lang="ts">
	import { cn } from '$utils/cn';
	import type { Snippet } from 'svelte';

	interface Props {
		title?: string;
		collapsible?: boolean;
		collapsed?: boolean;
		class?: string;
		children?: Snippet;
		header?: Snippet;
		actions?: Snippet;
		// A card page's section: its name (data-section, for the card's keys) and what happens
		// when the user opens or closes it (click, Enter or Space on the header)
		section?: string;
		ontoggle?: (collapsed: boolean) => void;
	}

	let {
		title,
		collapsible = false,
		collapsed = $bindable(false),
		class: className,
		children,
		header,
		actions,
		section,
		ontoggle
	}: Props = $props();

	function toggleCollapse() {
		if (collapsible) {
			collapsed = !collapsed;
			ontoggle?.(collapsed);
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (collapsible && (e.key === 'Enter' || e.key === ' ')) {
			e.preventDefault();
			toggleCollapse();
		}
	}
</script>

<div class={cn('card', className)} data-section={section}>
	{#if title || header}
		<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
		<!-- Not a Tab stop (Tab goes through the fields): clicked, or reached by the card's keys -->
		<div
			class="card-header"
			class:collapsible
			class:collapsed={collapsible && collapsed}
			role={collapsible ? 'button' : undefined}
			tabindex={collapsible ? -1 : undefined}
			aria-expanded={collapsible ? !collapsed : undefined}
			onclick={collapsible ? toggleCollapse : undefined}
			onkeydown={collapsible ? handleKeydown : undefined}
		>
			<div class="card-header-content">
				{#if header}
					{@render header()}
				{:else if title}
					<h2 class="card-title">{title}</h2>
				{/if}
			</div>
			{#if collapsible}
				<svg
					class="collapse-icon"
					class:rotated={!collapsed}
					xmlns="http://www.w3.org/2000/svg"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
				>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
				</svg>
			{/if}
		</div>
	{/if}

	{#if !collapsible || !collapsed}
		<div class="card-body">
			{#if children}
				{@render children()}
			{/if}
		</div>
	{/if}

	{#if actions}
		<div class="card-actions">
			{@render actions()}
		</div>
	{/if}
</div>

<style>
	.card {
		@apply bg-surface border border-gray-200 rounded-lg;
		@apply shadow-sm;
	}

	:global(.dark) .card {
		background-color: var(--color-bg-primary);
		border-color: var(--color-border-primary);
	}

	.card-header {
		@apply px-4 py-3 border-b border-gray-200;
		@apply flex items-center justify-between;
	}

	:global(.dark) .card-header {
		border-color: var(--color-border-primary);
	}

	.card-header.collapsible {
		@apply cursor-pointer select-none;
		@apply transition-colors duration-150;
	}

	.card-header.collapsible:hover {
		@apply bg-gray-50;
	}

	:global(.dark) .card-header.collapsible:hover {
		background-color: var(--color-bg-tertiary);
	}

	.card-header.collapsible:focus {
		@apply outline-none ring-2 ring-inset ring-blue-500;
	}

	.card-header.collapsed {
		@apply border-b-0;
	}

	.card-header-content {
		@apply flex-1;
	}

	.card-title {
		@apply text-xl font-semibold text-gray-900;
	}

	:global(.dark) .card-title {
		color: var(--color-text-primary);
	}

	.collapse-icon {
		@apply w-5 h-5 text-gray-500 transition-transform duration-200;
	}

	:global(.dark) .collapse-icon {
		color: var(--color-text-muted);
	}

	.collapse-icon.rotated {
		@apply rotate-180;
	}

	.card-body {
		@apply p-4;
	}

	.card-actions {
		@apply px-4 py-3 border-t border-gray-200 bg-gray-50 flex justify-end gap-3;
	}

	:global(.dark) .card-actions {
		background-color: var(--color-bg-secondary);
		border-color: var(--color-border-primary);
	}
</style>
