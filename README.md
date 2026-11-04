<p align="center"><img src="goeasyerp-brand/goeasyerp-wordmark-slate.svg" alt="GoEasyERP" height="64"></p>

# GoEasyERP

Open-source ERP for small businesses: finance, sales, purchasing, inventory, projects and fixed
assets in one system. Go backend, SvelteKit frontend, PostgreSQL or SQLite.

The repository, Go module, container images and database keep the technical name `openerp`.

## Project Structure

```
openerp/
├── cmd/api-server/      # Go API server entry point
├── backend/
│   ├── api/             # HTTP handlers, middleware, server
│   ├── definitions/     # Table, page and menu definitions (YAML)
│   ├── generated/       # Code generated from the table definitions (never edit)
│   ├── business-logic/  # Table triggers, codeunits, migrations
│   └── foundation/      # Core framework (types, database, i18n, etc.)
├── frontend/            # SvelteKit frontend
├── tools/tablegen/      # Code generation tool
└── translations/        # i18n files (en-US, nb-NO)
```

## Build Commands

### API Server (Go)
```bash
# Build
cd cmd/api-server && go build -o api-server .

# Run (from project root)
./cmd/api-server/api-server

# Run without building
go run ./cmd/api-server
```

### Frontend (SvelteKit)
```bash
cd frontend

# Install dependencies
npm install

# Development server    
npm run dev

# Production build
npm run build
```

### Code Generation (tablegen)
```bash
# Generate the table code from backend/definitions/tables (after changing a table YAML)
go generate ./backend/generated/tables
```

### General Go Commands
```bash
# Format code
go fmt ./...

# Check code
go vet ./...

# Update dependencies
go mod tidy
```

## Database

Connect to SQLite database:
```bash
sqlite3 test.db
```

Useful SQLite commands:
```sql
-- List all tables
.tables

-- Show table schema
.schema "cronus$Payment Terms"

-- Show all data with formatting
.mode column
.headers on
SELECT * FROM "cronus$Payment Terms";

-- Exit
.exit
```

## Demo Data

Codeunit 901 "Create Demo Data" fills a company with demo data: countries
(NO, DK, GB), payment terms, and Norwegian, Danish and English customers with a
year of customer ledger entries. The data is in `backend/business-logic/demodata/data/*.yaml`.

1. Create a company, e.g. `demo01` with display name "Demo Company 01", and switch to it (Ctrl+O).
2. Add a Job Queue line: Object ID to Run `901`, Parameter `SMALL`, `LARGE` or `HEAVY`.
3. Run it (F9), or set it to Ready with a Next Start to let the scheduler run it.

| Size  | Customers | Ledger entries | Use |
|-------|-----------|----------------|-----|
| SMALL | 20        | ~540           | demos, screenshots, E2E |
| LARGE | 10,000    | ~140,000       | paging, search and performance tests |
| HEAVY | 200       | ~190,000 (~1,000 per customer, 2 years) | SIFT / FlowField volume tests |

The run is one transaction (all or nothing), produces the same data every time
(fixed random seed, dates relative to the run date), and refuses a company that
already has customers, so it can never mix demo data into a real company.
Every run is logged in Job Queue Entries.
