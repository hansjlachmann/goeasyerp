import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { shortcutHelpSections } from '../shortcutHelp';
import { HELP, HOME, MENU } from '#lib/services/i18n.svelte.js';

// Every text in the Keyboard Shortcuts window must have a translation in every language
const languages = ['en-US', 'nb-NO', 'da-DK'];
const keys = [
	...Object.values(HELP),
	MENU.SHORTCUTS,
	...Object.values(HOME),
	...shortcutHelpSections.flatMap((s) => [s.title, ...s.items.map((i) => i.description)])
];

describe('shortcut help translations', () => {
	for (const lang of languages) {
		it(`has every key in ${lang}`, () => {
			const file = readFileSync(resolve(__dirname, `../../../../../translations/${lang}/messages.yaml`), 'utf8');
			const missing = keys.filter((k) => !new RegExp(`^\\s+${k}:`, 'm').test(file));
			expect(missing).toEqual([]);
		});
	}
});
