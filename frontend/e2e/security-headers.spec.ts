import { test, expect, type Page } from "@playwright/test";

// The Content Security Policy and the other security headers (S25): pages carry them, and the
// app runs under them — no script, style, picture, font or frame it uses is refused. The second
// test needs the backend (E2E_PASSWORD, see card-focus.spec.ts).
const user = process.env.E2E_USER || "ADMIN";
const password = process.env.E2E_PASSWORD || "";

/** Collects the page's CSP violations (the init script runs before the page's own scripts) */
async function watchViolations(page: Page) {
  await page.addInitScript(() => {
    const w = window as unknown as { __csp: string[] };
    w.__csp = [];
    document.addEventListener("securitypolicyviolation", (e) => {
      w.__csp.push(`${e.violatedDirective} ${e.blockedURI}`);
    });
  });
}

const violations = (page: Page) =>
  page.evaluate(() => (window as unknown as { __csp: string[] }).__csp);

test("the login page carries the security headers and runs under its CSP", async ({ page }) => {
  await watchViolations(page);
  const response = await page.goto("/login");
  const headers = response!.headers();
  expect(headers["content-security-policy"]).toContain("script-src 'self'");
  expect(headers["content-security-policy"]).toContain("frame-ancestors 'self'");
  expect(headers["x-frame-options"]).toBe("SAMEORIGIN");
  expect(headers["x-content-type-options"]).toBe("nosniff");
  await page.waitForLoadState("networkidle");
  await expect(page.locator("h1")).toContainText("GoEasyERP"); // the app started
  expect(await violations(page)).toEqual([]);
});

test("lists and cards run under the CSP", async ({ page, request }) => {
  test.skip(!password, "needs the backend with demo data (E2E_PASSWORD)");
  const init = await request.post("/api/auth/init", {
    data: { user_id: user, user_name: user, email: "", password, setup_token: process.env.SETUP_TOKEN ?? "" },
  });
  expect([200, 403]).toContain(init.status());

  await watchViolations(page);
  await page.goto("/login");
  await page.fill("#userid", user);
  await page.fill("#password", password);
  await page.waitForLoadState("networkidle");
  await page.click(".login-button");
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 15000 });
  await page.waitForLoadState("networkidle");

  // the API answers: not kept in the browser's cache
  const api = await page.evaluate(async () => {
    const r = await fetch("/api/session");
    return r.headers.get("cache-control");
  });
  expect(api).toBe("no-store");

  await page.goto("/pages/22"); // Customer List
  await page.waitForSelector(".table-container", { timeout: 20000 });
  await page.waitForLoadState("networkidle");
  await page.keyboard.press("Enter"); // the card of the first customer
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(500);
  expect(await violations(page)).toEqual([]);
});
