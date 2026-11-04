package migrations

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// Migration represents a database migration
type Migration interface {
	// Version returns the sequential version number (1, 2, 3...)
	Version() int

	// Name returns the migration name (e.g., "rename_customer_name")
	Name() string

	// Description returns a human-readable description
	Description() string

	// Up applies the migration
	Up(ctx *Context) error
}

// Context provides context for migration execution
type Context struct {
	DB        *sql.DB
	DBType    database.DBType
	Companies []string // All company names in the database
	tx        *sql.Tx  // Current transaction (set by runner)
}

// Record represents a migration record in the _migrations table
type Record struct {
	Version      int
	Name         string
	Description  string
	AppliedAt    time.Time
	DurationMs   int64
	Status       string // "completed", "failed"
	ErrorMessage string
}

// Tx returns the current transaction
func (ctx *Context) Tx() *sql.Tx {
	return ctx.tx
}

// SetTx sets the transaction (called by runner)
func (ctx *Context) SetTx(tx *sql.Tx) {
	ctx.tx = tx
}

// ForEachCompanyTable executes a function for each company's table
// tableName should be the base name (e.g., "Customer"), not the full name
func (ctx *Context) ForEachCompanyTable(tableName string, fn func(fullTableName string) error) error {
	for _, company := range ctx.Companies {
		fullTableName := fmt.Sprintf("%s$%s", company, tableName)
		if err := fn(fullTableName); err != nil {
			return fmt.Errorf("failed on table %s: %w", fullTableName, err)
		}
	}
	return nil
}

// ExecuteSQL executes a SQL statement in the current transaction
func (ctx *Context) ExecuteSQL(query string, args ...interface{}) error {
	_, err := ctx.tx.Exec(query, args...)
	return err
}

// QuerySQL executes a query and returns rows
func (ctx *Context) QuerySQL(query string, args ...interface{}) (*sql.Rows, error) {
	return ctx.tx.Query(query, args...)
}
