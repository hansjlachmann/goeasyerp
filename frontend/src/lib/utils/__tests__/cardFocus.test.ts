import { describe, it, expect, beforeEach } from 'vitest';
import { cardFields, wrapTarget, restoreTarget } from '../cardFocus';

const all = () => true;

describe('card focus', () => {
	let container: HTMLElement;
	beforeEach(() => {
		document.body.innerHTML = `
			<div id="card">
				<div class="card" data-section="General">
					<div class="card-header" tabindex="-1">General</div>
					<input id="no" /><input id="name" />
				</div>
				<div class="subpage-part"><input id="line-cell" /></div>
				<div class="card" data-section="Invoicing">
					<input id="bill_to" /><input id="skipped" tabindex="-1" /><input id="off" disabled />
				</div>
			</div>`;
		container = document.getElementById('card')!;
	});

	it('lists the card fields in page order, without the lines and unreachable controls', () => {
		expect(cardFields(container, all).map((e) => e.id)).toEqual(['no', 'name', 'bill_to']);
	});

	it('wraps Tab at the ends of the card', () => {
		expect(wrapTarget(container, 'end', true, all)?.id).toBe('no'); // Tab past the last field
		expect(wrapTarget(container, 'start', true, all)?.id).toBe('bill_to'); // Shift+Tab before the first
		expect(wrapTarget(container, 'start', false, all)?.id).toBe('no'); // Tab from the action bar
		expect(wrapTarget(container, 'end', false, all)?.id).toBe('bill_to');
		expect(wrapTarget(document.createElement('div'), 'end', true, all)).toBeNull();
	});

	it('puts the cursor back into the last field, else the one with its id, else the first', () => {
		const name = document.getElementById('name')!;
		expect(restoreTarget(container, name, 'name', all)).toBe(name);
		const gone = document.createElement('input');
		expect(restoreTarget(container, gone, 'bill_to', all)?.id).toBe('bill_to');
		expect(restoreTarget(container, null, '', all)?.id).toBe('no');
		expect(restoreTarget(container, null, '', (el) => el.id !== 'no')?.id).toBe('name');
	});
});
