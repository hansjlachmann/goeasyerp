# GoEasyERP Extensions

This document describes how to extend GoEasyERP with custom tables, pages, and codeunits.

## Object and field numbers: application.yaml

Every app has an `application.yaml` (app.json) with a GUID that never changes and the
number series it may use. The base application's is `backend/definitions/application.yaml`:

| Numbers | Base application |
|---------|------------------|
| Objects (tables, pages, codeunits, reports; unique per object type) | 1 – 999 |
| Fields of its tables | 1 – 999 |
| System tables (section `system_tables`) | 2,000,000,000 – 2,000,000,199 |

An extension app has its own `application.yaml` at the root of its extensions directory, with
its own series that must not overlap the base application's or another app's, for example:

```yaml
application:
  id: 0b7f…-…                 # a new GUID, never changed
  name: My Company Extension
  publisher: My Company
  contact: https://example.com
  version: 1.0.0
  prefix: lp                  # the start of the app's own names (see below)
  number_series:
    objects: [{from: 50000, to: 50999}]
    fields:  [{from: 50000, to: 50999}]   # also the fields it adds to base tables
  dependencies:
    - id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd   # the base application
      name: GoEasyERP Base Application
      min_version: 0.1.97
```

`extmerge` refuses an extension without an `application.yaml`, series that overlap, and
objects or fields outside the extension's series. At startup the server checks every table,
page and codeunit against the installed apps and lists them in Installed Apps (page 907).

### Dependencies

Every extension app depends on the base application; it may also depend on other extension
apps (and then extend their tables and pages and use their objects). A dependency names the
app's `id`, its `name` and the lowest version it needs (`min_version`). The apps of a build are
checked together and merged in dependency order — the base first, an app after the apps it
depends on. Refused, at build time and when the server starts: a dependency that is not part of
the build, a version below `min_version`, an id whose app has another name, and apps that
depend on each other in a circle. The merge writes each app's `application.yaml` to
`backend/definitions/apps/<id>.yaml`; the server reads them at startup.

### Names: the app's prefix

An app's own names start with its `prefix` (2–10 lower case letters and digits) and a
separator, so two apps cannot clash by name: the fields it adds to tables of the base or of
another app (`lp_loyalty_points`), its tables (`LP_Loyalty_Tier`) and its pages
(`LP Loyalty Tiers`). The fields of its own tables need no prefix.

### Texts: translations/<language>/*.yaml

An app's captions and messages are in `translations/<language>/` of its directory, written like
the base application's (`tables.yaml`: `tables.<table>.caption`, `tables.<table>.fields.<field>`;
`pages.yaml`; `common.yaml`: `common.actions.<action>`, `common.sections.<section>`,
`common.menu.<entry>`; `messages.yaml` …), in every language of the base application. They are
copied next to the base's texts as files of their own (`<prefix>-<file>.yaml`). An app only adds
texts: a key the base application or another app already has is refused, as is a language the
base application does not have.

## Extension Types

### 1. New Objects

Create entirely new tables, pages, or codeunits using IDs in the extension range:

```yaml
# 50000-custom-table.yaml
table:
  id: 50000
  name: "Custom_Inventory"
  # ... full table definition
```

### 2. Extensions to Existing Objects

Extend core objects using `.extend.yaml` files:

```yaml
# 21-customer-card.extend.yaml
extends: 21

actions:
  - name: Custom Report
    caption: Custom Report
    run_object: codeunit:50100   # a codeunit of the extension (its series)
    promoted: true

layout:
  sections:
    - name: General          # Add to existing section
      fields:
        - source: custom_field
          caption: Custom Field
          editable: true

    - name: Custom Section   # Add new section
      caption: Custom Data
      fields:
        - source: custom_data
          editable: true
```

## Extension File Format

### Page Extension

An app adds to a page; it never changes or removes what the page has. The page is merged as
it is written (every key kept), and everything added is marked with the app (`app: <id>`).

```yaml
extends: 21                     # Required: ID of the page to extend
version: ">=0.1.9"              # Optional: compatible core versions

layout:
  sections:                     # A card's sections
    - name: General             # An existing section: only fields are added to it
      fields:
        - source: lp_loyalty_points
          after: credit_limit_lcy   # placed after (or before:) a field of the section
    - name: Loyalty             # A new section
      caption: Loyalty
      after: Payments           # placed after (or before:) a section; else at the end
      fields:
        - source: lp_tier_code
  repeater:                     # A list page's columns
    fields:
      - source: lp_loyalty_points
        after: name
        width: 100

actions:
  - name: Loyalty Tiers         # A button, after (or before:) an action
    after: Sales
    run_page: 50000
  - name: Recalculate Points    # An item of an existing menu button (at any depth)
    in: Functions
    run_object: codeunit:50000
```

### Menu Extension

An app adds entries to a main menu file (`menus/<file>.extend.yaml`) — its own pages,
reports and routines — into a sub-menu named by its path from the top, or as a new module:

```yaml
extends: admin.yaml
items:
  - in: [Sales, Reports]        # the sub-menu (one name or the path of names)
    after: Customer - List      # placed after (or before:) an entry; else at the end
    name: Loyalty Points
    run_object: report:50000
  - name: Loyalty               # without in: a new module
    items:
      - name: Loyalty Tiers
        page_id: 50000
```

### Table Extension

```yaml
extends_table: <table_id>    # Required: ID of parent table to extend

fields:                      # Written like the fields of a table definition (every key is kept)
  - name: custom_field
    no: 50000                # Required: field number from the extension's series (above 999), unique in the table
    type: types.Text
    length: 100
    caption: Custom Field
```

Field numbers identify fields (field IDs): `extmerge` refuses an extension field without a
number in the app's field series, or with a number or name the table (or another extension)
already uses, and a field name without the app's prefix. A number is never changed or reused.

## Merge Rules

| Element | Behavior |
|---------|----------|
| Card fields, list columns, sections, actions, menu entries | Added — at the end, or `after:` / `before:` an existing one (by field `source` or by `name`) |
| `in:` on an action | Added to the items of that menu button |
| An existing section named in `layout.sections` | Only its `fields` are added; its other settings stay |
| Something with the `source` or `name` of an existing one | Refused: an app adds, it never replaces |
| Unknown keys in an extension file, an anchor that does not exist | Refused, naming it |
| Everything added | Marked `app: <id>` |

`scripts/check-extension-fixture.sh` builds the base application with the test app in
`testdata/apps/loyalty-points` the way an installation's build does (CI runs it).

## Build Process

Extensions are merged at build time using the `extmerge` tool:

```bash
# Merge extension apps with core (--extensions per app, or --apps for a directory of apps)
extmerge --core ./backend/definitions \
         --extensions ./extensions \
         --output ./merged \
         --translations ./translations --translations-output ./merged-translations

# then generate the table code from the merged definitions
go generate ./backend/generated/tables

# Or in Docker build (merges, regenerates and builds)
docker build --build-arg EXTENSIONS_PATH=./extensions .
```

## Directory Structure (Extension Repository)

```
my-goeasyerp-extension/
├── extensions/
│   ├── application.yaml                 # The extension app: id, number series, dependencies
│   ├── tables/
│   │   └── 18-customer.extend.yaml      # Extend Customer table
│   ├── pages/
│   │   └── 21-customer-card.extend.yaml # Extend Customer Card
│   ├── menus/
│   │   └── admin.extend.yaml            # Entries in the main menu
│   └── logic/                           # Go module: business logic (see below)
│       ├── customer.go                  # Triggers for the Customer fields
│       └── 50100-custom-report.go       # New codeunit
├── custom/
│   ├── tables/
│   │   └── 50000-inventory.yaml         # New table
│   └── pages/
│       └── 50001-inventory-list.yaml    # New page
├── openerp.yaml                          # Core version config
└── .github/workflows/build.yml
```

## openerp.yaml (Extension Config)

```yaml
core:
  repository: hansjlachmann/goeasyerp
  version: v0.1.9

customer:
  name: my-company
  registry: ghcr.io/my-company

id_ranges:
  tables: 50000-50999
  pages: 50000-50999
  codeunits: 50000-50999
```

## Extension apps: business logic (planned)

The base application is built so that an extension app can add business logic without
changing the base code. What already works in the base:

- **Generated code is derived from the merged definitions.** `backend/generated/tables` is
  regenerated after the merge, so a field from `customer.extend.yaml` becomes a typed field
  of `gtables.Customer` and gets its own `OnValidate_<Field>` trigger.
- **Business logic is a list per table.** The base application registers its triggers in
  `backend/business-logic/tables` (`gtables.RegisterCustomerLogic(customerLogic{})`). An
  extension registers its own logic type for the same table, and every registered logic runs
  in registration order (base first). The first error stops the operation.

```go
// my-goeasyerp-extension/extensions/logic/customer.go
package logic

import gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

type customerLogic struct{}

func init() { gtables.RegisterCustomerLogic(customerLogic{}) }

// Runs after the base application's Customer OnInsert
func (customerLogic) OnInsert(rec *gtables.Customer) error {
	if rec.Loyalty_points == 0 {
		rec.Loyalty_points = 100
	}
	return nil
}

func (customerLogic) OnValidate_Loyalty_points(rec *gtables.Customer) error { … }
```

Still to build (`TODO.md`, "Extension apps"):

- **Linking the extension's Go module into the build:** a `go.work` file or a `replace`
  directive pointing at the merged, regenerated base, plus a generated file with a blank
  import per installed extension.
- **Field-name collision rules.**
- **OnBefore/OnAfter and codeunit events.**
