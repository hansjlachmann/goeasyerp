import { describe, expect, it } from 'vitest';
import { sessionEnded } from './sessionEnd.js';

describe('sessionEnded', () => {
	it('a 401 of an API call on a page ends the session', () => {
		expect(sessionEnded('/api/tables/Customer/list?x=1', 401, '/pages/22')).toBe(true);
		expect(sessionEnded('http://localhost:8000/api/auth/user', 401, '/')).toBe(true);
	});
	it('other answers do not', () => {
		expect(sessionEnded('/api/tables/Customer/list', 403, '/pages/22')).toBe(false);
		expect(sessionEnded('/api/tables/Customer/list', 500, '/pages/22')).toBe(false);
	});
	it('the login and setup answers and the login page are left alone', () => {
		expect(sessionEnded('/api/auth/login', 401, '/pages/22')).toBe(false);
		expect(sessionEnded('/api/auth/init', 401, '/login')).toBe(false);
		expect(sessionEnded('/api/auth/user', 401, '/login')).toBe(false);
		expect(sessionEnded('/api/auth/user', 401, '/help/shortcuts')).toBe(false);
	});
});
