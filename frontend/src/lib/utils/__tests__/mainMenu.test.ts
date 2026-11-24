import { describe, it, expect } from 'vitest';
import { visibleRows, searchMenu, parentKeys } from '../mainMenu';
import type { MenuGroup } from '#lib/types/pages.js';

const modules: MenuGroup[] = [
	{
		name: 'General Ledger',
		items: [
			{ name: 'Chart of Accounts', page_id: 16, enabled: true },
			{ separator: true },
			{
				name: 'Setup',
				enabled: true,
				items: [
					{ name: 'Accounting Periods', page_id: 100, enabled: true },
					{ name: 'General Posting Groups', enabled: true, items: [{ name: 'Business Posting Groups', page_id: 312, enabled: true }] }
				]
			},
			{ name: 'Budgets', page_id: 113, enabled: false }
		]
	},
	{ name: 'Sales & Receivables', items: [{ name: 'Customers', page_id: 22, enabled: true }, { name: 'Budgets (sales)', enabled: false }] }
];

describe('mainMenu', () => {
	it('shows a sub-menu closed, its entries once expanded', () => {
		const items = modules[0].items;
		expect(visibleRows(items, new Set(), '/General Ledger').map((r) => r.item.name ?? '—')).toEqual(['Chart of Accounts', '—', 'Setup', 'Budgets']);
		const rows = visibleRows(items, new Set(['/General Ledger/Setup']), '/General Ledger');
		expect(rows.map((r) => `${r.depth}:${r.item.name ?? '—'}`)).toEqual([
			'0:Chart of Accounts', '0:—', '0:Setup', '1:Accounting Periods', '1:General Posting Groups', '0:Budgets'
		]);
		expect(rows[2].submenu && rows[2].expanded).toBe(true);
	});

	it('searches the entries of all modules, the available ones first, with their path', () => {
		const found = searchMenu(modules, 'budg');
		expect(found.map((r) => r.item.name)).toEqual(['Budgets', 'Budgets (sales)']);
		const groups = searchMenu(modules, 'business posting');
		expect(groups).toHaveLength(1);
		expect(groups[0].path).toEqual(['General Ledger', 'Setup', 'General Posting Groups', 'Business Posting Groups']);
		expect(groups[0].module).toBe(0);
		expect(searchMenu(modules, '  ')).toEqual([]);
	});

	it('gives the sub-menus around an entry', () => {
		expect(parentKeys('/General Ledger/Setup/General Posting Groups/Business Posting Groups')).toEqual([
			'/General Ledger', '/General Ledger/Setup', '/General Ledger/Setup/General Posting Groups'
		]);
	});
});
