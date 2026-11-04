// Package sift maintains SIFT totals tables (SumIndexFields).
//
// A table key can declare sum_index_fields in its YAML definition. For each such key the
// database keeps a totals table — one row per distinct key value combination with the
// summed fields and a row count — up to date with triggers that run in the same
// statement as the change to the entry table, so totals and entries can never disagree.
// FlowFields read the totals instead of adding up every entry.
//
// The generated table code supplies the SQL for each key (Key); this package names the
// objects, creates them, fills them, and rebuilds them when the key's definition changes.
package sift

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// maxIdentifier is PostgreSQL's identifier limit (NAMEDATALEN - 1) in bytes. Longer names
// are silently truncated by Postgres, which could make two objects collide.
const maxIdentifier = 63

// ObjectName returns name if it fits a database identifier, otherwise its first bytes
// plus a hash of the whole name, so long company/table/key names stay unique.
func ObjectName(name string) string {
	if len(name) <= maxIdentifier {
		return name
	}
	sum := sha1.Sum([]byte(name))
	prefix := name
	for len(prefix) > maxIdentifier-9 { // 54 bytes + "_" + 8 hex characters = 63
		_, size := lastRune(prefix)
		prefix = prefix[:len(prefix)-size]
	}
	return prefix + "_" + hex.EncodeToString(sum[:])[:8]
}

func lastRune(s string) (rune, int) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i]&0xC0 != 0x80 { // start byte of a UTF-8 sequence
			return rune(s[i]), len(s) - i
		}
	}
	return 0, 1
}

// TableName is the totals table of a SIFT key: "company$Table$SIFT$key" (no company
// prefix for global tables, whose company is "").
func TableName(company, table, key string) string {
	base := table + "$SIFT$" + key
	if company != "" {
		base = company + "$" + base
	}
	return ObjectName(base)
}

// Key is one SIFT key of a table, as generated from its YAML definition.
type Key struct {
	Name string
	// Fingerprint identifies the definition (key fields, sum fields, generator version):
	// when it differs from the stored one the totals are rebuilt.
	Fingerprint string
	// Create creates the totals table, its triggers and fills it from the entries, in order.
	Create []string
	// Drop removes everything Create made (stored, so a key removed from the YAML can
	// still be dropped later).
	Drop []string
}

const definitionTable = "_sift_definition"

// storageOptions are the Postgres storage parameters of every totals table. Each posting
// updates a totals row; only the sum columns and cnt change (not indexed), so with free
// space on the page (fillfactor) the update is HOT: the new row version stays on the page,
// the index is not touched and the old version is pruned on the next page access. The
// autovacuum settings suit small tables with very many updates (the default waits for 20 %
// of the table to be dead): vacuum and analyze after a fixed number of changed rows.
const storageOptions = "fillfactor = 50, autovacuum_vacuum_scale_factor = 0, autovacuum_vacuum_threshold = 1000, " +
	"autovacuum_analyze_scale_factor = 0, autovacuum_analyze_threshold = 1000"

// Sync makes the database match keys for one table of one company: keys that are new or
// whose fingerprint changed are (re)built, keys no longer defined are dropped, unchanged
// keys cost one small query. Each rebuild runs in its own transaction; on Postgres an
// advisory lock makes concurrent startups (several pods) build a key only once.
func Sync(db database.Executor, dbType database.DBType, company, table string, keys []Key) error {
	if err := ensureDefinitionTable(db); err != nil {
		return err
	}
	stored, err := storedDefinitions(db, dbType, company, table)
	if err != nil {
		return err
	}

	for _, key := range keys {
		if def, ok := stored[key.Name]; ok && def.fingerprint == key.Fingerprint {
			// Unchanged — unless its totals table is gone (company deleted and created
			// again, manual drop, partial restore): then rebuild
			name := TableName(company, table, key.Name)
			exists, err := tableExists(db, dbType, name)
			if err != nil {
				return err
			}
			if exists {
				if err := ensureStorageOptions(db, dbType, name); err != nil {
					return fmt.Errorf("SIFT %s.%s: %w", table, key.Name, err)
				}
				delete(stored, key.Name)
				continue
			}
		}
		previous := stored[key.Name]
		delete(stored, key.Name)
		start := time.Now()
		if err := rebuild(db, dbType, company, table, key, previous.drop); err != nil {
			return fmt.Errorf("SIFT %s.%s: %w", table, key.Name, err)
		}
		log.Printf("SIFT: built totals of %s$%s key %s in %v", company, table, key.Name, time.Since(start).Round(time.Millisecond))
	}

	// Keys removed from the definition
	for name, def := range stored {
		if err := inTx(db, func(tx database.Executor) error {
			if err := runAll(tx, def.drop); err != nil {
				return err
			}
			_, err := tx.Exec(placeholders(dbType, `DELETE FROM "`+definitionTable+`" WHERE company = ? AND table_name = ? AND key_name = ?`), company, table, name)
			return err
		}); err != nil {
			return fmt.Errorf("SIFT %s.%s: drop: %w", table, name, err)
		}
	}
	return nil
}

// DropCompany removes all SIFT objects and definitions of a company (company deleted or
// renamed). db may be a transaction.
func DropCompany(db database.Executor, dbType database.DBType, company string) error {
	if err := ensureDefinitionTable(db); err != nil {
		return err
	}
	rows, err := db.Query(placeholders(dbType, `SELECT drop_sql FROM "`+definitionTable+`" WHERE company = ?`), company)
	if err != nil {
		return err
	}
	var drops [][]string
	for rows.Next() {
		var dropSQL string
		if err := rows.Scan(&dropSQL); err != nil {
			_ = rows.Close()
			return err
		}
		drops = append(drops, splitStatements(dropSQL))
	}
	_ = rows.Close()
	// Inside a transaction a failed statement aborts the whole transaction on Postgres:
	// isolate each one in a savepoint (company rename/delete run in a transaction)
	_, isDB := db.(*sql.DB)
	savepoints := dbType == database.DBTypePostgres && !isDB
	for _, d := range drops {
		// The entry table may already be gone; DROP TRIGGER ... ON a missing table fails
		// on Postgres, so drop statement by statement and ignore those errors
		for _, stmt := range d {
			if !savepoints {
				_, _ = db.Exec(stmt)
				continue
			}
			if _, err := db.Exec(`SAVEPOINT sift_drop`); err != nil {
				return err
			}
			if _, err := db.Exec(stmt); err != nil {
				if _, err := db.Exec(`ROLLBACK TO SAVEPOINT sift_drop`); err != nil {
					return err
				}
			}
			if _, err := db.Exec(`RELEASE SAVEPOINT sift_drop`); err != nil {
				return err
			}
		}
	}
	_, err = db.Exec(placeholders(dbType, `DELETE FROM "`+definitionTable+`" WHERE company = ?`), company)
	return err
}

// DropAll removes the SIFT objects and definitions of every company (and of the global tables):
// a change to the entry tables' columns their triggers would block; the next Sync builds them
// again. db may be a transaction.
func DropAll(db database.Executor, dbType database.DBType) error {
	if err := ensureDefinitionTable(db); err != nil {
		return err
	}
	rows, err := db.Query(`SELECT DISTINCT company FROM "` + definitionTable + `"`)
	if err != nil {
		return err
	}
	var companies []string
	for rows.Next() {
		var company string
		if err := rows.Scan(&company); err != nil {
			_ = rows.Close()
			return err
		}
		companies = append(companies, company)
	}
	_ = rows.Close()
	for _, company := range companies {
		if err := DropCompany(db, dbType, company); err != nil {
			return err
		}
	}
	return nil
}

// Suspend drops the SIFT totals and triggers of one table of a company (and forgets their
// definitions), so a bulk load inserts its entries without a totals update per row — which
// is slow and leaves a dead row version per entry on Postgres. Afterwards the table's
// SyncKeys builds the totals again in one pass. Use both in the load's transaction, so a
// failed load restores the totals with everything else.
func Suspend(db database.Executor, dbType database.DBType, company, table string) error {
	if err := ensureDefinitionTable(db); err != nil {
		return err
	}
	stored, err := storedDefinitions(db, dbType, company, table)
	if err != nil {
		return err
	}
	for name, def := range stored {
		if err := runAll(db, def.drop); err != nil {
			return fmt.Errorf("SIFT %s.%s: suspend: %w", table, name, err)
		}
	}
	_, err = db.Exec(placeholders(dbType, `DELETE FROM "`+definitionTable+`" WHERE company = ? AND table_name = ?`), company, table)
	return err
}

// ensureStorageOptions gives a totals table created before storageOptions existed its
// settings (Postgres). Only pages written from now on keep the free space; a rebuild
// (Verify SIFT REPAIR) rewrites the whole table.
func ensureStorageOptions(db database.Executor, dbType database.DBType, name string) error {
	if dbType != database.DBTypePostgres {
		return nil
	}
	var options sql.NullString
	if err := db.QueryRow(`SELECT array_to_string(reloptions, ',') FROM pg_class WHERE relname = $1 AND relkind = 'r'`, name).Scan(&options); err != nil {
		return err
	}
	if strings.Contains(options.String, "fillfactor=50") {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE "` + strings.ReplaceAll(name, `"`, `""`) + `" SET (` + storageOptions + `)`)
	return err
}

// vacuum runs VACUUM ANALYZE on a totals table after a bulk change (Postgres). VACUUM cannot
// run inside a transaction: within one (db is not a *sql.DB) autovacuum is left to do it.
func vacuum(db database.Executor, dbType database.DBType, name string) error {
	if _, ok := db.(*sql.DB); !ok || dbType != database.DBTypePostgres {
		return nil
	}
	_, err := db.Exec(`VACUUM ANALYZE "` + strings.ReplaceAll(name, `"`, `""`) + `"`)
	return err
}

func tableExists(db database.Executor, dbType database.DBType, name string) (bool, error) {
	var found int
	var err error
	if dbType == database.DBTypePostgres {
		err = db.QueryRow(`SELECT COUNT(*) FROM pg_class WHERE relname = $1 AND relkind = 'r'`, name).Scan(&found)
	} else {
		err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	}
	return found > 0, err
}

type storedDefinition struct {
	fingerprint string
	drop        []string
}

func ensureDefinitionTable(db database.Executor) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS "` + definitionTable + `" (
		company VARCHAR(100) NOT NULL,
		table_name VARCHAR(100) NOT NULL,
		key_name VARCHAR(100) NOT NULL,
		fingerprint VARCHAR(64) NOT NULL,
		drop_sql TEXT NOT NULL,
		PRIMARY KEY (company, table_name, key_name)
	)`)
	return err
}

func storedDefinitions(db database.Executor, dbType database.DBType, company, table string) (map[string]storedDefinition, error) {
	rows, err := db.Query(placeholders(dbType, `SELECT key_name, fingerprint, drop_sql FROM "`+definitionTable+`" WHERE company = ? AND table_name = ?`), company, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]storedDefinition{}
	for rows.Next() {
		var name, fingerprint, dropSQL string
		if err := rows.Scan(&name, &fingerprint, &dropSQL); err != nil {
			return nil, err
		}
		out[name] = storedDefinition{fingerprint: fingerprint, drop: splitStatements(dropSQL)}
	}
	return out, rows.Err()
}

// rebuild drops the key's old objects, creates and fills the new ones and stores the
// fingerprint, all in one transaction. Creating the triggers locks the entry table
// against writes until commit, so no entry can be missed between fill and trigger.
func rebuild(db database.Executor, dbType database.DBType, company, table string, key Key, previousDrop []string) error {
	return inTx(db, func(tx database.Executor) error {
		if dbType == database.DBTypePostgres {
			// Serialize with other instances starting at the same time, then re-check
			if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, company+"$"+table+"$SIFT$"+key.Name); err != nil {
				return err
			}
			var fingerprint string
			err := tx.QueryRow(`SELECT fingerprint FROM "`+definitionTable+`" WHERE company = $1 AND table_name = $2 AND key_name = $3`, company, table, key.Name).Scan(&fingerprint)
			if err == nil && fingerprint == key.Fingerprint {
				if exists, err := tableExists(tx, dbType, TableName(company, table, key.Name)); err == nil && exists {
					return nil // another instance built it meanwhile
				}
			}
		}
		if err := runAll(tx, previousDrop); err != nil {
			return err
		}
		if err := runAll(tx, key.Drop); err != nil { // leftovers of a failed earlier attempt
			return err
		}
		if err := runAll(tx, key.Create); err != nil {
			return err
		}
		if _, err := tx.Exec(placeholders(dbType, `DELETE FROM "`+definitionTable+`" WHERE company = ? AND table_name = ? AND key_name = ?`), company, table, key.Name); err != nil {
			return err
		}
		_, err := tx.Exec(placeholders(dbType, `INSERT INTO "`+definitionTable+`" (company, table_name, key_name, fingerprint, drop_sql) VALUES (?, ?, ?, ?, ?)`),
			company, table, key.Name, key.Fingerprint, joinStatements(key.Drop))
		return err
	})
}

// inTx runs fn in a transaction when db is a *sql.DB; inside an existing transaction
// it runs fn directly.
func inTx(db database.Executor, fn func(database.Executor) error) error {
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return fn(db)
	}
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func runAll(db database.Executor, statements []string) error {
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%w (in: %s)", err, firstLine(stmt))
		}
	}
	return nil
}

// Statements are stored separated by a line that no generated statement contains.
const statementSeparator = "\n--;--\n"

func joinStatements(statements []string) string {
	return strings.Join(statements, statementSeparator)
}

func splitStatements(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, statementSeparator)
}

func placeholders(dbType database.DBType, query string) string {
	if dbType != database.DBTypePostgres {
		return query
	}
	n := 0
	var b strings.Builder
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func firstLine(s string) string {
	if line, _, more := strings.Cut(strings.TrimSpace(s), "\n"); more {
		return line + " ..."
	}
	return strings.TrimSpace(s)
}
