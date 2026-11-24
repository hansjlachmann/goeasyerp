// npm audit of the packages the frontend server runs although they are dev dependencies:
// SvelteKit and adapter-node are bundled into build/ (the production image runs their code),
// so `npm audit --omit=dev` never sees their advisories. This reads the whole audit and fails
// on an advisory of moderate severity or above that reaches one of them — the package itself
// or one of its dependencies (cookie through SvelteKit) — unless it was reviewed below.
//
// Run from frontend/: node scripts/audit-server-packages.mjs

import { execFileSync } from 'node:child_process';

// The packages whose code ends up in the server bundle
const BUNDLED = ['@sveltejs/kit', '@sveltejs/adapter-node'];

const LEVELS = { info: 0, low: 1, moderate: 2, high: 3, critical: 4 };
const FAIL_FROM = LEVELS.moderate;

// Advisories reviewed and found not to apply to GoEasyERP. Each entry needs a reason; remove
// it when the package is updated past the advisory (the script reports entries no longer seen).
const REVIEWED = {
	// SvelteKit remote functions (form / query.batch): the frontend uses none ($app/server,
	// *.remote.ts). Fixed in Kit 3.1.1.
	'GHSA-3r8g-jcqq-3p5j': 'remote functions `form`: not used',
	'GHSA-6cf5-gc4q-fm3x': 'remote functions `query.batch`: not used'
};

function audit() {
	try {
		return execFileSync('npm', ['audit', '--json'], { encoding: 'utf8', maxBuffer: 64 << 20 });
	} catch (err) {
		// npm audit exits non-zero when it finds something; the report is still on stdout
		if (err.stdout) return err.stdout;
		throw err;
	}
}

const report = JSON.parse(audit());
const vulnerabilities = report.vulnerabilities ?? {};

/** Whether a vulnerable package is bundled or a dependency of a bundled one */
function reachesBundle(name, seen = new Set()) {
	if (BUNDLED.includes(name)) return true;
	if (seen.has(name)) return false;
	seen.add(name);
	return (vulnerabilities[name]?.effects ?? []).some((parent) => reachesBundle(parent, seen));
}

const found = [];
for (const [name, vulnerability] of Object.entries(vulnerabilities)) {
	if (!reachesBundle(name)) continue;
	for (const via of vulnerability.via) {
		if (typeof via === 'string') continue; // listed under that package's own entry
		const id = (via.url ?? '').split('/').pop();
		found.push({ id, name, severity: via.severity, title: via.title, range: via.range, url: via.url });
	}
}

let failed = false;
for (const advisory of found) {
	const reviewed = REVIEWED[advisory.id];
	const level = LEVELS[advisory.severity] ?? 0;
	let mark = 'note';
	if (reviewed) mark = 'reviewed';
	else if (level >= FAIL_FROM) {
		mark = 'FAIL';
		failed = true;
	}
	console.log(`${mark.padEnd(8)} ${advisory.severity.padEnd(8)} ${advisory.name} ${advisory.range}: ${advisory.title}`);
	console.log(`         ${advisory.url}${reviewed ? ` — ${reviewed}` : ''}`);
}
for (const id of Object.keys(REVIEWED)) {
	if (!found.some((advisory) => advisory.id === id)) {
		console.log(`stale    ${id} is no longer reported: remove it from REVIEWED`);
	}
}
if (found.length === 0) console.log(`No advisories for ${BUNDLED.join(', ')} and their dependencies.`);

if (failed) {
	console.log('\nAn advisory above reaches the server bundle: update the package, or review it and add it to REVIEWED with the reason.');
	process.exit(1);
}
