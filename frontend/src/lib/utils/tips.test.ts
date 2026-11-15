import { describe, it, expect, beforeEach } from 'vitest';
import { TIPS, tipClosed, closeTip } from './tips';

describe('tips', () => {
	beforeEach(() => sessionStorage.clear());

	it('every tip has its own id and a translation key', () => {
		const ids = Object.values(TIPS).map((tip) => tip.id);
		expect(new Set(ids).size).toBe(ids.length);
		for (const tip of Object.values(TIPS)) expect(tip.text).toMatch(/^TIP_/);
	});

	it('a closed tip stays closed for the session, the others stay open', () => {
		expect(tipClosed(TIPS.mainMenu.id)).toBe(false);
		closeTip(TIPS.mainMenu.id);
		closeTip(TIPS.mainMenu.id);
		expect(tipClosed(TIPS.mainMenu.id)).toBe(true);
		expect(tipClosed(TIPS.newNumber.id)).toBe(false);
		expect(JSON.parse(sessionStorage.getItem('tips-closed') ?? '[]')).toEqual([TIPS.mainMenu.id]);
	});
});
