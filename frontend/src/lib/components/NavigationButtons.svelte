<script lang="ts">
	/**
	 * Edge navigation buttons component (classic ERP style)
	 * Displays previous/next buttons on the left and right edges of a card
	 */
	interface Props {
		onPrevious?: () => void;
		onNext?: () => void;
		canNavigatePrevious?: boolean;
		canNavigateNext?: boolean;
	}

	let {
		onPrevious,
		onNext,
		canNavigatePrevious = false,
		canNavigateNext = false
	}: Props = $props();
</script>

<!-- Previous Button (Left Edge); not Tab stops — the keyboard has Ctrl+Shift+↑/↓ -->
<button
	class="edge-nav-btn edge-nav-left"
	tabindex="-1"
	onclick={onPrevious}
	disabled={!canNavigatePrevious}
	title="Previous Record (Ctrl+Up)"
	aria-label="Previous Record"
>
	<svg
		xmlns="http://www.w3.org/2000/svg"
		class="h-6 w-6"
		fill="none"
		viewBox="0 0 24 24"
		stroke="currentColor"
	>
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
	</svg>
</button>

<!-- Next Button (Right Edge) -->
<button
	class="edge-nav-btn edge-nav-right"
	tabindex="-1"
	onclick={onNext}
	disabled={!canNavigateNext}
	title="Next Record (Ctrl+Down)"
	aria-label="Next Record"
>
	<svg
		xmlns="http://www.w3.org/2000/svg"
		class="h-6 w-6"
		fill="none"
		viewBox="0 0 24 24"
		stroke="currentColor"
	>
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
	</svg>
</button>

<style>
	:global(.edge-nav-btn) {
		@apply absolute top-1/2 -translate-y-1/2 z-50;
		@apply w-9 h-9 rounded-full;
		@apply bg-gray-700 dark:bg-gray-600;
		@apply text-white;
		@apply flex items-center justify-center;
		@apply shadow-lg;
		@apply transition-all duration-200;
		@apply hover:bg-gray-600 dark:hover:bg-gray-500;
		@apply hover:scale-110;
		@apply focus:outline-none focus:ring-2 focus:ring-blue-500;
	}

	:global(.edge-nav-btn:disabled) {
		@apply opacity-30 cursor-not-allowed;
		@apply hover:scale-100 hover:bg-gray-700 dark:hover:bg-gray-600;
	}

	:global(.edge-nav-left) {
		left: 4px;
	}

	:global(.edge-nav-right) {
		right: 4px;
	}
</style>
