<script lang="ts">
	// The server is in maintenance (utils/maintenance.ts): a notice over the whole app instead of
	// the error of the page or call that met it. It checks every CHECK_INTERVAL_MS — and on Try
	// again — whether the server is back, and then loads the page again (a full load: the server
	// runs another program now). Its texts come with the server's answer (the translations cannot
	// be loaded during maintenance); the loaded translations are the fallback.
	import { onDestroy } from 'svelte';
	import Button from './Button.svelte';
	import { t } from '$lib/services/i18n.svelte';
	import { maintenance, maintenanceOver, CHECK_INTERVAL_MS } from '$lib/utils/maintenance';

	let checking = $state(false);
	let timer: ReturnType<typeof setInterval> | null = null;

	async function check() {
		if (checking) return;
		checking = true;
		try {
			const over = await maintenanceOver();
			if (over) {
				window.location.reload();
			}
		} finally {
			checking = false;
		}
	}

	$effect(() => {
		if ($maintenance && !timer) {
			timer = setInterval(check, CHECK_INTERVAL_MS);
		}
		if (!$maintenance && timer) {
			clearInterval(timer);
			timer = null;
		}
	});

	onDestroy(() => {
		if (timer) clearInterval(timer);
	});
</script>

{#if $maintenance}
	<div class="maintenance-overlay" role="alertdialog" aria-modal="true" aria-labelledby="maintenance-title" aria-describedby="maintenance-text" data-own-keys>
		<div class="maintenance-panel" class:failed={$maintenance.failed}>
			<h2 id="maintenance-title" class="maintenance-title">{$maintenance.title || t('APP_MAINTENANCE_TITLE')}</h2>
			<p id="maintenance-text" class="maintenance-text" aria-live="polite">{$maintenance.message}</p>
			<div class="maintenance-buttons">
				<Button variant="primary" onclick={check} disabled={checking}>{$maintenance.retry || t('APP_MAINTENANCE_RETRY')}</Button>
			</div>
		</div>
	</div>
{/if}

<style>
	.maintenance-overlay {
		position: fixed;
		inset: 0;
		z-index: 10000; /* above dialogs and toasts: nothing else works during maintenance */
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
		background: rgb(var(--page-bg) / 0.92);
	}
	.maintenance-panel {
		max-width: 32rem;
		width: 100%;
		padding: 1.5rem;
		border-radius: 0.5rem;
		border-top: 4px solid rgb(var(--accent-500));
		background: rgb(var(--surface));
		color: rgb(var(--fg));
		box-shadow: 0 10px 30px rgb(var(--gray-900) / 0.2);
	}
	:global(.dark) .maintenance-panel {
		background: rgb(var(--gray-800));
	}
	.maintenance-panel.failed {
		border-top-color: rgb(var(--gray-400));
	}
	.maintenance-title {
		font-size: 1.25rem;
		font-weight: 600;
		margin-bottom: 0.75rem;
	}
	.maintenance-text {
		line-height: 1.5;
		margin-bottom: 1.25rem;
	}
	.maintenance-buttons {
		display: flex;
		justify-content: flex-end;
	}
</style>
