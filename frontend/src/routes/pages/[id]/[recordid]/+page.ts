import type { PageLoad } from './$types';
import { parsePageUrl } from '$lib/utils/pageUrl';

export const load: PageLoad = ({ params, url }) => {
	const page = parsePageUrl(url, parseInt(params.id, 10), params.recordid);
	return {
		pageId: page.pageId,
		recordId: page.recordId,
		navigationQuery: page.navigationQuery,
		edit: page.edit
	};
};
