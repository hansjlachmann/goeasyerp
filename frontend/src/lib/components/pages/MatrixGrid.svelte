<script lang="ts">
	// A card section of a table's records against periods (classic matrix forms: G/L Budget by
	// Periods): each cell is the row record's FlowField in the column's period, filtered by the
	// card record (the budget) and the filters above the matrix. The server calculates the cells
	// (POST /api/pages/:id/matrix); a value typed into a cell goes to the page's business logic
	// (POST /api/pages/:id/matrix/validate), then the matrix is calculated again (totals follow).
	// Keys: arrows move the selected cell, typing / F2 / Enter edit it, Enter / Tab confirm,
	// Esc cancels; Show Entries opens the cell's entries.
	import { untrack, tick } from 'svelte';
	import { api } from '$lib/services/api';
	import { t, PERIOD } from '$lib/services/i18n.svelte';
	import { currentLanguage } from '$lib/stores/session';
	import { cellStyle, formatDate, formatDecimal } from '$lib/utils/fieldHelpers';
	import { fromDecimalInput, toDecimalInput } from '$lib/utils/regional';
	import { matrixCellValue, roundingFactors } from '$lib/utils/matrix';
	import type { Matrix, MatrixColumn, MatrixRow } from '$lib/types/pages';
	import type { TableFilter } from '$lib/types/api';

	interface Props {
		pageId: number;
		section: string;
		matrix: Matrix;
		record: Record<string, any>;
		// opens a drilldown page over the card with these filters
		ondrilldown?: (pageId: number, filters: TableFilter[]) => void;
	}

	let { pageId, section, matrix, record, ondrilldown }: Props = $props();

	const locale = $derived($currentLanguage || 'en-US');
	const columnCount = 12; // periods shown at once

	let viewBy = $state(2);
	let viewAs = $state(0); // 0 Net Change, 1 Balance at Date
	let roundingFactor = $state(0);
	let date = $state(''); // the first column's start (blank: the work date)
	let filterValues = $state<Record<string, string>>({});
	let columns = $state<MatrixColumn[]>([]);
	let period = $state<MatrixColumn | undefined>(); // a calculated matrix: its one period
	let rows = $state<MatrixRow[]>([]);
	let loading = $state(false);
	let error = $state('');

	// the selected cell and the one being edited
	let selRow = $state(-1);
	let selCol = $state(-1);
	let editing = $state(false);
	let editText = $state('');
	let editRow = -1; // the cell being edited (the selection may move on before it is confirmed)
	let editCol = -1;
	let saving = false;
	let gridElement = $state<HTMLDivElement>();

	$effect.pre(() => {
		viewBy = matrix.view_by ?? 2;
	});

	const viewByChoices = [
		{ value: 0, key: PERIOD.DAY },
		{ value: 1, key: PERIOD.WEEK },
		{ value: 2, key: PERIOD.MONTH },
		{ value: 3, key: PERIOD.QUARTER },
		{ value: 4, key: PERIOD.YEAR },
		{ value: 5, key: PERIOD.ACCOUNTING }
	];

	// The card record's values the matrix depends on (its record filters)
	const recordKey = $derived(JSON.stringify(Object.values(matrix.record_filters ?? {}).map((f) => record?.[f] ?? null)));

	function userFilters() {
		return Object.entries(filterValues)
			.filter(([, v]) => v.trim() !== '')
			.map(([field, expression]) => ({ field, expression: expression.trim() }));
	}

	function request(step = 0) {
		return {
			section,
			record: $state.snapshot(record),
			filters: userFilters(),
			view_by: viewBy,
			view_as: viewAs,
			date,
			step,
			columns: columnCount
		};
	}

	async function load(step = 0) {
		if (!record) return;
		loading = true;
		error = '';
		try {
			const result = await api.matrix(pageId, request(step));
			columns = result.columns;
			rows = result.rows;
			date = result.date;
			period = result.period;
			if (selRow >= rows.length) selRow = rows.length - 1;
			if (selCol >= columns.length) selCol = columns.length - 1;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	// Load when the card record (its filter fields) or the view changes; the first columns start
	// at the work date's period
	$effect(() => {
		void recordKey;
		void viewAs;
		untrack(() => load());
	});

	function setViewBy(value: number) {
		if (viewBy === value) return;
		viewBy = value;
		date = '';
		load();
	}

	// Previous / next: a set of periods (a calculated matrix: one period)
	function move(direction: number) {
		load(direction * (matrix.calculated ? 1 : columnCount));
	}

	function periodName(column: MatrixColumn): string {
		if (column.name) return column.name;
		if (!column.start) return '';
		const d = new Date(column.start + 'T00:00:00');
		switch (viewBy) {
			case 2:
				return d.toLocaleDateString(locale, { month: 'short', year: 'numeric' });
			case 4:
				return String(d.getFullYear());
			default:
				return matrix.calculated && column.end && column.end !== column.start
					? `${formatDate(column.start, locale)}..${formatDate(column.end, locale)}`
					: formatDate(column.start, locale);
		}
	}

	function cellText(value: string, column: MatrixColumn): string {
		if (matrix.calculated) {
			// as calculated (rounded by the column), a text (an underline) as it is
			const text = (value ?? '').trim();
			if (!/^-?\d+(\.\d+)?$/.test(text)) return text;
			return formatDecimal(text, column.decimal_places ?? '2:2') + (column.percent ? ' %' : '');
		}
		const shown = matrixCellValue(value, roundingFactor);
		return shown ? formatDecimal(shown.value, shown.places) : '';
	}

	function rowField(row: MatrixRow, source: string): string {
		const v = row.fields[source];
		return v === undefined || v === null ? '' : String(v);
	}

	// --- selection and editing ---

	async function focusGrid() {
		await tick();
		const cell = gridElement?.querySelector<HTMLElement>(`[data-cell="${selRow}-${selCol}"]`);
		cell?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
		if (!editing) gridElement?.focus({ preventScroll: true });
	}

	function select(row: number, col: number) {
		if (rows.length === 0 || columns.length === 0) return;
		selRow = Math.max(0, Math.min(rows.length - 1, row));
		selCol = Math.max(0, Math.min(columns.length - 1, col));
		focusGrid();
	}

	async function startEdit(initial?: string) {
		if (!matrix.editable || selRow < 0 || selCol < 0) return;
		const shown = matrixCellValue(rows[selRow].values[selCol], roundingFactor);
		editText = initial ?? (shown ? toDecimalInput(shown.value, locale) : '');
		editRow = selRow;
		editCol = selCol;
		editing = true;
		await tick();
		const input = gridElement?.querySelector<HTMLInputElement>('input.cell-input');
		input?.focus();
		if (initial === undefined) input?.select();
	}

	function cancelEdit() {
		editing = false;
		focusGrid();
	}

	// Confirm the typed value, then move by (dRow, dCol) from the cell — move: false when the
	// selection already moved (a click on another cell)
	async function confirmEdit(dRow: number, dCol: number, move = true) {
		if (saving || editRow < 0) return;
		const r0 = editRow;
		const c0 = editCol;
		editRow = -1;
		const row = rows[r0];
		const column = columns[c0];
		const text = editText.trim();
		const shown = matrixCellValue(row.values[c0], roundingFactor);
		const unchanged = text === (shown ? toDecimalInput(shown.value, locale) : '');
		editing = false;
		if (!unchanged) {
			saving = true;
			error = '';
			try {
				await api.matrixValidate(pageId, {
					...request(),
					key: row.key,
					start: column.start ?? '',
					end: column.end ?? '',
					value: text === '' ? '0' : fromDecimalInput(text, locale),
					rounding_factor: roundingFactor
				});
				await load();
			} catch (e) {
				error = e instanceof Error ? e.message : String(e);
			} finally {
				saving = false;
			}
		}
		if (move) select(r0 + dRow, c0 + dCol);
	}

	function handleGridKey(e: KeyboardEvent) {
		if (editing || selRow < 0) return;
		switch (e.key) {
			case 'ArrowUp':
				select(selRow - 1, selCol);
				break;
			case 'ArrowDown':
				select(selRow + 1, selCol);
				break;
			case 'ArrowLeft':
				select(selRow, selCol - 1);
				break;
			case 'ArrowRight':
				select(selRow, selCol + 1);
				break;
			case 'Home':
				select(e.ctrlKey ? 0 : selRow, 0);
				break;
			case 'End':
				select(e.ctrlKey ? rows.length - 1 : selRow, columns.length - 1);
				break;
			case 'PageUp':
				select(selRow - 10, selCol);
				break;
			case 'PageDown':
				select(selRow + 10, selCol);
				break;
			case 'Tab':
				if (e.shiftKey ? selCol === 0 : selCol === columns.length - 1) return; // leave the matrix
				select(selRow, selCol + (e.shiftKey ? -1 : 1));
				break;
			case 'Enter':
			case 'F2':
				startEdit();
				break;
			case 'Delete':
				if (!matrix.editable) return;
				editText = '';
				editRow = selRow;
				editCol = selCol;
				confirmEdit(0, 0);
				break;
			default:
				if (e.key.length === 1 && !e.ctrlKey && !e.altKey && !e.metaKey && /[\d.,\-]/.test(e.key)) {
					startEdit(e.key);
					break;
				}
				return;
		}
		e.preventDefault();
		e.stopPropagation();
	}

	function handleInputKey(e: KeyboardEvent) {
		switch (e.key) {
			case 'Enter':
				confirmEdit(1, 0);
				break;
			case 'Tab':
				confirmEdit(0, e.shiftKey ? -1 : 1);
				break;
			case 'Escape':
				cancelEdit();
				break;
			case 'ArrowUp':
				confirmEdit(-1, 0);
				break;
			case 'ArrowDown':
				confirmEdit(1, 0);
				break;
			default:
				return;
		}
		e.preventDefault();
		e.stopPropagation();
	}

	// Show Entries: the drilldown page filtered to the row, the column's dates, the card record
	// and the user's filters
	function drilldown(rowIndex = selRow, colIndex = selCol) {
		if (!matrix.drilldown || !ondrilldown || rowIndex < 0 || colIndex < 0) return;
		const row = rows[rowIndex];
		const column = columns[colIndex];
		const filters: TableFilter[] = [];
		if (matrix.drilldown_row_field && row.drilldown) {
			filters.push({ field: matrix.drilldown_row_field, expression: row.drilldown });
		}
		if (matrix.drilldown_date_field) {
			const range = viewAs === 1 ? `..${column.end}` : `${column.start}..${column.end}`;
			filters.push({ field: matrix.drilldown_date_field, expression: range });
		}
		for (const [field, recordField] of Object.entries(matrix.drilldown_record_filters ?? {})) {
			const value = record?.[recordField];
			if (value !== undefined && value !== null && value !== '') filters.push({ field, expression: String(value) });
		}
		for (const f of matrix.filters ?? []) {
			const expression = (filterValues[f.source] ?? '').trim();
			if (expression && f.drilldown_filter_field) filters.push({ field: f.drilldown_filter_field, expression });
		}
		ondrilldown(matrix.drilldown, filters);
	}
</script>

<div class="matrix" data-own-keys>
	<div class="grid-controls">
		{#if !matrix.no_periods}
		<div class="control-group" role="radiogroup" aria-label={t(PERIOD.VIEW_BY)}>
			<span class="control-label">{t(PERIOD.VIEW_BY)}</span>
			{#each viewByChoices as choice}
				<button type="button" class="choice" class:active={viewBy === choice.value} aria-pressed={viewBy === choice.value} onclick={() => setViewBy(choice.value)}>
					{t(choice.key)}
				</button>
			{/each}
		</div>
		{#if !matrix.calculated}
		<div class="control-group" role="radiogroup" aria-label={t(PERIOD.VIEW_AS)}>
			<span class="control-label">{t(PERIOD.VIEW_AS)}</span>
			<button type="button" class="choice" class:active={viewAs === 0} aria-pressed={viewAs === 0} onclick={() => (viewAs = 0)}>
				{t(PERIOD.NET_CHANGE)}
			</button>
			<button type="button" class="choice" class:active={viewAs === 1} aria-pressed={viewAs === 1} onclick={() => (viewAs = 1)}>
				{t(PERIOD.BALANCE_AT_DATE)}
			</button>
		</div>
		<div class="control-group" role="radiogroup" aria-label={t(PERIOD.ROUNDING_FACTOR)}>
			<span class="control-label">{t(PERIOD.ROUNDING_FACTOR)}</span>
			{#each roundingFactors as factor}
				<button type="button" class="choice" class:active={roundingFactor === factor} aria-pressed={roundingFactor === factor} onclick={() => (roundingFactor = factor)}>
					{factor === 0 ? t(PERIOD.ROUNDING_NONE) : ['', '1', '1000', '1000000'][factor]}
				</button>
			{/each}
		</div>
		{/if}
		<div class="control-group">
			<button type="button" class="choice" title={t(PERIOD.PREVIOUS)} aria-label={t(PERIOD.PREVIOUS)} onclick={() => move(-1)}>‹</button>
			{#if matrix.calculated && period}
				<span class="current-period">{periodName(period)}</span>
			{/if}
			<button type="button" class="choice" title={t(PERIOD.NEXT)} aria-label={t(PERIOD.NEXT)} onclick={() => move(1)}>›</button>
		</div>
		{/if}
		{#if matrix.drilldown && ondrilldown}
			<div class="control-group">
				<button type="button" class="choice" disabled={selRow < 0 || selCol < 0} onclick={() => drilldown()}>{t(PERIOD.SHOW_ENTRIES)}</button>
			</div>
		{/if}
	</div>

	{#if (matrix.filters ?? []).length > 0}
		<div class="grid-filters">
			{#each matrix.filters ?? [] as filter (filter.source)}
				<label class="filter-field">
					<span>{filter.caption ?? filter.source}</span>
					{#if filter.check_box}
						<input
							type="checkbox"
							class="filter-check"
							checked={filterValues[filter.source] === 'YES'}
							onchange={(e) => {
								filterValues[filter.source] = (e.currentTarget as HTMLInputElement).checked ? 'YES' : '';
								load();
							}}
						/>
					{:else}
						<input
							type="text"
							class="filter-input"
							value={filterValues[filter.source] ?? ''}
							onchange={(e) => {
								filterValues[filter.source] = (e.currentTarget as HTMLInputElement).value.toUpperCase();
								load();
							}}
						/>
					{/if}
				</label>
			{/each}
		</div>
	{/if}

	{#if error}
		<p class="grid-error" role="alert">{error}</p>
	{/if}

	<div
		class="grid-table-wrap"
		class:loading
		bind:this={gridElement}
		tabindex="0"
		role="grid"
		aria-readonly={!matrix.editable}
		onkeydown={handleGridKey}
		onfocus={() => {
			if (selRow < 0 && rows.length > 0) select(0, 0);
		}}
	>
		<table class="grid-table">
			<thead>
				<tr>
					{#each matrix.fields as field (field.source)}
						<th class="row-field" style:min-width={field.width ? `${field.width}px` : undefined}>{field.caption ?? field.source}</th>
					{/each}
					{#each columns as column, c (c)}
						<th class="num">{matrix.calculated ? column.name : periodName(column)}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each rows as row, r (row.key)}
					<tr>
						{#each matrix.fields as field (field.source)}
							{@const cs = cellStyle(field, row.fields)}
							<td class="row-field {cs.classes}" style={cs.style}>{rowField(row, field.source)}</td>
						{/each}
						{#each columns as column, c (c)}
							{@const strong = cellStyle(matrix.fields[0], row.fields).classes}
							<td
								class="num cell {strong}"
								class:selected={r === selRow && c === selCol}
								data-cell="{r}-{c}"
								onmousedown={(e) => {
									if (editing && r === editRow && c === editCol) return; // in the input
									e.preventDefault(); // the grid keeps the focus (an input being edited blurs and saves)
									if (editing) confirmEdit(0, 0, false);
									select(r, c);
								}}
								ondblclick={() => {
									select(r, c);
									if (matrix.editable) startEdit();
									else drilldown(r, c);
								}}
							>
								{#if editing && r === selRow && c === selCol}
									<input
										class="cell-input"
										type="text"
										inputmode="decimal"
										bind:value={editText}
										onkeydown={handleInputKey}
										onblur={() => {
											if (editing) confirmEdit(0, 0, false);
										}}
									/>
								{:else}
									{cellText(row.values[c], column)}
								{/if}
							</td>
						{/each}
					</tr>
				{:else}
					{#if !loading}
						<tr><td colspan={matrix.fields.length + columns.length} class="empty">{t(PERIOD.NO_ROWS)}</td></tr>
					{/if}
				{/each}
			</tbody>
		</table>
	</div>
</div>

<style>
	.matrix {
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

	.choice:disabled {
		opacity: 0.5;
		cursor: default;
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

	.filter-check {
		width: 1rem;
		height: 1rem;
		margin: 0.4rem 0;
		accent-color: rgb(var(--accent-600));
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
		overflow: auto;
		max-height: 70vh;
		outline: none;
	}

	.grid-table-wrap.loading {
		opacity: 0.6;
	}

	.grid-table {
		border-collapse: collapse;
		font-size: 0.875rem;
	}

	.grid-table th,
	.grid-table td {
		padding: 0.25rem 0.625rem;
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
		position: sticky;
		top: 0;
		z-index: 1;
		background-color: rgb(var(--surface));
	}

	:global(.dark) .grid-table th {
		background-color: rgb(var(--gray-800));
	}

	.grid-table .num {
		text-align: right;
		min-width: 7rem;
	}

	.cell {
		cursor: cell;
	}

	.cell.selected {
		outline: 2px solid rgb(var(--accent-600));
		outline-offset: -2px;
	}

	.cell-input {
		@apply bg-surface text-fg;
		width: 100%;
		min-width: 6rem;
		text-align: right;
		border: none;
		outline: none;
		padding: 0;
		font: inherit;
	}

	.empty {
		color: rgb(var(--gray-500));
		text-align: center;
	}
</style>
