import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

const savePreference = vi.fn(async () => {});
const getPreferences = vi.fn(async (): Promise<{ preference_name: string; preference_data: string }[]> => []);
// the browser storage helpers ($app/environment is not available in the unit tests)
vi.mock('./storage', () => ({
	getJson: (key: string, fallback: unknown) => JSON.parse(localStorage.getItem(key) ?? 'null') ?? fallback,
	setJson: (key: string, value: unknown) => localStorage.setItem(key, JSON.stringify(value)),
	getBoolean: (key: string, fallback: boolean) => (localStorage.getItem(key) === null ? fallback : localStorage.getItem(key) === 'true'),
	setBoolean: (key: string, value: boolean) => localStorage.setItem(key, value ? 'true' : 'false'),
	remove: (key: string) => localStorage.removeItem(key)
}));
vi.mock('$lib/services/api', () => ({ api: { savePreference, getPreferences } }));

const storage = await import('./customizationStorage');

describe('page layouts on the server', () => {
	beforeEach(() => {
		localStorage.clear();
		savePreference.mockClear();
		getPreferences.mockReset();
		vi.useFakeTimers();
	});

	it('sends a change to the server shortly after the last one', () => {
		storage.saveColumnWidths('HANS', 22, { name: 200 });
		storage.saveColumnWidths('HANS', 22, { name: 240 });
		storage.saveRowNumbersPreference('HANS', 22, true);
		expect(savePreference).not.toHaveBeenCalled();
		vi.advanceTimersByTime(600);
		expect(savePreference).toHaveBeenCalledTimes(1);
		expect(savePreference).toHaveBeenCalledWith(22, 'LAYOUT', 'DEFAULT', { customizations: {}, widths: { name: 240 }, row_numbers: true }, false);
	});

	it('keeps an anonymous layout in the browser only', () => {
		storage.saveColumnWidths('anonymous', 22, { name: 200 });
		vi.advanceTimersByTime(600);
		expect(savePreference).not.toHaveBeenCalled();
	});

	it("takes the server's layout into the browser and tells the pages", async () => {
		vi.useRealTimers();
		getPreferences.mockResolvedValue([
			{ preference_name: 'DEFAULT', preference_data: JSON.stringify({ customizations: { city: { visible: false } }, widths: { no: 90 }, row_numbers: true }) }
		]);
		const version = get(storage.layoutVersion);
		await storage.syncPageLayout('HANS', 31);
		expect(storage.loadPageCustomizations('HANS', 31)).toEqual({ city: { visible: false } });
		expect(storage.loadColumnWidths('HANS', 31)).toEqual({ no: 90 });
		expect(storage.loadRowNumbersPreference('HANS', 31)).toBe(true);
		expect(get(storage.layoutVersion)).toBe(version + 1);
		// once per page and session
		await storage.syncPageLayout('HANS', 31);
		expect(getPreferences).toHaveBeenCalledTimes(1);
	});

	it('sends a layout kept in the browser when the server has none', async () => {
		vi.useRealTimers();
		getPreferences.mockResolvedValue([]);
		localStorage.setItem('column-widths-HANS-16', JSON.stringify({ name: 300 }));
		await storage.syncPageLayout('HANS', 16);
		expect(savePreference).toHaveBeenCalledWith(16, 'LAYOUT', 'DEFAULT', { customizations: {}, widths: { name: 300 }, row_numbers: false }, false);
	});
});
