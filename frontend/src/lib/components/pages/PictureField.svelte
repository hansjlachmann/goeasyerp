<script lang="ts">
	// A picture field (a BLOB such as Item.Picture) on a card: shows the stored picture and lets
	// the user import an image (the server checks and re-encodes it), export or delete it.
	// The record's JSON only says whether there is a picture; the image has its own endpoint.
	import { withTabCompanyParam } from '#lib/utils/tabCompany.js';
	import { t, PICTURE } from '#lib/services/i18n.svelte.js';
	import { confirm } from '#lib/stores/confirm.js';
	import { toast } from '#lib/stores/toast.js';

	interface Props {
		table: string;
		field: string;
		recordId: string;
		caption?: string;
		hasPicture: boolean;
		editable?: boolean;
		onchange?: (hasPicture: boolean) => void;
	}

	let { table, field, recordId, caption, hasPicture, editable = false, onchange }: Props = $props();

	let version = $state(0);
	let fileInput: HTMLInputElement | null = $state(null);
	// a record with a blank key (a setup table) has no key segment, like the other record URLs
	const url = $derived(
		`/api/tables/${encodeURIComponent(table)}/blob/${encodeURIComponent(field)}` + (recordId === '' ? '' : `/${encodeURIComponent(recordId)}`)
	);

	async function importPicture(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		const response = await fetch(url, { method: 'PUT', body: file, headers: { 'Content-Type': 'application/octet-stream' } });
		const result = await response.json().catch(() => ({}));
		if (!response.ok || result.success === false) {
			toast.error(result.error ?? response.statusText);
			return;
		}
		version++;
		onchange?.(true);
	}

	function deletePicture() {
		confirm.show(t(PICTURE.DELETE_TITLE), t(PICTURE.DELETE_CONFIRM), async () => {
			const response = await fetch(url, { method: 'DELETE' });
			const result = await response.json().catch(() => ({}));
			if (!response.ok || result.success === false) {
				toast.error(result.error ?? response.statusText);
				return;
			}
			version++;
			onchange?.(false);
		});
	}
</script>

<div class="picture-field">
	{#if caption}<span class="picture-caption">{caption}</span>{/if}
	<div class="picture-box">
		{#if hasPicture}
			<img src={withTabCompanyParam(`${url}?v=${version}`)} alt={caption ?? field} />
		{:else}
			<span class="picture-none">{t(PICTURE.NONE)}</span>
		{/if}
	</div>
	<div class="picture-actions">
		{#if editable}
			<button type="button" class="picture-button" onclick={() => fileInput?.click()}>{t(PICTURE.IMPORT)}</button>
			<input bind:this={fileInput} type="file" accept="image/png,image/jpeg,image/webp" class="picture-file" onchange={importPicture} />
		{/if}
		{#if hasPicture}
			<a class="picture-button" href={withTabCompanyParam(url)} download={`${recordId}`}>{t(PICTURE.EXPORT)}</a>
			{#if editable}
				<button type="button" class="picture-button" onclick={deletePicture}>{t(PICTURE.DELETE)}</button>
			{/if}
		{/if}
	</div>
</div>

<style>
	.picture-field {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
		grid-column: 1 / -1;
	}
	.picture-caption {
		font-size: 0.8125rem;
		color: rgb(var(--gray-600));
	}
	:global(.dark) .picture-caption {
		color: rgb(var(--gray-400));
	}
	.picture-box {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20rem;
		max-width: 100%;
		min-height: 10rem;
		border: 1px solid rgb(var(--gray-300));
		border-radius: 0.375rem;
		overflow: hidden;
	}
	:global(.dark) .picture-box {
		border-color: rgb(var(--gray-700));
	}
	.picture-box img {
		max-width: 100%;
		max-height: 20rem;
		object-fit: contain;
	}
	.picture-none {
		color: rgb(var(--gray-500));
		font-size: 0.875rem;
	}
	.picture-actions {
		display: flex;
		gap: 0.5rem;
	}
	.picture-button {
		font-size: 0.875rem;
		color: rgb(var(--accent-600));
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		text-decoration: none;
	}
	:global(.dark) .picture-button {
		color: rgb(var(--accent-400));
	}
	.picture-button:hover {
		text-decoration: underline;
	}
	.picture-file {
		display: none;
	}
</style>
