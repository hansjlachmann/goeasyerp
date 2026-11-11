// A Yes/No question the server's code asks during an API request (classic CONFIRM in a table
// trigger): the API client shows it with QuestionDialog and sends the request again with the
// answer (services/api.ts)
import { writable } from 'svelte/store';

export interface QuestionState {
	text: string;
	answer: (yes: boolean) => void;
}

/** The question shown (null: none); questions asked while one is open wait their turn */
export const question = writable<QuestionState | null>(null);

let queue: Promise<unknown> = Promise.resolve();

/** Asks the user a Yes/No question; resolves with the answer (Esc and close: No) */
export function ask(text: string): Promise<boolean> {
	const asked = queue.then(
		() =>
			new Promise<boolean>((resolve) => {
				question.set({
					text,
					answer: (yes) => {
						question.set(null);
						resolve(yes);
					}
				});
			})
	);
	queue = asked;
	return asked;
}
