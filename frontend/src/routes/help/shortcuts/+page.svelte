<script lang="ts">
	import { onMount } from 'svelte';
	import { t, HELP } from '$lib/services/i18n.svelte';
	import { shortcutHelpSections } from '$lib/utils/shortcutHelp';

	// Keyboard Shortcuts help, opened from the user menu as a detached window
	// (window.open). The root layout renders it without the menu bar.

	function close() {
		// Only closes when opened by script (the normal case); otherwise go to the app
		window.close();
		if (!window.closed) window.location.href = '/';
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			close();
		}
	}

	// Window title follows the translations as they load
	$effect(() => {
		document.title = `${t(HELP.TITLE)} – GoEasyERP`;
	});

	onMount(() => window.focus());
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="help-page">
	<header class="help-header">
		<div>
			<h1 class="text-xl font-semibold text-gray-900 dark:text-gray-50">{t(HELP.TITLE)}</h1>
		</div>
		<button
			type="button"
			onclick={close}
			class="text-sm px-3 py-1.5 rounded border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
		>
			{t(HELP.CLOSE)}
		</button>
	</header>

	{#each shortcutHelpSections as section (section.title)}
		<section class="help-section">
			<h2 class="text-base font-semibold text-blue-600 dark:text-blue-400 mb-2">{t(section.title)}</h2>
			<table class="help-table">
				<thead>
					<tr>
						<th class="keys-col">{t(HELP.COL_KEYS)}</th>
						<th>{t(HELP.COL_ACTION)}</th>
					</tr>
				</thead>
				<tbody>
					{#each section.items as item (item.description)}
						<tr>
							<td class="keys-col">
								{#each item.keys as key, i (key)}
									{#if i > 0}<span class="key-sep">/</span>{/if}<kbd>{key}</kbd>
								{/each}
							</td>
							<td class="text-gray-700 dark:text-gray-300">{t(item.description)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</section>
	{/each}
</div>

<style>
	.help-page {
		max-width: 52rem;
		margin: 0 auto;
		padding: 1.25rem 1.5rem 2rem;
	}

	.help-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1.25rem;
	}

	.help-section {
		margin-bottom: 1.5rem;
	}

	.help-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.875rem;
	}

	.help-table th {
		text-align: left;
		font-weight: 600;
		padding: 0.375rem 0.5rem;
		color: rgb(var(--gray-600));
		border-bottom: 1px solid #e0e0e0;
	}

	.help-table td {
		padding: 0.375rem 0.5rem;
		border-bottom: 1px solid #f0f0f0;
		vertical-align: top;
	}

	.keys-col {
		width: 16rem;
		white-space: nowrap;
	}

	.key-sep {
		margin: 0 0.25rem;
		color: rgb(var(--gray-400));
	}

	kbd {
		display: inline-block;
		min-width: 1.5rem;
		padding: 0.0625rem 0.375rem;
		font-family: inherit;
		font-size: 0.8125rem;
		text-align: center;
		color: rgb(var(--fg));
		background: rgb(var(--gray-50));
		border: 1px solid #c8c8c8;
		border-bottom-width: 2px;
		border-radius: 0.25rem;
	}

	:global(.dark) .help-table th {
		color: rgb(var(--gray-400));
		border-bottom-color: rgb(var(--gray-700));
	}

	:global(.dark) .help-table td {
		border-bottom-color: rgb(var(--gray-700));
	}

	:global(.dark) kbd {
		color: rgb(var(--gray-50));
		background: rgb(var(--gray-800));
		border-color: rgb(var(--gray-600));
	}
</style>
