import { describe, expect, it } from 'vitest';
import { rootFontScale, scaledWidth } from './fontScale';

describe('fontScale', () => {
	it('scales the definition widths', () => {
		expect(scaledWidth(120, 1)).toBe(120);
		expect(scaledWidth(120, 1.25)).toBe(150);
		expect(scaledWidth(90, 1.1)).toBe(99);
		expect(scaledWidth(100, 0)).toBe(100);
	});

	it('reads the root font size against 16px', () => {
		const root = document.createElement('div');
		root.style.fontSize = '20px';
		document.body.appendChild(root);
		expect(rootFontScale(root)).toBe(1.25);
		root.remove();
	});
});
