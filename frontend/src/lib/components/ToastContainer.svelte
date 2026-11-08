<script lang="ts">
	import { toastStore } from '$lib/stores/toast';
	import Toast from './Toast.svelte';
</script>

{#if $toastStore.toasts.length > 0}
	<div class="toast-container" aria-live="polite">
		{#each $toastStore.toasts as toast (toast.id)}
			<Toast
				message={toast.message}
				type={toast.type}
				onclose={() => toastStore.removeToast(toast.id)}
			/>
		{/each}
	</div>
{/if}

<style>
	.toast-container {
		@apply fixed bottom-4 right-4;
		/* above modals (z-50) and floating menus (1000): a message about a modal card shows over it */
		z-index: 1100;
		@apply flex flex-col gap-2;
	}
</style>
