import { describe, expect, it } from 'vitest';
import { dataUrlBase64, readFileBase64 } from '../fileInput';

describe('fileInput', () => {
	it('takes the base64 part of a data URL', () => {
		expect(dataUrlBase64('data:text/plain;base64,QUJD')).toBe('QUJD');
		expect(dataUrlBase64('data:;base64,')).toBe('');
		expect(dataUrlBase64('QUJD')).toBe('');
	});

	it('reads a file as base64', async () => {
		const file = new Blob(['ABC'], { type: 'text/plain' });
		expect(await readFileBase64(file)).toBe('QUJD');
	});
});
