package tables

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// BeginTransaction starts a write transaction for business logic that must change several
// tables together (posting a journal): open the records on the returned transaction, then
// Commit (refused while a table is inconsistent) or Rollback. The executor must be a database,
// not a transaction already running.
func BeginTransaction(db database.Executor) (*sql.Tx, error) {
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("a transaction is already running")
	}
	return sqlDB.Begin()
}

// LockTable locks a table for the rest of the transaction (classic LOCKTABLE): on PostgreSQL
// a transaction-scoped advisory lock on the company's table, so a second transaction locking
// the same table waits until the first ends. Row locks (SELECT … FOR UPDATE) alone do not stop
// two postings from reading the same last entry and computing the same next Entry No. — rows
// inserted after the read are not locked. SQLite has one writer at a time anyway. Outside a
// transaction it does nothing.
func LockTable(db database.Executor, dbType database.DBType, company, table string) {
	if dbType != database.DBTypePostgres {
		return
	}
	if _, ok := db.(*sql.Tx); !ok {
		return
	}
	name := table
	if company != "" {
		name = company + "$" + table
	}
	_, err := db.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, name)
	if err != nil {
		log.Printf("LockTable %s: %v", name, err)
	}
}
