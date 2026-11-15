import { describe, it, expect } from 'vitest';
import { recordKeyPath } from './api';

describe('recordKeyPath', () => {
	it('encodes each value of the key', () => {
		expect(recordKeyPath('2%8D')).toBe('2%258D');
		expect(recordKeyPath('A/B')).toBe('A%2FB');
		expect(recordKeyPath('30 DAYS')).toBe('30%20DAYS');
	});
	it('keeps the commas between the values of a composite key', () => {
		expect(recordKeyPath('DOMESTIC,STANDARD')).toBe('DOMESTIC,STANDARD');
		expect(recordKeyPath('2,PI%1')).toBe('2,PI%251');
	});
});
