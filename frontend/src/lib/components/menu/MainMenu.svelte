<!--
	The main menu (Main Menu, classic forms 330–337): the modules in a column on the left, the
	chosen module's menu on the right — its entries in the classic order, separators as gaps, and
	sub-menus (Setup, Activities …) that expand in place. Entries whose page or report is not built
	yet are greyed out. A search over all entries sits below the modules (Ctrl+F on the home page).

	Keyboard: ↑↓ move through the modules or the entries, → goes into the module's menu or opens a
	sub-menu, ← closes it or goes back to the modules, Enter opens an entry, Home/End the first or
	last one.
-->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount, tick } from 'svelte';
	import type { MenuDefinition, MenuItem } from '$lib/types/pages';
	import { t, HOME } from '$lib/services/i18n.svelte';
	import { visibleRows, searchMenu, parentKeys, type MenuRow, type MenuSearchResult } from '$lib/utils/mainMenu';
	import ObjectRunner from '../ObjectRunner.svelte';

	interface Props {
		menu: MenuDefinition;
	}
	let { menu }: Props = $props();

	// The module shown and the sub-menus expanded are remembered; the entry opened last gets the
	// focus when the user comes back to the menu
	const MODULE_KEY = 'openerp-main-menu-module';
	const EXPANDED_KEY = 'openerp-main-menu-expanded';
	const LAST_ENTRY_KEY = 'openerp-main-menu-last';

	const modules = $derived(menu.menu ?? []);
	let moduleIndex = $state(0);
	let expanded = $state(new Set<string>());
	let query = $state('');
	let runner: ObjectRunner | undefined = $state();
	let modulesEl: HTMLElement | null = $state(null);
	let entriesEl: HTMLElement | null = $state(null);
	let searchEl: HTMLInputElement | null = $state(null);

	const current = $derived(modules[moduleIndex]);
	const rows = $derived(visibleRows(current?.items, expanded, `/${current?.name ?? ''}`));
	const results = $derived(query.trim() ? searchMenu(modules, query) : []);

	onMount(() => {
		const stored = Number(localStorage.getItem(MODULE_KEY));
		if (Number.isInteger(stored) && stored >= 0 && stored < modules.length) moduleIndex = stored;
		try {
			expanded = new Set(JSON.parse(sessionStorage.getItem(EXPANDED_KEY) ?? '[]'));
		} catch {
			expanded = new Set();
		}
		void focusInitial();
	});

	async function focusInitial() {
		await tick();
		const last = sessionStorage.getItem(LAST_ENTRY_KEY);
		const entry = last ? entriesEl?.querySelector<HTMLButtonElement>(`button[data-key="${CSS.escape(last)}"]`) : null;
		(entry ?? moduleButtons()[moduleIndex])?.focus();
	}

	function selectModule(i: number) {
		moduleIndex = i;
		localStorage.setItem(MODULE_KEY, String(i));
	}

	function setExpanded(next: Set<string>) {
		expanded = next;
		sessionStorage.setItem(EXPANDED_KEY, JSON.stringify([...next]));
	}

	function toggle(row: MenuRow) {
		const next = new Set(expanded);
		if (next.has(row.key)) next.delete(row.key);
		else next.add(row.key);
		setExpanded(next);
	}

	// Open an entry: its page, or run its report or codeunit (with dialogs, the ObjectRunner)
	function open(item: MenuItem, key: string) {
		if (!item.enabled) return;
		sessionStorage.setItem(LAST_ENTRY_KEY, key);
		if (item.page_id) goto(`/pages/${item.page_id}`);
		else if (item.run_object) void runner?.run(item.run_object, {});
	}

	function activate(row: MenuRow) {
		if (row.submenu) toggle(row);
		else open(row.item, row.key);
	}

	// A search result: shown in its module with its sub-menus expanded when the user comes back
	function openResult(r: MenuSearchResult) {
		const key = '/' + r.path.join('/');
		selectModule(r.module);
		const next = new Set(expanded);
		for (const k of parentKeys(key)) next.add(k);
		setExpanded(next);
		query = '';
		open(r.item, key);
	}

	const moduleButtons = () => Array.from(modulesEl?.querySelectorAll<HTMLButtonElement>('button[data-module]') ?? []);
	const entryButtons = () => Array.from(entriesEl?.querySelectorAll<HTMLButtonElement>('button[data-key], button[data-result]') ?? []);

	function focusEntry(index: number) {
		const entries = entryButtons();
		entries[Math.max(0, Math.min(index, entries.length - 1))]?.focus();
	}

	function handleModuleKeydown(event: KeyboardEvent, i: number) {
		if (event.ctrlKey || event.altKey || event.metaKey) return;
		const buttons = moduleButtons();
		let target = -1;
		switch (event.key) {
			case 'ArrowDown':
				target = Math.min(i + 1, buttons.length - 1);
				break;
			case 'ArrowUp':
				target = Math.max(i - 1, 0);
				break;
			case 'Home':
				target = 0;
				break;
			case 'End':
				target = buttons.length - 1;
				break;
			case 'ArrowRight':
			case 'Enter':
				event.preventDefault();
				selectModule(i);
				void tick().then(() => focusEntry(0));
				return;
			default:
				return;
		}
		event.preventDefault();
		selectModule(target);
		buttons[target]?.focus();
	}

	function handleEntryKeydown(event: KeyboardEvent, row: MenuRow | null, index: number) {
		if (event.ctrlKey || event.altKey || event.metaKey) return;
		const count = entryButtons().length;
		switch (event.key) {
			case 'ArrowDown':
				focusEntry(index + 1);
				break;
			case 'ArrowUp':
				if (index === 0 && query) searchEl?.focus();
				else focusEntry(index - 1);
				break;
			case 'Home':
				focusEntry(0);
				break;
			case 'End':
				focusEntry(count - 1);
				break;
			case 'ArrowRight':
				if (row?.submenu) {
					if (!row.expanded) toggle(row);
					else focusEntry(index + 1);
				}
				break;
			case 'ArrowLeft':
				if (row?.submenu && row.expanded) {
					toggle(row);
				} else if (row && row.depth > 0) {
					// to the sub-menu this entry is in
					const parent = parentKeys(row.key).at(-1);
					entriesEl?.querySelector<HTMLButtonElement>(`button[data-key="${CSS.escape(parent ?? '')}"]`)?.focus();
				} else {
					moduleButtons()[moduleIndex]?.focus();
				}
				break;
			default:
				return;
		}
		event.preventDefault();
	}

	function handleSearchKeydown(event: KeyboardEvent) {
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			focusEntry(0);
		} else if (event.key === 'Enter') {
			event.preventDefault();
			const first = results.find((r) => r.item.enabled);
			if (first) openResult(first);
		} else if (event.key === 'Escape' && query) {
			event.preventDefault();
			event.stopPropagation();
			query = '';
		}
	}

	// Ctrl+F on the home page: the menu search
	function handleWindowKeydown(event: KeyboardEvent) {
		if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') {
			event.preventDefault();
			searchEl?.focus();
			searchEl?.select();
		}
	}
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<div class="main-menu">
	<nav class="modules" bind:this={modulesEl} aria-label={t(HOME.MENU_MODULES)}>
		{#each modules as module, i}
			<button
				type="button"
				data-module
				class="module"
				class:selected={i === moduleIndex}
				aria-current={i === moduleIndex ? 'true' : undefined}
				onclick={() => selectModule(i)}
				onfocus={() => selectModule(i)}
				onkeydown={(e) => handleModuleKeydown(e, i)}
			>
				{module.name}
			</button>
		{/each}
		<input
			bind:this={searchEl}
			bind:value={query}
			class="search"
			type="search"
			placeholder={t(HOME.MENU_SEARCH)}
			aria-label={t(HOME.MENU_SEARCH)}
			onkeydown={handleSearchKeydown}
		/>
	</nav>

	<section class="entries" bind:this={entriesEl}>
		{#if query.trim()}
			<h2 class="module-title">{t(HOME.MENU_SEARCH_RESULTS)}</h2>
			{#if results.length === 0}
				<p class="empty">{t(HOME.MENU_NO_RESULTS)}</p>
			{/if}
			{#each results as r, i}
				<button
					type="button"
					data-result
					class="entry"
					class:unavailable={!r.item.enabled}
					aria-disabled={!r.item.enabled}
					title={r.item.enabled ? undefined : t(r.item.denied ? HOME.MENU_NO_PERMISSION : HOME.MENU_NOT_AVAILABLE)}
					onclick={() => openResult(r)}
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.preventDefault();
							openResult(r);
						} else handleEntryKeydown(e, null, i);
					}}
				>
					<span>{r.item.name}</span>
					<span class="path">{r.path.slice(0, -1).join(' › ')}</span>
				</button>
			{/each}
		{:else if current}
			<h2 class="module-title">{current.name}</h2>
			{#each rows as row, i (row.key)}
				{#if row.item.separator}
					<div class="gap" style:margin-left="{row.depth * 1.25}rem"></div>
				{:else}
					{@const index = rows.slice(0, i).filter((r) => !r.item.separator).length}
					<button
						type="button"
						data-key={row.key}
						class="entry"
						class:submenu={row.submenu}
						class:unavailable={!row.item.enabled}
						aria-disabled={!row.item.enabled}
						aria-expanded={row.submenu ? row.expanded : undefined}
						title={row.item.enabled ? undefined : t(row.item.denied ? HOME.MENU_NO_PERMISSION : HOME.MENU_NOT_AVAILABLE)}
						style:padding-left="{0.5 + row.depth * 1.25}rem"
						onclick={() => activate(row)}
						onkeydown={(e) => {
							if (e.key === 'Enter') {
								e.preventDefault();
								activate(row);
							} else handleEntryKeydown(e, row, index);
						}}
					>
						{#if row.submenu}<span class="caret">{row.expanded ? '▾' : '▸'}</span>{/if}
						<span>{row.item.name}</span>
					</button>
				{/if}
			{/each}
		{/if}
	</section>
</div>

<ObjectRunner bind:this={runner} />

<style>
	/* Dark mode: :global(.dark) — the dark: variants of @apply do not reach a scoped style */
	.main-menu {
		@apply grid gap-6;
		grid-template-columns: minmax(12rem, 16rem) 1fr;
	}
	.modules {
		@apply flex flex-col gap-1 border-r border-gray-200 pr-4;
	}
	.module {
		@apply text-left rounded px-3 py-2 text-gray-700 hover:bg-gray-100;
	}
	.module.selected {
		@apply bg-blue-50 text-heading font-semibold;
	}
	.module:focus {
		@apply outline-none ring-2 ring-blue-400;
	}
	.search {
		@apply mt-4 rounded border border-gray-300 bg-transparent px-3 py-1.5 text-sm;
	}
	.entries {
		@apply flex flex-col items-start;
	}
	.module-title {
		@apply text-lg font-semibold text-gray-700 mb-2;
	}
	.entry {
		@apply flex items-baseline gap-2 rounded py-0.5 pr-2 text-left text-heading hover:underline;
	}
	.entry:focus {
		@apply outline-none bg-blue-50 underline;
	}
	.entry.submenu {
		@apply font-semibold text-gray-700;
	}
	.entry.unavailable {
		@apply text-gray-400 no-underline cursor-default;
	}
	.caret {
		@apply inline-block w-3 text-gray-500;
	}
	.path {
		@apply text-xs text-gray-500;
	}
	.gap {
		@apply h-3;
	}
	.empty {
		@apply text-gray-500;
	}
	:global(.dark) .modules {
		@apply border-gray-700;
	}
	:global(.dark) .module {
		@apply text-gray-300;
	}
	:global(.dark) .module:hover {
		@apply bg-gray-800;
	}
	:global(.dark) .module.selected {
		@apply bg-blue-900 text-blue-300;
	}
	:global(.dark) .search {
		@apply border-gray-600;
	}
	:global(.dark) .module-title,
	:global(.dark) .entry.submenu {
		@apply text-gray-300;
	}
	:global(.dark) .entry {
		@apply text-blue-400;
	}
	:global(.dark) .entry:focus {
		@apply bg-blue-900;
	}
	:global(.dark) .entry.unavailable {
		@apply text-gray-600;
	}
</style>
