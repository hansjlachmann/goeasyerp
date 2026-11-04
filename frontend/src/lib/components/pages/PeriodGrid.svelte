<script lang="ts">
	// A card section of periods (classic matrix-by-periods subform): the card record's FlowFields
	// for each day, week, month, quarter, year or accounting period — or, with rows from a table
	// (locations, variants), for each of its records in one period. The server calculates the
	// rows (POST /api/pages/:id/period-grid); a value's drilldown opens its page filtered to the
	// record and the row's dates.
	import { untrack } from 'svelte';
	import { api } from '$lib/services/api';
	import { t, PERIOD } from '$lib/services/i18n.svelte';
	import { currentLanguage } from '$lib/stores/session';
	import { formatDate, formatDecimal } from '$lib/utils/fieldHelpers';
	import type { Field, PeriodGrid, PeriodGridRow } from '$lib/types/pages';
	import type { TableFilter } from '$lib/types/api';

	interface Props {
		pageId: number;
		section: string;
		grid: PeriodGrid;
		record: Record<string, any>;
		captions: Record<string, string>;
		// opens a drilldown page over the card with these filters
		ondrilldown?: (pageId: number, filters: TableFilter[]) => void;
	}

	let { pageId, section, grid, record, captions, ondrilldown }: Props = $props();

	const locale = $derived($currentLanguage || 'en-US');
	const periodRows = 12; // periods shown at once

	let viewBy = $state(0);
	let viewAs = $state(0); // 0 Net Change, 1 Balance at Date
	let date = $state(''); // the first period's start (blank: the work date)
	let filterValues = $state<Record<string, string>>({});
	let rows = $state<PeriodGridRow[]>([]);
	let loading = $state(false);
	let error = $state('');

	$effect.pre(() => {
		viewBy = grid.view_by ?? 0;
	});

	const viewByChoices = [
		{ value: 0, key: PERIOD.DAY },
		{ value: 1, key: PERIOD.WEEK },
		{ value: 2, key: PERIOD.MONTH },
		{ value: 3, key: PERIOD.QUARTER },
		{ value: 4, key: PERIOD.YEAR },
		{ value: 5, key: PERIOD.ACCOUNTING }
	];

	async function load(step = 0) {
		if (!record) return;
		loading = true;
		error = '';
		try {
			const filters = Object.entries(filterValues)
				.filter(([, v]) => v.trim() !== '')
				.map(([field, expression]) => ({ field, expression: expression.trim() }));
			const result = await api.periodGrid(pageId, {
				section,
				record: $state.snapshot(record),
				filters,
				view_by: viewBy,
				view_as: viewAs,
				date,
				step,
				rows: grid.rows ? 100 : periodRows
			});
			rows = result.rows;
			date = result.date;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	// Load when the record, the period length or the view changes (load itself reads and sets
	// the date and the rows: untracked). The first set of periods has the work date's period in
	// the middle, so the recent periods show too.
	let firstLoad = true;
	$effect(() => {
		void JSON.stringify(record ?? {});
		void viewAs;
		untrack(() => {
			if (firstLoad) {
				firstLoad = false;
				loadCentered();
			} else {
				load();
			}
		});
	});

	// The work date's period in the middle of the set (rows from a table: the work date's period)
	function loadCentered() {
		date = '';
		load(grid.rows ? 0 : -Math.floor(periodRows / 2));
	}

	// A new period length starts again around the work date
	function setViewBy(value: number) {
		if (viewBy === value) return;
		viewBy = value;
		loadCentered();
	}

	function setViewAs(value: number) {
		if (viewAs !== value) viewAs = value;
	}

	// Previous / next: a set of periods (one period when the rows are records)
	function move(direction: number) {
		load(direction * (grid.rows ? 1 : periodRows));
	}

	function periodName(row: PeriodGridRow): string {
		if (row.period_name) return row.period_name;
		if (viewBy === 2) {
			const d = new Date(row.period_start + 'T00:00:00');
			return d.toLocaleDateString(locale, { month: 'long', year: 'numeric' });
		}
		return formatDate(row.period_start, locale);
	}

	function cellText(column: Field, value: unknown): string {
		if (value === undefined || value === null || value === '') return '';
		return formatDecimal(value, column.decimal_places);
	}

	// The drilldown of a value: its page filtered to the record and the row's dates (Balance at
	// Date: up to the end) and, for rows from a table, the row's key
	function drilldown(column: Field, row: PeriodGridRow) {
		if (!column.drilldown || !ondrilldown) return;
		const filters: TableFilter[] = [];
		if (column.drilldown_filter_field && column.drilldown_filter_value) {
			const value = record?.[column.drilldown_filter_value];
			if (value === undefined || value === null || value === '') return;
			filters.push({ field: column.drilldown_filter_field, expression: String(value) });
		}
		const dateField = column.drilldown_date_field || grid.drilldown_date_field;
		if (dateField) {
			const range = viewAs === 1 ? `..${row.period_end}` : `${row.period_start}..${row.period_end}`;
			filters.push({ field: dateField, expression: range });
		}
		for (const f of grid.filters ?? []) {
			const expression = (filterValues[f.source] ?? '').trim();
			const target = f.drilldown_filter_field;
			if (expression && target) filters.push({ field: target, expression });
		}
		if (grid.rows?.drilldown_filter_field && row.key) {
			filters.push({ field: grid.rows.drilldown_filter_field, expression: row.key });
		}
		ondrilldown(column.drilldown, filters);
	}
</script>

<div class="period-grid" data-own-keys>
	<div class="grid-controls">
		<div class="control-group" role="radiogroup" aria-label={t(PERIOD.VIEW_BY)}>
			<span class="control-label">{t(PERIOD.VIEW_BY)}</span>
			{#each viewByChoices as choice}
				<button
					type="button"
					class="choice"
					class:active={viewBy === choice.value}
					aria-pressed={viewBy === choice.value}
					onclick={() => setViewBy(choice.value)}
				>
					{t(choice.key)}
				</button>
			{/each}
		</div>
		<div class="control-group" role="radiogroup" aria-label={t(PERIOD.VIEW_AS)}>
			<span class="control-label">{t(PERIOD.VIEW_AS)}</span>
			<button type="button" class="choice" class:active={viewAs === 0} aria-pressed={viewAs === 0} onclick={() => setViewAs(0)}>
				{t(PERIOD.NET_CHANGE)}
			</button>
			<button type="button" class="choice" class:active={viewAs === 1} aria-pressed={viewAs === 1} onclick={() => setViewAs(1)}>
				{t(PERIOD.BALANCE_AT_DATE)}
			</button>
		</div>
		<div class="control-group">
			<button type="button" class="choice" title={t(PERIOD.PREVIOUS)} aria-label={t(PERIOD.PREVIOUS)} onclick={() => move(-1)}>‹</button>
			{#if grid.rows && rows.length > 0}
				<span class="current-period">{periodName(rows[0])}</span>
			{/if}
			<button type="button" class="choice" title={t(PERIOD.NEXT)} aria-label={t(PERIOD.NEXT)} onclick={() => move(1)}>›</button>
		</div>
	</div>

	{#if (grid.filters ?? []).length > 0}
		<div class="grid-filters">
			{#each grid.filters as filter (filter.source)}
				<label class="filter-field">
					<span>{captions[filter.source] ?? filter.source}</span>
					<input
						type="text"
						class="filter-input"
						value={filterValues[filter.source] ?? ''}
						onchange={(e) => {
							filterValues[filter.source] = (e.currentTarget as HTMLInputElement).value.toUpperCase();
							load();
						}}
					/>
				</label>
			{/each}
		</div>
	{/if}

	{#if error}
		<p class="grid-error">{error}</p>
	{/if}

	<div class="grid-table-wrap" class:loading>
		<table class="grid-table">
			<thead>
				<tr>
					{#if grid.rows}
						<th>{t(PERIOD.CODE)}</th>
						<th>{t(PERIOD.DESCRIPTION)}</th>
					{:else}
						<th>{t(PERIOD.START)}</th>
						<th>{t(PERIOD.NAME)}</th>
					{/if}
					{#each grid.columns as column (column.source)}
						<th class="num">{captions[column.source] ?? column.source}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each rows as row (row.key ?? row.period_start)}
					<tr>
						{#if grid.rows}
							<td>{row.key}</td>
							<td>{row.caption}</td>
						{:else}
							<td>{formatDate(row.period_start, locale)}</td>
							<td>{periodName(row)}</td>
						{/if}
						{#each grid.columns as column (column.source)}
							<td class="num">
								{#if column.drilldown && ondrilldown}
									<button type="button" class="drilldown" onclick={() => drilldown(column, row)}>
										{cellText(column, row.values[column.source]) || '0'}
									</button>
								{:else}
									{cellText(column, row.values[column.source])}
								{/if}
							</td>
						{/each}
					</tr>
				{:else}
					{#if !loading}
						<tr><td colspan={grid.columns.length + 2} class="empty">{t(PERIOD.NO_PERIODS)}</td></tr>
					{/if}
				{/each}
			</tbody>
		</table>
	</div>
</div>

<style>
	.period-grid {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.grid-controls {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
		align-items: center;
	}

	.control-group {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.control-label {
		font-size: 0.8125rem;
		color: rgb(var(--gray-600));
		margin-right: 0.25rem;
	}

	:global(.dark) .control-label {
		color: rgb(var(--gray-400));
	}

	.choice {
		@apply bg-surface text-fg;
		padding: 0.25rem 0.625rem;
		font-size: 0.8125rem;
		border: 1px solid rgb(var(--gray-300));
		border-radius: 0.25rem;
		cursor: pointer;
	}

	:global(.dark) .choice {
		background-color: rgb(var(--gray-700));
		border-color: rgb(var(--gray-600));
		color: rgb(var(--gray-100));
	}

	.choice.active,
	:global(.dark) .choice.active {
		@apply bg-primary-600 border-primary-600 text-white;
	}

	.current-period {
		font-size: 0.875rem;
		min-width: 8rem;
		text-align: center;
	}

	.grid-filters {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
	}

	.filter-field {
		display: flex;
		flex-direction: column;
		font-size: 0.8125rem;
		gap: 0.25rem;
	}

	.filter-input {
		@apply bg-surface text-fg;
		padding: 0.25rem 0.5rem;
		border: 1px solid rgb(var(--gray-300));
		border-radius: 0.25rem;
		min-width: 10rem;
	}

	:global(.dark) .filter-input {
		background-color: rgb(var(--gray-700));
		border-color: rgb(var(--gray-600));
		color: rgb(var(--gray-100));
	}

	.grid-error {
		@apply text-red-600;
		font-size: 0.875rem;
	}

	.grid-table-wrap {
		overflow-x: auto;
	}

	.grid-table-wrap.loading {
		opacity: 0.6;
	}

	.grid-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.875rem;
	}

	.grid-table th,
	.grid-table td {
		padding: 0.375rem 0.625rem;
		border-bottom: 1px solid rgb(var(--gray-200));
		text-align: left;
		white-space: nowrap;
	}

	:global(.dark) .grid-table th,
	:global(.dark) .grid-table td {
		border-bottom-color: rgb(var(--gray-700));
	}

	.grid-table th {
		font-weight: 600;
	}

	.grid-table .num {
		text-align: right;
		font-family: var(--font-mono); /* amounts: IBM Plex Mono */
	}

	.drilldown {
		background: none;
		border: none;
		padding: 0;
		color: rgb(var(--accent-600));
		cursor: pointer;
		text-decoration: underline;
		font: inherit;
	}

	:global(.dark) .drilldown {
		color: rgb(var(--accent-400));
	}

	.empty {
		color: rgb(var(--gray-500));
		text-align: center;
	}
</style>
