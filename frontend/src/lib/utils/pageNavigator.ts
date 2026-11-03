// How a page moves on: in the browser it navigates (goto), in a page frame — a page shown
// over another one, opened from a page in a modal (the New of a lookup) — the pages it opens
// open over it too, and closing one (Esc, the close button) goes back to the page under it:
// back the way the pages were opened. The page frames host (PageFrames.svelte) sets the
// navigator of the pages inside it.

import { getContext, setContext } from 'svelte';
import { goto, replaceState } from '$app/navigation';

export interface PageNavigator {
	/** another page: a drilldown, an action's page, a list's card */
	open(url: string): void;
	/** this page with another record or other parameters (record navigation, a journal batch) */
	move(url: string, options?: { replaceState?: boolean }): void;
	/** back where the page came from — in the browser to url (none: the browser's back) */
	close(url?: string): void;
	/** the browser address of this page (in a frame: none) */
	replaceState(url: string): void;
	/** this page's URL (in a frame: the frame's) */
	location(): URL;
	/** the page is in a frame: it closes back to the page under it */
	framed: boolean;
}

export const browserNavigator: PageNavigator = {
	open: (url) => goto(url),
	move: (url, options) => goto(url, options),
	close: (url) => (url ? goto(url) : window.history.back()),
	replaceState: (url) => replaceState(url, {}),
	location: () => new URL(window.location.href),
	framed: false
};

const key = Symbol('page-navigator');

/** The navigator of the component's page (call during component init) */
export function pageNavigator(): PageNavigator {
	return getContext<PageNavigator | undefined>(key) ?? browserNavigator;
}

/** Sets the navigator of the pages inside the component */
export function setPageNavigator(navigator: PageNavigator) {
	setContext(key, navigator);
}
