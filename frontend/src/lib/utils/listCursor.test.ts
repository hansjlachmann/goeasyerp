import { describe, expect, it } from 'vitest';
import { moveCursor, ROW_SELECTOR } from './listCursor';
import { rowsToTsv, tsvField } from './clipboard';

describe('moveCursor', () => {
	it('moves left and right within the row, left of the first column to the row selector', () => {
		expect(moveCursor({ row: 3, col: 1 }, 'left', 4, 10)).toEqual({ row: 3, col: 0 });
		expect(moveCursor({ row: 3, col: 0 }, 'left', 4, 10)).toEqual({ row: 3, col: ROW_SELECTOR });
		expect(moveCursor({ row: 3, col: ROW_SELECTOR }, 'left', 4, 10)).toEqual({ row: 3, col: ROW_SELECTOR });
		expect(moveCursor({ row: 3, col: ROW_SELECTOR }, 'right', 4, 10)).toEqual({ row: 3, col: 0 });
		expect(moveCursor({ row: 3, col: 3 }, 'right', 4, 10)).toEqual({ row: 3, col: 3 });
	});

	it('Tab goes on in the next row, Shift+Tab in the previous one', () => {
		expect(moveCursor({ row: 3, col: 2 }, 'next', 4, 10)).toEqual({ row: 3, col: 3 });
		expect(moveCursor({ row: 3, col: 3 }, 'next', 4, 10)).toEqual({ row: 4, col: 0 });
		expect(moveCursor({ row: 3, col: ROW_SELECTOR }, 'next', 4, 10)).toEqual({ row: 3, col: 0 });
		expect(moveCursor({ row: 3, col: 0 }, 'previous', 4, 10)).toEqual({ row: 2, col: 3 });
		expect(moveCursor({ row: 3, col: ROW_SELECTOR }, 'previous', 4, 10)).toEqual({ row: 2, col: 3 });
	});

	it('stays at the ends of the list', () => {
		expect(moveCursor({ row: 9, col: 3 }, 'next', 4, 10)).toEqual({ row: 9, col: 3 });
		expect(moveCursor({ row: 0, col: 0 }, 'previous', 4, 10)).toEqual({ row: 0, col: 0 });
	});

	it('keeps a column beyond fewer columns (hidden ones) on the last', () => {
		expect(moveCursor({ row: 0, col: 7 }, 'right', 4, 10)).toEqual({ row: 0, col: 3 });
	});
});

describe('rowsToTsv', () => {
	it('writes captions and rows as tab-separated lines', () => {
		expect(rowsToTsv(['No.', 'Name'], [['10000', 'The Cannon Group'], ['20000', 'Selangorian']])).toBe(
			'No.\tName\n10000\tThe Cannon Group\n20000\tSelangorian'
		);
	});

	it('keeps a value with tabs and line breaks in one cell', () => {
		expect(tsvField('a\tb\r\nc')).toBe('a b c');
	});
});
