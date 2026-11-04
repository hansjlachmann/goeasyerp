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

```yaml
extends: <page_id>           # Required: ID of parent page to extend
version: ">=0.1.9"           # Optional: Compatible core versions

# Add actions (appended to existing actions)
actions:
  - name: Action Name
    caption: Display Caption
    shortcut: Ctrl+X         # Optional
    promoted: true           # Optional
    run_object: codeunit:ID  # Or run_page: ID

# Extend layout
layout:
  sections:
    - name: Existing Section # Merge into existing section
      fields:
        - source: field_name
          caption: Caption
          editable: true

    - name: New Section      # Add new section
      caption: Section Caption
      fields:
        - source: field_name
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
number above the base application's 1–999, or with a number or name the table (or another
extension) already uses. Each extension uses its own number series (planned: `application.yaml`,
see TODO "Field and object numbering"). A number is never changed or reused.

## Merge Rules

| Element | Behavior |
|---------|----------|
| `actions` | Appended to parent's action list |
| `layout.sections` (existing name) | Fields merged into that section |
| `layout.sections` (new name) | Added as new section |
| `fields` (same source) | Overrides parent field definition |
| Other properties | Overrides parent value |

## Build Process

Extensions are merged at build time using the `extmerge` tool:

```bash
# Merge extensions with core
extmerge --core ./backend/definitions \
         --extensions ./extensions \
         --output ./merged

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
  repository: hansjlachmann/openerp
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

import gtables "github.com/hansjlachmann/openerp/backend/generated/tables"

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
- **A lossless page merge in `extmerge`:** today it drops page keys it does not know.
- **Field-name collision rules.**
- **OnBefore/OnAfter and codeunit events.**
