import { describe, it, expect } from 'vitest';
import {
	applyServerValues,
	getRecordId,
	recordKeyText,
	hasPrimaryKey,
	canInsertNewRecord,
	isNewRecord,
	getRecordKey,
	deepCopy,
	hasRecordChanged,
	hasUserEdits,
	shouldInsertNewRecord,
	stripInternalFields,
	findSelectedRecord,
	rowFieldEditable,
	rowFieldVisible
} from '../recordHelpers';

describe('getRecordId', () => {
	it('returns undefined for null record', () => {
		expect(getRecordId(null)).toBeUndefined();
	});

	it('returns undefined for undefined record', () => {
		expect(getRecordId(undefined)).toBeUndefined();
	});

	it('returns value from specified primary key field', () => {
		const record = { customer_no: 'CUST-001', name: 'Test' };
		expect(getRecordId(record, 'customer_no')).toBe('CUST-001');
	});

	it('converts number to string', () => {
		const record = { id: 123 };
		expect(getRecordId(record, 'id')).toBe('123');
	});

	it('falls back to "no" field', () => {
		const record = { no: 'ABC-001', name: 'Test' };
		expect(getRecordId(record)).toBe('ABC-001');
	});

	it('falls back to "code" field', () => {
		const record = { code: 'CODE-001', name: 'Test' };
		expect(getRecordId(record)).toBe('CODE-001');
	});

	it('falls back to "user_id" field', () => {
		const record = { user_id: 'USER-001', name: 'Test' };
		expect(getRecordId(record)).toBe('USER-001');
	});

	it('falls back to "id" field', () => {
		const record = { id: 'ID-001', name: 'Test' };
		expect(getRecordId(record)).toBe('ID-001');
	});

	it('returns undefined for empty primary key', () => {
		const record = { no: '', name: 'Test' };
		expect(getRecordId(record)).toBeUndefined();
	});
});

describe('isNewRecord', () => {
	it('returns true for null record', () => {
		expect(isNewRecord(null)).toBe(true);
	});

	it('returns true for record without ID', () => {
		const record = { name: 'Test' };
		expect(isNewRecord(record)).toBe(true);
	});

	it('returns false for record with ID', () => {
		const record = { no: 'ABC-001', name: 'Test' };
		expect(isNewRecord(record)).toBe(false);
	});
});

describe('getRecordKey', () => {
	it('returns _tempId if present', () => {
		const record = { _tempId: 'temp-123', no: 'ABC-001' };
		expect(getRecordKey(record)).toBe('temp-123');
	});

	it('returns primary key value', () => {
		const record = { no: 'ABC-001', name: 'Test' };
		expect(getRecordKey(record)).toBe('ABC-001');
	});

	it('keeps the persisted key while the primary key is being edited', () => {
		// The user is typing a new No. into a saved row: the key must not change per character
		const record = { _key: 'C00020', no: 'Ab', name: 'Test' };
		expect(getRecordKey(record, 'no')).toBe('C00020');
		// Blank persisted key (setup table) is a valid key too
		expect(getRecordKey({ _key: '', primary_key: 'x' }, 'primary_key')).toBe('');
	});

	it('returns fallback for record without ID', () => {
		const record = { name: 'Test' };
		const key = getRecordKey(record);
		expect(key).toMatch(/^record-/);
	});
});

describe('deepCopy', () => {
	it('creates a deep copy of object', () => {
		const original = { a: 1, b: { c: 2 } };
		const copy = deepCopy(original);

		expect(copy).toEqual(original);
		expect(copy).not.toBe(original);
		expect(copy.b).not.toBe(original.b);
	});

	it('handles arrays', () => {
		const original = [1, 2, { a: 3 }];
		const copy = deepCopy(original);

		expect(copy).toEqual(original);
		expect(copy).not.toBe(original);
	});
});

describe('hasRecordChanged', () => {
	it('returns false for identical records', () => {
		const record = { no: 'ABC', name: 'Test', value: 100 };
		expect(hasRecordChanged(record, record)).toBe(false);
	});

	it('returns true when string field changed', () => {
		const current = { no: 'ABC', name: 'Changed' };
		const original = { no: 'ABC', name: 'Original' };
		expect(hasRecordChanged(current, original)).toBe(true);
	});

	it('returns true when number field changed', () => {
		const current = { no: 'ABC', value: 200 };
		const original = { no: 'ABC', value: 100 };
		expect(hasRecordChanged(current, original)).toBe(true);
	});

	it('handles number/string comparison', () => {
		const current = { no: 'ABC', value: '100' };
		const original = { no: 'ABC', value: 100 };
		expect(hasRecordChanged(current, original)).toBe(false);
	});

	it('treats null and empty string as equivalent', () => {
		const current = { no: 'ABC', name: '' };
		const original = { no: 'ABC', name: null };
		expect(hasRecordChanged(current, original)).toBe(false);
	});

	it('ignores fields starting with underscore', () => {
		const current = { no: 'ABC', _internal: 'changed' };
		const original = { no: 'ABC', _internal: 'original' };
		expect(hasRecordChanged(current, original)).toBe(false);
	});
});

describe('hasUserEdits', () => {
	const pristine = { no: '', posting_date: '2026-10-02', amount: '0', active: true, type: 0 };

	it('is false for a pre-populated row the user never touched', () => {
		expect(hasUserEdits({ ...pristine, _isNew: true, _tempId: 't1' }, pristine)).toBe(false);
	});

	it('treats input strings equal to typed defaults as unchanged', () => {
		expect(hasUserEdits({ ...pristine, type: '0' }, pristine)).toBe(false);
	});

	it('is true once a field differs from its initial value', () => {
		expect(hasUserEdits({ ...pristine, no: 'C1' }, pristine)).toBe(true);
	});

	it('counts toggling a defaulted boolean as an edit', () => {
		expect(hasUserEdits({ ...pristine, active: false }, pristine)).toBe(true);
	});
});

describe('shouldInsertNewRecord', () => {
	const pks = [{ source: 'no', required: true }];

	// Table-driven: (initial values, user edits) → insert / no insert
	const cases: Array<{ name: string; pristine: Record<string, any>; edits: Record<string, any>; pks?: Array<{ source: string; required?: boolean }>; insert: boolean }> = [
		{ name: 'pre-populated row with zero user edits', pristine: { no: '', posting_date: '2026-10-02', amount: '0' }, edits: {}, insert: false },
		{ name: 'blank row with zero user edits', pristine: { no: '', name: '' }, edits: {}, insert: false },
		{ name: 'user fills the primary key', pristine: { no: '', name: '' }, edits: { no: 'C1' }, insert: true },
		{ name: 'user fills a non-key field while the required key is blank', pristine: { no: '', name: '' }, edits: { name: 'Acme' }, insert: false },
		{ name: 'required key supplied by defaults, user edits another field', pristine: { no: 'G0001', amount: '0' }, edits: { amount: '100' }, insert: true },
		{ name: 'composite key: first required part only', pristine: { user_id: '', role_id: '', company: '' }, edits: { user_id: 'HANS' }, pks: [{ source: 'user_id', required: true }, { source: 'role_id', required: true }, { source: 'company' }], insert: false },
		{ name: 'composite key: required parts filled, optional part blank', pristine: { user_id: '', role_id: '', company: '' }, edits: { user_id: 'HANS', role_id: 'READER' }, pks: [{ source: 'user_id', required: true }, { source: 'role_id', required: true }, { source: 'company' }], insert: true },
		{ name: 'optional key part missing entirely', pristine: { user_id: '', role_id: '' }, edits: { user_id: 'HANS', role_id: 'READER' }, pks: [{ source: 'user_id', required: true }, { source: 'role_id', required: true }, { source: 'company' }], insert: false },
		{ name: 'table without primary key fields, user edit', pristine: { name: '' }, edits: { name: 'x' }, pks: [], insert: true }
	];

	for (const c of cases) {
		it(`${c.insert ? 'inserts' : 'does not insert'}: ${c.name}`, () => {
			const record = { ...c.pristine, ...c.edits, _isNew: true, _tempId: 't1' };
			expect(shouldInsertNewRecord(record, c.pristine, c.pks ?? pks)).toBe(c.insert);
		});
	}

	it('never inserts a record that is not new', () => {
		expect(shouldInsertNewRecord({ no: 'C1' }, { no: '' }, pks)).toBe(false);
	});
});

describe('stripInternalFields', () => {
	it('removes underscore-prefixed flags', () => {
		expect(stripInternalFields({ no: 'C1', _isNew: true, _tempId: 't1', _pristine: { no: '' } })).toEqual({ no: 'C1' });
	});
});

describe('findSelectedRecord', () => {
	const records = [
		{ user_id: 'ADMIN', name: 'Administrator' },
		{ user_id: 'HANS', name: 'Hans' },
		{ user_id: 'ZOE', name: 'Zoe' }
	];

	it('returns the displayed row, not records[index], when the list is searched', () => {
		// Search for "hans": the only displayed row is HANS, at index 0
		const displayed = [records[1]];
		expect(findSelectedRecord(displayed, 0, records, false, 'user_id')?.user_id).toBe('HANS');
	});

	it('returns the displayed row when the list is sorted descending', () => {
		const displayed = [...records].reverse();
		expect(findSelectedRecord(displayed, 0, records, false, 'user_id')?.user_id).toBe('ZOE');
	});

	it('returns the saved record for an editable copy, by its persisted key', () => {
		// The user edited the key cell of HANS but has not saved yet
		const displayed = [{ ...records[1], user_id: 'HANS2', _key: 'HANS' }];
		expect(findSelectedRecord(displayed, 0, records, true, 'user_id')).toBe(records[1]);
	});

	it('returns null for an uncommitted new row or no selection', () => {
		const displayed = [records[0], { user_id: '', _isNew: true, _tempId: 't1' }];
		expect(findSelectedRecord(displayed, 1, records, true, 'user_id')).toBeNull();
		expect(findSelectedRecord(displayed, -1, records, false, 'user_id')).toBeNull();
		expect(findSelectedRecord(displayed, 5, records, false, 'user_id')).toBeNull();
	});
});

describe('hasPrimaryKey', () => {
	it('needs every primary key field filled in', () => {
		expect(hasPrimaryKey({ user_id: 'HANS' }, ['user_id'])).toBe(true);
		expect(hasPrimaryKey({ password: 'autofilled' }, ['user_id'])).toBe(false); // browser autofill before the key
		expect(hasPrimaryKey({ user_id: '' }, ['user_id'])).toBe(false);
		expect(hasPrimaryKey({ user_id: '  ' }, ['user_id'])).toBe(false);
		expect(hasPrimaryKey({ a: 'X', b: '' }, ['a', 'b'])).toBe(false);
		expect(hasPrimaryKey({ a: 'X', b: 0 }, ['a', 'b'])).toBe(true); // 0 is a value
		expect(hasPrimaryKey({ user_id: 'HANS' }, [])).toBe(true); // no key metadata: the backend decides
	});
});

describe('applyServerValues', () => {
	it('takes over values the server changed, in place', () => {
		const form: Record<string, any> = { no: 'J1', notification_email: 'a@x.no;b@x.no ', number_of_entries: 3 };
		const sent = { ...form };
		applyServerValues(form, sent, { no: 'J1', notification_email: 'a@x.no; b@x.no', number_of_entries: 4 });
		expect(form).toEqual({ no: 'J1', notification_email: 'a@x.no; b@x.no', number_of_entries: 4 });
	});

	it('keeps a field the user changed while the save was running', () => {
		const sent = { no: 'J1', description: 'Old', notification_email: 'a@x.no;b@x.no' };
		const form: Record<string, any> = { ...sent, description: 'Typed meanwhile' };
		applyServerValues(form, sent, { no: 'J1', description: 'Old trimmed', notification_email: 'a@x.no; b@x.no' });
		expect(form.description).toBe('Typed meanwhile');
		expect(form.notification_email).toBe('a@x.no; b@x.no');
	});

	it('ignores typed differences and internal fields', () => {
		const form: Record<string, any> = { status: '1', _key: 'J1' };
		applyServerValues(form, { ...form }, { status: 1, _key: 'other' });
		expect(form).toEqual({ status: '1', _key: 'J1' });
		applyServerValues(form, { ...form }, null);
		expect(form.status).toBe('1');
	});
});

describe('canInsertNewRecord', () => {
	it('waits for the key, or with an auto key for a change', () => {
		expect(canInsertNewRecord({ no: '', name: 'A' }, {}, ['no'])).toBe(false);
		expect(canInsertNewRecord({ no: 'C1', name: '' }, {}, ['no'])).toBe(true);
		expect(canInsertNewRecord({ no: '', name: '' }, {}, ['no'], true)).toBe(false);
		expect(canInsertNewRecord({ no: '', name: '', blocked: false }, { blocked: false }, ['no'], true)).toBe(false);
		expect(canInsertNewRecord({ no: '', name: 'A' }, {}, ['no'], true)).toBe(true);
		expect(canInsertNewRecord({ no: 'E00001', _isNew: true }, { no: '' }, ['no'], true)).toBe(true);
	});
});

describe('recordKeyText', () => {
	it('names the key fields with their captions and values', () => {
		expect(recordKeyText({ no: 'C00010', name: 'X' }, ['no'], { no: 'No.' })).toBe('No.: C00010');
		expect(
			recordKeyText(
				{ journal_template_name: 'GENERAL', journal_batch_name: 'DEFAULT', line_no: 10000 },
				['journal_template_name', 'journal_batch_name', 'line_no'],
				{ journal_template_name: 'Journal Template Name', journal_batch_name: 'Journal Batch Name', line_no: 'Line No.' }
			)
		).toBe('Journal Template Name: GENERAL, Journal Batch Name: DEFAULT, Line No.: 10000');
	});
	it('shows options by caption, blanks as quotes, no record as nothing', () => {
		expect(recordKeyText({ type: 1, code: '' }, ['type', 'code'], {}, { type: { '1': 'Item' } })).toBe("type: Item, code: ''");
		expect(recordKeyText(undefined, ['no'])).toBe('');
	});
});

describe('row field states', () => {
	const row = { no: 'L1', _field_states: { amount: { editable: false }, note: { visible: false } } };
	it('follows the page trigger per row', () => {
		expect(rowFieldEditable(row, 'amount')).toBe(false);
		expect(rowFieldEditable(row, 'no')).toBe(true);
		expect(rowFieldVisible(row, 'note')).toBe(false);
		expect(rowFieldEditable(row, 'note')).toBe(false); // hidden: not editable either
		expect(rowFieldVisible({ no: 'L2' }, 'note')).toBe(true);
		expect(rowFieldEditable(undefined, 'no')).toBe(true);
	});
	it('is not sent back', () => {
		expect(stripInternalFields(row)).toEqual({ no: 'L1' });
	});
});
