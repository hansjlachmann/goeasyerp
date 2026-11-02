// Matrix cells (card sections of records against periods): the amounts shown with a rounding
// factor (classic FormatAmount: none, whole numbers, thousands or millions with one decimal)

/** The rounding factors a matrix offers: 0 None, 1 "1", 2 "1000", 3 "1000000" */
export const roundingFactors = [0, 1, 2, 3] as const;

/** A plain decimal text with its point moved `places` digits to the left (1234.5, 3 → 1.2345) */
export function shiftDecimal(text: string, places: number): string {
	const m = /^(-?)(\d*)(?:\.(\d*))?$/.exec(text.trim());
	if (!m || places <= 0) return text;
	const digits = (m[2] || '0').padStart(places + 1, '0');
	const int = digits.slice(0, digits.length - places).replace(/^0+(?=\d)/, '');
	const frac = (digits.slice(digits.length - places) + (m[3] ?? '')).replace(/0+$/, '');
	return m[1] + int + (frac ? '.' + frac : '');
}

/**
 * A cell's value (a plain decimal text) as shown with the rounding factor: the value and the
 * decimal places to format it with; blank for zero
 */
export function matrixCellValue(value: string, roundingFactor: number): { value: string; places: string } | null {
	const text = (value ?? '').trim();
	if (text === '' || /^-?0*(\.0*)?$/.test(text)) return null;
	switch (roundingFactor) {
		case 1:
			return { value: text, places: '0:0' };
		case 2:
			return { value: shiftDecimal(text, 3), places: '1:1' };
		case 3:
			return { value: shiftDecimal(text, 6), places: '1:1' };
		default:
			return { value: text, places: '2:2' };
	}
}
