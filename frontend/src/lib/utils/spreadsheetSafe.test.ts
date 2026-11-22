import { describe, expect, it } from 'vitest';
import { rowsToTsv, spreadsheetSafe } from './clipboard';
import { errorDetailsToCsv } from './errorDetails';

describe('spreadsheetSafe', () => {
	it('marks texts a spreadsheet would run as a formula', () => {
		expect(spreadsheetSafe('=HYPERLINK("http://x.no")')).toBe(`'=HYPERLINK("http://x.no")`);
		expect(spreadsheetSafe('+cmd')).toBe(`'+cmd`);
		expect(spreadsheetSafe('-2+3')).toBe(`'-2+3`);
		expect(spreadsheetSafe('@SUM(A1)')).toBe(`'@SUM(A1)`);
		expect(spreadsheetSafe('\t=1')).toBe(`'\t=1`);
	});

	it('leaves numbers and other texts as they are', () => {
		for (const value of ['-1 234,56', '-1,234.56', '+12', '-0.5%', "-1'000.00", 'Fjordlys AS', 'C00010', '', 'a=b']) {
			expect(spreadsheetSafe(value)).toBe(value);
		}
	});

	it('is applied to copied rows and to the error details file', () => {
		expect(rowsToTsv(['Name', 'Amount'], [['=1+1', '-5,00']])).toBe("Name\tAmount\n'=1+1\t-5,00");
		const csv = errorDetailsToCsv({ title: 'T', sections: [{ caption: 'S', columns: [{ caption: 'Name' }], rows: [['=1+1']] }] });
		expect(csv).toContain("'=1+1");
	});
});
