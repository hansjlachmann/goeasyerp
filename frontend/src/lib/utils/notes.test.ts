import { describe, it, expect } from 'vitest';
import { notesOf } from './notes';

function answer(header?: string): Response {
	return new Response('{}', { headers: header ? { 'X-App-Notes': header } : {} });
}

describe('notesOf', () => {
	it('reads the notes the server sends with an answer', () => {
		// Go's url.QueryEscape: spaces as +, other characters percent-encoded
		const header = '%5B%22Du+kan+ikke+tildele+numre+st%C3%B8rre+enn+C00030%22%5D';
		expect(notesOf(answer(header))).toEqual(['Du kan ikke tildele numre større enn C00030']);
	});

	it('gives none without the header or with a broken one', () => {
		expect(notesOf(answer())).toEqual([]);
		expect(notesOf(answer('%5Bbroken'))).toEqual([]);
	});
});
