<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { t, MENU } from '#lib/services/i18n.svelte.js';
	import { companyLabel, filterCompanies, type CompanyInfo } from '#lib/utils/company.js';

	// Switch Company dialog (Ctrl+O). Typing filters the list,
	// ArrowUp/Down move, Enter switches, Escape closes. Companies show their display
	// name; `current` and `onselect` use the technical name.
	interface Props {
		companies: CompanyInfo[];
		current: string;
		switching?: boolean;
		onselect: (company: string) => void;
		onclose: () => void;
	}

	let { companies, current, switching = false, onselect, onclose }: Props = $props();

	let filter = $state('');
	let highlighted = $state(0);
	let inputElement: HTMLInputElement | null = $state(null);
	let listElement: HTMLUListElement | null = $state(null);

	const filtered = $derived(filterCompanies(companies, filter));

	// Start on the current company
	onMount(() => {
		highlighted = Math.max(0, companies.findIndex((c) => c.name === current));
		tick().then(() => {
			inputElement?.focus();
			scrollHighlightedIntoView();
		});
	});

	function handleInput() {
		highlighted = 0;
	}

	function scrollHighlightedIntoView() {
		const item = listElement?.children[highlighted] as HTMLElement | undefined;
		item?.scrollIntoView({ block: 'nearest' });
	}

	function move(delta: number) {
		if (filtered.length === 0) return;
		highlighted = Math.min(filtered.length - 1, Math.max(0, highlighted + delta));
		tick().then(scrollHighlightedIntoView);
	}

	function choose(company: CompanyInfo | undefined) {
		if (!company || switching) return;
		if (company.name === current) {
			onclose();
			return;
		}
		onselect(company.name);
	}

	function handleKeydown(event: KeyboardEvent) {
		// Keys stay in the dialog: the page underneath must not act on them
		event.stopPropagation();
		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				move(1);
				break;
			case 'ArrowUp':
				event.preventDefault();
				move(-1);
				break;
			case 'PageDown':
				event.preventDefault();
				move(10);
				break;
			case 'PageUp':
				event.preventDefault();
				move(-10);
				break;
			case 'Enter':
				event.preventDefault();
				choose(filtered[highlighted]);
				break;
			case 'Escape':
				event.preventDefault();
				onclose();
				break;
			case 'Tab':
				// Keep focus in the dialog
				event.preventDefault();
				break;
		}
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="company-switch-backdrop" onclick={onclose} onkeydown={handleKeydown} role="presentation">
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="company-switch-dialog bg-surface dark:bg-gray-800 border border-gray-200 dark:border-gray-700"
		onclick={(e) => e.stopPropagation()}
		role="dialog"
		aria-modal="true"
		aria-labelledby="company-switch-title"
		tabindex="-1"
	>
		<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
			<h2 id="company-switch-title" class="text-base font-semibold text-gray-900 dark:text-gray-50">
				{t(MENU.SWITCH_COMPANY)}
			</h2>
		</div>
		<div class="p-3">
			<input
				bind:this={inputElement}
				bind:value={filter}
				oninput={handleInput}
				type="text"
				autocomplete="off"
				placeholder={t(MENU.SWITCH_COMPANY_FILTER)}
				aria-label={t(MENU.SWITCH_COMPANY_FILTER)}
				class="w-full px-2 py-1.5 text-sm rounded-sm border border-gray-300 dark:border-gray-600 bg-surface dark:bg-gray-900 text-gray-900 dark:text-gray-50 focus:outline-hidden focus:border-blue-600 dark:focus:border-blue-400"
			/>
		</div>
		<ul bind:this={listElement} class="max-h-72 overflow-y-auto pb-2" role="listbox">
			{#each filtered as company, i (company.name)}
				<li role="option" aria-selected={i === highlighted}>
					<button
						type="button"
						tabindex="-1"
						disabled={switching}
						onclick={() => choose(company)}
						onmousemove={() => (highlighted = i)}
						class="w-full text-left px-4 py-2 text-sm flex items-center justify-between disabled:opacity-50"
						class:bg-gray-100={i === highlighted}
						class:dark:bg-gray-700={i === highlighted}
						class:text-blue-600={company.name === current}
						class:dark:text-blue-400={company.name === current}
						class:font-medium={company.name === current}
						class:text-gray-700={company.name !== current}
						class:dark:text-gray-300={company.name !== current}
					>
						{companyLabel(company)}
						{#if company.name === current}
							<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
							</svg>
						{/if}
					</button>
				</li>
			{:else}
				<li class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">{t(MENU.NO_COMPANIES)}</li>
			{/each}
		</ul>
	</div>
</div>

<style>
	.company-switch-backdrop {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 15vh;
		background: rgba(0, 0, 0, 0.4);
	}

	.company-switch-dialog {
		width: 24rem;
		max-width: calc(100vw - 2rem);
		border-radius: 0.375rem;
		box-shadow: 0 10px 25px rgba(0, 0, 0, 0.25);
		outline: none;
	}
</style>
