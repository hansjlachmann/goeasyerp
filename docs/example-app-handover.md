# Handover: the example extension app "Loyalty Points"

This document is the complete brief for building GoEasyERP's first real extension app — an
example other app authors can copy — in a repository of its own. It is written for a developer
(or an AI coding session) starting in that empty repository with no other context: everything
needed is here or in the files it points to.

## 1. What you are building and why

GoEasyERP is an ERP system (Go backend, SvelteKit frontend, PostgreSQL), public at
`https://github.com/hansjlachmann/goeasyerp`. Its base application can be extended by
**extension apps**: separate repositories that add tables, fields, page parts, menu entries,
texts and business logic, merged into an installation's build. The extension machinery was
built and tested in the base repository with a small test app; this app is the first one built
the way real apps will be — in its own repository, released, installed through the user
interface — and the example the guide points authors to.

**The app:** customers collect loyalty points from posted sales invoices and lose them with
credit memos; their loyalty tier follows their points. It touches every kind of extension:
fields on a base table, a table and page of its own, fields, a section, a column and buttons on
base pages, a main menu entry, a page function, a table trigger, a posting event, texts in three
languages.

## 2. Read first

In the base repository, at the release you build against (tag `v0.1.161` or later):

1. **`EXTENSIONS.md`** — the guide to extension apps: every file format, the merge rules, the Go
   module and `Register`, triggers and events, building, installing, enabling. This brief does not
   repeat it; follow it.
2. **`testdata/apps/loyalty-points/`** — the base's own test app: a minimal version of this app
   (it is merged into the base in the base's CI). Use it as a working reference for the file
   formats, not as code to copy unchanged: this app is larger and is the one people will read.
3. **`scripts/check-extension-fixture.sh`** — how the base builds and tests itself with an app;
   your CI does the same with this app.
4. For the YAML formats beyond the guide: `backend/definitions/tables/*.yaml` and
   `backend/definitions/pages/*.yaml` (e.g. `customer.yaml`, `21-customer-card.yaml`,
   `22-customer-list.yaml`); for Go business-logic style: `backend/business-logic/`.

## 3. The repository

- **Name:** `goeasyerp-loyalty` (the owner may choose another). Public. Licence: ask the owner
  (the base is MIT).
- **Layout** (as in `EXTENSIONS.md` §1): the app in `extensions/` — `application.yaml`,
  `tables/`, `pages/`, `menus/`, `custom/tables/`, `custom/pages/`, `translations/<language>/`,
  `go.mod` and the Go files — plus `README.md` (what the app does, how to install it, how to
  develop it) and `.github/workflows/ci.yml`.
- **Go module path:** `github.com/<owner>/goeasyerp-loyalty/extensions` (the module is the
  `extensions/` folder), requiring `github.com/hansjlachmann/goeasyerp` at the base release
  targeted.

## 4. application.yaml

- `id`: a new GUID, lower case — generate it once and never change it.
- `name`: `Loyalty Points`; `publisher` and `contact`: ask the owner (contact: a public URL).
- `version`: `1.0.0` for the first release (release tags `v1.0.0` …).
- `prefix`: `lp`.
- `number_series`: objects and fields `50100`–`50199` (keep clear of `50000`–`50099`, the base's
  test app).
- `dependencies`: the base application — `id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd`,
  `name: GoEasyERP Base Application`, `min_version`: the base release targeted.

## 5. Functional requirements

**F1 — Loyalty tiers.** A table `LP_Loyalty_Tier` (50100): Code (Code 10, primary key),
Description (Text 50), Minimum Points (whole number, not below 0). A list page `LP Loyalty Tiers`
(50100), editable, to maintain them. A customer reaches a tier when its points are at least the
tier's Minimum Points; the tier with the highest such minimum is the customer's.

**F2 — Customer fields** (table 18): `lp_loyalty_points` (whole number, not below 0, not editable
by users — set by the app's logic) and `lp_tier_code` (Code 10, relation to `LP_Loyalty_Tier`,
not editable by users — follows the points).

**F3 — Customer Card (page 21):** Loyalty Points in the General section after Credit Limit
(LCY); a new section **Loyalty** after Payments with Loyalty Tier; in the card's Customer menu a
**Loyalty Tiers** item opening page 50100 and a **Recalculate Points** item (F7).
**Customer List (page 22):** a Loyalty Points column after Name; a **Loyalty Tiers** button.

**F4 — Main menu:** under Sales, after Customers, an entry **Loyalty Tiers** opening page 50100.

**F5 — Points from posted invoices and credit memos.** Subscribe to the sales posting's
`salesposting.OnAfterPost`:

- an invoice posted (`InvoiceNo` set): the bill-to customer gets **1 point per whole 10.00 of the
  invoice's amount excluding tax, in local currency** — the sum of the posted invoice's lines'
  Amount (`Sales_Invoice_Line`, `document_no` = InvoiceNo), divided by the invoice header's
  Currency Factor when it has a currency (`Sales_Invoice_Header` `currency_code`/`currency_factor`);
- a credit memo posted (`CrMemoNo` set): the same calculation on `Sales_Cr_Memo_Line` /
  `Sales_Cr_Memo_Header` takes points off, never below 0.

The change goes through the customer's modify with triggers, so F6 runs. It is part of the
posting's transaction: an error undoes the posting.

**F6 — The tier follows the points.** The app's Customer `OnInsert` and `OnModify` set
`lp_tier_code` from `lp_loyalty_points` (F1; blank when no tier is reached).

**F7 — Recalculate Points.** A page function `table:Customer.LpRecalculatePoints` (registered by
the app; the Customer Card's menu item): sets the customer's points from all its posted invoices
and credit memos (as F5, all at once), saves the customer (F6 runs) and reloads the card
(`Refresh`), with a message naming the new points.

**F8 — Texts** in en-US, nb-NO and da-DK for every caption the app adds: its table and fields,
the Customer fields, its page, the section, the actions, the menu entry, its messages. The
Norwegian and Danish texts are your own wording (not copied from any other product).

## 6. Technical requirements

- **Only add.** Never change the base repository; everything goes through the extension files and
  `Register`. If the base lacks something the app needs (an event, a hook), do not work around it
  — write it up for the base (§9).
- **Business logic in the backend.** All rules (F5–F7) are Go code in the app; pages only show.
- **Go style of the base's business logic:** one statement per line — never `if x := f(); cond`
  (call, then check: `err := f()` / `if err != nil { return err }`); record variables opened from
  a record (`cust := gtables.NewCustomer(rec)`); loops `for found := rec.FindSet(); found; found =
  rec.Next() { … }`; `GetOrError` / `ModifyOrError` instead of hand-written not-found errors;
  every error checked.
- **Messages to users** come from the app's translations (`messages.yaml` keys, read through the
  base's i18n), never English strings in Go code.
- **Tests (Go):** in the app's module, against a database the tests create (the base's
  `tables.CreateCompanyTables` on SQLite in memory), covering F1/F6 (tier boundaries), F5 (an
  invoice and a credit memo posted through the base's sales posting — the base's
  `salesposting` package tests show how to set up a company with posting groups and post a
  document — points and tier after each, points never below 0), F7, and that nothing runs while
  the app is disabled (`extensions.SetDisabled`, as the base's test app does).
- **Public repository:** no hostnames, IP addresses, customer or company names of installations,
  credentials or tokens in files, commits or issues. Describe behaviour in your own words; do not
  name other ERP products.
- **Commits:** Conventional Commits (`feat:`, `fix:` …); a changelog per release.

## 7. CI — `.github/workflows/ci.yml`

On every push and pull request:

1. Check out this repository and the base repository at the targeted release tag (into
   `base/`).
2. Copy `extensions/` into `base/apps/<app id>/` and run, in `base/`: `go run ./tools/extmerge
   --core backend/definitions --apps apps --output merged --translations translations
   --translations-output merged-translations --go-root .`, replace `backend/definitions` and
   `translations` with the merged ones, `go generate ./backend/generated/tables`, `go build ./...`.
3. Run the base's checks of the merged build: `go test ./backend/api/handlers
   ./backend/business-logic/apps ./backend/foundation/apps ./cmd/api-server` (page definitions,
   captions in every language, installed apps, the app's code installed).
4. Run the app's own tests: `cd apps/<app id> && go test ./...` (inside `base/`, so `go.work`
   joins the app with the merged base).
5. `go vet` and `gofmt` on the app's code.

`scripts/check-extension-fixture.sh` in the base repository is the same sequence for the base's
test app — follow it.

## 8. Releasing and installing

- A release is a tag `vX.Y.Z` matching `version` in `application.yaml`, with release notes.
- To install it in an installation: its installation repository (base:
  `deploy/installation-template/README.md`) and **Installed Apps → Install App** with this
  repository's URL, the tag and the folder `extensions`; **Build List** → commit `apps.yaml` →
  the installation's workflow builds the image → the server pulls and restarts (`EXTENSIONS.md`
  §10).
- The README tells an installation's administrator exactly that, plus what the app adds and the
  setup it needs (create the tiers on page 50100; Recalculate Points once for existing customers).

## 9. When the base needs a change

Write it down for the base repository's maintainer instead of patching around it: what the app
needs, where in the base (file, function, the point in the code), why, and how you would test it.
Typical: a new event (e.g. at a point the app must react to), a value an event should pass. Keep
the app working without it where you can, or mark the requirement blocked.

## 10. Done when

- [ ] The repository has the layout of §3, `application.yaml` of §4, a README.
- [ ] F1–F8 are built; texts in all three languages.
- [ ] CI (§7) passes: the merged build, the base's checks, the app's tests, vet/format.
- [ ] Tests cover F1/F5/F6/F7 and the disabled app.
- [ ] Release `v1.0.0` is tagged with release notes.
- [ ] Installed in a test installation through Install App and the installation repository: the
      Customer Card and List show the points and tier, posting an invoice adds points, a credit memo
      takes them off, the tier follows, Recalculate Points works, Disable App hides it all and
      Enable App brings it back.
- [ ] Anything the base was missing is written up (§9).
