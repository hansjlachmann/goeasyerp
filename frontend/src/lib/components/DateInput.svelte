<script lang="ts">
	// A date field: shows the date in the user's format (My Settings) and reads what the user
	// types when the field is left (Tab, Enter, the cursor elsewhere) — short entries (05, 0509,
	// 050926), any separator, t, w, weekdays and date formulas (utils/dateEntry.ts). An entry that
	// is not a date is refused: an error and the cursor stays. Alt+↓ / F4 / the button open a
	// calendar (arrows, PageUp/PageDown months, Enter picks, Esc closes). The value is ISO
	// (YYYY-MM-DD, '' blank); other attributes (data-row, data-col, class, tabindex …) go on the
	// input, so lists and cards focus it like any input.
	import type { HTMLInputAttributes } from 'svelte/elements';
	import { floating } from '#lib/actions/floating.js';
	import { dateEntryToIso, isoDate } from '#lib/utils/dateEntry.js';
	import { formatDateRegional, datePattern, getWorkDate } from '#lib/utils/regional.js';
	import { currentLanguage } from '#lib/stores/session.js';
	import { toast } from '#lib/stores/toast.js';
	import { t, DATE } from '#lib/services/i18n.svelte.js';

	interface Props extends Omit<HTMLInputAttributes, 'value' | 'onchange' | 'onblur' | 'onkeydown'> {
		value: string | null | undefined;
		onchange?: (iso: string) => void;
		onblur?: (e: FocusEvent) => void;
		onkeydown?: (e: KeyboardEvent) => void;
		/** No calendar button (list cells: the calendar opens with Alt+↓ / F4) */
		compact?: boolean;
		inputRef?: HTMLInputElement | null;
	}

	let { value, onchange, onblur, onkeydown, compact = false, inputRef = $bindable(null), class: className = '', ...rest }: Props = $props();

	let locale = $state('en-US');
	currentLanguage.subscribe((lang) => (locale = lang || 'en-US'));

	const shown = (v: string | null | undefined) => formatDateRegional(String(v ?? ''), locale);
	let text = $state('');
	let dirty = $state(false);
	let invalid = $state(false);
	// the field shows the value as long as the user has not typed; a value that is not a date
	// yet (a list cell started by typing a character) is typed text
	$effect(() => {
		const v = value;
		if (dirty) return;
		if (v && !/^\d{4}-\d{2}-\d{2}/.test(String(v))) {
			text = String(v);
			dirty = true;
		} else {
			text = shown(v);
		}
	});

	/** Reads the typed text; false (an error, the cursor stays) when it is not a date */
	function commit(): boolean {
		if (!dirty) return true;
		const iso = dateEntryToIso(text, { locale });
		if (iso === null) {
			invalid = true;
			toast.error(t(DATE.INVALID, text));
			return false;
		}
		invalid = false;
		dirty = false;
		text = shown(iso);
		if (iso !== (value ?? '')) onchange?.(iso);
		return true;
	}

	// --- the calendar ---
	let calendarOpen = $state(false);
	let month = $state(new Date()); // the first day of the month shown
	let cursor = $state(new Date()); // the highlighted day

	function startDate(): Date {
		const iso = dirty ? dateEntryToIso(text, { locale }) : value;
		const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso ?? '');
		return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : getWorkDate();
	}
	function openCalendar() {
		cursor = startDate();
		month = new Date(cursor.getFullYear(), cursor.getMonth(), 1);
		calendarOpen = true;
	}
	function moveCursor(days: number, months = 0) {
		const next = months ? new Date(cursor.getFullYear(), cursor.getMonth() + months, Math.min(cursor.getDate(), 28)) : new Date(cursor.getFullYear(), cursor.getMonth(), cursor.getDate() + days);
		cursor = next;
		month = new Date(next.getFullYear(), next.getMonth(), 1);
	}
	function pick(day: Date) {
		const iso = isoDate(day);
		calendarOpen = false;
		dirty = false;
		invalid = false;
		text = shown(iso);
		if (iso !== (value ?? '')) onchange?.(iso);
		inputRef?.focus();
	}
	/** The days shown: Monday of the first week to Sunday of the last */
	const days = $derived.by(() => {
		const first = new Date(month.getFullYear(), month.getMonth(), 1);
		const offset = (first.getDay() + 6) % 7;
		const start = new Date(first.getFullYear(), first.getMonth(), 1 - offset);
		return Array.from({ length: 42 }, (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i));
	});
	const weekdays = $derived.by(() => {
		const fmt = new Intl.DateTimeFormat(locale, { weekday: 'short' });
		return Array.from({ length: 7 }, (_, i) => fmt.format(new Date(2026, 0, 5 + i))); // 5.1.2026 is a Monday
	});
	const monthTitle = $derived(new Intl.DateTimeFormat(locale, { month: 'long', year: 'numeric' }).format(month));
	const sameDay = (a: Date, b: Date) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
	const today = new Date();

	function handleKeydown(e: KeyboardEvent) {
		if (calendarOpen) {
			const moves: Record<string, () => void> = {
				ArrowLeft: () => moveCursor(-1),
				ArrowRight: () => moveCursor(1),
				ArrowUp: () => moveCursor(-7),
				ArrowDown: () => moveCursor(7),
				PageUp: () => moveCursor(0, -1),
				PageDown: () => moveCursor(0, 1)
			};
			if (moves[e.key]) {
				e.preventDefault();
				moves[e.key]();
				return;
			}
			if (e.key === 'Enter') {
				e.preventDefault();
				pick(cursor);
				return;
			}
			if (e.key === 'Escape' || e.key === 'F4' || (e.key === 'ArrowUp' && e.altKey)) {
				e.preventDefault();
				calendarOpen = false;
				return;
			}
			if (e.key === 'Tab') calendarOpen = false;
		} else if ((e.key === 'ArrowDown' && e.altKey) || e.key === 'F4') {
			e.preventDefault();
			openCalendar();
			return;
		}
		// keys that leave the field (in a list also the arrows at the edge of the text) read it first
		const input = e.currentTarget as HTMLInputElement;
		const atStart = input.selectionStart === 0 && input.selectionEnd === 0;
		const atEnd = input.selectionStart === input.value.length;
		const leaving =
			e.key === 'Enter' || e.key === 'Tab' || e.key === 'ArrowUp' || e.key === 'ArrowDown' || e.key === 'PageUp' || e.key === 'PageDown' ||
			(e.key === 'ArrowLeft' && atStart) || (e.key === 'ArrowRight' && atEnd);
		if (leaving && !commit()) {
			e.preventDefault();
			return;
		}
		if (e.key === 'Escape' && dirty) {
			dirty = false;
			invalid = false;
			text = shown(value);
		}
		onkeydown?.(e);
	}

	function handleBlur(e: FocusEvent) {
		calendarOpen = false;
		if (!commit()) {
			// the cursor stays in the field until it holds a date
			setTimeout(() => inputRef?.focus(), 0);
			return;
		}
		onblur?.(e);
	}
</script>

<span class="date-input" data-own-keys={calendarOpen ? '' : undefined}>
	<input
		{...rest}
		bind:this={inputRef}
		type="text"
		autocomplete="off"
		class={invalid ? `${className} input-error` : className}
		placeholder={rest.placeholder ?? datePattern(locale)}
		value={text}
		aria-invalid={invalid || rest['aria-invalid']}
		oninput={(e) => {
			text = (e.currentTarget as HTMLInputElement).value;
			dirty = true;
			invalid = false;
		}}
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>
	{#if !compact}
		<button
			type="button"
			class="calendar-button"
			tabindex="-1"
			title={t(DATE.CALENDAR)}
			aria-label={t(DATE.CALENDAR)}
			onmousedown={(e) => e.preventDefault()}
			onclick={() => {
				if (calendarOpen) calendarOpen = false;
				else openCalendar();
				inputRef?.focus();
			}}
		>
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M8 3v4M16 3v4M3 10h18" /></svg>
		</button>
	{/if}
	{#if calendarOpen}
		<!-- the calendar keeps the focus in the field (mousedown) -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="calendar" use:floating={inputRef} onmousedown={(e) => e.preventDefault()}>
			<div class="calendar-head">
				<button type="button" tabindex="-1" aria-label={t(DATE.PREV_MONTH)} onclick={() => moveCursor(0, -1)}>‹</button>
				<span class="calendar-title">{monthTitle}</span>
				<button type="button" tabindex="-1" aria-label={t(DATE.NEXT_MONTH)} onclick={() => moveCursor(0, 1)}>›</button>
			</div>
			<div class="calendar-grid">
				{#each weekdays as wd (wd)}
					<span class="weekday">{wd}</span>
				{/each}
				{#each days as day (day.getTime())}
					<button
						type="button"
						tabindex="-1"
						class="day"
						class:other-month={day.getMonth() !== month.getMonth()}
						class:today={sameDay(day, today)}
						class:cursor={sameDay(day, cursor)}
						onclick={() => pick(day)}>{day.getDate()}</button
					>
				{/each}
			</div>
		</div>
	{/if}
</span>

<style>
	.date-input {
		position: relative;
		display: block;
		width: 100%;
	}
	.date-input input {
		width: 100%;
	}
	.date-input:has(.calendar-button) input {
		padding-right: 2rem;
	}
	.calendar-button {
		position: absolute;
		right: 0.4rem;
		top: 50%;
		transform: translateY(-50%);
		width: 1.2rem;
		height: 1.2rem;
		color: rgb(var(--gray-500));
	}
	.calendar-button:hover {
		color: rgb(var(--fg));
	}
	.calendar {
		background: rgb(var(--surface));
		border: 1px solid rgb(var(--gray-300));
		border-radius: 0.375rem;
		box-shadow: 0 8px 24px rgb(0 0 0 / 0.2);
		padding: 0.5rem;
		min-width: 15rem !important;
		width: 15rem;
		font-size: 0.85rem;
	}
	:global(.dark) .calendar {
		background: rgb(var(--gray-800));
		border-color: rgb(var(--gray-700));
	}
	.calendar-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.25rem;
	}
	.calendar-head button {
		width: 1.75rem;
		height: 1.75rem;
		border-radius: 0.25rem;
		font-size: 1.1rem;
	}
	.calendar-head button:hover,
	.day:hover {
		background: rgb(var(--gray-200));
	}
	:global(.dark) .calendar-head button:hover,
	:global(.dark) .day:hover {
		background: rgb(var(--gray-700));
	}
	.calendar-title {
		font-weight: 600;
		text-transform: capitalize;
	}
	.calendar-grid {
		display: grid;
		grid-template-columns: repeat(7, 1fr);
		gap: 1px;
		text-align: center;
	}
	.weekday {
		font-size: 0.7rem;
		color: rgb(var(--gray-500));
		padding: 0.15rem 0;
	}
	.day {
		padding: 0.25rem 0;
		border-radius: 0.25rem;
	}
	.day.other-month {
		color: rgb(var(--gray-400));
	}
	.day.today {
		font-weight: 700;
		color: rgb(var(--accent-600));
	}
	.day.cursor {
		background: rgb(var(--accent-600));
		color: rgb(var(--surface));
	}
</style>
