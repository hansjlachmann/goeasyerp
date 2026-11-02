<script lang="ts">
	// The preview of a printed report (Preview on its request page): the PDF in the browser's
	// viewer over the page, with Print (the browser's print dialog) and PDF (download); Esc or
	// Close closes it.
	import Modal from './Modal.svelte';
	import Button from './Button.svelte';
	import { t, BTN, DLG } from '$lib/services/i18n.svelte';
	import { printPreview, printPdf, downloadPdf, closePreview } from '$lib/utils/printout';

	const preview = $derived($printPreview);

	// the next preview waiting (a document printed with several reports) comes up
	function close() {
		closePreview();
	}
</script>

{#if preview}
	<Modal open={true} size="expanded" onclose={close}>
		<div class="print-preview">
			<div class="preview-bar">
				<h3 class="preview-title">{t(DLG.PRINT_PREVIEW)} – {preview.filename}</h3>
				<div class="preview-actions">
					<Button variant="primary" onclick={() => printPdf(preview.blob)}>{t(BTN.PRINT)}</Button>
					<Button variant="secondary" onclick={() => downloadPdf(preview.blob, preview.filename)}>{t(BTN.PDF)}</Button>
					<Button variant="secondary" onclick={close}>{t(BTN.CLOSE)}</Button>
				</div>
			</div>
			<iframe class="preview-frame" src={preview.url} title={preview.filename}></iframe>
		</div>
	</Modal>
{/if}

<style>
	.print-preview {
		display: flex;
		flex-direction: column;
		height: 85vh;
	}
	.preview-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.5rem 1rem;
		border-bottom: 1px solid rgb(var(--gray-200));
	}
	:global(.dark) .preview-bar {
		border-bottom-color: rgb(var(--gray-700));
	}
	.preview-title {
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.preview-actions {
		display: flex;
		gap: 0.5rem;
	}
	.preview-frame {
		flex: 1;
		width: 100%;
		border: 0;
		background: rgb(var(--gray-100));
	}
</style>
