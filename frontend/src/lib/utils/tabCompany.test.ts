import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getTabCompany, setTabCompany, clearTabCompany, withTabCompany, withTabCompanyParam, TAB_COMPANY_HEADER } from './tabCompany';

function ok(): Response {
	return new Response('{}', { status: 200 });
}

describe('tab company', () => {
	beforeEach(() => sessionStorage.clear());

	it('is kept per tab and forgotten at log out', () => {
		expect(getTabCompany()).toBe('');
		setTabCompany('beta');
		expect(getTabCompany()).toBe('beta');
		clearTabCompany();
		expect(getTabCompany()).toBe('');
	});

	it('goes with every API request of the app, not with other requests', async () => {
		const base = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => ok());
		const f = withTabCompany(base as unknown as typeof fetch, () => {});
		await f('/api/session');
		expect(base.mock.calls[0][1]).toBeUndefined(); // no company yet: the request as it is
		setTabCompany('beta');
		await f('/api/tables/Customer/list', { headers: { 'Content-Type': 'application/json' } });
		const headers = new Headers(base.mock.calls[1][1]?.headers);
		expect(headers.get(TAB_COMPANY_HEADER)).toBe('beta');
		expect(headers.get('Content-Type')).toBe('application/json');
		await f('/favicon.png');
		await f('https://example.com/api/x');
		expect(base.mock.calls[2][1]).toBeUndefined();
		expect(base.mock.calls[3][1]).toBeUndefined();
	});

	it('forgets a company the server refuses and reports it', async () => {
		setTabCompany('gone');
		const refused = new Response('{}', { status: 409, headers: { 'X-Tab-Company': 'unavailable' } });
		const onUnavailable = vi.fn();
		const f = withTabCompany((async () => refused) as unknown as typeof fetch, onUnavailable);
		await f('/api/session');
		expect(getTabCompany()).toBe('');
		expect(onUnavailable).toHaveBeenCalledOnce();
		// another 409 (a duplicate key) is the page's business
		setTabCompany('beta');
		const conflict = withTabCompany((async () => new Response('{}', { status: 409 })) as unknown as typeof fetch, onUnavailable);
		await conflict('/api/tables/Customer/insert');
		expect(getTabCompany()).toBe('beta');
	});

	it('puts the company into image and link URLs', () => {
		expect(withTabCompanyParam('/api/tables/Item/blob/picture/1000')).toBe('/api/tables/Item/blob/picture/1000');
		setTabCompany('beta');
		expect(withTabCompanyParam('/api/tables/Item/blob/picture/1000')).toBe('/api/tables/Item/blob/picture/1000?company=beta');
		expect(withTabCompanyParam('/api/x?v=2')).toBe('/api/x?v=2&company=beta');
	});
});
