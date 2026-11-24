// Tips for new users: short hints shown where they help (My Settings: Show tips). All tips are
// listed here — a tip's id, its text key and where it shows — so new ones are added in one place.
// A tip closed with × stays closed for the browser session; "Don't show tips" switches the
// setting off for the user (saved on the server).
import { TIP } from '#lib/services/i18n.svelte.js';

export interface TipDef {
	id: string;
	text: string; // translation key
}

export const TIPS = {
	/** The main menu (home page): how to move in it */
	mainMenu: { id: 'main-menu', text: TIP.MAIN_MENU },
	/** A new card whose No. comes from a number series (page metadata number_from_series) */
	newNumber: { id: 'new-number', text: TIP.NEW_NUMBER }
} satisfies Record<string, TipDef>;

const storageKey = 'tips-closed';

function closedTips(): string[] {
	try {
		return JSON.parse(sessionStorage.getItem(storageKey) ?? '[]');
	} catch {
		return [];
	}
}

/** The tip was closed in this browser session */
export function tipClosed(id: string): boolean {
	return closedTips().includes(id);
}

/** Closes the tip for this browser session */
export function closeTip(id: string) {
	try {
		const closed = closedTips();
		if (!closed.includes(id)) sessionStorage.setItem(storageKey, JSON.stringify([...closed, id]));
	} catch {
		// no session storage: the tip closes until the page is drawn again
	}
}
