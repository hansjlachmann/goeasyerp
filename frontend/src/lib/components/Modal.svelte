<script lang="ts" module>
	// The open modals, the last one on top: Esc closes the top one only, and the pages under
	// a modal ignore the keyboard (isModalOpen) — unless they are inside one (RunModal)
	const openModals: { id: symbol; content: () => HTMLElement | null }[] = [];

	/** A modal (message box, confirmation, input dialog, modal card …) is open */
	export function isModalOpen(): boolean {
		return openModals.length > 0;
	}

	/** The element is in the top modal (a page in it has the keyboard, the pages under it not) */
	export function isInTopModal(element: Element | null | undefined): boolean {
		const top = openModals[openModals.length - 1]?.content();
		return !!element && !!top && top.contains(element);
	}
</script>

<script lang="ts">
	import { tick } from 'svelte';
	import { portal } from '$lib/actions/portal';
	import type { Snippet } from 'svelte';

	interface Props {
		open?: boolean;
		onclose?: () => void;
		title?: string;
		size?: 'normal' | 'expanded' | 'fullscreen';
		// Ctrl+Shift+F12 on the top modal (a modal card's full screen button)
		ontogglefullscreen?: () => void;
		// false: Esc is the page's in it (a page frame: the page closes itself, after its own
		// uses of Esc — an edit, the filter pane)
		closeOnEscape?: boolean;
		children: Snippet;
	}

	let { open = false, onclose, title, size = 'normal', ontogglefullscreen, closeOnEscape = true, children }: Props = $props();

	// Reference to modal content for focus management
	let modalContentRef = $state<HTMLDivElement | null>(null);

	// Esc closes the top modal — after the controls in it had the key (an open dropdown, a
	// cell being edited, a filter field take Esc first and prevent its default). A modal
	// without onclose (a running job) keeps Esc from the page under it. Ctrl+Shift+F12 toggles
	// full screen of the top modal when it has one (modal cards).
	const id = Symbol('modal');
	$effect(() => {
		if (!open) return;
		openModals.push({ id, content: () => modalContentRef });
		function handleKey(event: KeyboardEvent) {
			if (openModals[openModals.length - 1]?.id !== id) return;
			if (event.key === 'F12' && event.ctrlKey && event.shiftKey && ontogglefullscreen) {
				event.preventDefault();
				ontogglefullscreen();
				return;
			}
			if (event.key !== 'Escape' || !closeOnEscape) return;
			if (event.defaultPrevented) return;
			event.preventDefault();
			onclose?.();
		}
		document.addEventListener('keydown', handleKey);
		return () => {
			document.removeEventListener('keydown', handleKey);
			const i = openModals.findIndex((m) => m.id === id);
			if (i >= 0) openModals.splice(i, 1);
		};
	});

	// Prevent background scroll when modal is open and manage focus
	$effect(() => {
		if (open) {
			document.body.style.overflow = 'hidden';
			// Focus the modal after it renders
			tick().then(() => {
				if (modalContentRef) {
					// Find the first focusable element inside the modal
					const focusable = modalContentRef.querySelector<HTMLElement>(
						'input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"])'
					);
					if (focusable) {
						focusable.focus();
					} else {
						// If no focusable element, focus the modal itself
						modalContentRef.focus();
					}
				}
			});
		} else {
			document.body.style.overflow = '';
		}

		return () => {
			document.body.style.overflow = '';
		};
	});

	// Size classes
	const sizeClasses = {
		normal: 'max-w-2xl',
		expanded: 'max-w-6xl',
		fullscreen: 'max-w-full h-full m-0'
	};
</script>

{#if open}
	<!-- Drawn at the app root, above the whole page (portal) -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" use:portal onclick={onclose} role="presentation">
		<div
			class="modal-content {sizeClasses[size]}"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby={title ? 'modal-title' : undefined}
			tabindex="-1"
			bind:this={modalContentRef}
		>
			{@render children()}
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		@apply fixed inset-0 z-50 flex items-center justify-center;
		@apply bg-black/50 backdrop-blur-sm;
		@apply p-4;
	}

	.modal-content {
		@apply bg-surface rounded-lg shadow-2xl;
		@apply w-full max-h-[90vh] overflow-hidden;
		@apply flex flex-col;
		animation: modalFadeIn 0.2s ease-out;
	}

	:global(.dark) .modal-content {
		background-color: rgb(var(--gray-800)); /* gray-800 */
	}

	@keyframes modalFadeIn {
		from {
			opacity: 0;
			transform: scale(0.95);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}
</style>
