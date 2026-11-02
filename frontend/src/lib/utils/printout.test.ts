import { describe, expect, it } from 'vitest';
import { resultFileOf } from './printout';

describe('resultFileOf', () => {
	it('decodes a result file', () => {
		const file = resultFileOf({ file: btoa('00 INTRASTAT\r\n'), filename: 'Intrastat 2610.txt' });
		expect(file?.filename).toBe('Intrastat 2610.txt');
		expect(file?.blob.size).toBe(14);
	});
	it('is null without a file', () => {
		expect(resultFileOf({ pdf: 'x' })).toBeNull();
		expect(resultFileOf(undefined)).toBeNull();
	});
});
