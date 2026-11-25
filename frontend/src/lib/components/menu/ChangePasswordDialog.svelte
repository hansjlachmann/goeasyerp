<script lang="ts">
	// Change Password (user menu, S38): the current password, the new one twice. The server
	// checks the rules (at least 10 characters, not a common password, not the user ID) and ends
	// the user's other sessions; this browser stays logged in.
	import Modal from '#lib/components/Modal.svelte';
	import { t, PWD, MENU, BTN, ERR } from '#lib/services/i18n.svelte.js';
	import { api } from '#lib/services/api.js';
	import { toast } from '#lib/stores/toast.js';

	interface Props {
		onclose: () => void;
	}
	let { onclose }: Props = $props();

	let current = $state('');
	let next = $state('');
	let confirmNext = $state('');
	let error = $state('');
	let busy = $state(false);

	async function change(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		if (next !== confirmNext) {
			error = t(ERR.PASSWORD_MISMATCH);
			return;
		}
		if (next.length < 10) {
			error = t(ERR.PASSWORD_TOO_SHORT);
			return;
		}
		busy = true;
		try {
			const response = await api.changePassword(current, next);
			if (response.success) {
				toast.success(t(PWD.CHANGED));
				onclose();
			} else {
				error = response.error || t(ERR.GENERIC);
			}
		} catch (err: any) {
			// the API client throws {status, message}
			error = err?.message || t(ERR.GENERIC);
		} finally {
			busy = false;
		}
	}
</script>

<Modal open={true} {onclose} title={t(MENU.CHANGE_PASSWORD)}>
	<form class="change-password" data-change-password onsubmit={change}>
		<div class="dialog-header">
			<h2 id="modal-title" class="dialog-title">{t(MENU.CHANGE_PASSWORD)}</h2>
			<button type="button" class="close-btn" onclick={onclose} aria-label={t(BTN.CLOSE)}>×</button>
		</div>

		<label for="pwd-current">{t(PWD.CURRENT)}</label>
		<!-- svelte-ignore a11y_autofocus -->
		<input id="pwd-current" type="password" autocomplete="current-password" bind:value={current} disabled={busy} autofocus />

		<label for="pwd-new">{t(PWD.NEW)}</label>
		<input id="pwd-new" type="password" autocomplete="new-password" bind:value={next} disabled={busy} />

		<label for="pwd-confirm">{t(PWD.CONFIRM)}</label>
		<input id="pwd-confirm" type="password" autocomplete="new-password" bind:value={confirmNext} disabled={busy} />

		<p class="hint">{t(PWD.RULES)}</p>
		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="buttons">
			<button type="button" class="btn-secondary" onclick={onclose} disabled={busy}>{t(BTN.CANCEL)}</button>
			<button type="submit" class="btn-primary" disabled={busy || !current || !next || !confirmNext}>{t(PWD.CHANGE)}</button>
		</div>
	</form>
</Modal>

<style>
	@reference '../../../app.css';
	.change-password {
		@apply p-6 flex flex-col gap-2 min-w-80;
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
	label {
		@apply text-sm font-medium text-gray-700 mt-1;
	}
	:global(.dark) label {
		@apply text-gray-300;
	}
	input {
		@apply w-full py-1.5 px-2 bg-surface border border-gray-300 rounded-md text-fg;
	}
	:global(.dark) input {
		@apply border-gray-600;
	}
	.hint {
		@apply text-xs text-gray-500 mt-1;
	}
	.error {
		@apply text-sm text-red-600;
	}
	:global(.dark) .error {
		@apply text-red-400;
	}
	.buttons {
		@apply flex justify-end gap-2 mt-3;
	}
</style>
