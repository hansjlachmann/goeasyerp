import { describe, it, expect } from 'vitest';
import { toApiFlowFilter } from '../flowFilter';

const today = new Date(2026, 9, 6); // 6 Oct 2026

describe('toApiFlowFilter', () => {
	it('converts local dates (nb-NO dd.mm.yy) to ISO', () => {
		expect(toApiFlowFilter('01.01.26..31.03.26', 'date', 'nb-NO', today)).toEqual({ value: '2026-01-01..2026-03-31' });
		expect(toApiFlowFilter('..31.03.26', 'date', 'nb-NO', today)).toEqual({ value: '..2026-03-31' });
		expect(toApiFlowFilter('01.09.2026..', 'date', 'nb-NO', today)).toEqual({ value: '2026-09-01..' });
		expect(toApiFlowFilter(' 15.05.26 ', 'date', 'nb-NO', today)).toEqual({ value: '2026-05-15' });
	});

	it('handles t, alternatives and <>', () => {
		expect(toApiFlowFilter('01.10.26..t', 'date', 'da-DK', today)).toEqual({ value: '2026-10-01..2026-10-06' });
		expect(toApiFlowFilter('01.01.26..31.01.26|01.08.26..31.08.26', 'date', 'nb-NO', today)).toEqual({
			value: '2026-01-01..2026-01-31|2026-08-01..2026-08-31'
		});
		expect(toApiFlowFilter('<>t', 'date', 'nb-NO', today)).toEqual({ value: '<>2026-10-06' });
	});

	it('keeps closing dates', () => {
		expect(toApiFlowFilter('01.01.26..C31.12.26', 'date', 'nb-NO', today)).toEqual({ value: '2026-01-01..C2026-12-31' });
		expect(toApiFlowFilter('C311226', 'date', 'nb-NO', today)).toEqual({ value: 'C2026-12-31' });
	});

	it('reads en-US month/day order and ISO input', () => {
		expect(toApiFlowFilter('01/02/26..03/31/26', 'date', 'en-US', today)).toEqual({ value: '2026-01-02..2026-03-31' });
		expect(toApiFlowFilter('2026-01-01..2026-03-31', 'date', 'nb-NO', today)).toEqual({ value: '2026-01-01..2026-03-31' });
	});

	it('reads dotted dates as day.month.year in every UI language', () => {
		// the user typed Norwegian dates while their UI language is English (US)
		expect(toApiFlowFilter('01.04.26..30.06.26', 'date', 'en-US', today)).toEqual({ value: '2026-04-01..2026-06-30' });
		expect(toApiFlowFilter('01.04..30.06', 'date', 'en-US', today)).toEqual({ value: '2026-04-01..2026-06-30' });
		expect(toApiFlowFilter('1.4.2026', 'date', 'nb-NO', today)).toEqual({ value: '2026-04-01' });
		// slashes follow the UI language
		expect(toApiFlowFilter('04/01/26', 'date', 'en-US', today)).toEqual({ value: '2026-04-01' });
		expect(toApiFlowFilter('01/04/26', 'date', 'nb-NO', today)).toEqual({ value: '2026-04-01' });
		// language codes from the Language table (NOB) work as locales
		expect(toApiFlowFilter('01.04.26', 'date', 'nob', today)).toEqual({ value: '2026-04-01' });
	});

	it('reports what it cannot read', () => {
		expect(toApiFlowFilter('31.02.26', 'date', 'nb-NO', today).error).toBe('31.02.26');
		expect(toApiFlowFilter('01.01.26..yesterday', 'date', 'nb-NO', today).error).toBe('yesterday');
		expect(toApiFlowFilter('..', 'date', 'nb-NO', today).error).toBe('..');
		expect(toApiFlowFilter('01.01.26..02.01.26..03.01.26', 'date', 'nb-NO', today).error).toBeDefined();
	});

	it('leaves empty and non-date filters as typed', () => {
		expect(toApiFlowFilter('  ', 'date', 'nb-NO', today)).toEqual({ value: '' });
		expect(toApiFlowFilter('c0001*', 'code', 'nb-NO', today)).toEqual({ value: 'c0001*' });
	});
});
