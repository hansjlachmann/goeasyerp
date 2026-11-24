<script lang="ts">
	import Skeleton from '#lib/components/Skeleton.svelte';
	import Spinner from '#lib/components/Spinner.svelte';
	import ProgressBar from '#lib/components/ProgressBar.svelte';

	interface Props {
		sections?: number;
		fieldsPerSection?: number;
	}

	let { sections = 3, fieldsPerSection = 4 }: Props = $props();
</script>

<div class="skeleton-page">
	<!-- Header skeleton -->
	<div class="skeleton-header">
		<div class="flex items-center gap-4">
			<Skeleton width="180px" height="2rem" />
			<div class="flex gap-2">
				<Skeleton width="70px" height="2rem" rounded="md" />
				<Skeleton width="70px" height="2rem" rounded="md" />
				<Skeleton width="70px" height="2rem" rounded="md" />
			</div>
		</div>
		<div class="flex items-center gap-3">
			<Spinner size="sm" />
			<ProgressBar simulated={true} showPercent={true} />
		</div>
	</div>

	<!-- Sections skeleton -->
	<div class="skeleton-sections">
		{#each Array(sections) as _, sectionIndex}
			<div class="skeleton-card">
				<!-- Card header -->
				<div class="skeleton-card-header">
					<Skeleton width="120px" height="1.5rem" />
				</div>

				<!-- Card fields -->
				<div class="skeleton-card-body">
					{#each Array(fieldsPerSection) as _, fieldIndex}
						<div class="skeleton-field">
							<Skeleton width="80px" height="0.875rem" class="field-label" />
							<Skeleton width="100%" height="2.25rem" rounded="md" />
						</div>
					{/each}
				</div>
			</div>
		{/each}
	</div>
</div>

<style>
	@reference '../../../app.css';
	.skeleton-page {
		@apply flex flex-col h-full gap-4;
	}

	.skeleton-header {
		@apply flex items-center justify-between px-6 py-4;
		@apply bg-surface border-b border-gray-200;
	}

	:global(.dark) .skeleton-header {
		background-color: rgb(var(--gray-800));
		border-color: rgb(var(--gray-700));
	}

	.skeleton-sections {
		@apply flex flex-col gap-4 overflow-y-auto flex-1 px-1;
	}

	.skeleton-card {
		@apply bg-surface border border-gray-200 rounded-lg overflow-hidden;
	}

	:global(.dark) .skeleton-card {
		background-color: rgb(var(--gray-800));
		border-color: rgb(var(--gray-700));
	}

	.skeleton-card-header {
		@apply px-4 py-3 border-b border-gray-200;
	}

	:global(.dark) .skeleton-card-header {
		border-color: rgb(var(--gray-700));
	}

	.skeleton-card-body {
		@apply grid grid-cols-1 md:grid-cols-2 gap-4 p-4;
	}

	.skeleton-field {
		@apply flex flex-col gap-1.5;
	}

	.skeleton-field :global(.field-label) {
		@apply mb-1;
	}
</style>
