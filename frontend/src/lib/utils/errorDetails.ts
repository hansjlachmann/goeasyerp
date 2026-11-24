import type { ErrorDetails } from '#lib/services/jobs.js';
import { downloadBlob } from '#lib/utils/printout.js';
import { spreadsheetSafe } from '#lib/utils/clipboard.js';

/** A CSV cell: quoted when it holds the separator, a quote or a line break */
function csvCell(text: string): string {
	const value = spreadsheetSafe(text);
	return /[";\r\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}

/** An error's details as CSV (semicolons, so decimal commas stay in their cell): each section
 *  its caption, the column captions and the rows, a blank line between sections */
export function errorDetailsToCsv(details: ErrorDetails): string {
	const lines: string[] = [csvCell(details.title)];
	for (const section of details.sections) {
		lines.push('', csvCell(section.caption));
		lines.push(section.columns.map((c) => csvCell(c.caption)).join(';'));
		for (const row of section.rows) lines.push(row.map(csvCell).join(';'));
	}
	return lines.join('\r\n') + '\r\n';
}

/** Saves an error's details as a CSV file (with a byte order mark, so spreadsheets read UTF-8) */
export function downloadErrorDetails(details: ErrorDetails, filename: string) {
	const blob = new Blob(['﻿' + errorDetailsToCsv(details)], { type: 'text/csv;charset=utf-8' });
	downloadBlob(blob, filename);
}
