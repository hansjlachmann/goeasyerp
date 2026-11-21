// Confirmation modal store
import { writable } from 'svelte/store';

interface ConfirmState {
	open: boolean;
	title: string;
	message: string;
	action: (() => Promise<void>) | null;
	// Button text and style (default: a red "Delete", the usual confirmation)
	confirmText?: string;
	variant?: 'danger' | 'warning' | 'info';
	// The text the user must type before the button confirms (a company's name before it is
	// deleted with all its data)
	typeToConfirm?: string;
}

const initialState: ConfirmState = {
	open: false,
	title: '',
	message: '',
	action: null
};

function createConfirmStore() {
	const { subscribe, set, update } = writable<ConfirmState>(initialState);

	return {
		subscribe,

		/**
		 * Show a confirmation modal
		 * @param title - Modal title
		 * @param message - Confirmation message
		 * @param action - Async action to execute on confirm
		 * @param options - Button text and style (default: a red Delete button)
		 */
		show(
			title: string,
			message: string,
			action: () => Promise<void>,
			options: { confirmText?: string; variant?: 'danger' | 'warning' | 'info'; typeToConfirm?: string } = {}
		) {
			set({
				open: true,
				title,
				message,
				action,
				...options
			});
		},

		/**
		 * Confirm and execute the action
		 */
		async confirm() {
			let actionToExecute: (() => Promise<void>) | null = null;

			update(state => {
				actionToExecute = state.action;
				return { ...state, open: false };
			});

			if (actionToExecute !== null) {
				await (actionToExecute as () => Promise<void>)();
			}

			set(initialState);
		},

		/**
		 * Cancel and close the modal
		 */
		cancel() {
			set(initialState);
		}
	};
}

export const confirm = createConfirmStore();
