// The user's regional settings (My Settings) in the app: loaded after login, cached in this
// browser so the first page already shows the user's formats, and handed to utils/regional.ts
// (formatDate, formatDecimal, the date entry). regionalVersion changes when the formats change,
// so the page is drawn again with them.
import { writable, derived, get } from 'svelte/store';
import { browser } from '$app/env';
import { api, type MySettings } from '#lib/services/api.js';
import { setRegional, setRegionalLanguage } from '#lib/utils/regional.js';
import { currentLanguage } from './session';

const cacheKey = 'regional-cache';
const empty: MySettings = { date_format: '', number_format: '', work_date: '' };

export const regional = writable<MySettings>(empty);
/** Changes when the formats change: the layout draws the page again */
export const regionalVersion = writable(0);
/** Tips for new users are shown (My Settings: Show tips; on unless the user switched them off) */
export const showTips = derived(regional, (s) => s.show_tips !== false);

function cached(): MySettings {
	if (!browser) return empty;
	try {
		return { ...empty, ...JSON.parse(localStorage.getItem(cacheKey) ?? '{}') };
	} catch {
		return empty;
	}
}

function adopt(s: MySettings) {
	const before = get(regional);
	regional.set(s);
	setRegional(s);
	if (browser) {
		try {
			localStorage.setItem(cacheKey, JSON.stringify({ date_format: s.date_format, number_format: s.number_format, show_tips: s.show_tips }));
		} catch {
			// no browser storage (private window): the settings come from the server
		}
	}
	if (before.date_format !== s.date_format || before.number_format !== s.number_format || before.work_date !== s.work_date) {
		regionalVersion.update((v) => v + 1);
	}
}

/** Loads the user's settings (after login, app start) */
export async function loadRegional() {
	adopt(await api.getMySettings());
}

/** Saves settings (the fields given) and shows the app with them */
export async function saveRegional(patch: Partial<MySettings>): Promise<MySettings> {
	const s = await api.saveMySettings(patch);
	adopt(s);
	return s;
}

/** Forgets the user's work date (logout); the formats stay cached for the next login */
export function clearRegional() {
	const c = cached();
	regional.set({ ...c, work_date: '' });
	setRegional({ ...c, work_date: '' });
}

if (browser) {
	const c = cached();
	regional.set(c);
	setRegional(c);
	currentLanguage.subscribe((lang) => setRegionalLanguage(lang));
}
