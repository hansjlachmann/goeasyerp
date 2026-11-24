<script lang="ts">
	import Skeleton from '#lib/components/Skeleton.svelte';
	import Spinner from '#lib/components/Spinner.svelte';
	import ProgressBar from '#lib/components/ProgressBar.svelte';

	interface Props {
		rows?: number;
		columns?: number;
	}

	let { rows = 8, columns = 5 }: Props = $props();
</script>

<div class="skeleton-page">
	<!-- Header skeleton -->
	<div class="skeleton-header">
		<div class="flex items-center gap-4">
			<Skeleton width="200px" height="2rem" />
			<div class="flex gap-2">
				<Skeleton width="70px" height="2rem" rounded="md" />
				<Skeleton width="70px" height="2rem" rounded="md" />
				<Skeleton width="70px" height="2rem" rounded="md" />
				<Skeleton width="80px" height="2rem" rounded="md" />
			</div>
		</div>
		<div class="flex items-center gap-3">
			<Spinner size="sm" />
			<ProgressBar simulated={true} showPercent={true} />
		</div>
	</div>

	<!-- Table skeleton -->
	<div class="skeleton-table">
		<!-- Table header -->
		<div class="skeleton-thead">
			{#each Array(columns) as _, i}
				<div class="skeleton-th" style="width: {100 / columns}%">
					<Skeleton width="80%" height="1rem" />
				</div>
			{/each}
		</div>

		<!-- Table rows -->
		<div class="skeleton-tbody">
			{#each Array(rows) as _, rowIndex}
				<div class="skeleton-tr" class:even={rowIndex % 2 === 1}>
					{#each Array(columns) as _, colIndex}
						<div class="skeleton-td" style="width: {100 / columns}%">
							<Skeleton
								width="{60 + Math.random() * 30}%"
								height="1rem"
							/>
						</div>
					{/each}
				</div>
			{/each}
		</div>
	</div>

	<!-- Status bar skeleton -->
	<div class="skeleton-status">
		<Skeleton width="120px" height="1rem" />
	</div>
</div>

<style>
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

	.skeleton-table {
		@apply flex-1 border border-gray-200 rounded-lg overflow-hidden;
	}

	:global(.dark) .skeleton-table {
		border-color: rgb(var(--gray-700));
	}

	.skeleton-thead {
		@apply flex bg-surface px-4 py-3 border-b border-gray-300;
	}

	:global(.dark) .skeleton-thead {
		background-color: rgb(var(--gray-800));
		border-color: rgb(var(--gray-700));
	}

	.skeleton-th {
		@apply px-2;
	}

	.skeleton-tbody {
		@apply bg-surface;
	}

	:global(.dark) .skeleton-tbody {
		background-color: rgb(var(--gray-900));
	}

	.skeleton-tr {
		@apply flex px-4 py-2 border-b border-gray-200;
	}

	:global(.dark) .skeleton-tr {
		border-color: rgb(var(--gray-700));
	}

	.skeleton-tr.even {
		@apply bg-gray-50;
	}

	:global(.dark) .skeleton-tr.even {
		background-color: rgba(30, 30, 30, 0.5);
	}

	.skeleton-td {
		@apply px-2 py-1;
	}

	.skeleton-status {
		@apply px-4 py-2 bg-gray-50 border-t border-gray-200 rounded-b;
	}

	:global(.dark) .skeleton-status {
		background-color: rgb(var(--gray-800));
		border-color: rgb(var(--gray-700));
	}
</style>
