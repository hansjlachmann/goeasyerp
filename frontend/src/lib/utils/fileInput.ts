/**
 * A file the user picks in an input dialog (a report option of type "file", e.g. Import
 * Consolidation from File): its contents go to the server as base64 text
 */

/** The base64 part of a data URL ("data:text/plain;base64,QUJD" → "QUJD"; "" when there is none) */
export function dataUrlBase64(dataUrl: string): string {
	const comma = dataUrl.indexOf(',');
	if (!dataUrl.startsWith('data:') || comma < 0) return '';
	return dataUrl.slice(comma + 1);
}

/** The contents of a file as base64 */
export function readFileBase64(file: Blob): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onload = () => resolve(dataUrlBase64(String(reader.result ?? '')));
		reader.onerror = () => reject(reader.error);
		reader.readAsDataURL(file);
	});
}
