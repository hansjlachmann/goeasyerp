// Content of the Keyboard Shortcuts help window (/help/shortcuts). Key names are shown as
// written; titles and descriptions are translation keys (translations/{lang}/messages.yaml).
// Keep this in step with the keyboard tables in CLAUDE.md when shortcuts change.

export interface ShortcutHelpItem {
	keys: string[]; // alternatives, e.g. ['Alt+N', 'Ctrl+Insert']
	description: string; // translation key
}

export interface ShortcutHelpSection {
	title: string; // translation key
	items: ShortcutHelpItem[];
}

export const SHORTCUT_HELP_URL = '/help/shortcuts';
export const SHORTCUT_HELP_WINDOW = 'openerp-shortcuts';

/**
 * Opens the Keyboard Shortcuts help as a detached window. The window name makes a second
 * call reuse (and raise) the same window instead of opening another one.
 */
export function openShortcutHelp() {
	const width = 820;
	const height = 760;
	const left = Math.max(0, window.screenX + (window.outerWidth - width) / 2);
	const top = Math.max(0, window.screenY + (window.outerHeight - height) / 2);
	const win = window.open(
		SHORTCUT_HELP_URL,
		SHORTCUT_HELP_WINDOW,
		`popup=yes,width=${width},height=${height},left=${left},top=${top}`
	);
	win?.focus();
}

export const shortcutHelpSections: ShortcutHelpSection[] = [
	{
		title: 'HELP_SEC_GENERAL',
		items: [
			{ keys: ['Ctrl+O'], description: 'HELP_SWITCH_COMPANY' },
			{ keys: ['Esc'], description: 'HELP_ESCAPE' }
		]
	},
	{
		title: 'HELP_SEC_DATES',
		items: [
			{ keys: ['05', '0509', '050926'], description: 'HELP_DATE_SHORT' },
			{ keys: ['t', 'w'], description: 'HELP_DATE_TODAY_WORKDATE' },
			{ keys: ['mo … su'], description: 'HELP_DATE_WEEKDAY' },
			{ keys: ['+1w', '-3d', 'cm'], description: 'HELP_DATE_FORMULA' },
			{ keys: ['Alt+↓', 'F4'], description: 'HELP_DATE_CALENDAR' }
		]
	},
	{
		title: 'HELP_SEC_MENU',
		items: [
			{ keys: ['↑', '↓', '←', '→'], description: 'HELP_MENU_ARROWS' },
			{ keys: ['Home', 'End'], description: 'HELP_MENU_HOME_END' },
			{ keys: ['Enter'], description: 'HELP_MENU_OPEN' },
			{ keys: ['Ctrl+F'], description: 'HELP_MENU_SEARCH' }
		]
	},
	{
		title: 'HELP_SEC_LIST',
		items: [
			{ keys: ['↑', '↓'], description: 'HELP_LIST_ROWS' },
			{ keys: ['←', '→', 'Tab', 'Shift+Tab'], description: 'HELP_LIST_CELL_CURSOR' },
			{ keys: ['Shift+↑', 'Shift+↓', 'Shift+PgUp', 'Shift+PgDn', 'Shift+Home', 'Shift+End'], description: 'HELP_LIST_SELECT_RANGE' },
			{ keys: ['Ctrl+A'], description: 'HELP_LIST_SELECT_ALL' },
			{ keys: ['Ctrl+Click', 'Shift+Click'], description: 'HELP_LIST_SELECT_CLICK' },
			{ keys: ['Click+Drag'], description: 'HELP_LIST_SELECT_DRAG' },
			{ keys: ['Ctrl+C'], description: 'HELP_LIST_COPY_RECORDS' },
			{ keys: ['Ctrl+V'], description: 'HELP_LIST_PASTE_RECORDS' },
			{ keys: ['Home', 'End', 'Ctrl+Home', 'Ctrl+End'], description: 'HELP_LIST_FIRST_LAST' },
			{ keys: ['PgUp', 'PgDn'], description: 'HELP_LIST_PAGE' },
			{ keys: ['Enter'], description: 'HELP_LIST_OPEN' },
			{ keys: ['F2'], description: 'HELP_LIST_EDIT_CELLS' },
			{ keys: ['Ctrl+E'], description: 'HELP_LIST_EDIT' },
			{ keys: ['Alt+N', 'Ctrl+Insert'], description: 'HELP_LIST_NEW' },
			{ keys: ['Ctrl+D'], description: 'HELP_LIST_DELETE' },
			{ keys: ['Ctrl+F'], description: 'HELP_LIST_SEARCH' },
			{ keys: ['Ctrl+F7'], description: 'HELP_LIST_FILTER_PANE' },
			{ keys: ['Ctrl+Alt+Q'], description: 'HELP_LIST_NAVIGATE' },
			{ keys: ['Ctrl+Alt+F9'], description: 'HELP_PREVIEW_POSTING' },
			{ keys: ['Shift+F11'], description: 'HELP_POST_PRINT' },
			{ keys: ['Esc'], description: 'HELP_LIST_ESCAPE' }
		]
	},
	{
		title: 'HELP_SEC_CELLS',
		items: [
			{ keys: ['↑', '↓', '←', '→'], description: 'HELP_CELL_ARROWS' },
			{ keys: ['Tab', 'Shift+Tab'], description: 'HELP_CELL_TAB' },
			{ keys: ['Enter'], description: 'HELP_CELL_ENTER' },
			{ keys: ['PgUp', 'PgDn'], description: 'HELP_CELL_PAGE' },
			{ keys: ['F2'], description: 'HELP_CELL_F2' },
			{ keys: ['F8'], description: 'HELP_CELL_F8' },
			{ keys: ['Delete'], description: 'HELP_CELL_DELETE' },
			{ keys: ['Backspace'], description: 'HELP_CELL_BACKSPACE' },
			{ keys: ['Ctrl+C', 'Ctrl+V'], description: 'HELP_CELL_COPY_PASTE' },
			{ keys: ['Space'], description: 'HELP_CELL_SPACE' },
			{ keys: ['Esc'], description: 'HELP_CELL_ESCAPE' }
		]
	},
	{
		title: 'HELP_SEC_CARD',
		items: [
			{ keys: ['Tab', 'Enter'], description: 'HELP_CARD_NEXT' },
			{ keys: ['Shift+Tab', 'Shift+Enter'], description: 'HELP_CARD_PREV' },
			{ keys: ['Ctrl+↑', 'Ctrl+↓'], description: 'HELP_CARD_PREV_NEXT_RECORD' },
			{ keys: ['Ctrl+Home', 'Ctrl+End'], description: 'HELP_CARD_FIRST_LAST_RECORD' },
			{ keys: ['Shift+F2'], description: 'HELP_CARD_ASSIST_EDIT' },
			{ keys: ['Ctrl+Shift+F12'], description: 'HELP_CARD_FULLSCREEN' },
			{ keys: ['Esc'], description: 'HELP_CARD_ESCAPE' }
		]
	},
	{
		title: 'HELP_SEC_LOOKUP',
		items: [
			{ keys: ['↓', 'Alt+↓', 'F4'], description: 'HELP_LOOKUP_OPEN' },
			{ keys: ['↑', '↓'], description: 'HELP_LOOKUP_MOVE' },
			{ keys: ['PgUp', 'PgDn'], description: 'HELP_LOOKUP_PAGE' },
			{ keys: ['Alt+N'], description: 'HELP_LOOKUP_NEW' },
			{ keys: ['Enter'], description: 'HELP_LOOKUP_PICK' },
			{ keys: ['Esc'], description: 'HELP_LOOKUP_ESCAPE' }
		]
	}
];
