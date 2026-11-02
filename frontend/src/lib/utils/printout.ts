// A printed report's printout (the job result's data: pdf base64, filename, output) — shown
// in the preview, sent to the browser's print dialog, or downloaded as a PDF file.
import { writable } from 'svelte/store';

export type PrintOutput = 'preview' | 'print' | 'pdf';

export interface Printout {
	pdf: string; // base64
	filename: string;
	output: PrintOutput;
}

/** The printout in a job result's data, or null when it holds none */
export function printoutOf(data: Record<string, unknown> | undefined | null): Printout | null {
	if (!data || typeof data.pdf !== 'string' || data.pdf === '') return null;
	const output = data.output === 'preview' || data.output === 'print' ? data.output : 'pdf';
	const filename = typeof data.filename === 'string' && data.filename !== '' ? data.filename : 'document.pdf';
	return { pdf: data.pdf, filename, output };
}

/** The PDF of a base64 text (with or without a data URL prefix) */
export function pdfBlob(base64: string): Blob {
	const clean = base64.replace(/^data:application\/pdf;base64,/, '');
	const chars = atob(clean);
	const bytes = new Uint8Array(chars.length);
	for (let i = 0; i < chars.length; i++) bytes[i] = chars.charCodeAt(i);
	return new Blob([bytes], { type: 'application/pdf' });
}

/** Saves the PDF as a file */
export function downloadPdf(blob: Blob, filename: string) {
	downloadBlob(blob, filename);
}

/** Saves a blob as a file (the browser's download) */
export function downloadBlob(blob: Blob, filename: string) {
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** Opens the browser's print dialog on the PDF (a hidden frame, removed after printing) */
export function printPdf(blob: Blob) {
	const url = URL.createObjectURL(blob);
	const frame = document.createElement('iframe');
	frame.style.position = 'fixed';
	frame.style.width = '0';
	frame.style.height = '0';
	frame.style.border = '0';
	frame.style.right = '0';
	frame.style.bottom = '0';
	frame.src = url;
	frame.onload = () => {
		frame.contentWindow?.focus();
		frame.contentWindow?.print();
		// The dialog blocks until it is closed in most browsers; the frame goes later anyway
		setTimeout(() => {
			frame.remove();
			URL.revokeObjectURL(url);
		}, 60000);
	};
	document.body.appendChild(frame);
}

/** The preview shown over the page (PrintPreview.svelte in the root layout) */
export const printPreview = writable<{ url: string; blob: Blob; filename: string } | null>(null);

/** Shows, prints or downloads a printout as chosen */
export function handlePrintout(printout: Printout) {
	const blob = pdfBlob(printout.pdf);
	if (printout.output === 'preview') {
		printPreview.set({ url: URL.createObjectURL(blob), blob, filename: printout.filename });
	} else if (printout.output === 'print') {
		printPdf(blob);
	} else {
		downloadPdf(blob, printout.filename);
	}
}

/** A file in a job result's data (a processing report's declaration: file base64, filename) */
export function resultFileOf(data: Record<string, unknown> | undefined | null): { blob: Blob; filename: string } | null {
	if (!data || typeof data.file !== 'string') return null;
	const chars = atob(data.file);
	const bytes = new Uint8Array(chars.length);
	for (let i = 0; i < chars.length; i++) bytes[i] = chars.charCodeAt(i);
	const filename = typeof data.filename === 'string' && data.filename !== '' ? data.filename : 'file.txt';
	return { blob: new Blob([bytes], { type: 'application/octet-stream' }), filename };
}
