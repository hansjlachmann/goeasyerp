# GoEasyERP extension apps

An extension app adds to GoEasyERP without changing it: new tables and pages, fields on the
base application's tables, fields, columns and buttons on its pages, entries in the main menu,
texts, and business logic that runs alongside the base application's. This guide is for the
author of an app.

**Principles**

- An app only adds. It never edits the base application's files or code, and it never replaces
  or removes what the base application (or another app) has — the build refuses that.
- An app is its own repository, with its own version, releases and CI. It is tested against the
  base release it needs.
- An installation's build merges the base release with the installation's apps and compiles them
  into one program: one database transaction for the base's and the apps' logic, typed fields.
  Apps are never loaded at runtime as separately built code.
- Existing modules are never split out of the base application; new functionality an
  installation needs beyond the base comes as apps.

The test app `testdata/apps/loyalty-points` of this repository has every part described here;
`scripts/check-extension-fixture.sh` builds the base application with it the way an
installation's build does (CI runs it on every change of the base). An app in a repository of
its own, released and installed as described here, is the Test Extension App,
`https://github.com/hansjlachmann/goeasyerp-test-extension-app` — copy its layout, CI and README.

## 1. An app at a glance

```
goeasyerp-loyalty/                       # the app's repository
├── extensions/                          # the app's directory (its folder in a build)
│   ├── application.yaml                 # identity, prefix, number series, dependencies
│   ├── tables/
│   │   └── 18-customer.extend.yaml      # fields added to Customer
│   ├── pages/
│   │   ├── 21-customer-card.extend.yaml # fields, sections, buttons added to the Customer Card
│   │   └── 22-customer-list.extend.yaml # columns, buttons added to the Customer List
│   ├── menus/
│   │   └── admin.extend.yaml            # entries added to the main menu
│   ├── custom/
│   │   ├── tables/50000-loyalty-tier.yaml    # the app's own tables
│   │   └── pages/50000-loyalty-tiers.yaml    # the app's own pages
│   ├── translations/<language>/*.yaml   # the app's texts (en-US, nb-NO, da-DK)
│   ├── go.mod                           # the app's Go module (business logic)
│   └── *.go                             # func Register() and the logic
└── .github/workflows/ci.yml             # builds and tests the app against the base release
```

## 2. application.yaml — identity, numbers, prefix, dependencies

```yaml
application:
  id: 0f72f6dc-d125-44df-90c1-e61420d854fe   # a new GUID (lower case), never changed
  name: Loyalty Points
  publisher: My Company
  contact: https://example.com
  version: 1.0.0                             # the app's version (its release tags: v1.0.0)
  prefix: lp                                 # the start of the app's own names (below)
  number_series:
    objects: [{from: 50000, to: 50099}]      # its tables, pages, reports, routines
    fields: [{from: 50000, to: 50099}]       # the fields it adds, also to base tables
  dependencies:
    - id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd
      name: GoEasyERP Base Application
      min_version: 0.1.161                   # the lowest base release it works with
```

**Numbers.** Every object (table, page, report, routine; unique per object type) and every field
has a number that never changes and is never reused. An app's series must not overlap the base
application's or another app's. The base application (`backend/definitions/application.yaml`)
uses objects 1–999, 1000–1299, 1400–1499, 5400–5499, 5600–5699, 5700–5799 and 9300–9399, fields
1–999, 5400–5499 and 5600–5699, and system tables 2,000,000,000–2,000,000,199. Apps take their
series from 50000 up.

**Prefix.** An app's own names start with its `prefix` (2–10 lower case letters and digits,
starting with a letter) and a separator, so two apps cannot clash by name: the fields it adds to
tables of the base or of another app (`lp_loyalty_points`), its tables (`LP_Loyalty_Tier`) and
its pages (`LP Loyalty Tiers`). The fields of its own tables need none. Name the functions an app
registers on base tables with it too (`LpRecalculatePoints`).

**Dependencies.** Every app depends on the base application; it may also depend on other apps
(and then extend their tables and pages and use their objects and events). A dependency names
the app's `id`, its `name` and the lowest version it needs (`min_version`). The apps of a build
are checked together and merged in dependency order — the base first, an app after the apps it
depends on, so its logic also runs after theirs. Refused, when the app is chosen to install, at
build time and when the server starts: a dependency that is not part of the build, a version
below `min_version`, an id whose app has another name, and apps that depend on each other in a
circle.

## 3. The app's own tables and pages — custom/

New tables (`custom/tables/*.yaml`) and pages (`custom/pages/<id>-<name>.yaml`) are written
exactly like the base application's — its `backend/definitions/tables/*.yaml` and
`backend/definitions/pages/*.yaml` are the examples to follow — with numbers from the app's
series and names with its prefix:

```yaml
table:
  id: 50000
  name: "LP_Loyalty_Tier"
  fields:
    - name: code
      no: 50000
      type: types.Code
      length: 10
      primary_key: true
    - name: min_points
      no: 50002
      type: int
      validation: {min: 0}
  keys:
    - name: Primary
      fields: [code]
      clustered: true
```

## 4. Fields on base tables — tables/<id>-<name>.extend.yaml

```yaml
extends_table: 18                # the table's number
fields:                          # written like a table definition's fields (every key kept)
  - name: lp_loyalty_points
    no: 50000                    # from the app's field series
    type: int
    validation: {min: 0}
  - name: lp_tier_code
    no: 50001
    type: types.Code
    length: 10
    table_relation: {table: LP_Loyalty_Tier, field: code}
```

The fields become typed fields of the generated record (`rec.Lp_loyalty_points` on
`gtables.Customer`) and columns of the table (added when the server starts, data kept when the
app is removed). Refused: a field without a number in the app's series, a number or name the
table already has, a name without the prefix.

## 5. Pages — pages/<id>-<name>.extend.yaml

An app adds card fields and sections, list columns and buttons to a page; it never changes or
removes what the page has. The page is merged as it is written (every key kept).

```yaml
extends: 21                     # the page's number
layout:
  sections:                     # a card's sections
    - name: General             # an existing section: only fields are added to it
      fields:
        - source: lp_loyalty_points
          after: credit_limit_lcy   # placed after (or before:) a field of the section
          editable: false
    - name: Loyalty             # a new section
      caption: Loyalty
      after: Payments           # placed after (or before:) a section; else at the end
      fields:
        - source: lp_tier_code
  repeater:                     # a list page's columns
    fields:
      - source: lp_loyalty_points
        after: name
        width: 100
actions:
  - name: Loyalty Tiers         # a button, after (or before:) an action
    after: Sales
    promoted: true
    run_page: 50000
  - name: Recalculate Points    # an item of an existing menu button (at any depth)
    in: Customer
    run_object: "table:Customer.LpRecalculatePoints"
```

| Element | Behaviour |
|---------|-----------|
| Card fields, list columns, sections, actions | Added — at the end, or `after:` / `before:` an existing one (a field by its `source`, the others by `name`) |
| `in:` on an action | Added to the items of that menu button |
| An existing section named in `layout.sections` | Only its `fields` are added; its other settings stay |
| Something with the `source` or `name` of an existing one | Refused: an app adds, it never replaces |
| Unknown keys in an extension file, an anchor that does not exist | Refused, naming it |
| Everything added | Marked with the app (`app: <id>`): left out while the app is disabled |

## 6. The main menu — menus/<file>.extend.yaml

```yaml
extends: admin.yaml               # the menu file
items:
  - in: Sales                     # a sub-menu: one name, or the path of names from the top
    after: Customers              # placed after (or before:) an entry; else at the end
    name: Loyalty Tiers
    page_id: 50000
  - in: [Sales, Reports]
    name: Loyalty Points
    run_object: "report:50001"
  - name: Loyalty                 # without in: a new module
    items:
      - name: Loyalty Tiers
        page_id: 50000
```

## 7. Texts — translations/<language>/*.yaml

The app's captions and messages, written like the base application's — `tables.yaml`
(`tables.<table>.caption`, `tables.<table>.fields.<field>`, `tables.<table>.options.<field>.<value>`),
`pages.yaml` (`pages.<page>.caption`), `common.yaml` (`common.actions.<action>`,
`common.sections.<section>`, `common.menu.<entry>`), `messages.yaml` — in every language of the
base application (en-US, nb-NO, da-DK). Keys are the names in lower case with every run of other
characters one `_` ("Loyalty Tiers" → `loyalty_tiers`). They are copied next to the base's texts
as files of their own (`<prefix>-<file>.yaml`). An app only adds texts: a key the base
application or another app already has is refused, as is a language the base application does
not have.

## 8. Business logic — the app's Go module

The app's Go code is a Go module in its directory (`go.mod` next to `application.yaml`). Its
go.mod requires the base module (`github.com/hansjlachmann/goeasyerp`) at the base release it
builds on; the module's root package has `func Register()`, which the program calls at startup
— after the base application's logic, the apps in dependency order. Everything an app registers
in `Register` belongs to the app (it stops while the app is disabled).

```go
package loyaltypoints

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Register registers the app's business logic
func Register() {
	gtables.RegisterCustomerLogic(customerLogic{})
	salesposting.OnAfterPost.Subscribe(addInvoicePoints)
}

type customerLogic struct{}

// OnModify runs after the base application's Customer OnModify
func (customerLogic) OnModify(rec *gtables.Customer) error { … }
```

**Table triggers.** Business logic is a list per table: the base application's logic and every
app's run in registration order, and the first error stops the operation (the API rolls it
back). An app's logic type has the triggers it needs: `OnInsert`, `OnModify`, `OnDelete`,
`OnRename`, `OnValidate_<Field>` (also of the fields it added) — `func(rec *gtables.<Table>)
error` — and the before-triggers `OnBeforeInsert`, `OnBeforeModify`, `OnBeforeDelete`,
`OnBeforeRename` — `func(rec *gtables.<Table>, handled *bool) error` — which run before the
triggers of that kind; when one sets `*handled = true` the triggers of that kind (the base
application's and every app's) are skipped for that record, the definition's field checks still
run. Use `handled` only to replace base behaviour, sparingly. A method named like a trigger the
table does not have stops the program at startup.

**Events.** For logic that is not a table trigger — posting, prices — the base application
publishes events at chosen points (`backend/foundation/events`), and an app subscribes in its
`Register`. A subscriber runs inside the publisher's transaction: its error stops the operation
and nothing of it is saved.

| Event | Passes | When |
|-------|--------|------|
| `salesposting.OnBeforePost` | `*salesposting.BeforePost` (the sales header) | a sales document is about to be posted — an error refuses it |
| `salesposting.OnAfterPost` | `*salesposting.AfterPost` (the header, ShipmentNo, InvoiceNo, CrMemoNo) | a sales document was posted |
| `purchposting.OnBeforePost` | `*purchposting.BeforePost` (the purchase header) | a purchase document is about to be posted — an error refuses it |
| `purchposting.OnAfterPost` | `*purchposting.AfterPost` (the header, ReceiptNo, InvoiceNo, CrMemoNo) | a purchase document was posted |

More events are added to the base where an app needs one: ask for it with the point in the code
and what the app needs to see or change there.

**Page functions, routines and reports.** A button's `run_object: "table:<Table>.<Function>"`
calls a function the app registers with `functions.Register("table", "<Table>", "<Function>",
functions.For(fn))` (with `functions.WithDialog()` when it asks or shows progress); its own
routines and reports register with numbers from its series, as the base application's do.

**Base conventions apply.** Write logic the way the base application's is written
(`backend/business-logic/` is the example): the generated record methods (`Get`, `FindSet`/`Next`,
`Validate`, `GetOrError`/`ModifyOrError` …), errors with texts from the translations (never an
English text shown to users), and no posting that would leave the general ledger out of balance
(the base refuses it).

### What an app's Go code may use (the app audit)

Every app is audited before an installation builds it into the server — by the installation's
builder, by Upload App (before the package is recorded) and in an installation repository's CI
pipeline. An app that fails is not built. Run the same audit on your app before you release it:

```bash
go run github.com/hansjlachmann/goeasyerp/cmd/app-audit path/to/your-app
```

The audit reads the source (`*_test.go` files are left out: they are not built into the server).
It refuses:

- **Getting around Go's rules:** `import "unsafe"`, cgo (`import "C"`, `#cgo`), `//go:linkname`,
  `//go:generate` and other compiler directives (`//go:build`, `//go:embed` and `//go:noinline` are
  allowed, but an app's files have no build constraints: every file is built), assembly, C and
  object files.
- **Standard library packages outside the allowlist:** computation is allowed (`strings`,
  `strconv`, `fmt`, `errors`, `time`, `math`, `sort`, `slices`, `maps`, `regexp`, `bytes`,
  `bufio`, `io`, `context`, `sync`, `unicode`, `encoding/json|csv|base64|hex|xml|binary`,
  hashing, `net/url` …); the environment, files, processes, the network and the database are not
  (`os`, `os/exec`, `syscall`, `net`, `net/http`, `database/sql`, `reflect`, `runtime`, `plugin`,
  `log` …).
- **GoEasyERP packages outside the app API:** an app may use the records
  (`backend/generated/tables`), `backend/business-logic` (its functions and events, but not
  `apps`, `migrations`, `scheduler`, `jobqueue`, `docmail`, `demodata`, `themerepo`,
  `refusedposting`), and from `backend/foundation` `types`, `system`, `functions`, `events`,
  `codeunits`, `errors`, `i18n`, `format`, `dateformula` and `flowfilter` — not `database`,
  `tables`, `secrets`, `session`, `mail` or the server's other internals.
- **Functions that give the server's powers**, whatever their package: the record's database
  connection and SQL (`GetDB`, `SetDB`, `Exec`, `Query`, `QueryRow`, `Begin` …), other
  companies (`ChangeCompany`), other users' rights (`SetCurrentAccess`, `SetCurrentContext`),
  tables on a database of the app's choosing (`InitWithDBType`, `InitTemporary`, `DBSource`), the
  balance check (`Consistent`), the environment (`Getenv`).
- **Changing another package's variables** (`pkg.X = …`, `&pkg.X`): an app does not take over
  the base's behaviour.
- **Modules:** `go.mod` requires only the base (and the modules App Build Setup's *Allowed Go
  Modules* names), and has no `replace` or `exclude` directives.
- **Known vulnerabilities in the modules an app brings in:** an app that requires modules besides
  the base is checked with `govulncheck` once it is merged into the base being built (its code
  compiles only there): a function with a known vulnerability that the app's code calls in such a
  module is refused (rule `vulnerability`, the finding names the vulnerability, e.g.
  `GO-2026-1234`, and the version that fixes it). Vulnerabilities in the base's own modules and
  the Go standard library are the base's to fix in its releases. To check yourself, merge your
  app into a base checkout (`go run ./tools/extmerge … --go-root .`, then
  `go generate ./backend/generated/tables`) and run
  `go run ./cmd/app-audit -govulncheck apps/your-app` there.

A finding that is right for an app (an import the allowlist does not have yet) can be approved
by a SUPER user for **exactly one source** — the Git commit or the uploaded package's SHA-256
the report names — in Installed Apps → App Audit Exceptions. A new version, or a tag that moved,
is audited afresh. An approval is recorded in the change log and as a security event.
Installed Apps → Build → Audit Report gives an app's latest report: the one of the latest build
that took its source, or for an uploaded package no build has taken yet, an audit made then.

The audit does not make an app harmless: an app that passes can still post wrong amounts or
change records in the companies where it runs. It closes the server-wide powers — other
companies, the secrets, files, the network, the database behind the base's checks, other users'
rights. A module allowed in App Build Setup is built in without being audited: allow only modules
you trust.

## 9. Building

`tools/extmerge` merges apps into the base definitions; the base's Dockerfile does it when
`EXTENSIONS_PATH` is a directory of apps (or one app):

```bash
extmerge --core backend/definitions --apps apps --output merged \
         --translations translations --translations-output merged-translations \
         --go-root .
# then: the merged definitions and translations replace the base's, and
go generate ./backend/generated/tables   # the generated records get the apps' fields
go build ./cmd/api-server
```

The merge checks the apps (identity, numbers, prefix, dependencies, what they add), writes each
app's `application.yaml` to `backend/definitions/apps/<id>.yaml` (the server reads the installed
apps from there), and with `--go-root` writes `go.work` (the base and the apps' modules, the base
release an app requires pointed at the base being built) and `backend/installedapps/apps_gen.go` (each
app's `Register`, in dependency order). Both are build outputs, never committed.

**An app's CI** checks out the base release it targets, merges the app into it as above, builds,
and runs the base's page, menu, caption and installed-app tests and the app's own Go tests — the
steps of `scripts/check-extension-fixture.sh` in the base repository.

## 10. Installing, updating and uninstalling

An app's code is compiled into the program, so installing one takes a build. **App Build
Setup** chooses who builds, one or the other:

- **The installation's own builder (Local Builder).** No repository is needed. The builder runs
  next to the server (`cmd/app-builder`, image `goeasyerp/builder:<version>`). It takes the
  build list from the database and builds the program with the apps into a volume it shares
  with the server, and the server restarts by itself on the new program. Each build and its log
  are listed in App Builds (page 912).
- **A CI pipeline.** The installation has an **installation repository** (template:
  `deploy/installation-template/`) with its build list `apps.yaml` and a GitHub Actions workflow
  that builds and pushes the backend image: the base release of the list with its apps. The
  server runs that image (`BACKEND_IMAGE` in `prod.env`).

Setting either one up is described in `docs/operations.md`.

On **Installed Apps** (Administration → Installed Apps; SUPER only):

- **Install App** asks for the app's GitHub repository, its version (a release tag) and the folder
  of its `application.yaml`. The repository must be allowed in App Build Setup (Allowed
  Repositories: GitHub owners or owner/repository, blank allows none). It resolves the tag to its
  commit and records it (Repository Commit), reads `application.yaml` at that commit
  (`GITHUB_TOKEN` of the server for private repositories), checks it against the installed apps
  and those already chosen — numbers, prefix, dependencies and their versions — and shows it as
  **Pending Install**. A newer version of an installed app is an update.
- **Pinned to a commit:** every build — Build Apps, the automatic build after a base upgrade, the
  CI pipeline — takes exactly the recorded commit and refuses when the tag now points to another
  one. A published version never changes: release changes as a new version (a new tag), never by
  moving a tag.
- **Uninstall App** marks an app **Pending Removal** (not while another app depends on it).
  **Cancel Pending** takes back a pending install, update or removal.
- **Build → Build Apps** (Local Builder) asks the builder to build the planned apps. When the
  build is done the server runs it, and the apps show as Installed.
- **Build → Build List** (CI pipeline) downloads `apps.yaml` — the base version and the apps of the next build in
  dependency order, each with its repository, version and folder — to commit to the installation
  repository. Its workflow builds the image; after `scripts/prod.sh pull && up -d` the server shows
  the apps as Installed, and removed apps are gone (their data stays).
- **Upload App** takes an **app package** instead of a repository (App Build Setup: Allow Upload).
  The installation's own builder builds it; a CI pipeline cannot, so Build List refuses a list
  with an uploaded app.

**App packages.** A package is a `.zip` of the app's folder: its `application.yaml`,
definitions, Go code and translations. The files go either at the root of the zip or inside one
top folder, so zipping the `extensions` folder as it is works:

```bash
cd my-app && zip -r ../my-app-1.0.0.zip extensions
```

The package is checked as untrusted input:
- it may be at most 2 MB zipped and 20 MB unpacked;
- it may only contain `.yaml`, `.yml`, `.go`, `.mod`, `.sum`, `.json`, `.md` and `.txt` files,
  plus `LICENSE`, `NOTICE` and `README`;
- links, hidden files and folders, and paths outside the app are refused.

Then its `application.yaml` is checked as Install App checks it. Attach the package to every
release of the app, next to the tag, so installations without GitHub access can upload it.

## 11. Disabling and enabling

Installed Apps has **Disable App** and **Enable App** (SUPER only). A disabled app stays built in
and keeps its data, but its business logic, event subscribers and field checks (also the table
relation checks of its fields) do not run, the page parts and menu entries it added are left out or
greyed out, and its pages, reports, routines and functions are refused. The base application cannot
be disabled; an app cannot be disabled while an enabled app depends on it, nor enabled while an app
it depends on is disabled. Every server reads the switch again within 30 seconds.

## 12. Developing an app against a local container

An app is developed against GoEasyERP in a local Docker container: the development container,
which builds the app into the server each time it is published. The GoEasyERP VS Code extension
publishes with F5; the steps below are what it does, and work by hand too.

**The development container** is the one-computer installation with the app builder and the
development endpoint (`docker-compose.dev.yml`, over `docker-compose.local.yml`):

```bash
docker compose -f docker-compose.local.yml -f docker-compose.dev.yml up -d
```

with a `.env` next to the two files holding `JWT_SECRET`, `POSTGRES_PASSWORD`,
`OPENERP_ENCRYPTION_KEY`, `SETUP_TOKEN` and `BUILDER_DB_PASSWORD` (long random texts),
`COMPOSE_PROFILES=apps` (the builder) and `GOEASYERP_PORT=8080`. It runs at
`http://localhost:8080`, for this computer only, with a demo company (`DEMO_DATA`, default
`SMALL`; empty for none), as a Compose project of its own (`goeasyerp-dev`: its own database, not
a trial installation's). Make the first user with the setup token (the login page, or
`POST /api/auth/init`).

**The development endpoint** exists only on a server started with `GOEASYERP_DEV_ENDPOINT=true`
(the development compose file sets it) and a database that does not look like production — every
company with customers is a demo company, and at most 10 users; otherwise the server logs why and
the routes do not exist. It answers this computer only (the container's network included);
`GOEASYERP_DEV_ENDPOINT_REMOTE=true` lets other computers publish too (a shared test server).
Log in as for the pages (`POST /api/auth/login`, the session cookie); a SUPER user publishes.

| Request | Does |
|---|---|
| `POST /api/dev/apps` | The app's package (a `.zip` of its folder, as for Upload App) as the body. Checked and audited as Upload App does; the same version may be published again (the package is replaced). Requests a build and answers `{build_no, app_id, name, version, sha256}`. An app the audit refuses: status 422 with `data.findings` (`rule`, `file`, `line`, `key`, `detail`). Refused while another build is requested or running. |
| `GET /api/dev/builds/<no>[?from=<byte>]` | The build: `status` (Requested, Running, Succeeded, Failed), `done`, `error_message`, `log` (from the byte `from`; `log_size` is where the next request starts), `running_build` and `live` — true once the server runs this build. |
| `DELETE /api/dev/apps/<app id>` | Unpublishes the app (Uninstall App) and requests the build without it. |
| `GET /api/version` | `{version, build}`: the base version and the build the server runs (0: none). |
| `GET /api/dev/schemas`, `GET /api/dev/schemas/<name>` | The JSON schemas of the YAML files (below); no login. |

A publish is: package → `POST /api/dev/apps` → read the build until `done` (a failed build's log
says why) → wait until `/api/version` names the build → open the browser. On a workstation the
first build of a new container takes about two minutes (the build cache is empty); after that a
change of the app's code is built in seconds and live in about 15 seconds. While a development
server is set up the server looks for a new build every 2 seconds (otherwise 30).

Production never takes apps this way: a released version is installed with Install App or Upload
App (section 10).

**JSON schemas of the YAML files.** `backend/schemas/` has a JSON schema for each kind of file an
app has — `application.schema.json`, `table.schema.json` (also an app's `custom/tables`),
`page.schema.json` (`custom/pages`), `menu.schema.json`, `table-extension.schema.json`,
`page-extension.schema.json`, `menu-extension.schema.json`, `translations.schema.json` — made
from the Go types that read the files (`go run ./tools/schemagen`), so they know every key,
describe it with the base's own comments and mark a key nobody reads (a typo) as an error. The
GoEasyERP VS Code extension uses them; any editor with a YAML language server can too, with a
line at the top of the file naming the schema of the base version the app builds on:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/hansjlachmann/goeasyerp/v0.1.194/backend/schemas/page-extension.schema.json
```

