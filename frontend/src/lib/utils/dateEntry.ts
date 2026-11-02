// Date entry: what the user types in a date field, read as classic ERP did — short digits
// filled from the work date (05 = the 5th of the work date's month, 0509, 050926, 05092026), any
// separator (5.9, 5/9/26, 2026-09-05), t = today, w = the work date, weekdays (mo, tu … — also
// Norwegian and Danish) of the work date's week, and date formulas from the work date (+1W, -3D,
// CM, -1M+CM). Day and month follow the user's date order (My Settings). A C before a date is
// its closing date (C311226 = C2026-12-31: the period end closing entries are posted on).

import { applyDateFormula } from './dateFormula';
import { dateOrder, getWorkDate, type DateOrder } from './regional';

/** Weekday names (≥ 2 letters of them) in the languages of the app: index 1 = Monday … 7 */
const weekdayNames: string[][] = [
	[],
	['monday', 'mandag'],
	['tuesday', 'tirsdag'],
	['wednesday', 'onsdag'],
	['thursday', 'torsdag'],
	['friday', 'fredag'],
	['saturday', 'lørdag', 'lordag'],
	['sunday', 'søndag', 'sondag']
];

/** A two-digit year in the window 1930–2029 */
function fullYear(year: number, digits: number): number {
	if (digits > 2) return year;
	return year < 30 ? 2000 + year : 1900 + year;
}

/** A real date of year, month (1-12) and day, else null */
function makeDate(year: number, month: number, day: number): Date | null {
	if (month < 1 || month > 12 || day < 1 || day > 31 || year < 1 || year > 9999) return null;
	const d = new Date(year, month - 1, day);
	if (d.getFullYear() !== year || d.getMonth() !== month - 1 || d.getDate() !== day) return null;
	return d;
}

/** The day, month and (blank: the work date's) year of number parts in the user's order */
function fromParts(parts: string[], order: DateOrder, base: Date): Date | null {
	const n = parts.map(Number);
	if (parts.length === 1) return makeDate(base.getFullYear(), base.getMonth() + 1, n[0]);
	// a four-digit first part is a year (ISO 2026-09-05, also 2026-09)
	const yearFirst = order === 'YMD' || parts[0].length === 4;
	if (parts.length === 2) {
		if (yearFirst && parts[0].length === 4) return makeDate(n[0], n[1], 1);
		const [day, month] = order === 'MDY' || order === 'YMD' ? [n[1], n[0]] : [n[0], n[1]];
		return makeDate(base.getFullYear(), month, day);
	}
	if (parts.length === 3) {
		if (yearFirst) return makeDate(fullYear(n[0], parts[0].length), n[1], n[2]);
		if (order === 'MDY') return makeDate(fullYear(n[2], parts[2].length), n[0], n[1]);
		return makeDate(fullYear(n[2], parts[2].length), n[1], n[0]);
	}
	return null;
}

/** Digits without separators split into parts by their count and the user's order */
function digitParts(digits: string, order: DateOrder): string[] | null {
	switch (digits.length) {
		case 1:
		case 2:
			return [digits];
		case 3: // 509: 5.09 (day first) or 5/09 (month first)
			return [digits.slice(0, 1), digits.slice(1)];
		case 4:
			return [digits.slice(0, 2), digits.slice(2)];
		case 6:
			return order === 'YMD' ? [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4)] : [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4)];
		case 8:
			return order === 'YMD' ? [digits.slice(0, 4), digits.slice(4, 6), digits.slice(6)] : [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4)];
	}
	return null;
}

/**
 * The date a user's entry gives, null when it cannot be read ('' is a blank date: the caller
 * clears the field). options: the date order (default the user's) and the work date (default the
 * user's, else today).
 */
export function parseDateEntry(input: string, options: { locale?: string; order?: DateOrder; workDate?: Date; today?: Date } = {}): Date | null {
	const text = input.trim().toLowerCase();
	if (text === '') return null;
	const order = options.order ?? dateOrder(options.locale ?? 'en-US');
	const base = options.workDate ?? getWorkDate();
	const now = options.today ?? new Date();
	const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());

	if (text === 't' || text === 'today') return today;
	if (text === 'w' || text === 'workdate') return new Date(base.getFullYear(), base.getMonth(), base.getDate());
	if (/^[a-zæøå]{2,}$/.test(text)) {
		for (let weekday = 1; weekday <= 7; weekday++) {
			if (weekdayNames[weekday].some((name) => name.startsWith(text))) {
				const isoWeekday = base.getDay() === 0 ? 7 : base.getDay();
				return new Date(base.getFullYear(), base.getMonth(), base.getDate() + weekday - isoWeekday);
			}
		}
	}
	// a date formula from the work date (letters: D W M Q Y C)
	if (/[a-z]/.test(text)) return applyDateFormula(text, base);

	if (/^\d+$/.test(text)) {
		const parts = digitParts(text, order);
		if (!parts) return null;
		if (parts.length === 2 && order === 'YMD') return fromParts([parts[0], parts[1]], 'MDY', base);
		return fromParts(parts, order, base);
	}
	const parts = text.split(/[.\/\-\s]+/).filter((p) => p !== '');
	if (parts.length === 0 || parts.some((p) => !/^\d+$/.test(p))) return null;
	return fromParts(parts, order, base);
}

/** A date as ISO (YYYY-MM-DD) */
export function isoDate(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** The ISO date of an entry, '' for a blank entry, null when it cannot be read; a closing date
 * as C2026-12-31 */
export function dateEntryToIso(input: string, options: { locale?: string; order?: DateOrder; workDate?: Date; today?: Date } = {}): string | null {
	if (input.trim() === '') return '';
	// C and a date (digits, t or workdate — not CM, CW … the formulas): the date's closing date
	const closing = /^c\s*(\d.*|t|today|workdate)$/i.exec(input.trim());
	if (closing) {
		const d = parseDateEntry(closing[1], options);
		return d ? 'C' + isoDate(d) : null;
	}
	const d = parseDateEntry(input, options);
	return d ? isoDate(d) : null;
}
