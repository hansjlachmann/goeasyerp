import { describe, expect, it } from 'vitest';
import { ANSWERS_HEADER, QUESTION_HEADER, fetchAnswering, questionOf } from './questions';

function answer(status: number, question?: string): Response {
	return new Response('{}', { status, headers: question ? { [QUESTION_HEADER]: encodeURIComponent(question) } : {} });
}

describe('questions', () => {
	it('reads the question of a 409 answer only', () => {
		expect(questionOf(answer(409, 'Change it?'))).toBe('Change it?');
		expect(questionOf(answer(200, 'Change it?'))).toBe('');
		expect(questionOf(answer(409))).toBe('');
		expect(questionOf(new Response('{}', { status: 409, headers: { [QUESTION_HEADER]: 'Do+it%3F' } }))).toBe('Do it?');
	});

	it('asks and sends the request again with the answers so far', async () => {
		const sent: (string | null)[] = [];
		const send = (async (_input: RequestInfo | URL, init?: RequestInit) => {
			const answers = new Headers(init?.headers).get(ANSWERS_HEADER);
			sent.push(answers);
			if (!answers) return answer(409, 'First?');
			if (answers === '1') return answer(409, 'Second?');
			return answer(200);
		}) as typeof globalThis.fetch;
		const asked: string[] = [];
		const response = await fetchAnswering(send, async (q) => (asked.push(q), q === 'First?'), '/api/x', {
			method: 'PUT',
			body: '{}',
			headers: { 'Content-Type': 'application/json' }
		});
		expect(response.status).toBe(200);
		expect(asked).toEqual(['First?', 'Second?']);
		expect(sent).toEqual([null, '1', '1,0']);
	});

	it('returns an answer without a question at once', async () => {
		let calls = 0;
		const send = (async () => (calls++, answer(409))) as unknown as typeof globalThis.fetch;
		const response = await fetchAnswering(send, async () => true, '/api/x');
		expect(response.status).toBe(409);
		expect(calls).toBe(1);
	});
});
