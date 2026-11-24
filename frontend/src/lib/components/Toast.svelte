<script lang="ts">
	import type { ToastType } from '#lib/stores/toast.js';

	interface Props {
		message: string;
		type: ToastType;
		onclose: () => void;
	}

	let { message, type, onclose }: Props = $props();

	const icons = {
		success: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />`,
		error: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />`,
		warning: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />`,
		info: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />`
	};
</script>

<div class="toast toast-{type}" role="alert">
	<div class="toast-icon">
		<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
			{@html icons[type]}
		</svg>
	</div>
	<p class="toast-message">{message}</p>
	<button type="button" class="toast-close" tabindex="-1" onclick={onclose} aria-label="Close">
		<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
		</svg>
	</button>
</div>

<style>
	.toast {
		@apply flex items-center gap-3 px-4 py-3 rounded-lg shadow-lg;
		@apply border min-w-[300px] max-w-[450px];
		animation: slideIn 0.3s ease-out;
	}

	@keyframes slideIn {
		from {
			transform: translateX(100%);
			opacity: 0;
		}
		to {
			transform: translateX(0);
			opacity: 1;
		}
	}

	.toast-success {
		@apply bg-green-50 border-green-200 text-green-800;
	}

	:global(.dark) .toast-success {
		background-color: rgba(34, 197, 94, 0.15);
		border-color: rgba(34, 197, 94, 0.3);
		color: #86efac;
	}

	.toast-error {
		@apply bg-red-50 border-red-200 text-red-800;
	}

	:global(.dark) .toast-error {
		background-color: rgba(239, 68, 68, 0.15);
		border-color: rgba(239, 68, 68, 0.3);
		color: #fca5a5;
	}

	.toast-warning {
		@apply bg-yellow-50 border-yellow-200 text-yellow-800;
	}

	:global(.dark) .toast-warning {
		background-color: rgba(234, 179, 8, 0.15);
		border-color: rgba(234, 179, 8, 0.3);
		color: #fde047;
	}

	.toast-info {
		@apply bg-blue-50 border-blue-200 text-blue-800;
	}

	:global(.dark) .toast-info {
		background-color: rgba(0, 131, 143, 0.15);
		border-color: rgba(0, 131, 143, 0.3);
		color: rgb(var(--accent-300));
	}

	.toast-icon {
		@apply flex-shrink-0;
	}

	.toast-icon svg {
		@apply w-5 h-5;
	}

	.toast-success .toast-icon {
		@apply text-green-500;
	}

	.toast-error .toast-icon {
		@apply text-red-500;
	}

	.toast-warning .toast-icon {
		@apply text-yellow-500;
	}

	.toast-info .toast-icon {
		@apply text-blue-500;
	}

	.toast-message {
		@apply flex-1 text-sm font-medium;
	}

	.toast-close {
		@apply flex-shrink-0 p-1 rounded hover:bg-black/10 transition-colors;
	}

	:global(.dark) .toast-close:hover {
		background-color: rgba(255, 255, 255, 0.1);
	}

	.toast-close svg {
		@apply w-4 h-4 opacity-60;
	}

	.toast-close:hover svg {
		@apply opacity-100;
	}
</style>
