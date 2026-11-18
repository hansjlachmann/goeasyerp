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

An app's code is compiled into the program, so installing one takes a build. Each installation
with apps has an **installation repository** (template: `deploy/installation-template/`) with its
build list `apps.yaml` and a GitHub Actions workflow that builds the backend image — the base
release of the list with its apps — and pushes it; the server runs that image (`BACKEND_IMAGE` in
`prod.env`, `docs/operations.md`).

On **Installed Apps** (Administration → Installed Apps; SUPER only):

- **Install App** asks for the app's GitHub repository, its version (a release tag) and the folder
  of its `application.yaml`, reads that file from GitHub (`GITHUB_TOKEN` of the server for private
  repositories), checks it against the installed apps and those already chosen — numbers, prefix,
  dependencies and their versions — and shows it as **Pending Install**. A newer version of an
  installed app is an update.
- **Uninstall App** marks an app **Pending Removal** (not while another app depends on it).
  **Cancel Pending** takes back a pending install, update or removal.
- **Build List** downloads `apps.yaml` — the base version and the apps of the next build in
  dependency order, each with its repository, version and folder — to commit to the installation
  repository. Its workflow builds the image; after `scripts/prod.sh pull && up -d` the server shows
  the apps as Installed, and removed apps are gone (their data stays).

## 11. Disabling and enabling

Installed Apps has **Disable App** and **Enable App** (SUPER only). A disabled app stays built in
and keeps its data, but its business logic, event subscribers and field checks (also the table
relation checks of its fields) do not run, the page parts and menu entries it added are left out or
greyed out, and its pages, reports, routines and functions are refused. The base application cannot
be disabled; an app cannot be disabled while an enabled app depends on it, nor enabled while an app
it depends on is disabled. Every server reads the switch again within 30 seconds.
