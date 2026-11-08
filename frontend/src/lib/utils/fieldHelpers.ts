import { runPagePath } from './runPage';
import { formatDateRegional, formatNumberText } from './regional';
import type { Field } from '$lib/types/pages';

// --- Date/DateTime type detection and formatting ---

/** Check if a field type represents a Date (from YAML "types.Date" or Go "Date") */
export function isDateType(fieldType: string | undefined): boolean {
	if (!fieldType) return false;
	return fieldType === 'types.Date' || fieldType === 'Date';
}

/** Check if a field type represents a Code (from YAML "types.Code" or Go "Code"): typed in any
 * case, uppercased when the field is left */
export function isCodeType(fieldType: string | undefined): boolean {
	if (!fieldType) return false;
	return fieldType === 'types.Code' || fieldType === 'Code' || fieldType === 'code';
}

/** Check if a field type represents a Decimal (YAML "types.Decimal" or Go "Decimal"): shown and
 * typed with the user's decimal separator */
export function isDecimalType(fieldType: string | undefined): boolean {
	if (!fieldType) return false;
	return fieldType === 'types.Decimal' || fieldType === 'Decimal' || fieldType === 'decimal';
}

/** Check if a field type is a whole number (YAML "int", Go "Integer") */
export function isIntegerType(fieldType: string | undefined): boolean {
	if (!fieldType) return false;
	return fieldType === 'int' || fieldType === 'int64' || fieldType === 'Integer' || fieldType === 'BigInteger';
}

/** Numbers (decimals and whole numbers) are aligned right, in lists, cards and footers */
export function isNumberType(fieldType: string | undefined): boolean {
	return isDecimalType(fieldType) || isIntegerType(fieldType);
}

/** Check if a field type represents a DateTime (from YAML "types.DateTime" or Go "DateTime") */
export function isDateTimeType(fieldType: string | undefined): boolean {
	if (!fieldType) return false;
	return fieldType === 'types.DateTime' || fieldType === 'DateTime';
}

/** Format an ISO date (YYYY-MM-DD) for display: the user's date format (My Settings), else the
 * language's */
export function formatDate(isoDate: string, locale: string): string {
	return formatDateRegional(isoDate, locale);
}

/** Format an ISO datetime string to locale display format */
export function formatDateTime(isoDateTime: string, locale: string): string {
	if (!isoDateTime) return '';
	try {
		const date = new Date(isoDateTime);
		if (isNaN(date.getTime())) return isoDateTime;
		return new Intl.DateTimeFormat(locale, {
			year: 'numeric',
			month: '2-digit',
			day: '2-digit',
			hour: '2-digit',
			minute: '2-digit'
		}).format(date);
	} catch {
		return isoDateTime;
	}
}

/** Value of an `<input type="datetime-local">` for an API datetime (ISO/RFC3339, usually UTC):
 * the user's local time as "YYYY-MM-DDTHH:mm". The input shows nothing for a value with a zone. */
export function toDateTimeInput(isoDateTime: unknown): string {
	if (!isoDateTime) return '';
	const date = new Date(String(isoDateTime));
	if (isNaN(date.getTime())) return '';
	const pad = (n: number) => String(n).padStart(2, '0');
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** API datetime (RFC3339 UTC) for the local time a `datetime-local` input holds; "" when cleared */
export function fromDateTimeInput(localDateTime: string): string {
	if (!localDateTime) return '';
	const date = new Date(localDateTime); // no zone: read as local time
	if (isNaN(date.getTime())) return '';
	return date.toISOString().replace(/\.\d{3}Z$/, 'Z');
}

/**
 * Get the display caption for a field
 * @param fieldSource - The field source name
 * @param captions - Record of field captions from the page definition
 * @param fieldCaption - Optional caption from the field definition
 * @returns The caption to display
 */
export function getFieldCaption(
	fieldSource: string,
	captions: Record<string, string>,
	fieldCaption?: string
): string {
	return captions[fieldSource] || fieldCaption || fieldSource;
}

/**
 * Get CSS classes for field styling based on field properties
 * @param field - The field object with style and importance properties
 * @returns Space-separated CSS class string
 */
export function getFieldStyleClasses(field: Field | { style?: string; importance?: string }): string {
	const classes: string[] = [];

	// Importance styling
	if (field.importance === 'Promoted') {
		classes.push('font-semibold');
	}

	// Style-based coloring
	switch (field.style) {
		case 'Strong':
			classes.push('text-heading dark:text-blue-400 font-bold');
			break;
		case 'Attention':
			classes.push('text-orange-600 dark:text-orange-400 font-medium');
			break;
		case 'Favorable':
			classes.push('text-green-600 dark:text-green-400');
			break;
		case 'Unfavorable':
			classes.push('text-red-600 dark:text-red-400');
			break;
	}

	return classes.join(' ');
}

/**
 * Format a value for display
 * @param val - The value to format
 * @returns Formatted string representation
 */
export function formatValue(val: any): string {
	if (val === null || val === undefined) {
		return '';
	}
	if (typeof val === 'boolean') {
		return val ? 'Yes' : 'No';
	}
	return String(val);
}

/**
 * Format an option field value (convert stored integer to display text)
 * @param value - The stored value (typically an integer index)
 * @param options - Map of option values to display labels
 * @returns The display label or formatted value
 */
export function formatOptionValue(
	value: any,
	options?: Record<string, string>
): string {
	if (value === undefined || value === null) {
		return '';
	}
	if (options) {
		const stringValue = String(value);
		return options[stringValue] || stringValue;
	}
	return formatValue(value);
}

/**
 * A lookup rendered as LookupDropdown (table-style dropdown): it has columns and either its
 * rows (small related table, sent along — also none: an empty related table still shows its
 * dropdown, as classic) or a lazy_url to load them on demand (large table).
 */
export function isAdvancedLookup(
	lookup?: { columns?: { source: string }[]; rows?: any[] | null; lazy_url?: string }
): boolean {
	return !!lookup?.columns?.length && (Array.isArray(lookup.rows) || !!lookup.lazy_url);
}

/**
 * Format a lookup field value for display
 * @param value - The stored key value
 * @param lookups - Lookup data with simple map or rows/columns
 * @param showKeyWithDescription - Whether to show "key - description" format
 * @returns The display value
 */
export function formatLookupValue(
	value: any,
	lookups?: { simple?: Record<string, string>; rows?: any[]; columns?: { source: string }[] },
	showKeyWithDescription: boolean = true
): string {
	if (value === undefined || value === null || value === '') {
		return '';
	}
	if (!lookups) {
		return String(value);
	}

	const stringValue = String(value);

	// For advanced lookup with rows, get description from first column
	if (lookups.rows && lookups.columns && lookups.columns.length > 0) {
		const row = lookups.rows.find(r => r._key === stringValue);
		if (row) {
			return row[lookups.columns[0].source] ?? stringValue;
		}
	}

	// For simple lookup
	if (lookups.simple) {
		const description = lookups.simple[stringValue];
		if (description && description !== stringValue && showKeyWithDescription) {
			return `${stringValue} - ${description}`;
		}
		if (description) {
			return description;
		}
	}

	return stringValue;
}

/**
 * Item customization type for visibility checks
 */
export interface ItemCustomization {
	visible: boolean;
	order?: number;
	section?: string;
}

/**
 * Check if a field/column is visible based on customizations
 * @param field - The field object with source and visible properties
 * @param customizations - User customizations for the page
 * @returns Whether the field should be visible
 */
export function isItemVisible(
	field: { source: string; visible?: boolean },
	customizations: Record<string, ItemCustomization>
): boolean {
	// If user has customized this field, use that preference
	if (field.source in customizations) {
		return customizations[field.source].visible;
	}
	// Otherwise use the field's visible property (default true)
	return field.visible !== false;
}

/**
 * Format a decimal with the field's decimal places (YAML `decimal_places: "min:max"`): at least
 * min and at most max digits after the point, rounded half away from zero (on the digits, no
 * float rounding). Without places or for a value that is not a plain decimal: as formatValue.
 */
export function formatDecimal(val: any, places: string | undefined): string {
	const spec = /^(\d+):(\d+)$/.exec(places ?? '');
	const text = formatValue(val).trim();
	const num = /^(-?)(\d*)(?:\.(\d*))?$/.exec(text);
	if (!spec || !num || text === '' || text === '-') return formatValue(val);
	const min = Number(spec[1]);
	const max = Number(spec[2]);
	let int = num[2] || '0';
	let frac = num[3] ?? '';
	if (frac.length > max) {
		const roundUp = frac[max] >= '5';
		frac = frac.slice(0, max);
		if (roundUp) {
			const digits = (BigInt(int + frac) + 1n).toString().padStart(int.length + max, '0');
			int = digits.slice(0, digits.length - max) || '0';
			frac = digits.slice(digits.length - max);
		}
	}
	frac = frac.replace(/0+$/, '').padEnd(min, '0');
	int = int.replace(/^0+(?=\d)/, '');
	const sign = num[1] && /[1-9]/.test(int + frac) ? '-' : '';
	// shown in the user's number format (My Settings), else the language's: grouped, its decimal
	// separator
	return formatNumberText(sign + int + (frac ? '.' + frac : ''));
}

/**
 * The title of a card: the page caption followed by the values of the table's
 * data_caption_fields of the record (option values as their captions), "Caption – 10000 · Name"
 */
export function cardTitle(
	caption: string,
	captionFields: string[] | undefined,
	record: Record<string, any> | undefined,
	options?: Record<string, Record<string, string>>
): string {
	const values = (captionFields ?? [])
		.map((f) => (options?.[f] ? formatOptionValue(record?.[f], options[f]) : formatValue(record?.[f])))
		.filter((v) => v.trim() !== '');
	return values.length ? `${caption} – ${values.join(' · ')}` : caption;
}

/**
 * A list cell's look for its record: the field's style classes — or the style_expr style when
 * its condition holds — and an indent of `indent_field` levels (1rem each)
 */
export function cellStyle(field: Field, record: Record<string, any> | undefined): { classes: string; style: string } {
	let style = field.style;
	const expr = field.style_expr;
	if (expr && record) {
		const value = String(record[expr.field] ?? '');
		if (expr.equal !== undefined && value === String(expr.equal)) style = expr.style;
		if (expr.not_equal !== undefined && value !== String(expr.not_equal)) style = expr.style;
	}
	const levels = field.indent_field ? Number(record?.[field.indent_field]) || 0 : 0;
	return {
		classes: getFieldStyleClasses({ style, importance: field.importance }),
		style: levels > 0 ? `padding-left: ${levels}rem` : ''
	};
}

/**
 * The lookup of a field for a record: a conditional relation (variants) picks the lookup by the
 * record's type field (e.g. Bal. Account Type → G/L accounts or customers); none for a type
 * without a related table
 */
export function lookupFor<
	L extends {
		variants?: Record<string, L>;
		variant_field?: string;
		filters?: { column: string; field?: string; const?: string }[];
		rows?: Array<Record<string, any>> | null;
		new_path?: string;
	}
>(lookups: Record<string, L> | undefined, field: string, record: Record<string, any> | undefined): L | undefined {
	let lookup = lookups?.[field];
	if (lookup?.variants) {
		const type = record?.[lookup.variant_field ?? ''];
		lookup = lookup.variants[String(type ?? '')];
	}
	// A relation with WHERE filters: only the rows matching the record (a level of its terms)
	if (lookup?.filters?.length && lookup.rows) {
		const filters = lookup.filters;
		const rows = lookup.rows.filter((row) =>
			filters.every((f) => String(row[f.column] ?? '') === String((f.field ? record?.[f.field] : f.const) ?? ''))
		);
		lookup = { ...lookup, rows };
	}
	// …and its New opens the related list filtered the same way, so the new record gets the
	// values (an item's unit of measure gets the item's No.)
	const newPageId = (lookup as { new_page_id?: number } | undefined)?.new_page_id;
	if (lookup?.filters?.length && newPageId) {
		const path = runPagePath(
			{ run_page: newPageId, run_page_filters: lookup.filters.map((f) => ({ field: f.column, value: f.field, const: f.const })) },
			record
		);
		if (path) lookup = { ...lookup, new_path: path };
	}
	return lookup;
}

/** A picture field (BLOB: YAML "[]byte", API "Blob") — shown by PictureField, never edited inline */
export function isBlobType(fieldType: string | undefined): boolean {
	return fieldType === '[]byte' || fieldType === 'Blob' || fieldType === 'BLOB';
}
