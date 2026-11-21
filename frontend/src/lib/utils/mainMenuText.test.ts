import { describe, it, expect } from 'vitest';
import { mainMenuTextClass, type MainMenuTextLine } from './mainMenuText';

const line = (patch: Partial<MainMenuTextLine>): MainMenuTextLine => ({ text: 'x', style: 'normal', bold: false, italic: false, ...patch });

describe('mainMenuTextClass', () => {
	it('gives each style its look, plus bold and italic', () => {
		expect(mainMenuTextClass(line({ style: 'heading' }))).toContain('text-3xl');
		expect(mainMenuTextClass(line({ style: 'large' }))).toContain('text-xl');
		expect(mainMenuTextClass(line({ style: 'small' }))).toContain('text-sm');
		expect(mainMenuTextClass(line({ bold: true, italic: true }))).toMatch(/font-bold.*italic/);
		expect(mainMenuTextClass(line({}))).not.toContain('font-bold');
	});

	it('shows an unknown style as normal', () => {
		expect(mainMenuTextClass(line({ style: 'huge' as MainMenuTextLine['style'] }))).toBe(mainMenuTextClass(line({})));
	});
});
