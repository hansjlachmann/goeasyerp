// The main menu (Main Menu, classic forms 330–337): modules, each with its menu of entries —
// pages, reports/codeunits, sub-menus (Setup, Activities …) and separators. These helpers give
// the rows a module shows with its sub-menus expanded or not, and the menu search.
import type { MenuGroup, MenuItem } from '#lib/types/pages.js';

export interface MenuRow {
	item: MenuItem;
	depth: number; // 0 for the module's own entries, 1 and 2 inside sub-menus
	key: string; // the entry's path of names: unique within the menu
	submenu: boolean;
	expanded: boolean;
}

// The rows of a module's menu: its entries, and the entries of the expanded sub-menus below
// their sub-menu (separators included)
export function visibleRows(items: MenuItem[] | undefined, expanded: Set<string>, parentKey = '', depth = 0): MenuRow[] {
	const rows: MenuRow[] = [];
	(items ?? []).forEach((item, i) => {
		const key = `${parentKey}/${item.separator ? `—${i}` : item.name}`;
		const submenu = (item.items?.length ?? 0) > 0;
		const isExpanded = submenu && expanded.has(key);
		rows.push({ item, depth, key, submenu, expanded: isExpanded });
		if (isExpanded) rows.push(...visibleRows(item.items, expanded, key, depth + 1));
	});
	return rows;
}

export interface MenuSearchResult {
	item: MenuItem;
	module: number; // index of the module
	path: string[]; // module › sub-menus › entry
}

// The entries (not sub-menus or separators) whose name contains every word of the query, case
// insensitive, the available ones first
export function searchMenu(modules: MenuGroup[], query: string): MenuSearchResult[] {
	const words = query.toLowerCase().split(/\s+/).filter(Boolean);
	if (words.length === 0) return [];
	const results: MenuSearchResult[] = [];
	const walk = (items: MenuItem[] | undefined, module: number, path: string[]) => {
		for (const item of items ?? []) {
			if (item.separator || !item.name) continue;
			if (item.items?.length) {
				walk(item.items, module, [...path, item.name]);
				continue;
			}
			const name = item.name.toLowerCase();
			if (words.every((w) => name.includes(w))) results.push({ item, module, path: [...path, item.name] });
		}
	};
	modules.forEach((m, i) => walk(m.items, i, [m.name]));
	return results.sort((a, b) => Number(b.item.enabled === true) - Number(a.item.enabled === true));
}

// The sub-menus to expand so that an entry (its key) is visible
export function parentKeys(key: string): string[] {
	const parts = key.split('/').slice(1);
	const keys: string[] = [];
	for (let i = 1; i < parts.length; i++) keys.push('/' + parts.slice(0, i).join('/'));
	return keys;
}
