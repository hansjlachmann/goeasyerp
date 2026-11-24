import { writable, derived } from 'svelte/store';
import type { SessionState } from '#lib/types/session.js';
import { getTabCompany, setTabCompany } from '#lib/utils/tabCompany.js';

// Session store - manages user session state
function createSessionStore() {
	const { subscribe, set, update } = writable<SessionState>({
		database: null,
		company: null,
		userID: null,
		userName: null,
		userFullName: null,
		language: 'en-US',
		isAuthenticated: false,
		isLoading: false
	});

	return {
		subscribe,

		// Initialize session from API
		async initialize() {
			update(s => ({ ...s, isLoading: true }));

			try {
				const response = await fetch('/api/session');
				if (response.ok) {
					const result = await response.json();
					const data = result.data; // API returns { success: true, data: { ... } }
					// a new tab stays in the company it started in (the cookie's, the one chosen last)
					if (data.user_id && data.company && !getTabCompany()) setTabCompany(data.company);
					set({
						database: data.database,
						company: data.company,
						userID: data.user_id,
						userName: data.user_name,
						userFullName: data.user_full_name,
						language: data.language || 'en-US',
						isAuthenticated: !!data.user_id,
						isLoading: false
					});
				} else {
					update(s => ({ ...s, isLoading: false }));
				}
			} catch (error) {
				console.error('Failed to initialize session:', error);
				update(s => ({ ...s, isLoading: false }));
			}
		},

		// Set language
		setLanguage(language: string) {
			update(s => ({ ...s, language }));
		},

		// Set company
		setCompany(company: string) {
			setTabCompany(company);
			update(s => ({ ...s, company }));
		},

		// Clear session (logout)
		clear() {
			set({
				database: null,
				company: null,
				userID: null,
				userName: null,
				userFullName: null,
				language: 'en-US',
				isAuthenticated: false,
				isLoading: false
			});
		}
	};
}

export const session = createSessionStore();

// Derived stores
export const currentLanguage = derived(session, $session => $session.language);
export const isAuthenticated = derived(session, $session => $session.isAuthenticated);
export const currentCompany = derived(session, $session => $session.company);
