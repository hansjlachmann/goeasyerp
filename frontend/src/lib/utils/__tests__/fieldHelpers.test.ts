import { describe, it, expect } from 'vitest';
import type { LookupData } from '$lib/types/api';
import { isAdvancedLookup, lookupFor, formatLookupValue, isCodeType, toDateTimeInput, fromDateTimeInput, isBlobType } from '../fieldHelpers';

describe('isAdvancedLookup', () => {
	const columns = [{ source: 'code' }];
	it('is a table-style dropdown with columns and rows, or columns and a lazy URL', () => {
		expect(isAdvancedLookup({ columns, rows: [{ _key: 'A' }] })).toBe(true);
		expect(isAdvancedLookup({ columns, lazy_url: '/api/tables/X/lookup/f' })).toBe(true);
		expect(isAdvancedLookup({ columns, rows: null, lazy_url: '/api/tables/X/lookup/f' })).toBe(true);
	});
	it('is also for an empty related table (the dropdown shows no rows)', () => {
		expect(isAdvancedLookup({ columns, rows: [] })).toBe(true);
	});
	it('is not without columns, or without rows and lazy URL', () => {
		expect(isAdvancedLookup(undefined)).toBe(false);
		expect(isAdvancedLookup({ columns: [], rows: [{ _key: 'A' }] })).toBe(false);
		expect(isAdvancedLookup({ columns, rows: null })).toBe(false);
	});
});

describe('formatLookupValue', () => {
	it('shows the key when on-demand rows are not loaded', () => {
		expect(formatLookupValue('C00010', { columns: [{ source: 'no' }] })).toBe('C00010');
	});
});

describe('isCodeType', () => {
	it('recognizes Code fields as the page metadata (YAML) and the table API (Go) name them', () => {
		expect(isCodeType('types.Code')).toBe(true);
		expect(isCodeType('Code')).toBe(true);
		expect(isCodeType('code')).toBe(true);
	});
	it('is false for other types', () => {
		expect(isCodeType('types.Text')).toBe(false);
		expect(isCodeType(undefined)).toBe(false);
	});
});

describe('datetime-local input values', () => {
	it('shows an API datetime as local time and sends it back as UTC', () => {
		const iso = '2026-10-08T10:00:00Z';
		const local = toDateTimeInput(iso);
		expect(local).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
		expect(fromDateTimeInput(local)).toBe(iso);
	});

	it('is empty for no or an invalid value', () => {
		expect(toDateTimeInput('')).toBe('');
		expect(toDateTimeInput(undefined)).toBe('');
		expect(toDateTimeInput('not a date')).toBe('');
		expect(fromDateTimeInput('')).toBe('');
	});
});

describe('lookupFor', () => {
	const gl: LookupData = { columns: [{ source: 'no', width: 100 }], rows: [{ _key: '1000' }] };
	const cust: LookupData = { columns: [{ source: 'no', width: 100 }], rows: [{ _key: 'C1' }] };
	const lookups: Record<string, LookupData> = {
		payment_terms_code: gl,
		bal_account_no: { variant_field: 'bal_account_type', variants: { '0': gl, '1': cust } }
	};
	it('picks the variant by the record type, none for a type without a table', () => {
		expect(lookupFor(lookups, 'bal_account_no', { bal_account_type: 1 })).toBe(cust);
		expect(lookupFor(lookups, 'bal_account_no', { bal_account_type: 0 })).toBe(gl);
		expect(lookupFor(lookups, 'bal_account_no', { bal_account_type: 2 })).toBeUndefined();
		expect(lookupFor(lookups, 'payment_terms_code', {})).toBe(gl);
		expect(lookupFor(lookups, 'nothing', {})).toBeUndefined();
	});
});

describe('lookupFor with relation filters', () => {
	it('keeps the rows matching the record', () => {
		const levels: LookupData = {
			columns: [{ source: 'no', width: 60 }],
			rows: [
				{ _key: '1', no: 1, reminder_terms_code: 'DOM' },
				{ _key: '2', no: 2, reminder_terms_code: 'DOM' },
				{ _key: '1', no: 1, reminder_terms_code: 'FOR' }
			],
			filters: [{ column: 'reminder_terms_code', field: 'reminder_terms_code' }]
		};
		expect(lookupFor({ reminder_level: levels }, 'reminder_level', { reminder_terms_code: 'DOM' })?.rows?.length).toBe(2);
		expect(lookupFor({ reminder_level: levels }, 'reminder_level', { reminder_terms_code: 'FOR' })?.rows?.length).toBe(1);
	});
});

describe('isBlobType', () => {
	it('recognizes picture fields in both spellings', () => {
		expect(isBlobType('[]byte')).toBe(true);
		expect(isBlobType('Blob')).toBe(true);
		expect(isBlobType('types.Text')).toBe(false);
		expect(isBlobType(undefined)).toBe(false);
	});
});

describe('lookupFor New with relation filters', () => {
	it('opens the related list filtered like the relation', () => {
		const lookups = {
			base_unit_of_measure: {
				rows: [{ _key: 'PCS', item_no: '1000' }, { _key: 'BOX', item_no: '2000' }],
				filters: [{ column: 'item_no', field: 'no' }],
				new_page_id: 5404
			}
		} as unknown as Record<string, LookupData>;
		const lookup = lookupFor(lookups, 'base_unit_of_measure', { no: '1000' });
		expect(lookup?.rows?.map((r) => r._key)).toEqual(['PCS']);
		expect(lookup?.new_path).toBe('/pages/5404?filter=item_no%3D1000');
		// a new item (no No. yet): plain New
		expect(lookupFor(lookups, 'base_unit_of_measure', { no: '' })?.new_path).toBeUndefined();
	});
});

describe('lookupFor on-demand rows of a filtered relation', () => {
	it("sends the record's values of the relation's field filters", () => {
		const lookups = {
			unit_of_measure_code: {
				lazy_url: '/api/tables/Sales_Line/lookup/unit_of_measure_code',
				filters: [{ column: 'item_no', field: 'no' }, { column: 'blocked', const: '0' }]
			}
		};
		const lookup = lookupFor(lookups, 'unit_of_measure_code', { no: '1000' });
		expect(lookup?.lazy_url).toBe('/api/tables/Sales_Line/lookup/unit_of_measure_code?record_no=1000');
		const variant = lookupFor(
			{ x: { lazy_url: '/api/tables/T/lookup/x?variant=2', filters: [{ column: 'c', field: 'f' }] } },
			'x',
			{}
		);
		expect(variant?.lazy_url).toBe('/api/tables/T/lookup/x?variant=2&record_f=');
	});
});
