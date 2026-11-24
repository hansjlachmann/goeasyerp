<script lang="ts">
	import { cn } from '#lib/utils/cn.js';

	interface Props {
		variant?: 'primary' | 'secondary' | 'danger' | 'success';
		size?: 'sm' | 'md' | 'lg';
		disabled?: boolean;
		type?: 'button' | 'submit' | 'reset';
		tabindex?: number;
		title?: string;
		onclick?: (event: MouseEvent) => void;
		icon?: import('svelte').Snippet;
		children?: import('svelte').Snippet;
		class?: string;
	}

	let {
		variant = 'primary',
		size = 'md',
		disabled = false,
		type = 'button',
		tabindex,
		title,
		onclick,
		icon,
		children,
		class: className
	}: Props = $props();

	const variants = {
		primary: 'btn-primary',
		secondary: 'btn-secondary',
		danger: 'btn-danger',
		success: 'btn-success'
	};

	const sizes = {
		sm: 'px-3 py-1.5 text-sm',
		md: 'px-4 py-2',
		lg: 'px-6 py-3 text-lg'
	};

	function handleClick(event: MouseEvent) {
		if (!disabled && onclick) {
			onclick(event);
		}
	}
</script>

<button
	{type}
	{disabled}
	{tabindex}
	{title}
	onclick={handleClick}
	class={cn('btn', variants[variant], sizes[size], className)}
>
	{#if icon}
		<span class="inline-flex items-center mr-1.5">
			{@render icon()}
		</span>
	{/if}
	{#if children}
		{@render children()}
	{/if}
</button>
