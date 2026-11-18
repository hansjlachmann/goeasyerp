# The example extension app "Test Extension App"

GoEasyERP's first extension app built in a repository of its own — released, installed through
the user interface, and the example `EXTENSIONS.md` points app authors to:
`https://github.com/hansjlachmann/goeasyerp-test-extension-app` (release `v1.0.0`). This document
records what the app is and how it was built, so the next app can follow the same steps.

## 1. What it is and why

GoEasyERP's base application can be extended by **extension apps**: separate repositories that add
tables, fields, page parts, menu entries, texts and business logic, merged into an installation's
build. The extension machinery was built and tested in the base repository with a small test app
(`testdata/apps/loyalty-points`); this app is the first one built the way real apps are — in its
own repository, with its own CI and releases.

**The app:** customers belong to a **customer category**. It is deliberately small and has no
business logic: a table and page of its own, a field on a base table, a field and a column on base
pages, a main menu entry, texts in three languages. The base's test app shows the parts it leaves
out (triggers, a page function, a section and buttons on base pages); `EXTENSIONS.md` describes
them all.

## 2. Read first

In the base repository, at the release the app builds against (tag `v0.1.161` or later):

1. **`EXTENSIONS.md`** — the guide to extension apps: every file format, the merge rules, the Go
   module and `Register`, triggers and events, building, installing, enabling.
2. **`testdata/apps/loyalty-points/`** — the base's own test app, merged into the base in the
   base's CI; a working reference for the parts this app does not use.
3. **`scripts/check-extension-fixture.sh`** — how the base builds and tests itself with an app; the
   app's CI does the same.

## 3. The repository

- **Name:** `goeasyerp-test-extension-app`, public.
- **Layout** (`EXTENSIONS.md` §1): the app in `extensions/` — `application.yaml`, `custom/tables/`,
  `custom/pages/`, `tables/`, `pages/`, `menus/`, `translations/<language>/`, `go.mod`, `app.go`,
  `app_test.go` — plus `README.md` (what the app adds, installing, disabling, developing),
  `CHANGELOG.md`, `NOTES.md` (findings for the base's maintainer, §9) and
  `.github/workflows/ci.yml`.
- **Go module path:** `github.com/hansjlachmann/goeasyerp-test-extension-app/extensions` (the
  module is the `extensions/` folder), requiring `github.com/hansjlachmann/goeasyerp`; the build
  points it at the base being built (`go.work`, written by extmerge).

## 4. application.yaml

- `id`: `c7c1e38d-51b4-453d-97ad-32fd54456d4b` — never changes.
- `name`: `Test Extension App`; `publisher`: `hansjlachmann`; `contact`: the repository's URL.
- `version`: `1.0.0` (release tags `v1.0.0` …).
- `prefix`: `tea` (every name the app adds starts with `tea_` / `TEA_` / `TEA `).
- `number_series`: objects and fields `50100`–`50199` (clear of `50000`–`50099`, the base's test
  app).
- `dependencies`: the base application — `id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd`,
  `name: GoEasyERP Base Application`, `min_version: 0.1.161`.

## 5. What it adds

**F1 — Customer categories.** Table `TEA_Customer_Category` (50100): Code (Code 10, primary key,
required), Description (Text 50). Editable list page `TEA Customer Categories` (50100), caption
Customer Categories.

**F2 — Customer field** (table 18): `tea_customer_category_code` (no. 50100, Code 10, relation to
`TEA_Customer_Category`).

**F3 — Pages:** on the Customer Card (21) the field in the General section after Territory Code;
on the Customer List (22) a column after Name.

**F4 — Main menu:** under Sales, after Customers, an entry **Customer Categories** opening page
50100.

**F5 — Texts** in en-US, nb-NO and da-DK for every caption the app adds (own wording).

`Register()` in `extensions/app.go` is empty: the app has no business logic.

## 6. Technical rules it follows

- **Only add.** The base repository is never changed by the app; when the base lacks something,
  it is written up for the base (§9), not worked around.
- **Business logic in the backend** (none so far); pages only show.
- **Go style of the base's business logic** for any code it gets: one statement per line (no
  `if x := f(); cond`), record variables opened from a record, `for found := rec.FindSet(); found;
  found = rec.Next()`, `GetOrError` / `ModifyOrError`, every error checked; user messages from the
  app's translations.
- **Tests (Go)** in the app's module against an in-memory SQLite company
  (`tables.CreateCompanyTables`): the category table, the Customer field and its relation, and the
  app disabled (`extensions.SetDisabled`) — data kept, customer still saved.
- **Public repository:** no installation details, credentials or other products' names;
  Conventional Commits and a changelog per release.

## 7. CI — `.github/workflows/ci.yml`

On every push and pull request:

1. Check out the app and the base repository at the targeted release (into `base/`).
2. Copy `extensions/` into `base/apps/<app id>/`, merge with `go run ./tools/extmerge --core
   backend/definitions --apps apps --output merged --translations translations
   --translations-output merged-translations --go-root .`, replace `backend/definitions` and
   `translations` with the merged ones, `go generate ./backend/generated/tables`, `go build ./...`.
3. The base's checks of the merged build: `go test ./backend/api/handlers
   ./backend/business-logic/apps ./backend/foundation/apps ./cmd/api-server`.
4. The app's own tests: `cd apps/<app id> && go test ./...` (inside `base/`).
5. `go vet` and `gofmt` on the app's code.

## 8. Releasing and installing

- A release is a tag `vX.Y.Z` matching `version` in `application.yaml`, with release notes from
  `CHANGELOG.md`.
- Installing: **Installed Apps → Install App** with the repository URL, the tag and the folder
  `extensions`; **Build List** → commit `apps.yaml` to the installation repository
  (`deploy/installation-template/README.md`) → its workflow builds the image → the server pulls
  and restarts (`EXTENSIONS.md` §10). The app's README tells an administrator exactly that.

## 9. What the base needed

Findings go into the app's `NOTES.md` for the base's maintainer: what is needed, where in the base,
why, how to test it.

- **A disabled app's table relation checks still ran** (`ERR_RELATED_NOT_FOUND` for a value that
  is not a category while the app was disabled). Fixed in the base after 0.1.161: the generated
  `Validate` checks an app field's relation only while the app is enabled; the base's test app
  covers it. Once the app requires a base release with the fix, its disabled-app test can check
  that a missing category is accepted, and the note can be closed.

## 10. Status

- [x] Repository with the layout of §3, `application.yaml` of §4, README, changelog.
- [x] F1–F5 built; texts in all three languages.
- [x] CI (§7) passes.
- [x] Tests cover the table, the field and the disabled app.
- [x] Release `v1.0.0` tagged with release notes.
- [ ] Installed in a test installation through Install App and the installation repository: the
      Customer Card and List show the category, the menu entry opens Customer Categories, Disable
      App hides it all and Enable App brings it back.

## 11. Ideas for later releases

The original brief was a larger app — **loyalty points**: customers collect points from posted
sales invoices (a `salesposting.OnAfterPost` subscriber) and lose them with credit memos, a
loyalty tier table whose tier follows the points (Customer `OnInsert`/`OnModify`), a section and
buttons on the Customer Card and List, and a **Recalculate Points** page function. Building it
would show triggers, a posting event and a page function in a real app repository.
