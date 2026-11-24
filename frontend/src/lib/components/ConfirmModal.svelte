<script lang="ts">
	import Modal from './Modal.svelte';
	import { t, DLG, BTN } from '#lib/services/i18n.svelte.js';

	interface Props {
		open?: boolean;
		title?: string;
		message?: string;
		confirmText?: string;
		cancelText?: string;
		variant?: 'danger' | 'warning' | 'info';
		// the text to type before the button confirms (case-insensitive), e.g. a company's name
		typeToConfirm?: string;
		typeHint?: string; // what to say about it (default: DLG_DELETE_TYPE_KEY)
		onconfirm?: () => void;
		oncancel?: () => void;
	}

	// Default texts are looked up in the template, not as prop defaults: a prop default is
	// evaluated once at mount, possibly before the translations have loaded, and would then
	// show the raw key (e.g. "BTN_CANCEL") for good.
	let {
		open = false,
		title,
		message,
		confirmText,
		cancelText,
		variant = 'danger',
		typeToConfirm,
		typeHint,
		onconfirm,
		oncancel
	}: Props = $props();

	let typed = $state('');
	// a new question starts with an empty field
	$effect(() => {
		if (open) typed = '';
	});
	const confirmed = $derived(
		!typeToConfirm || typed.trim().toLowerCase() === typeToConfirm.trim().toLowerCase()
	);

	function confirmIfTyped() {
		if (confirmed) onconfirm?.();
	}

	function autofocus(node: HTMLInputElement) {
		node.focus();
	}

	const variantColors = {
		danger: 'bg-red-600 hover:bg-red-700',
		warning: 'bg-yellow-600 hover:bg-yellow-700',
		info: 'bg-blue-600 hover:bg-blue-700'
	};

	const variantIcons = {
		danger: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />`,
		warning: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />`,
		info: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />`
	};

	const iconColors = {
		danger: 'text-red-600',
		warning: 'text-yellow-600',
		info: 'text-blue-600'
	};
</script>

<Modal {open} onclose={oncancel}>
	<div class="p-6">
		<div class="flex items-start gap-4">
			<div class="flex-shrink-0 {iconColors[variant]}">
				<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					{@html variantIcons[variant]}
				</svg>
			</div>
			<div class="flex-1">
				<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					{title ?? t(DLG.CONFIRM_TITLE)}
				</h3>
				<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
					{message ?? t(DLG.CONFIRM_DEFAULT)}
				</p>
				{#if typeToConfirm}
					<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
						{typeHint ?? t(DLG.DELETE_TYPE_KEY, typeToConfirm)}
					</p>
					<input
						type="text"
						class="input mt-2 text-sm"
						autocomplete="off"
						spellcheck="false"
						bind:value={typed}
						use:autofocus
						onkeydown={(e) => {
							if (e.key === 'Enter') {
								e.preventDefault();
								confirmIfTyped();
							}
						}}
					/>
				{/if}
			</div>
		</div>
		<div class="mt-6 flex justify-end gap-3">
			<button
				type="button"
				class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-surface dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500"
				onclick={oncancel}
			>
				{cancelText ?? t(BTN.CANCEL)}
			</button>
			<button
				type="button"
				class="px-4 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed {variantColors[variant]}"
				disabled={!confirmed}
				onclick={confirmIfTyped}
			>
				{confirmText ?? t(BTN.CONFIRM)}
			</button>
		</div>
	</div>
</Modal>
