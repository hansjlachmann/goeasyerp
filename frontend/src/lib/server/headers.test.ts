import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { PAGE_HEADERS, STATIC_HEADERS, proxyHeaders } from './headers';

describe('proxyHeaders', () => {
	it('replaces the address headers the client sent by its own address', () => {
		const sent = new Headers({
			'x-forwarded-for': '1.2.3.4',
			'X-Real-IP': '1.2.3.4',
			forwarded: 'for=1.2.3.4',
			cookie: 'openerp_session=x',
			'content-type': 'application/json'
		});
		const out = proxyHeaders(sent, '203.0.113.9');
		expect(out.get('x-forwarded-for')).toBe('203.0.113.9');
		expect(out.get('x-real-ip')).toBe('203.0.113.9');
		expect(out.has('forwarded')).toBe(false);
		expect(out.get('cookie')).toBe('openerp_session=x');
		expect(out.get('content-type')).toBe('application/json');
	});

	it('sends no address when it is not known', () => {
		const out = proxyHeaders(new Headers({ 'x-forwarded-for': '1.2.3.4' }), '');
		expect(out.has('x-forwarded-for')).toBe(false);
	});
});

describe('STATIC_HEADERS', () => {
	// the web servers in front add them to the static files, with the pages' values
	for (const file of ['../nginx.conf', '../docker-compose.local.yml']) {
		it(`are added by ${file}`, () => {
			const text = readFileSync(resolve(process.cwd(), file), 'utf8');
			for (const name of STATIC_HEADERS) {
				expect(text).toContain(`add_header ${name} "${PAGE_HEADERS[name]}" always;`);
				expect(text).toContain(`proxy_hide_header ${name};`);
			}
		});
	}
});

describe('PAGE_HEADERS', () => {
	it('keeps other sites from framing the app', () => {
		expect(PAGE_HEADERS['X-Frame-Options']).toBe('SAMEORIGIN');
		expect(PAGE_HEADERS['X-Content-Type-Options']).toBe('nosniff');
	});
});
