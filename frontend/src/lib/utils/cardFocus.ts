// Cursor control on card pages: the card's own fields in Tab order, where Tab wraps to at the
// card's ends, and the field the cursor goes back to when the focus fell out of the card.
import { getFieldControls } from './fieldNavigation';

/** Whether a control is shown (jsdom has no layout: tests pass their own) */
export type IsVisible = (el: HTMLElement) => boolean;
const shown: IsVisible = (el) => el.offsetParent !== null;

/** The card's own field controls in Tab order — shown ones, not a subpage's cells */
export function cardFields(container: HTMLElement, isVisible: IsVisible = shown): HTMLElement[] {
	return getFieldControls(container).filter((el) => !el.closest('.subpage-part') && isVisible(el));
}

/**
 * Where the cursor goes when Tab reaches one of the card's ends (a focus sentinel before or
 * after its sections): coming from inside the card it wraps around — past the last field to the
 * first, before the first to the last —; coming from outside (the action bar) it enters at that
 * end's side. null: the card has no field to go to.
 */
export function wrapTarget(
	container: HTMLElement,
	end: 'start' | 'end',
	fromInside: boolean,
	isVisible: IsVisible = shown
): HTMLElement | null {
	const fields = cardFields(container, isVisible);
	const toFirst = end === 'end' ? fromInside : !fromInside;
	return (toFirst ? fields[0] : fields[fields.length - 1]) ?? null;
}

/**
 * The field to put the cursor back into: the last field the user was in when it is still on the
 * card, else the field with its id (the card drew it anew after a save), else the first field.
 */
export function restoreTarget(
	container: HTMLElement,
	last: HTMLElement | null,
	lastId: string,
	isVisible: IsVisible = shown
): HTMLElement | null {
	const fields = cardFields(container, isVisible);
	if (last && fields.includes(last)) return last;
	if (lastId) {
		const byId = fields.find((f) => f.id === lastId);
		if (byId) return byId;
	}
	return fields[0] ?? null;
}
