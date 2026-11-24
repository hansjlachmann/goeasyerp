<script lang="ts">
	interface Props {
		width?: string;
		height?: string;
		rounded?: 'none' | 'sm' | 'md' | 'lg' | 'full';
		class?: string;
	}

	let {
		width = '100%',
		height = '1rem',
		rounded = 'md',
		class: className = ''
	}: Props = $props();

	const roundedClasses = {
		none: '',
		sm: 'rounded-xs',
		md: 'rounded-sm',
		lg: 'rounded-lg',
		full: 'rounded-full'
	};
</script>

<div
	class="skeleton {roundedClasses[rounded]} {className}"
	style="width: {width}; height: {height};"
></div>

<style>
	.skeleton {
		background: linear-gradient(
			90deg,
			rgb(var(--gray-200)) 0%,
			rgb(var(--gray-100)) 50%,
			rgb(var(--gray-200)) 100%
		);
		background-size: 200% 100%;
		animation: shimmer 1.5s infinite;
	}

	:global(.dark) .skeleton {
		background: linear-gradient(
			90deg,
			rgb(var(--gray-700)) 0%,
			rgb(var(--gray-600)) 50%,
			rgb(var(--gray-700)) 100%
		);
		background-size: 200% 100%;
	}

	@keyframes shimmer {
		0% {
			background-position: 200% 0;
		}
		100% {
			background-position: -200% 0;
		}
	}
</style>
