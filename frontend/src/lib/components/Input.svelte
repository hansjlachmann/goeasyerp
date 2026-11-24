<script lang="ts">
	import { cn } from '#lib/utils/cn.js';

	export let value: string = '';
	export let type: 'text' | 'email' | 'password' | 'number' = 'text';
	export let placeholder: string = '';
	export let disabled: boolean = false;
	export let error: string | undefined = undefined;
	export let label: string | undefined = undefined;
	export let id: string | undefined = undefined;
</script>

<div class={cn('space-y-1', $$props.class)}>
	{#if label}
		<label for={id} class="block text-sm font-medium text-gray-700">
			{label}
		</label>
	{/if}

	<input
		{id}
		{type}
		{placeholder}
		{disabled}
		bind:value
		class={cn('input', error && 'input-error')}
		on:input
		on:change
		on:blur
		on:focus
	/>

	{#if error}
		<p class="text-sm text-red-600">{error}</p>
	{/if}
</div>
