import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import {
	maintenance,
	maintenanceOf,
	maintenanceOver,
	watchingFetch,
	checkInterval,
	CHECK_URL,
	CHECK_INTERVAL_MS,
	STARTING_CHECK_INTERVAL_MS
} from './maintenance';

function json(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const answer = {
	success: false,
	error: 'GoEasyERP is being updated to version 1.2.0',
	maintenance: true,
	failed: false,
	title: 'GoEasyERP is being updated',
	retry: 'Try again'
};

describe('maintenanceOf', () => {
	it('reads the notice of a maintenance answer, recognised by its marker', async () => {
		expect(await maintenanceOf(json(503, answer))).toEqual({
			title: 'GoEasyERP is being updated',
			message: 'GoEasyERP is being updated to version 1.2.0',
			retry: 'Try again',
			failed: false,
			starting: false
		});
		expect((await maintenanceOf(json(503, { ...answer, failed: true })))?.failed).toBe(true);
	});

	it('checks every few seconds while the server starts a newer build', async () => {
		const building = await maintenanceOf(json(503, answer));
		const starting = await maintenanceOf(json(503, { ...answer, starting: true }));
		expect(starting?.starting).toBe(true);
		expect(checkInterval(building!)).toBe(CHECK_INTERVAL_MS);
		expect(checkInterval(starting!)).toBe(STARTING_CHECK_INTERVAL_MS);
	});

	it('is none for other answers: another 503, another status, no JSON', async () => {
		expect(await maintenanceOf(json(503, { success: false, error: 'Backend unavailable' }))).toBeNull();
		expect(await maintenanceOf(json(500, answer))).toBeNull();
		expect(await maintenanceOf(new Response('<html>', { status: 503 }))).toBeNull();
	});

	it('leaves the answer readable for the caller', async () => {
		const response = json(503, answer);
		await maintenanceOf(response);
		expect((await response.json()).error).toBe(answer.error);
	});
});

describe('watchingFetch', () => {
	it('reports maintenance answers of API calls and passes every answer on unchanged', async () => {
		const report = vi.fn();
		const base = vi.fn(async () => json(503, answer));
		const watched = watchingFetch(base as unknown as typeof fetch, report);
		const response = await watched('/api/tables/Customer/list');
		expect(response.status).toBe(503);
		expect(report).toHaveBeenCalledTimes(1);
		await watched('/favicon.png');
		expect(report).toHaveBeenCalledTimes(1);
	});

	it('does not read normal answers', async () => {
		const report = vi.fn();
		const watched = watchingFetch((async () => json(200, { success: true })) as unknown as typeof fetch, report);
		expect((await watched('/api/version')).status).toBe(200);
		expect(report).not.toHaveBeenCalled();
	});
});

describe('maintenanceOver', () => {
	beforeEach(() => maintenance.set(null));

	it('is over when the server answers normally', async () => {
		const fetchFn = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => json(200, { version: '1.2.0' }));
		expect(await maintenanceOver(fetchFn as unknown as typeof fetch)).toBe(true);
		expect(fetchFn.mock.calls[0][0]).toBe(CHECK_URL);
	});

	it('is not over while the server answers maintenance, and takes its new notice', async () => {
		const failed = { ...answer, failed: true, error: 'the build failed' };
		expect(await maintenanceOver((async () => json(503, failed)) as unknown as typeof fetch)).toBe(false);
		expect(get(maintenance)?.message).toBe('the build failed');
	});

	it('is not over while the server restarts (no answer, or the proxy finds no backend)', async () => {
		expect(await maintenanceOver((async () => { throw new TypeError('network'); }) as unknown as typeof fetch)).toBe(false);
		expect(await maintenanceOver((async () => json(503, { error: 'Backend unavailable' })) as unknown as typeof fetch)).toBe(false);
		expect(await maintenanceOver((async () => new Response('', { status: 502 })) as unknown as typeof fetch)).toBe(false);
	});
});
