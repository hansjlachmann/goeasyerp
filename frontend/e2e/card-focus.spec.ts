import { test, expect, type Page } from "@playwright/test";

// Cursor control on card pages (CLAUDE.md "Cursor control on cards"), on a Sales Order of the
// demo data. Needs the backend: the API server started with DEMO_DATA (a company filled with
// demo data) behind the preview's /api proxy, and E2E_PASSWORD — the first user is created
// with it when the database has none (E2E_USER, default ADMIN). Skipped without them.
const user = process.env.E2E_USER || "ADMIN";
const password = process.env.E2E_PASSWORD || "";

test.skip(!password, "needs the backend with demo data (E2E_PASSWORD)");

type Active = {
  where: string;
  tag?: string;
  id?: string;
  cls?: string;
  section?: string;
};

const active = (page: Page): Promise<Active> =>
  page.evaluate(() => {
    const a = document.activeElement as HTMLElement | null;
    if (!a || a === document.body) return { where: "body" };
    const where = a.closest(".subpage-part")
      ? "lines"
      : a.closest(".sections-container")
        ? "card"
        : "other";
    return {
      where,
      tag: a.tagName,
      id: a.id,
      cls: a.className?.toString() ?? "",
      section: a.closest("[data-section]")?.getAttribute("data-section") ?? "",
    };
  });

/** The card's own field controls in Tab order (no lines) */
const focusField = (page: Page, which: "first" | "last" | number) =>
  page.evaluate((which) => {
    const fields = [
      ...document.querySelectorAll<HTMLElement>(
        ".sections-container .section-fields input, .sections-container .section-fields [role=combobox]",
      ),
    ].filter(
      (e) =>
        e.tabIndex >= 0 &&
        !(e as HTMLInputElement).disabled &&
        e.offsetParent !== null &&
        !e.closest(".subpage-part"),
    );
    const f =
      which === "first"
        ? fields[0]
        : which === "last"
          ? fields[fields.length - 1]
          : fields[which];
    f.focus();
    return f.id;
  }, which);

test("Tab, wrap-around, section toggle and focus guard on a Sales Order card", async ({
  page,
  request,
}) => {
  // The first user, when the database has none (403: users exist)
  const init = await request.post("/api/auth/init", {
    data: { user_id: user, user_name: user, email: "", password },
  });
  expect([200, 403]).toContain(init.status());

  await page.goto("/login");
  await page.fill("#userid", user);
  await page.fill("#password", password);
  await page.waitForLoadState("networkidle"); // the companies are loaded
  await page.click(".login-button");
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), {
    timeout: 15000,
  });

  // A demo sales order (fetched in the page: the session cookie is the browser's)
  const orderNo = await page.evaluate(async () => {
    const fixed = JSON.stringify([{ field: "document_type", value: 1 }]);
    const r = await fetch(
      `/api/tables/Sales_Header/list?offset=0&limit=1&fixed_filters=${encodeURIComponent(fixed)}`,
    );
    const body = await r.json();
    return (body.data?.records?.[0]?.no as string) ?? "";
  });
  expect(orderNo, "the demo data has a sales order").not.toBe("");

  await page.goto(`/pages/42/1,${orderNo}?edit=1`);
  await expect(page).toHaveURL(new RegExp(`/pages/42/`));
  await page.waitForSelector(".sections-container .subpage-part", {
    timeout: 20000,
  });
  await page.waitForTimeout(1000);

  await test.step("edit mode starts in a field", async () => {
    await expect.poll(async () => (await active(page)).where).toBe("card");
    expect((await active(page)).tag).toBe("INPUT");
  });

  await test.step("Tab stays in the card, never on a header, and reaches the lines", async () => {
    let reachedLines = false;
    for (let i = 0; i < 80; i++) {
      await page.keyboard.press("Tab");
      await page.waitForTimeout(80);
      const a = await active(page);
      expect(["card", "lines"], `Tab ${i + 1}: ${JSON.stringify(a)}`).toContain(
        a.where,
      );
      expect(a.cls ?? "").not.toContain("card-header");
      if (a.where === "lines") reachedLines = true;
      if (reachedLines && a.where === "card") break; // out of the lines again
    }
    expect(reachedLines).toBe(true);
  });

  await test.step("Tab past the last field wraps to the first, Shift+Tab back", async () => {
    const first = await focusField(page, "first");
    const last = await focusField(page, "last");
    await page.keyboard.press("Tab");
    await expect.poll(async () => (await active(page)).id).toBe(first);
    await page.keyboard.press("Shift+Tab");
    await expect.poll(async () => (await active(page)).id).toBe(last);
  });

  await test.step("Ctrl+Shift+Space closes and opens the section of the cursor", async () => {
    await focusField(page, "first");
    const section = (await active(page)).section ?? "";
    expect(section).not.toBe("");
    await page.keyboard.press("Control+Shift+Space");
    await expect
      .poll(async () => (await active(page)).cls ?? "")
      .toContain("card-header");
    expect(
      await page.locator(`[data-section="${section}"] .card-body`).count(),
    ).toBe(0);
    await page.keyboard.press("Control+Shift+Space");
    await expect.poll(async () => (await active(page)).tag).toBe("INPUT");
    expect((await active(page)).section).toBe(section);
  });

  await test.step("focus lost without a click goes back to the field", async () => {
    const id = await focusField(page, 2);
    await page.evaluate(() => (document.activeElement as HTMLElement).blur());
    await expect.poll(async () => (await active(page)).id).toBe(id);
  });

  await test.step("a click elsewhere is not overruled", async () => {
    const id = await focusField(page, 2);
    await page.mouse.click(5, 300);
    await page.waitForTimeout(300);
    const a = await active(page);
    expect(a.where !== "card" || a.id !== id).toBe(true);
  });
});
