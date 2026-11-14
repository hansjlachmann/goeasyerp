import { describe, it, expect } from 'vitest';
import { companyLabel, companyLabelFor, filterCompanies, type CompanyInfo } from '../company';

const companies: CompanyInfo[] = [
	{ name: 'demo01', display_name: 'Demo Company 01' },
	{ name: 'acme', display_name: '' }
];

describe('company labels', () => {
	it('shows the display name, else the technical name', () => {
		expect(companyLabel(companies[0])).toBe('Demo Company 01');
		expect(companyLabel(companies[1])).toBe('acme');
		expect(companyLabel({ name: 'x', display_name: '   ' })).toBe('x');
	});

	it('looks a technical name up, falling back to the name itself', () => {
		expect(companyLabelFor(companies, 'demo01')).toBe('Demo Company 01');
		expect(companyLabelFor(companies, 'unknown')).toBe('unknown');
		expect(companyLabelFor([], 'demo01')).toBe('demo01');
	});

	it('filters on display name and technical name, case-insensitive', () => {
		expect(filterCompanies(companies, 'company 01').map((c) => c.name)).toEqual(['demo01']);
		expect(filterCompanies(companies, 'ACM').map((c) => c.name)).toEqual(['acme']);
		expect(filterCompanies(companies, '  ')).toHaveLength(2);
	});
});
