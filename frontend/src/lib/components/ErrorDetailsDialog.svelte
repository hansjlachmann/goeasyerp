<script lang="ts">
	// What lies behind an error (Show Entries on the error dialog): the sections the server sent —
	// e.g. the totals per posting date and document (those that do not balance marked) and the
	// G/L entries of a posting that does not balance —, read-only, with Export (CSV) and Close.
	import Modal from '$components/Modal.svelte';
	import Button from '$components/Button.svelte';
	import { t, BTN } from '$lib/services/i18n.svelte';
	import type { ErrorDetails } from '$lib/services/jobs';
	import { downloadErrorDetails } from '$lib/utils/errorDetails';

	interface Props {
		details: ErrorDetails;
		onclose: () => void;
	}
	let { details, onclose }: Props = $props();
</script>

<Modal open={true} {onclose} title={details.title} size="expanded">
	<div class="error-details" data-error-details>
		<div class="dialog-header">
			<h2 id="modal-title" class="dialog-title">{details.title}</h2>
			<button type="button" class="close-btn" onclick={onclose} aria-label={t(BTN.CLOSE)}>×</button>
		</div>
		{#each details.sections as section, s (s)}
			<section class="details-section">
				<h3 class="section-caption">{section.caption}</h3>
				<div class="table-wrap">
					<table>
						<thead>
							<tr>
								{#each section.columns as column, c (c)}
									<th class:numeric={column.numeric}>{column.caption}</th>
								{/each}
							</tr>
						</thead>
						<tbody>
							{#each section.rows as row, r (r)}
								<tr class:marked={section.marked?.includes(r)}>
									{#each row as value, c (c)}
										<td class:numeric={section.columns[c]?.numeric}>{value}</td>
									{/each}
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>
		{/each}
		<div class="dialog-actions">
			<Button variant="secondary" onclick={() => downloadErrorDetails(details, `${details.title}.csv`)}>{t(BTN.EXPORT)}</Button>
			<Button variant="primary" onclick={onclose}>{t(BTN.CLOSE)}</Button>
		</div>
	</div>
</Modal>

<style>
	.error-details {
		@apply p-6 flex flex-col gap-4;
	}
	.dialog-header {
		@apply flex items-center justify-between;
	}
	.dialog-title {
		@apply text-xl font-bold;
		color: rgb(var(--heading));
	}
	:global(.dark) .dialog-title {
		color: rgb(var(--accent-400));
	}
	.close-btn {
		@apply text-2xl leading-none px-2;
		color: rgb(var(--gray-500));
	}
	.section-caption {
		@apply text-sm font-semibold mb-1;
		color: rgb(var(--gray-700));
	}
	:global(.dark) .section-caption {
		color: rgb(var(--gray-300));
	}
	.table-wrap {
		max-height: 18rem;
		overflow: auto;
		border: 1px solid rgb(var(--gray-300));
		border-radius: 0.375rem;
	}
	:global(.dark) .table-wrap {
		border-color: rgb(var(--gray-600));
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.85rem;
	}
	th {
		position: sticky;
		top: 0;
		background: rgb(var(--gray-100));
		text-align: left;
		font-weight: 600;
		padding: 0.35rem 0.6rem;
		white-space: nowrap;
	}
	:global(.dark) th {
		background: rgb(var(--gray-800));
	}
	td {
		padding: 0.3rem 0.6rem;
		border-top: 1px solid rgb(var(--gray-200));
		min-width: 5rem;
	}
	:global(.dark) td {
		border-top-color: rgb(var(--gray-700));
	}
	.numeric {
		text-align: right;
		font-family: var(--font-mono); /* amounts: IBM Plex Mono */
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	tr.marked td {
		background: rgb(239 68 68 / 0.12);
		font-weight: 600;
	}
	.dialog-actions {
		@apply flex justify-end gap-2;
	}
</style>
