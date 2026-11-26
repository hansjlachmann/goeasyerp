// Headers of the SvelteKit server's answers and of its /api proxy (hooks.server.ts)

// Headers every page answer carries (S25), wherever the app runs — behind nginx, a Kubernetes
// ingress or the dev server. The Content Security Policy comes from SvelteKit (vite.config.js).
export const PAGE_HEADERS: Record<string, string> = {
	'X-Frame-Options': 'SAMEORIGIN', // only the app frames its pages (clickjacking)
	'X-Content-Type-Options': 'nosniff',
	'Referrer-Policy': 'same-origin',
	'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=(), usb=()',
	'Cross-Origin-Resource-Policy': 'same-origin', // other sites' pages cannot load its answers
	// a page of another site the app opens (a hyperlink) gets no handle on the app's window
	'Cross-Origin-Opener-Policy': 'same-origin'
};

// The headers of the static files (scripts, styles, icons): the Node server sends them before
// the hook above runs, so the web server in front adds these (nginx.conf, the trial's
// docker-compose.local.yml — S53's nightly scan checks them)
export const STATIC_HEADERS = ['X-Content-Type-Options', 'Permissions-Policy', 'Cross-Origin-Resource-Policy'];

// Request headers the /api proxy never passes on as the client sent them: the client's
// address is the one this server sees (S28) — the backend trusts this proxy's X-Forwarded-For,
// so a forged one would get around the login limit
const ADDRESS_HEADERS = ['x-forwarded-for', 'x-real-ip', 'forwarded'];

/** The headers for the backend: the client's, its address headers replaced by its address */
export function proxyHeaders(headers: Headers, clientAddress: string): Headers {
	const out = new Headers(headers);
	for (const name of ADDRESS_HEADERS) out.delete(name);
	if (clientAddress) {
		out.set('x-forwarded-for', clientAddress);
		out.set('x-real-ip', clientAddress);
	}
	return out;
}
