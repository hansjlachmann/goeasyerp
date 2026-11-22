// Text to the system clipboard: a list cell's value or rows (Ctrl+C), pasted into another
// program as text (rows as tab-separated lines, as a spreadsheet expects them)

// A number as lists show it: a sign, digits, the user's group and decimal separators, a % —
// it starts with - or + but is no formula, and stays a number in the spreadsheet
const NUMBER = /^[+-]?[\d.,'\s\u00a0\u202f]*\d[\d.,'\s\u00a0\u202f]*%?$/;

/** A text as a spreadsheet takes it as text (S29): a value starting with = + - @ (or a tab or
 *  line break before them) would run as a formula when the file is opened — e.g. a customer
 *  named =HYPERLINK(…) —, so it gets a leading ' (the spreadsheet's mark for text, not shown).
 *  Numbers are left as they are. */
export function spreadsheetSafe(value: string): string {
	return /^[=+\-@\t\r]/.test(value) && !NUMBER.test(value) ? `'${value}` : value;
}

// A value as one tab-separated field: tabs and line breaks would split it into other cells
export function tsvField(value: string): string {
	return spreadsheetSafe(value).replace(/[\t\r\n]+/g, ' ');
}

// The column captions and the rows as tab-separated lines
export function rowsToTsv(captions: string[], rows: string[][]): string {
	return [captions, ...rows].map((cells) => cells.map(tsvField).join('\t')).join('\n');
}

// Writes the text to the system clipboard. The Clipboard API needs a secure context
// (https or localhost); elsewhere a hidden text area and the copy command do it.
export async function copyText(text: string): Promise<boolean> {
	if (navigator.clipboard?.writeText && window.isSecureContext) {
		try {
			await navigator.clipboard.writeText(text);
			return true;
		} catch {
			// fall back below
		}
	}
	const area = document.createElement('textarea');
	area.value = text;
	area.setAttribute('readonly', '');
	area.style.position = 'fixed';
	area.style.opacity = '0';
	const focused = document.activeElement as HTMLElement | null;
	document.body.appendChild(area);
	area.select();
	let ok = false;
	try {
		ok = document.execCommand('copy');
	} catch {
		ok = false;
	}
	area.remove();
	focused?.focus();
	return ok;
}
