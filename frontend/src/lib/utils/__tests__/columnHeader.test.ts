import { describe, it, expect } from 'vitest';
import { filterValueFor, withFilter, withoutFilter, moveColumn, hideColumn, captionWithBreaks } from '../columnHeader';

describe('column header menu', () => {
	it('filters to a value only when it can be matched exactly', () => {
		expect(filterValueFor('C00010')).toBe('C00010');
		expect(filterValueFor(' Oslo ')).toBe('Oslo');
		expect(filterValueFor(150.5)).toBe('150.5');
		expect(filterValueFor(0)).toBe('0');
		expect(filterValueFor(true)).toBe('1');
		expect(filterValueFor(false)).toBe('0');
		expect(filterValueFor('2026-01-10')).toBe('2026-01-10');
		expect(filterValueFor("O'Brien")).toBe("O'Brien"); // values are bound, quotes are harmless
		for (const v of ['', '  ', null, undefined, 'A|B', '10..20', 'AB*', '<>X', '>5']) {
			expect(filterValueFor(v)).toBeNull();
		}
	});

	it('sets or replaces one field filter and removes it', () => {
		const filters = [
			{ field: 'city', expression: 'Oslo' },
			{ field: 'name', expression: '*AS' }
		];
		expect(withFilter(filters, 'city', 'Bergen')).toEqual([
			{ field: 'name', expression: '*AS' },
			{ field: 'city', expression: 'Bergen' }
		]);
		expect(withoutFilter(filters, 'city')).toEqual([{ field: 'name', expression: '*AS' }]);
	});

	it('moves a dragged column to the target column place, hidden columns keep theirs', () => {
		const order = [
			{ source: 'no', visible: true },
			{ source: 'name', visible: true },
			{ source: 'hidden', visible: false },
			{ source: 'city', visible: true }
		];
		const out = moveColumn({}, order, 'city', 'name');
		const sorted = Object.entries(out)
			.sort((a, b) => (a[1].order ?? 0) - (b[1].order ?? 0))
			.map(([k]) => k);
		expect(sorted).toEqual(['no', 'city', 'name', 'hidden']);
		expect(out.hidden.visible).toBe(false);
		expect(moveColumn({ x: { visible: true } }, order, 'no', 'no')).toEqual({ x: { visible: true } });
	});

	it('hides a column, keeping its order', () => {
		expect(hideColumn({ city: { visible: true, order: 3 } }, 'city')).toEqual({ city: { visible: false, order: 3 } });
	});
});

describe('captionWithBreaks', () => {
	it('lets a caption wrap after a slash, showing nothing extra', () => {
		expect(captionWithBreaks('Income/Balance')).toBe('Income/\u200BBalance');
		expect(captionWithBreaks('Income/Balance').replace('\u200B', '')).toBe('Income/Balance');
	});
	it('leaves other captions and a trailing slash as they are', () => {
		expect(captionWithBreaks('Account Type')).toBe('Account Type');
		expect(captionWithBreaks('A/ B')).toBe('A/ B');
		expect(captionWithBreaks('A/')).toBe('A/');
	});
});
