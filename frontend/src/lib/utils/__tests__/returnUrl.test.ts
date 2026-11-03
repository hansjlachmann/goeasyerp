import { describe, it, expect, beforeEach } from 'vitest';
import { withReturn, safeReturnUrl } from '../returnUrl';

describe('returnUrl', () => {
	beforeEach(() => {
		window.history.replaceState({}, '', '/pages/22?filter=city%3DOslo&back=%2Fold&select=OLD');
	});

	it('adds the current page (with its own way back, without an earlier select) as back', () => {
		const url = withReturn('/pages/25?filter=customer_no%3DC00010', 'C00010');
		expect(url.startsWith('/pages/25?filter=customer_no%3DC00010&back=')).toBe(true);
		const back = new URL(url, 'http://app').searchParams.get('back');
		expect(back).toBe('/pages/22?filter=city%3DOslo&back=%2Fold&select=C00010');
		// back from there: the page it was opened from
		expect(new URL(back!, 'http://app').searchParams.get('back')).toBe('/old');
	});

	it('from a modal card: the way back opens the card again on the record (card=1)', () => {
		const back = new URL(withReturn('/pages/151?filter=no%3DC00010', 'C00010', true), 'http://app').searchParams.get('back')!;
		expect(back).toBe('/pages/22?filter=city%3DOslo&back=%2Fold&select=C00010&card=1');
		window.history.replaceState({}, '', back);
		const again = new URL(withReturn('/pages/25', 'C00020'), 'http://app').searchParams.get('back');
		expect(again).toBe('/pages/22?filter=city%3DOslo&back=%2Fold&select=C00020');
	});

	it('uses ? when the target has no query yet', () => {
		expect(withReturn('/pages/25')).toMatch(/^\/pages\/25\?back=/);
	});

	it('only accepts paths of this app', () => {
		expect(safeReturnUrl('/pages/22?select=C1')).toBe('/pages/22?select=C1');
		expect(safeReturnUrl('https://evil.example')).toBeUndefined();
		expect(safeReturnUrl('//evil.example')).toBeUndefined();
		expect(safeReturnUrl('/\\evil.example')).toBeUndefined();
		expect(safeReturnUrl('')).toBeUndefined();
		expect(safeReturnUrl(null)).toBeUndefined();
	});
});
