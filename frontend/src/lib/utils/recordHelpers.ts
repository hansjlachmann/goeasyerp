// Record utility functions
import type { PageDefinition, Field } from '$lib/types/pages';

/**
 * Get the primary key field name from a page definition
 * Searches through sections (Card pages) and repeater (List pages) for a field with primary_key: true
 */
export function getPrimaryKeyField(page: PageDefinition | null | undefined): string | undefined {
	if (!page) return undefined;

	// Prefer the backend-provided PK fields (works even when the PK is not displayed,
	// e.g. setup tables whose blank primary key is hidden).
	if (page.page.primary_key_fields && page.page.primary_key_fields.length > 0) {
		return page.page.primary_key_fields[0];
	}

	// Check repeater fields (List pages)
	if (page.page.layout.repeater?.fields) {
		const pkField = page.page.layout.repeater.fields.find((f: Field) => f.primary_key);
		if (pkField) return pkField.source;
	}

	// Check section fields (Card pages)
	if (page.page.layout.sections) {
		for (const section of page.page.layout.sections) {
			const pkField = section.fields.find((f: Field) => f.primary_key);
			if (pkField) return pkField.source;
		}
	}

	return undefined;
}

/**
 * Get all primary key field names from a page definition (supports composite keys)
 * Searches through sections (Card pages) and repeater (List pages) for fields with primary_key: true
 */
export function getPrimaryKeyFields(page: PageDefinition | null | undefined): string[] {
	if (!page) return [];

	// Prefer the backend-provided PK fields (authoritative; covers hidden PKs).
	if (page.page.primary_key_fields && page.page.primary_key_fields.length > 0) {
		return page.page.primary_key_fields;
	}

	const pkFields: string[] = [];

	// Check repeater fields (List pages)
	if (page.page.layout.repeater?.fields) {
		for (const f of page.page.layout.repeater.fields) {
			if (f.primary_key) pkFields.push(f.source);
		}
	}

	// Check section fields (Card pages)
	if (page.page.layout.sections) {
		for (const section of page.page.layout.sections) {
			for (const f of section.fields) {
				if (f.primary_key) pkFields.push(f.source);
			}
		}
	}

	return pkFields;
}

/**
 * Extract the primary key/ID from a record
 * @param record - The record to extract ID from
 * @param primaryKeyField - Optional primary key field name (from page definition)
 * @param primaryKeyFields - Optional array of PK field names for composite keys
 *
 * For composite keys: joins all PK values with comma (e.g., "HANS2,READER,TEST-COMPANY")
 * For single keys: returns the PK value directly
 * Otherwise falls back to checking common field names: no, code, user_id, id
 */
export function getRecordId(
	record: Record<string, any> | null | undefined,
	primaryKeyField?: string,
	primaryKeyFields?: string[]
): string | undefined {
	if (!record) return undefined;

	// Composite key: join all PK values with comma
	// Note: empty strings are valid PK values (e.g., blank company = all companies)
	if (primaryKeyFields && primaryKeyFields.length > 1) {
		const values = primaryKeyFields.map(f => record[f]);
		if (values.every(v => v !== undefined)) {
			return values.map(v => String(v ?? '')).join(',');
		}
		return undefined;
	}

	// If primary key field is specified, use it directly.
	// An empty string is a valid id (setup tables have a single blank primary key),
	// so only fall through when the field is entirely absent.
	if (primaryKeyField && record[primaryKeyField] !== undefined) {
		return String(record[primaryKeyField]);
	}

	// Fallback to common field names for backwards compatibility
	if (record.no !== undefined && record.no !== '') return String(record.no);
	if (record.code !== undefined && record.code !== '') return String(record.code);
	if (record.user_id !== undefined && record.user_id !== '') return String(record.user_id);
	if (record.id !== undefined && record.id !== '') return String(record.id);

	return undefined;
}

/**
 * Extract a display label from a record
 * @param record - The record to extract label from
 * @param primaryKeyField - Optional primary key field name (from page definition)
 */
export function getRecordLabel(
	record: Record<string, any> | null | undefined,
	primaryKeyField?: string
): string | undefined {
	return getRecordId(record, primaryKeyField);
}

/**
 * Check if a record is new (has no ID)
 * @param record - The record to check
 * @param primaryKeyField - Optional primary key field name (from page definition)
 */
export function isNewRecord(
	record: Record<string, any> | null | undefined,
	primaryKeyField?: string
): boolean {
	return !getRecordId(record, primaryKeyField);
}

/**
 * Get the record key for Svelte's keyed each blocks
 * Uses _tempId for new unsaved records, otherwise uses the primary key value
 * @param record - The record to get key from
 * @param primaryKeyField - Optional primary key field name (from page definition)
 * @param primaryKeyFields - Optional array of PK field names for composite keys
 */
export function getRecordKey(
	record: Record<string, any>,
	primaryKeyField?: string,
	primaryKeyFields?: string[]
): string {
	// For new unsaved records, use the temporary ID
	if (record._tempId) return record._tempId;

	// An editable row is keyed by its persisted key (_key), not the live field values: the
	// user may be typing a new primary key (Rename). Keying by the typed value gave
	// the row a new key per character, so Svelte recreated it, the input lost the focus,
	// and the blur saved the half-typed key and left editing.
	if (record._key !== undefined) return String(record._key);

	// Use the composite or single primary key value.
	// An empty string is a valid, stable key (setup tables have a single
	// blank primary key) — only fall through when there is no id at all. Returning a
	// random key here would give the keyed each an unstable key on every render,
	// destroying/recreating the row (breaks focus, edit state, and Svelte reconcile).
	const id = getRecordId(record, primaryKeyField, primaryKeyFields);
	if (id !== undefined) return id;

	// Last resort fallback - should not happen in practice
	return `record-${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * Deep copy an object using JSON serialization
 * Note: This won't work with functions, undefined, symbols, or circular references
 */
export function deepCopy<T>(obj: T): T {
	return JSON.parse(JSON.stringify(obj));
}

/**
 * Check if a record is empty (contains no user data, only internal flags)
 * @param record - The record to check
 */
export function isEmptyRecord(record: Record<string, any>): boolean {
	return !Object.keys(record).some(key =>
		!key.startsWith('_') && record[key] !== undefined && record[key] !== ''
	);
}

/**
 * Check if a record has any user-entered data (ignoring internal flags)
 * @param record - The record to check
 */
export function hasRecordData(record: Record<string, any>): boolean {
	return Object.keys(record).some(key =>
		!key.startsWith('_') && record[key] !== undefined && record[key] !== ''
	);
}

/**
 * Compare two field values loosely: inputs hold strings while the API returns typed
 * values (e.g. option index 0 vs "0"), and blank/null/undefined are all "no value".
 */
export function sameFieldValue(a: any, b: any): boolean {
	return String(a ?? '') === String(b ?? '');
}

/**
 * Take over the values the server changed while saving a card (a trigger tidied a field,
 * e.g. an e-mail list stored as "a; b", or recalculated a FlowField) into the form record,
 * in place so the inputs and their focus stay. Only fields the user has not changed again
 * since the save was sent (form value still equals the sent value) are updated.
 * @param form - The record the card's inputs are bound to
 * @param sent - The values as they were sent (a copy taken before the request)
 * @param saved - The record the API returned
 */
export function applyServerValues(
	form: Record<string, any>,
	sent: Record<string, any>,
	saved: Record<string, any> | null | undefined
): void {
	if (!saved) return;
	for (const [key, value] of Object.entries(saved)) {
		if (key.startsWith('_')) continue;
		if (sameFieldValue(value, sent[key])) continue; // unchanged by the server
		if (key in form && !sameFieldValue(form[key], sent[key])) continue; // edited meanwhile
		form[key] = value;
	}
}

/**
 * Remove internal underscore-prefixed flags (_isNew, _tempId, _pristine, ...) before a
 * record is sent to the API.
 */
export function stripInternalFields(record: Record<string, any>): Record<string, any> {
	return Object.fromEntries(Object.entries(record).filter(([key]) => !key.startsWith('_')));
}

/** Field states the page's OnAfterGetRecord trigger set for a list row (sent as `_field_states`) */
export type RowFieldStates = Record<string, { editable?: boolean; visible?: boolean }>;

/** Whether the page's trigger left a field of the row editable (a hidden field is not) */
export function rowFieldEditable(record: Record<string, any> | undefined, field: string): boolean {
	const state = (record?._field_states as RowFieldStates | undefined)?.[field];
	return state?.editable !== false && state?.visible !== false;
}

/** Whether the page's trigger left a field of the row visible */
export function rowFieldVisible(record: Record<string, any> | undefined, field: string): boolean {
	return (record?._field_states as RowFieldStates | undefined)?.[field]?.visible !== false;
}

/**
 * Check if the user has changed any field of a new record away from the values it was
 * initialized with. Init-supplied defaults are not user edits: a pre-populated row the
 * user never touched counts as untouched (discards it, never inserts it).
 * @param record - The new record
 * @param pristine - The values the record was initialized with
 */
export function hasUserEdits(record: Record<string, any>, pristine: Record<string, any>): boolean {
	return Object.keys(record).some(key => !key.startsWith('_') && !sameFieldValue(record[key], pristine[key]));
}

/**
 * Decide whether a new record should be INSERTed now. inserts on the first field
 * the user validates, while the cursor is still on the row — once the record has a user
 * edit and every required primary key field has a value. Optional primary key fields may
 * stay blank (e.g. blank company = all companies) but must be defined.
 * @param record - The new record
 * @param pristine - The values the record was initialized with
 * @param primaryKeyFields - Primary key fields with their required flag
 */
export function shouldInsertNewRecord(
	record: Record<string, any>,
	pristine: Record<string, any>,
	primaryKeyFields: Array<{ source: string; required?: boolean }>,
	autoKey = false
): boolean {
	if (record._isNew !== true || !hasUserEdits(record, pristine)) return false;
	// auto_key: the table's OnInsert assigns a blank key (the next number of a series)
	if (autoKey) return true;
	return primaryKeyFields.every(pk =>
		pk.required ? !sameFieldValue(record[pk.source], '') : record[pk.source] !== undefined
	);
}

/**
 * Whether every primary key field of a record has a value. A new record on a card (modal or
 * page) is only INSERTed once it does: inserting earlier fails with "… cannot be empty" —
 * e.g. when a browser's password manager fills the User card's password field before the
 * user typed a User ID. Values entered before stay on the form and go with the insert.
 */
export function hasPrimaryKey(record: Record<string, any>, primaryKeyFields: string[]): boolean {
	// No key metadata: leave the decision to the backend (it rejects an empty key)
	return primaryKeyFields.every(f => String(record[f] ?? '').trim() !== '');
}

/**
 * Whether a new card record (modal or page) may be INSERTed: once every primary key field has
 * a value — or, when the table's OnInsert assigns a blank key (page `auto_key`, a number
 * series), once the user changed something against the values the record started with
 * (pristine): leaving the empty No. field must not insert a blank record and use a number.
 */
export function canInsertNewRecord(
	record: Record<string, any>,
	pristine: Record<string, any>,
	primaryKeyFields: string[],
	autoKey = false
): boolean {
	if (!autoKey) return hasPrimaryKey(record, primaryKeyFields);
	return Object.keys(record).some(
		(key) => !key.startsWith('_') && !sameFieldValue(record[key], pristine[key]) && !(record[key] === false && pristine[key] === undefined)
	);
}

/**
 * Check if a record has changed from its original state
 * Handles type coercion for number/string comparisons
 */
export function hasRecordChanged(
	current: Record<string, any>,
	original: Record<string, any>
): boolean {
	const currentKeys = Object.keys(current);
	for (const key of currentKeys) {
		// Skip internal fields
		if (key.startsWith('_')) continue;

		const currentVal = current[key];
		const originalVal = original[key];

		// Handle null/undefined/empty string as equivalent
		const currentEmpty = currentVal == null || currentVal === '';
		const originalEmpty = originalVal == null || originalVal === '';
		if (currentEmpty && originalEmpty) continue;
		if (currentEmpty || originalEmpty) return true;

		// Compare values with type coercion for numbers/strings
		if (typeof currentVal === 'number' || typeof originalVal === 'number') {
			if (Number(currentVal) !== Number(originalVal)) return true;
		} else if (String(currentVal) !== String(originalVal)) {
			return true;
		}
	}
	return false;
}

/**
 * Resolve the record behind the selected list row. The selected index counts rows as
 * displayed (after search and column sort), so it must index displayRecords — indexing the
 * unfiltered records array returns a different record whenever the list is searched or
 * sorted. While cells are being edited the displayed rows are editable copies; the saved
 * record with the same persisted key (`_key`, else the current key values) is returned.
 * An uncommitted new row has no saved record and yields null.
 */
export function findSelectedRecord(
	displayRecords: Array<Record<string, any>>,
	selectedIndex: number,
	records: Array<Record<string, any>>,
	editing: boolean,
	primaryKeyField?: string,
	primaryKeyFields?: string[]
): Record<string, any> | null {
	const row = selectedIndex >= 0 ? displayRecords[selectedIndex] : undefined;
	if (!row || row._isNew) return null;
	if (!editing) return row;
	const key = row._key ?? getRecordId(row, primaryKeyField, primaryKeyFields);
	return records.find((r) => getRecordId(r, primaryKeyField, primaryKeyFields) === key) ?? null;
}

/**
 * The /init context of a new record on a page with OnNewRecord triggers: the page, the record
 * the new one is inserted next to (below it when below) and the values the new record already
 * has (the filters' and link values, which the triggers see); nothing for other pages
 */
export function newRecordContext(
	page: { id: number; has_new_record_triggers?: boolean },
	xRec: Record<string, any> | undefined,
	below: boolean,
	values?: Record<string, any>
): NewRecordContext | undefined {
	if (!page.has_new_record_triggers) return undefined;
	return { pageId: page.id, xRec: xRec ? stripInternalFields(xRec) : undefined, belowXRec: below, values };
}

export interface NewRecordContext {
	pageId: number;
	xRec?: Record<string, any>;
	belowXRec: boolean;
	values?: Record<string, any>;
}

/**
 * A record's primary key as the user reads it, for messages (the Delete confirmation):
 * "No.: C00010"; a composite key lists every key field, "Journal Template Name: GENERAL,
 * Journal Batch Name: DEFAULT, Line No.: 10000". Captions from the page, option values by
 * their caption; a blank value shows as ''.
 */
export function recordKeyText(
	record: Record<string, any> | null | undefined,
	primaryKeyFields: string[],
	captions: Record<string, string> = {},
	options: Record<string, Record<string, string>> = {}
): string {
	if (!record) return '';
	return primaryKeyFields
		.map((field) => {
			const value = record[field];
			let text = value === undefined || value === null || value === '' ? "''" : String(value);
			if (options[field] && value !== undefined && value !== null && value !== '') text = options[field][String(value)] ?? text;
			return `${captions[field] ?? field}: ${text}`;
		})
		.join(', ');
}
