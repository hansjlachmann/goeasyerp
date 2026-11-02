import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { printoutOf, printoutsOf, pdfBlob, showPreview, closePreview, printPreview } from '../printout';

describe('printoutOf', () => {
	it('takes the PDF, the file name and the output of a job result', () => {
		expect(printoutOf({ pdf: 'JVBERi0=', filename: 'Sales Quote SQ1.pdf', output: 'preview' })).toEqual({
			pdf: 'JVBERi0=',
			filename: 'Sales Quote SQ1.pdf',
			output: 'preview'
		});
		expect(printoutOf({ pdf: 'JVBERi0=', output: 'print' })?.output).toBe('print');
	});

	it('downloads when the output is not known, with a default name', () => {
		expect(printoutOf({ pdf: 'JVBERi0=' })).toEqual({ pdf: 'JVBERi0=', filename: 'document.pdf', output: 'pdf' });
		expect(printoutOf({ pdf: 'JVBERi0=', output: 'printer' })?.output).toBe('pdf');
	});

	it('is null without a PDF', () => {
		expect(printoutOf(undefined)).toBeNull();
		expect(printoutOf({ refresh: true })).toBeNull();
		expect(printoutOf({ pdf: '' })).toBeNull();
	});
});

describe('pdfBlob', () => {
	it('decodes base64, also with a data URL prefix', () => {
		for (const text of ['JVBERi0=', 'data:application/pdf;base64,JVBERi0=']) {
			const blob = pdfBlob(text);
			expect(blob.type).toBe('application/pdf');
			expect(blob.size).toBe(5); // %PDF-
		}
	});
});

describe('printoutsOf', () => {
	it('gives the first printout and the others printed with it', () => {
		const list = printoutsOf({
			pdf: 'JVBERi0=',
			filename: 'Sales - Shipment 102001.pdf',
			output: 'print',
			printouts: [{ pdf: 'JVBERi1=', filename: 'Sales - Invoice 103001.pdf', output: 'print' }, { pdf: '' }]
		});
		expect(list.map((p) => p.filename)).toEqual(['Sales - Shipment 102001.pdf', 'Sales - Invoice 103001.pdf']);
	});

	it('is empty without a printout', () => {
		expect(printoutsOf({ refresh: true })).toEqual([]);
		expect(printoutsOf(null)).toEqual([]);
	});
});

describe('preview queue', () => {
	it('shows the next preview when the one shown is closed', () => {
		const blob = new Blob(['x']);
		showPreview({ url: 'blob:1', blob, filename: 'a.pdf' });
		showPreview({ url: 'blob:2', blob, filename: 'b.pdf' });
		expect(get(printPreview)?.filename).toBe('a.pdf');
		closePreview();
		expect(get(printPreview)?.filename).toBe('b.pdf');
		closePreview();
		expect(get(printPreview)).toBeNull();
	});
});
