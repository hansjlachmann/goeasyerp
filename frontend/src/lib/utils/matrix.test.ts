import { describe, expect, it } from 'vitest';
import { matrixCellValue, shiftDecimal } from './matrix';

describe('shiftDecimal', () => {
	it('moves the point to the left', () => {
		expect(shiftDecimal('1234.5', 3)).toBe('1.2345');
		expect(shiftDecimal('-25000', 3)).toBe('-25');
		expect(shiftDecimal('12', 3)).toBe('0.012');
		expect(shiftDecimal('1500000.00', 6)).toBe('1.5');
	});
	it('leaves other texts', () => {
		expect(shiftDecimal('abc', 3)).toBe('abc');
		expect(shiftDecimal('12.5', 0)).toBe('12.5');
	});
});

describe('matrixCellValue', () => {
	it('is blank for zero', () => {
		expect(matrixCellValue('0', 0)).toBeNull();
		expect(matrixCellValue('0.00', 2)).toBeNull();
		expect(matrixCellValue('', 0)).toBeNull();
	});
	it('shows the amount in the rounding factor', () => {
		expect(matrixCellValue('1234.56', 0)).toEqual({ value: '1234.56', places: '2:2' });
		expect(matrixCellValue('1234.56', 1)).toEqual({ value: '1234.56', places: '0:0' });
		expect(matrixCellValue('1234.56', 2)).toEqual({ value: '1.23456', places: '1:1' });
		expect(matrixCellValue('-2500000', 3)).toEqual({ value: '-2.5', places: '1:1' });
	});
});
