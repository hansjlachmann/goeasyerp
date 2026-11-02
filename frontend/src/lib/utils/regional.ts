// The user's regional settings (My Settings): how dates and numbers are shown and entered, and
// the work date. The store (stores/regional.ts) sets them here, so formatting functions anywhere
// can use them; blank formats follow the UI language (Intl).

export type DateOrder = 'DMY' | 'MDY' | 'YMD';

let dateFormat = ''; // '' | 'dd.mm.yy' | 'dd.mm.yyyy' | 'dd/mm/yyyy' | 'mm/dd/yyyy' | 'yyyy-mm-dd'
let numberFormat = ''; // '' | '1,234.56' | '1.234,56' | '1 234,56' | '1234.56'
let workDate = ''; // ISO; '' = today
let language = 'en-US'; // the UI language (the formats when the user has none)

/** The UI language (the session's): the formats of users without their own */
export function setRegionalLanguage(lang: string) {
	language = lang || 'en-US';
}

/** The UI language set with setRegionalLanguage */
export function regionalLanguage(): string {
	return language;
}

/** Sets the user's settings (the regional store does, after loading or saving them) */
export function setRegional(settings: { date_format?: string; number_format?: string; work_date?: string }) {
	dateFormat = settings.date_format ?? '';
	numberFormat = settings.number_format ?? '';
	workDate = settings.work_date ?? '';
}

/** The work date: the user's, else today (local midnight) */
export function getWorkDate(): Date {
	const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(workDate);
	if (m) return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
	const now = new Date();
	return new Date(now.getFullYear(), now.getMonth(), now.getDate());
}

/** The language's date parts in order and its separator (Intl) */
function intlDateParts(locale: string): { order: DateOrder; separator: string } {
	try {
		const parts = new Intl.DateTimeFormat(locale, { year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date(2026, 0, 15));
		const kinds = parts.filter((p) => p.type === 'day' || p.type === 'month' || p.type === 'year').map((p) => p.type[0]).join('');
		const separator = parts.find((p) => p.type === 'literal')?.value ?? '/';
		const order: DateOrder = kinds.startsWith('y') ? 'YMD' : kinds.startsWith('m') ? 'MDY' : 'DMY';
		return { order, separator };
	} catch {
		return { order: 'MDY', separator: '/' };
	}
}

/** The order of day, month and year the user reads and types dates in */
export function dateOrder(locale: string): DateOrder {
	if (dateFormat.startsWith('dd')) return 'DMY';
	if (dateFormat.startsWith('mm')) return 'MDY';
	if (dateFormat.startsWith('yyyy')) return 'YMD';
	return intlDateParts(locale).order;
}

/** A date (ISO, also with a time) shown in the user's format; a closing date (C2026-12-31) with
 * its C in front */
export function formatDateRegional(isoDate: string, locale: string): string {
	if (!isoDate) return '';
	if (/^C\d{4}-\d{2}-\d{2}$/.test(isoDate)) return 'C' + formatDateRegional(isoDate.slice(1), locale);
	const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(isoDate);
	if (!m) return isoDate;
	const [, y, mo, d] = m;
	switch (dateFormat) {
		case 'dd.mm.yy':
			return `${d}.${mo}.${y.slice(2)}`;
		case 'dd.mm.yyyy':
			return `${d}.${mo}.${y}`;
		case 'dd/mm/yyyy':
			return `${d}/${mo}/${y}`;
		case 'mm/dd/yyyy':
			return `${mo}/${d}/${y}`;
		case 'yyyy-mm-dd':
			return `${y}-${mo}-${d}`;
	}
	try {
		return new Intl.DateTimeFormat(locale, { year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(Number(y), Number(mo) - 1, Number(d)));
	} catch {
		return isoDate;
	}
}

/** The pattern of the user's date format (an empty date field's hint): DD.MM.YY … */
export function datePattern(locale: string): string {
	if (dateFormat) return dateFormat.toUpperCase();
	const { order, separator } = intlDateParts(locale);
	return order.split('').map((c) => ({ D: 'DD', M: 'MM', Y: 'YYYY' })[c]).join(separator);
}

/** The thousands and decimal separators of the user's number format */
export function numberSeparators(locale: string): { thousands: string; decimal: string } {
	switch (numberFormat) {
		case '1,234.56':
			return { thousands: ',', decimal: '.' };
		case '1.234,56':
			return { thousands: '.', decimal: ',' };
		case '1 234,56':
			return { thousands: ' ', decimal: ',' };
		case '1234.56':
			return { thousands: '', decimal: '.' };
	}
	try {
		const parts = new Intl.NumberFormat(locale).formatToParts(1234567.5);
		const group = parts.find((p) => p.type === 'group')?.value ?? ',';
		return { thousands: group === ' ' || group === ' ' ? ' ' : group, decimal: parts.find((p) => p.type === 'decimal')?.value ?? '.' };
	} catch {
		return { thousands: ',', decimal: '.' };
	}
}

/** A plain decimal text ("-1234.5") in the user's format, grouped */
export function formatNumberText(text: string, locale: string = language): string {
	const m = /^(-?)(\d+)(?:\.(\d*))?$/.exec(text);
	if (!m) return text;
	const { thousands, decimal } = numberSeparators(locale);
	const int = m[2].replace(/\B(?=(\d{3})+(?!\d))/g, thousands);
	return m[1] + int + (m[3] ? decimal + m[3] : '');
}

/** A decimal as the user edits it: the user's decimal separator, no grouping */
export function toDecimalInput(value: unknown, locale: string): string {
	if (value === null || value === undefined) return '';
	const text = String(value);
	const { decimal } = numberSeparators(locale);
	return /^-?\d+(\.\d*)?$/.test(text) && decimal !== '.' ? text.replace('.', decimal) : text;
}

/** A decimal as the user typed it, for the API: grouping removed, the decimal point a dot */
export function fromDecimalInput(text: string, locale: string): string {
	const { thousands, decimal } = numberSeparators(locale);
	let t = text.replace(/[\s  ]/g, '');
	if (thousands && thousands !== ' ') t = t.split(thousands).join('');
	if (decimal !== '.') t = t.replace(decimal, '.');
	return t;
}
