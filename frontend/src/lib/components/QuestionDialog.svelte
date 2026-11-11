<script lang="ts">
	// The server's Yes/No questions (stores/question.ts): Yes / No, Esc is No; the focus goes
	// back to where it was (a list cell, a card field) when the question is answered
	import { tick } from 'svelte';
	import Modal from './Modal.svelte';
	import Button from './Button.svelte';
	import { t, DLG, BTN } from '$lib/services/i18n.svelte';
	import { question } from '$lib/stores/question';

	let body = $state<HTMLDivElement | null>(null);
	let returnFocus: HTMLElement | null = null;

	$effect(() => {
		if (!$question) return;
		returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
		tick().then(() => body?.querySelector<HTMLButtonElement>('.question-buttons .btn-primary')?.focus());
	});

	function answer(yes: boolean) {
		const current = $question;
		if (!current) return;
		current.answer(yes);
		const target = returnFocus;
		returnFocus = null;
		tick().then(() => target?.isConnected && target.focus({ preventScroll: true }));
	}
</script>

<Modal open={!!$question} onclose={() => answer(false)}>
	<div class="question" bind:this={body}>
		<h3 class="question-title">{t(DLG.CONFIRM_TITLE)}</h3>
		<p class="question-text">{$question?.text ?? ''}</p>
		<div class="question-buttons">
			<Button variant="secondary" onclick={() => answer(false)}>{t(BTN.NO)}</Button>
			<Button variant="primary" onclick={() => answer(true)}>{t(BTN.YES)}</Button>
		</div>
	</div>
</Modal>

<style>
	.question {
		@apply p-5;
		min-width: min(28rem, calc(100vw - 2rem));
		max-width: 36rem;
	}

	.question-title {
		@apply text-lg font-semibold mb-3;
		color: rgb(var(--heading));
	}

	:global(.dark) .question-title {
		color: rgb(var(--gray-100));
	}

	.question-text {
		@apply text-sm;
		color: rgb(var(--gray-700));
		white-space: pre-line;
	}

	:global(.dark) .question-text {
		color: rgb(var(--gray-300));
	}

	.question-buttons {
		@apply flex justify-end gap-3 mt-5;
	}
</style>
