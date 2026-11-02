import { describe, it, expect, afterEach } from 'vitest';
import { dateEntryToIso } from '../dateEntry';
import { applyDateFormula } from '../dateFormula';
import { setRegional, formatDateRegional, formatNumberText, fromDecimalInput, toDecimalInput, datePattern, dateOrder } from '../regional';

const workDate = new Date(2026, 9, 29); // Thursday 29.10.2026
const today = new Date(2026, 9, 29);
const dmy = (s: string) => dateEntryToIso(s, { order: 'DMY', workDate, today });
const mdy = (s: string) => dateEntryToIso(s, { order: 'MDY', workDate, today });
const ymd = (s: string) => dateEntryToIso(s, { order: 'YMD', workDate, today });

describe('date entry', () => {
	it('fills short digits from the work date', () => {
		expect(dmy('05')).toBe('2026-10-05');
		expect(dmy('5')).toBe('2026-10-05');
		expect(dmy('0509')).toBe('2026-09-05');
		expect(dmy('509')).toBe('2026-09-05');
		expect(dmy('050926')).toBe('2026-09-05');
		expect(dmy('05092026')).toBe('2026-09-05');
		expect(dmy('311299')).toBe('1999-12-31');
	});
	it('reads separators in the user order', () => {
		expect(dmy('5.9')).toBe('2026-09-05');
		expect(dmy('5/9/26')).toBe('2026-09-05');
		expect(dmy('05-09-2026')).toBe('2026-09-05');
		expect(dmy('2026-09-05')).toBe('2026-09-05');
		expect(dmy('5.')).toBe('2026-10-05');
	});
	it('reads month first and year first', () => {
		expect(mdy('0509')).toBe('2026-05-09');
		expect(mdy('5/9/26')).toBe('2026-05-09');
		expect(mdy('050926')).toBe('2026-05-09');
		expect(ymd('0509')).toBe('2026-05-09');
		expect(ymd('20260905')).toBe('2026-09-05');
		expect(ymd('26.9.5')).toBe('2026-09-05');
	});
	it('takes t, w, weekdays and date formulas', () => {
		const wd = new Date(2026, 8, 15); // Tuesday 15.09.2026
		const opts = { order: 'DMY' as const, workDate: wd, today };
		expect(dateEntryToIso('t', opts)).toBe('2026-10-29');
		expect(dateEntryToIso('w', opts)).toBe('2026-09-15');
		expect(dateEntryToIso('mo', opts)).toBe('2026-09-14');
		expect(dateEntryToIso('fr', opts)).toBe('2026-09-18');
		expect(dateEntryToIso('søn', opts)).toBe('2026-09-20');
		expect(dateEntryToIso('tors', opts)).toBe('2026-09-17');
		expect(dateEntryToIso('+1w', opts)).toBe('2026-09-22');
		expect(dateEntryToIso('-3d', opts)).toBe('2026-09-12');
		expect(dateEntryToIso('cm', opts)).toBe('2026-09-30');
		expect(dateEntryToIso('-1m+cm', opts)).toBe('2026-08-31');
	});
	it('refuses what is not a date, blank is blank', () => {
		for (const bad of ['32', '3102', '310226', '12345', '1-2-3-4', 'x', 'abc', '5.13', '+1x']) {
			expect(dmy(bad), bad).toBeNull();
		}
		expect(dmy('  ')).toBe('');
	});
});

describe('date formulas', () => {
	const d = new Date(2026, 0, 31);
	const iso = (f: string) => {
		const r = applyDateFormula(f, d);
		return r ? `${r.getFullYear()}-${String(r.getMonth() + 1).padStart(2, '0')}-${String(r.getDate()).padStart(2, '0')}` : null;
	};
	it('agrees with the server', () => {
		const cases: Record<string, string> = {
			'30D': '2026-03-02', '<1M>': '2026-02-28', cm: '2026-01-31', '-CM': '2026-01-01', '1M+CM': '2026-02-28',
			' 1 M ': '2026-02-28', 'CM+1D': '2026-02-01', '-1Y': '2025-01-31', WD1: '2026-02-02', '1Q': '2026-04-30', CY: '2026-12-31'
		};
		for (const [f, want] of Object.entries(cases)) expect(iso(f), f).toBe(want);
		for (const f of ['30', 'D', '1X', 'C', 'CX', '-WD1', 'WD8', '1M+', '30 DAYS', '<>']) expect(iso(f), f).toBeNull();
	});
});

describe('closing dates', () => {
	afterEach(() => setRegional({}));
	it('reads C and a date as its closing date, not the formulas CM, CW', () => {
		expect(dmy('C311226')).toBe('C2026-12-31');
		expect(dmy('c 31.12.26')).toBe('C2026-12-31');
		expect(dmy('ct')).toBe('C2026-10-29');
		expect(mdy('C12/31/2026')).toBe('C2026-12-31');
		expect(dmy('CM')).toBe('2026-10-31');
		expect(dmy('C')).toBeNull();
		expect(dmy('C99')).toBeNull();
	});
	it('shows a closing date with its C', () => {
		setRegional({ date_format: 'dd.mm.yy' });
		expect(formatDateRegional('C2026-12-31', 'en-US')).toBe('C31.12.26');
		expect(formatDateRegional('C2026-12-31', 'nb-NO')).toBe('C31.12.26');
	});
});

describe('regional formats', () => {
	afterEach(() => setRegional({}));
	it('shows dates and numbers in the user format', () => {
		setRegional({ date_format: 'dd.mm.yy', number_format: '1 234,56' });
		expect(formatDateRegional('2026-09-05', 'en-US')).toBe('05.09.26');
		expect(datePattern('en-US')).toBe('DD.MM.YY');
		expect(dateOrder('en-US')).toBe('DMY');
		expect(formatNumberText('-1234567.5', 'en-US')).toBe('-1 234 567,5');
		expect(toDecimalInput('1234.5', 'en-US')).toBe('1234,5');
		expect(fromDecimalInput('1 234,5', 'en-US')).toBe('1234.5');
		setRegional({ date_format: 'mm/dd/yyyy', number_format: '1.234,56' });
		expect(formatDateRegional('2026-09-05T10:00:00Z', 'nb-NO')).toBe('09/05/2026');
		expect(fromDecimalInput('1.234,5', 'nb-NO')).toBe('1234.5');
		expect(dateOrder('nb-NO')).toBe('MDY');
	});
	it('follows the language without settings', () => {
		expect(dateOrder('en-US')).toBe('MDY');
		expect(dateOrder('nb-NO')).toBe('DMY');
		expect(formatDateRegional('2026-09-05', 'nb-NO')).toBe('05.09.2026');
		expect(formatNumberText('1234.5', 'en-US')).toBe('1,234.5');
		expect(fromDecimalInput('1,234.5', 'en-US')).toBe('1234.5');
	});
});
