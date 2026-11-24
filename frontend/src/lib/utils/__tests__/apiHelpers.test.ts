import { describe, it, expect, vi, beforeEach } from 'vitest';

const { initialize } = vi.hoisted(() => ({ initialize: vi.fn(async () => {}) }));
vi.mock('#lib/stores/session.js', () => ({ session: { initialize } }));

import { handleApiResponse } from '../apiHelpers';

function response(headers: Record<string, string>): Response {
	return new Response(JSON.stringify({ success: true, data: { ok: 1 } }), { status: 200, headers });
}

describe('session change signalled by the backend', () => {
	beforeEach(() => initialize.mockClear());

	it('reloads the session when a response re-issued the session cookie', async () => {
		await handleApiResponse(response({ 'X-Session-Changed': '1' }), 'modify Company');
		expect(initialize).toHaveBeenCalledTimes(1);
	});

	it('leaves the session alone otherwise', async () => {
		await handleApiResponse(response({}), 'modify Customer');
		expect(initialize).not.toHaveBeenCalled();
	});
});
