import { describe, it, expect } from 'vitest';
import { parsePagePath } from './pageUrl';

describe('parsePagePath', () => {
	it('reads the list filters and FlowFilters a return URL keeps', () => {
		const page = parsePagePath(
			'/pages/22?filter=' + encodeURIComponent('no=C0*') + '&flowfilter=' + encodeURIComponent('date_filter=..30.06.26') + '&select=C00010'
		);
		expect(page?.pageId).toBe(22);
		expect(page?.filter).toEqual(['no=C0*']);
		expect(page?.flowFilter).toEqual(['date_filter=..30.06.26']);
		expect(page?.select).toBe('C00010');
		// filter and flowfilter are the app's, not the page's own parameters
		expect(page?.openParams).toEqual({});
	});
});
