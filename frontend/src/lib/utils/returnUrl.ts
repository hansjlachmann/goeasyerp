// Drilldowns and card actions open another page; Esc or the close button returns
// to the page they came from, on the same record. The target page gets the way back as
// ?back=<path of the origin page, with select=<record key>> — the origin's own back kept in
// it, so each Esc goes back one page the way the pages were opened.

// url with back=<the current page, with its own back>, selecting selectKey there when the user returns; openCard:
// the page is a list with a modal card that was open on that record — it opens again (card=1)
export function withReturn(url: string, selectKey?: string, openCard = false): string {
	const origin = new URL(window.location.href);
	origin.searchParams.delete('select');
	origin.searchParams.delete('card');
	if (selectKey) origin.searchParams.set('select', selectKey);
	if (selectKey && openCard) origin.searchParams.set('card', '1');
	const back = origin.pathname + origin.search;
	return url + (url.includes('?') ? '&' : '?') + 'back=' + encodeURIComponent(back);
}

// The back parameter if it is a path of this app (never an absolute or protocol-relative
// URL, which would let a link send the user to another site)
export function safeReturnUrl(value: string | null | undefined): string | undefined {
	if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('://') || value.includes('\\')) {
		return undefined;
	}
	return value;
}
