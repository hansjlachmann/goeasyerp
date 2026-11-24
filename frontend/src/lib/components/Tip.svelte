<script lang="ts">
	// A tip for new users (utils/tips.ts): shown while tips are on (My Settings) and the tip was
	// not closed in this session. × closes it; "Don't show tips" switches tips off for the user.
	import { t, TIP } from '#lib/services/i18n.svelte.js';
	import { showTips, saveRegional } from '#lib/stores/regional.js';
	import { tipClosed, closeTip, type TipDef } from '#lib/utils/tips.js';
	import { toast } from '#lib/stores/toast.js';

	interface Props {
		tip: TipDef;
	}
	let { tip }: Props = $props();

	// svelte-ignore state_referenced_locally - the tip's closed state is read once, then kept here
	let closed = $state(tipClosed(tip.id));

	function close() {
		closeTip(tip.id);
		closed = true;
	}

	async function turnOff() {
		try {
			await saveRegional({ show_tips: false });
		} catch (err) {
			toast.error(err instanceof Error ? err.message : String(err));
		}
	}
</script>

{#if $showTips && !closed}
	<div class="tip" role="note" data-tip={tip.id}>
		<span class="tip-icon" aria-hidden="true">💡</span>
		<span class="tip-text"><strong>{t(TIP.LABEL)}:</strong> {t(tip.text)}</span>
		<button type="button" class="tip-off" tabindex="-1" onclick={turnOff}>{t(TIP.TURN_OFF)}</button>
		<button type="button" class="tip-close" tabindex="-1" onclick={close} aria-label={t(TIP.CLOSE)} title={t(TIP.CLOSE)}>×</button>
	</div>
{/if}

<style>
	.tip {
		@apply flex items-center gap-2 rounded-md px-3 py-2 text-sm;
		background-color: rgb(var(--accent-50));
		border: 1px solid rgb(var(--accent-200));
		color: rgb(var(--gray-900));
	}
	:global(.dark) .tip {
		background-color: rgb(var(--gray-800));
		border-color: rgb(var(--accent-600) / 0.6);
		color: rgb(var(--gray-50));
	}
	.tip-text {
		@apply flex-1;
	}
	.tip-off {
		@apply text-xs underline whitespace-nowrap;
		color: rgb(var(--gray-400));
	}
	.tip-off:hover {
		color: rgb(var(--accent-600));
	}
	.tip-close {
		@apply text-lg leading-none px-1;
		color: rgb(var(--gray-400));
	}
	.tip-close:hover {
		color: rgb(var(--gray-900));
	}
	:global(.dark) .tip-close:hover {
		color: rgb(var(--gray-50));
	}
</style>
