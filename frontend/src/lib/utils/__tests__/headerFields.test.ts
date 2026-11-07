import { describe, expect, it } from 'vitest';
import { headerFieldInput, headerValueTyped } from '../headerFields';

function input(applied?: string, value = applied ?? ''): HTMLInputElement {
	const el = document.createElement('input');
	if (applied !== undefined) {
		el.setAttribute('data-header-param', 'batch');
		el.setAttribute('data-applied-value', applied);
	}
	el.value = value;
	return el;
}

describe('headerFieldInput', () => {
	it('finds a header field', () => {
		const el = input('DEFAULT');
		expect(headerFieldInput(el)).toBe(el);
	});
	it('leaves other inputs and elements alone', () => {
		expect(headerFieldInput(input())).toBeNull();
		expect(headerFieldInput(document.createElement('div'))).toBeNull();
		expect(headerFieldInput(null)).toBeNull();
	});
});

describe('headerValueTyped', () => {
	it('is false while the field shows the applied value', () => {
		expect(headerValueTyped(input('DEFAULT'))).toBe(false);
	});
	it('is true for a value not applied yet', () => {
		expect(headerValueTyped(input('DEFAULT', 'OTHER'))).toBe(true);
	});
});
