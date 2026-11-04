# Database Migrations

## Overview

The migration system handles **destructive** and **complex** schema changes that cannot be done with the automatic table sync. While the table sync handles additive changes (new tables, new columns), migrations handle:

- Column removal
- Column renaming
- Data transformations
- Index changes
- Any custom SQL operations

## How It Works

```
App Startup
    │
    ▼
EnterCompany()
    │
    ▼
AcquireMigrationLock() ─── fails ──► Wait/Retry
    │
    ▼ (lock acquired)
GetPendingMigrations()
    │
    ▼
For each migration:
    ├─► Begin Transaction
    ├─► Run migration across all Company$Tables
    ├─► Record in _migrations table
    └─► Commit Transaction
    │
    ▼
ReleaseMigrationLock()
    │
    ▼
Continue with table sync (additive changes)
```

## Infrastructure Tables

The system creates two global tables (not company-scoped):

### `_migrations`
Tracks which migrations have been applied:

| Column | Type | Description |
|--------|------|-------------|
| version | INTEGER | Migration version (primary key) |
| name | TEXT | Migration name |
| description | TEXT | Human-readable description |
| applied_at | TIMESTAMP | When the migration ran |
| duration_ms | INTEGER | How long it took |
| status | TEXT | "completed" or "failed" |
| error_message | TEXT | Error details if failed |

### `_migration_lock`
Distributed lock for Kubernetes safety (single-row table):

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Always 1 (enforced by constraint) |
| locked_by | TEXT | Pod/hostname holding the lock |
| locked_at | TIMESTAMP | When lock was acquired |
| expires_at | TIMESTAMP | Lock expiry (TTL: 5 minutes) |

## Creating a Migration

### 1. Create a new file

Create `backend/business-logic/migrations/NNN_description.go`:

```go
package migrations

import (
    fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
    Register(&Migration002RenameCustomerName{})
}

type Migration002RenameCustomerName struct{}

func (m *Migration002RenameCustomerName) Version() int {
    return 2
}

func (m *Migration002RenameCustomerName) Name() string {
    return "rename_customer_name_to_full_name"
}

func (m *Migration002RenameCustomerName) Description() string {
    return "Rename Customer.name column to full_name"
}

func (m *Migration002RenameCustomerName) Up(ctx *fmigrations.Context) error {
    return ctx.ForEachCompanyTable("Customer", func(table string) error {
        return ctx.RenameColumn(table, "name", "full_name")
    })
}
```

### 2. Version numbering

- Versions must be sequential: 1, 2, 3...
- Never reuse or skip version numbers
- The baseline migration (001) is already applied

### 3. Registration

Migrations are auto-registered via `init()` functions. No manual registration needed.

## Available Helper Functions

The migration context (`ctx`) provides these helpers:

### Column Operations

```go
// Rename a column
ctx.RenameColumn(tableName, oldName, newName)

// Drop a column
ctx.DropColumn(tableName, columnName)

// Add a column
ctx.AddColumn(tableName, columnName, definition)

// Add column only if it doesn't exist
ctx.AddColumnIfNotExists(tableName, columnName, definition)

// Check if column exists
exists, err := ctx.ColumnExists(tableName, columnName)

// Change column type (PostgreSQL alters in place; SQLite rebuilds the table).
// Note: on SQLite, CHECK and table-level constraints are not preserved — use
// RecreateTable with an explicit schema when the table relies on those.
ctx.ChangeColumnType(tableName, columnName, newType)
```

### Table Operations

```go
// Check if table exists
exists, err := ctx.TableExists(tableName)

// SQLite-safe table rebuild (create new / copy / drop / rename). Use for structural
// changes SQLite's ALTER TABLE can't do in place. newSchema is the full column
// definition list; copyColumns are the columns carried over from the old table.
// Recreate any needed indexes with CreateIndex afterwards.
ctx.RecreateTable(tableName, "id INTEGER PRIMARY KEY, name TEXT", []string{"id", "name"})

// Execute across all companies
ctx.ForEachCompanyTable("Customer", func(fullTableName string) error {
    // fullTableName = "CompanyA$Customer", "CompanyB$Customer", etc.
    return ctx.RenameColumn(fullTableName, "old", "new")
})
```

### Index Operations

```go
// Create an index
ctx.CreateIndex(indexName, tableName, "column1", "column2")

// Drop an index
ctx.DropIndex(indexName)
```

### Data Operations

```go
// Update data
ctx.UpdateData(tableName, "status = 'active'", "status = 'pending'")

// Delete data
ctx.DeleteData(tableName, "created_at < '2024-01-01'")

// Copy data between columns (useful before dropping)
ctx.CopyColumnData(tableName, sourceColumn, targetColumn)

// Set default for NULL values
ctx.SetColumnDefault(tableName, columnName, defaultValue)
```

### Raw SQL

```go
// Execute any SQL
ctx.ExecuteSQL("ALTER TABLE ... ")

// Query (returns *sql.Rows)
rows, err := ctx.QuerySQL("SELECT ...")
```

## Kubernetes Behavior

### Multiple pods starting simultaneously

1. First pod acquires the lock
2. Other pods wait (retry every 2 seconds, max 30 attempts)
3. First pod completes migrations and releases lock
4. Other pods see migrations already applied, skip them

### Pod crashes during migration

1. Lock has a TTL of 5 minutes
2. If pod crashes, lock expires automatically
3. Another pod can acquire the lock
4. Failed migration is rolled back (transaction)

### Lock identification

Each pod identifies itself by hostname (or `HOSTNAME` env var in Kubernetes).

## Best Practices

1. **Test migrations locally first** - Run against a copy of production data

2. **Keep migrations small** - One logical change per migration

3. **Never modify applied migrations** - Create a new migration instead

4. **Handle both directions** - Consider what happens if migration fails mid-way

5. **Use ForEachCompanyTable** - Always apply changes across all companies

6. **Backup before destructive changes** - Especially for column drops

## Example: Complex Migration

```go
func (m *Migration003MergeAddressFields) Up(ctx *fmigrations.Context) error {
    return ctx.ForEachCompanyTable("Customer", func(table string) error {
        // 1. Add new column
        if err := ctx.AddColumnIfNotExists(table, "full_address", "TEXT"); err != nil {
            return err
        }

        // 2. Migrate data
        if err := ctx.ExecuteSQL(fmt.Sprintf(`
            UPDATE "%s"
            SET full_address = address || ', ' || city || ' ' || postal_code
            WHERE full_address IS NULL
        `, table)); err != nil {
            return err
        }

        // 3. Drop old columns
        for _, col := range []string{"address", "city", "postal_code"} {
            if err := ctx.DropColumn(table, col); err != nil {
                return err
            }
        }

        return nil
    })
}
```

## File Structure

```
backend/
├── foundation/migrations/
│   ├── interface.go    # Migration interface, Context, Record
│   ├── schema.go       # Infrastructure table creation
│   ├── runner.go       # Migration runner with locking
│   └── helpers.go      # Helper functions
│
└── business-logic/migrations/
    ├── registry.go     # Migration registry
    ├── 001_initial.go  # Baseline migration
    └── 002_xxx.go      # Your migrations here
```
