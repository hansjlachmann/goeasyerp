import { describe, expect, it } from 'vitest';

import { mayReload, reloadsAtOnce } from './appUpdateRules';

describe('appUpdateRules', () => {
	it('reloads at once only pages that hold no input', () => {
		expect(reloadsAtOnce('/')).toBe(false); // the main menu may show a running routine's dialog
		expect(reloadsAtOnce('/login')).toBe(true);
		expect(reloadsAtOnce('/pages/21')).toBe(false);
		expect(reloadsAtOnce('/pages/22/C00010')).toBe(false);
		expect(reloadsAtOnce('/loginx')).toBe(false);
	});

	it('reloads at most once in 30 seconds', () => {
		const now = 1_000_000;
		expect(mayReload(null, now)).toBe(true);
		expect(mayReload(now - 5_000, now)).toBe(false);
		expect(mayReload(now - 30_000, now)).toBe(true);
		// a clock set back does not block reloads for good
		expect(mayReload(now + 60_000, now)).toBe(true);
	});
});
