/** The header the API sends a request's question in, with status 409 (middleware.QuestionHeader) */
export const QUESTION_HEADER = 'X-App-Question';
/** The header the answers so far go back in: "1" Yes, "0" No, in the order asked (middleware.AnswersHeader) */
export const ANSWERS_HEADER = 'X-App-Answers';
/** The server takes at most this many answers (middleware.maxAnswers) */
export const MAX_ANSWERS = 20;

/** The question of an API answer: the URL-encoded header of a 409 answer (form encoding: `+` is a space); '' when none */
export function questionOf(response: Response): string {
	if (response.status !== 409) return '';
	const header = response.headers?.get(QUESTION_HEADER);
	if (!header) return '';
	try {
		return decodeURIComponent(header.replace(/\+/g, ' '));
	} catch {
		return '';
	}
}

/**
 * Sends a request; while the server answers with a question (classic CONFIRM in a table
 * trigger — nothing was saved), asks the user and sends the request again with all answers so
 * far, until the server has its answers. The request body must be sendable again (a string,
 * FormData …, not a stream).
 */
export async function fetchAnswering(
	send: typeof globalThis.fetch,
	ask: (question: string) => Promise<boolean>,
	input: RequestInfo | URL,
	init?: RequestInit
): Promise<Response> {
	const answers: boolean[] = [];
	for (;;) {
		let request = init;
		if (answers.length > 0) {
			const headers = new Headers(init?.headers);
			headers.set(ANSWERS_HEADER, answers.map((a) => (a ? '1' : '0')).join(','));
			request = { ...init, headers };
		}
		const response = await send(input, request);
		const question = questionOf(response);
		if (!question || answers.length >= MAX_ANSWERS) return response;
		answers.push(await ask(question));
	}
}
