// The cell cursor of a list outside edit mode: a column of the cursor row, or the row selector
// box before the first column (ROW_SELECTOR: the whole row is selected, Ctrl+C copies the row)

export const ROW_SELECTOR = -1;

export type CursorMove = 'left' | 'right' | 'next' | 'previous';

export interface CursorCell {
	row: number; // absolute row (offset + window position)
	col: number; // column, or ROW_SELECTOR
}

// The cell after a move. Left/Right stay in the row (Left from the first column lands on the row
// selector); Tab (next) and Shift+Tab (previous) go on in the next / previous row at the
// row's end and skip the row selector. At the list's ends the cursor stays.
export function moveCursor(from: CursorCell, move: CursorMove, columns: number, rows: number): CursorCell {
	if (columns <= 0) return from;
	const last = columns - 1;
	const col = Math.min(from.col, last);
	switch (move) {
		case 'left':
			return { row: from.row, col: Math.max(col - 1, ROW_SELECTOR) };
		case 'right':
			return { row: from.row, col: Math.min(col + 1, last) };
		case 'next':
			if (col < last) return { row: from.row, col: col + 1 };
			return from.row < rows - 1 ? { row: from.row + 1, col: 0 } : { row: from.row, col: last };
		case 'previous':
			if (col > 0) return { row: from.row, col: col - 1 };
			return from.row > 0 ? { row: from.row - 1, col: last } : { row: from.row, col: 0 };
	}
}
