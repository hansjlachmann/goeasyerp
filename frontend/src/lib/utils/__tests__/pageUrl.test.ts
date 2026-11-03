import { describe, it, expect } from 'vitest';
import { parsePagePath } from '../pageUrl';

describe('parsePagePath', () => {
	it('reads a list page: filters, the way back, a new record, its own parameters', () => {
		const page = parsePagePath('/pages/457?filter=series_code%3DSO&back=%2Fpages%2F459&new=1&template=GENERAL');
		expect(page).toMatchObject({ pageId: 457, filter: ['series_code=SO'], returnUrl: '/pages/459', startNew: true, openParams: { template: 'GENERAL' } });
		expect(page?.recordId).toBeUndefined();
	});

	it('reads a card page: the record, Edit', () => {
		expect(parsePagePath('/pages/21/C%2000010?edit=1')).toMatchObject({ pageId: 21, recordId: 'C 00010', edit: true });
	});

	it('is null for a path that is not a page', () => {
		expect(parsePagePath('/')).toBeNull();
		expect(parsePagePath('/pages/abc')).toBeNull();
	});
});
