// A session that ends while the app is open (S39: the idle limit, a log out in another browser,
// a password changed elsewhere): the server answers 401 to the next call, and the app goes to
// the login page instead of showing the call's error. Calls that answer 401 for another reason
// (a wrong password at login, a wrong setup token) and the login page itself are left alone.

const ownAnswers = ['/api/auth/login', '/api/auth/init'];

/** Whether a call to url answering status, made on the page pathname, means the session ended */
export function sessionEnded(url: string, status: number, pathname: string): boolean {
	if (status !== 401) return false;
	if (pathname === '/login' || pathname.startsWith('/login/') || pathname.startsWith('/help/')) return false;
	const path = url.replace(/^https?:\/\/[^/]+/, '').split('?')[0];
	return path.startsWith('/api/') && !ownAnswers.includes(path);
}
