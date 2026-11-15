<script lang="ts">
	// My Settings (user menu): the user's date format, number format and work date. Each change
	// is saved at once (the backend keeps them per user; the work date goes back to today at the
	// next login) and the app shows dates and numbers with them.
	import Modal from '$components/Modal.svelte';
	import DateInput from '$components/DateInput.svelte';
	import { t, MYSET, BTN } from '$lib/services/i18n.svelte';
	import { toast } from '$lib/stores/toast';
	import { regional, saveRegional } from '$lib/stores/regional';
	import { currentLanguage } from '$lib/stores/session';
	import { formatDateRegional, formatNumberText } from '$lib/utils/regional';
	import type { MySettings } from '$lib/services/api';

	interface Props {
		onclose: () => void;
	}
	let { onclose }: Props = $props();

	/** The formats offered ('' = the language's) */
	const dateFormats = ['', 'dd.mm.yy', 'dd.mm.yyyy', 'dd/mm/yyyy', 'mm/dd/yyyy', 'yyyy-mm-dd'];
	const numberFormats = ['', '1,234.56', '1.234,56', '1 234,56', '1234.56'];

	const settings = $derived($regional);
	let busy = $state(false);

	async function save(patch: Partial<MySettings>) {
		busy = true;
		try {
			await saveRegional(patch);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : String(err));
		} finally {
			busy = false;
		}
	}

	// the preview: a date and an amount as they are shown now
	const previewDate = $derived.by(() => {
		void settings; // again when the settings change
		return formatDateRegional('2026-09-05', $currentLanguage || 'en-US');
	});
	const previewNumber = $derived.by(() => {
		void settings;
		return formatNumberText('-1234567.89', $currentLanguage || 'en-US');
	});
	const isToday = $derived(!settings.work_date || settings.work_date === settings.today);
</script>

<Modal open={true} {onclose} title={t(MYSET.TITLE)}>
	<div class="my-settings" data-my-settings>
		<div class="dialog-header">
			<h2 id="modal-title" class="dialog-title">{t(MYSET.TITLE)}</h2>
			<button type="button" class="close-btn" onclick={onclose} aria-label={t(BTN.CLOSE)}>×</button>
		</div>

		<div class="settings-grid">
			<label for="my-date-format">{t(MYSET.DATE_FORMAT)}</label>
			<select id="my-date-format" value={settings.date_format} disabled={busy} onchange={(e) => save({ date_format: e.currentTarget.value })}>
				{#each dateFormats as f (f)}
					<option value={f}>{f === '' ? t(MYSET.LANGUAGE_DEFAULT) : f.toUpperCase()}</option>
				{/each}
			</select>

			<label for="my-number-format">{t(MYSET.NUMBER_FORMAT)}</label>
			<select id="my-number-format" value={settings.number_format} disabled={busy} onchange={(e) => save({ number_format: e.currentTarget.value })}>
				{#each numberFormats as f (f)}
					<option value={f}>{f === '' ? t(MYSET.LANGUAGE_DEFAULT) : f}</option>
				{/each}
			</select>

			<label for="my-work-date">{t(MYSET.WORK_DATE)}</label>
			<div class="work-date-row">
				<DateInput id="my-work-date" class="work-date-input" value={settings.work_date || settings.today} onchange={(iso) => save({ work_date: iso === settings.today ? '' : iso })} />
				<button type="button" class="today-btn" disabled={busy || isToday} onclick={() => save({ work_date: '' })}>{t(MYSET.TODAY)}</button>
			</div>
		</div>
		<p class="hint">{t(MYSET.WORK_DATE_HINT)}</p>

		<div class="preview">
			<span class="preview-title">{t(MYSET.PREVIEW)}</span>
			<span data-preview-date>{previewDate}</span>
			<span data-preview-number>{previewNumber}</span>
		</div>
		<p class="hint">{t(MYSET.ENTRY_HINT)}</p>
	</div>
</Modal>

<style>
	.my-settings {
		@apply p-6 flex flex-col gap-3;
	}
	.dialog-header {
		@apply flex items-center justify-between mb-2;
	}
	.dialog-title {
		@apply text-xl font-bold text-heading;
	}

	:global(.dark) .dialog-title {
		@apply text-blue-400;
	}
	.close-btn {
		@apply text-2xl leading-none px-2 text-gray-500 hover:text-gray-900;
	}

	:global(.dark) .close-btn {
		@apply text-gray-400 hover:text-gray-50;
	}
	.settings-grid {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 0.6rem 1rem;
		align-items: center;
	}
	.settings-grid label {
		@apply text-sm font-medium text-gray-700;
	}

	:global(.dark) .settings-grid label {
		@apply text-gray-300;
	}
	.settings-grid select,
	.my-settings :global(.work-date-input) {
		@apply w-full py-1.5 px-2 bg-surface border border-gray-300 rounded-md text-fg;
	}

	:global(.dark) .my-settings :global(.work-date-input) {
		@apply border-gray-600;
	}
	.work-date-row {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}
	.today-btn {
		@apply px-3 py-1.5 text-sm rounded-md border border-gray-300 hover:bg-gray-100 disabled:opacity-50;
	}

	:global(.dark) .today-btn {
		@apply border-gray-600 hover:bg-gray-700;
	}
	.preview {
		@apply flex gap-4 text-sm px-3 py-2 rounded-md bg-gray-100;
	}

	:global(.dark) .preview {
		@apply bg-gray-700;
	}
	.preview-title {
		@apply font-semibold;
	}
	.hint {
		@apply text-xs text-gray-600;
	}

	:global(.dark) .hint {
		@apply text-gray-400;
	}
</style>
