import type { PageLoad } from './$types';
import { parsePageUrl } from '$lib/utils/pageUrl';

export const load: PageLoad = ({ params, url }) => {
	const page = parsePageUrl(url, parseInt(params.id, 10));
	return {
		pageId: page.pageId,
		openParams: page.openParams,
		recordId: page.recordId,
		filter: page.filter,
		flowFilter: page.flowFilter,
		returnUrl: page.returnUrl,
		select: page.select,
		openCard: page.openCard,
		startNew: page.startNew
	};
};
