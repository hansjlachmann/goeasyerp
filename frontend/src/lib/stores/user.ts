import { writable } from 'svelte/store';
import { getJson, setJson, remove } from '#lib/utils/storage.js';

interface User {
	user_id: string;
	user_name: string;
	email?: string;
	language?: string;
	translation_key?: string;
}

const USER_STORAGE_KEY = 'currentUser';

function createUserStore() {
	const { subscribe, set, update } = writable<User | null>(null);

	return {
		subscribe,
		setUser: (user: User | null) => {
			if (user) {
				setJson(USER_STORAGE_KEY, user);
			} else {
				remove(USER_STORAGE_KEY);
			}
			set(user);
		},
		loadFromStorage: () => {
			const user = getJson<User | null>(USER_STORAGE_KEY, null);
			set(user);
		},
		clear: () => {
			remove(USER_STORAGE_KEY);
			set(null);
		}
	};
}

export const currentUser = createUserStore();
