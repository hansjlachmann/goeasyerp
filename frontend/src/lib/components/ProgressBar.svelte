<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	interface Props {
		/** If true, simulates progress automatically */
		simulated?: boolean;
		/** Manual progress value (0-100) when not simulated */
		value?: number;
		/** Maximum value for simulated progress (default 95) */
		simulatedMax?: number;
		/** Duration in ms to reach simulatedMax (default 8000) */
		duration?: number;
		/** Show percentage text */
		showPercent?: boolean;
		/** Size variant */
		size?: 'sm' | 'md';
	}

	let {
		simulated = true,
		value = 0,
		simulatedMax = 95,
		duration = 8000,
		showPercent = true,
		size = 'sm'
	}: Props = $props();

	let progress = $state(0);
	let animationFrame: number | null = null;
	let startTime: number | null = null;

	// Easing function - starts fast, slows down as it approaches max
	function easeOutExpo(t: number): number {
		return t === 1 ? 1 : 1 - Math.pow(2, -10 * t);
	}

	function animate(currentTime: number) {
		if (!startTime) startTime = currentTime;

		const elapsed = currentTime - startTime;
		const t = Math.min(elapsed / duration, 1);

		// Apply easing and scale to simulatedMax
		progress = easeOutExpo(t) * simulatedMax;

		if (t < 1) {
			animationFrame = requestAnimationFrame(animate);
		}
	}

	onMount(() => {
		if (simulated) {
			animationFrame = requestAnimationFrame(animate);
		}
	});

	onDestroy(() => {
		if (animationFrame) {
			cancelAnimationFrame(animationFrame);
		}
	});

	// Use manual value when not simulated
	$effect(() => {
		if (!simulated) {
			progress = value;
		}
	});

	const displayPercent = $derived(Math.round(progress));
	const heightClass = $derived(size === 'sm' ? 'h-1.5' : 'h-2.5');
</script>

<div class="progress-container">
	<div class="progress-bar {heightClass}">
		<div
			class="progress-fill {heightClass}"
			style="width: {progress}%"
		></div>
	</div>
	{#if showPercent}
		<span class="progress-text">{displayPercent}%</span>
	{/if}
</div>

<style>
	@reference '../../app.css';
	.progress-container {
		@apply flex items-center gap-2;
	}

	.progress-bar {
		@apply flex-1 bg-gray-200 rounded-full overflow-hidden;
		min-width: 60px;
		max-width: 120px;
	}

	:global(.dark) .progress-bar {
		background-color: rgb(var(--gray-700));
	}

	.progress-fill {
		@apply bg-blue-500 rounded-full;
		transition: width 0.1s ease-out;
	}

	:global(.dark) .progress-fill {
		background-color: rgb(var(--accent-500));
	}

	.progress-text {
		@apply text-xs text-gray-500 font-medium tabular-nums;
		min-width: 32px;
	}

	:global(.dark) .progress-text {
		color: rgb(var(--gray-400));
	}
</style>
