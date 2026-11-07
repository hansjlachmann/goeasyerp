// Header fields above a list (a journal's Batch Name): an action's shortcut works there too, as
// the cursor starts there when the page opens. Pure helpers, kept out of ListPage so they are
// unit-tested.

/** The input of a list page's header field the event comes from, or null */
export function headerFieldInput(target: EventTarget | null): HTMLInputElement | null {
	if (!(target instanceof HTMLInputElement)) return null;
	return target.hasAttribute('data-header-param') ? target : null;
}

/** The user typed something that is not applied yet (Enter or leaving the field applies it) */
export function headerValueTyped(input: HTMLInputElement): boolean {
	return input.value !== (input.getAttribute('data-applied-value') ?? '');
}
