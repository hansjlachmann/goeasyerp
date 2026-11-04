package company

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/sift"
)

// A company's data lives in its own database objects: every company-scoped table is
// "company$Table", with indexes "company$Table$key", sequences and the SIFT totals tables,
// triggers and functions. Renaming a company renames all of them; deleting one drops them.

// NormalizeName returns a company name as stored (trimmed, lower case) or an error when it
// is not valid: 2–50 characters a-z, 0-9, _ and -. The name prefixes every table of the
// company, so it must stay a plain identifier.
func NormalizeName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return "", apperrors.CompanyInvalidName()
		}
	}
	if len(name) < 2 || len(name) > 50 {
		return "", apperrors.CompanyNameLength()
	}
	return name, nil
}

// object is a database object belonging to a company.
type object struct {
	name string
	kind string // "table", "index", "sequence", "view"
	// constraint: a Postgres index that backs a primary key/unique constraint (cannot be dropped)
	constraint bool
}

// objects lists the database objects whose name starts with "company$". The prefix is
// compared in Go: in a LIKE pattern "_" matches any character, so "a_b$%" would also find
// the objects of company "axb".
func objects(db database.Executor, dbType database.DBType, company string) ([]object, error) {
	prefix := company + "$"
	var rows *sql.Rows
	var err error
	if dbType == database.DBTypePostgres {
		rows, err = db.Query(`
			SELECT c.relname,
			       CASE c.relkind WHEN 'r' THEN 'table' WHEN 'p' THEN 'table' WHEN 'i' THEN 'index'
			                      WHEN 'I' THEN 'index' WHEN 'S' THEN 'sequence' ELSE 'view' END,
			       EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conindid = c.oid)
			FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = current_schema() AND c.relkind IN ('r', 'p', 'i', 'I', 'S', 'v', 'm')
			  AND left(c.relname, $1) = $2
			ORDER BY c.relname`, len([]rune(prefix)), prefix)
	} else {
		rows, err = db.Query(`
			SELECT name, type, 0 FROM sqlite_master
			WHERE type IN ('table', 'index', 'view') AND substr(name, 1, ?) = ?
			ORDER BY name`, len([]rune(prefix)), prefix)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []object
	for rows.Next() {
		var o object
		if err := rows.Scan(&o.name, &o.kind, &o.constraint); err != nil {
			return nil, err
		}
		if strings.HasPrefix(o.name, prefix) {
			out = append(out, o)
		}
	}
	return out, rows.Err()
}

// Tables returns the tables of a company ("company$…"), SIFT totals tables included.
func Tables(db database.Executor, dbType database.DBType, company string) ([]string, error) {
	objs, err := objects(db, dbType, company)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, o := range objs {
		if o.kind == "table" {
			out = append(out, o.name)
		}
	}
	return out, nil
}

// DropObjects drops every table of a company with its SIFT triggers, functions and
// definitions (the company is being deleted). Run it in the transaction that deletes the
// Company record, so a failure leaves the company intact.
func DropObjects(db database.Executor, dbType database.DBType, company string) error {
	if err := sift.DropCompany(db, dbType, company); err != nil {
		return fmt.Errorf("drop SIFT totals: %w", err)
	}
	tables, err := Tables(db, dbType, company)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	for _, t := range tables {
		if _, err := db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, quote(t))); err != nil {
			return fmt.Errorf("drop table %s: %w", t, err)
		}
	}
	return nil
}

// RenameObjects renames the database objects of company oldName to newName (Rename
// of a Company): the SIFT objects are dropped (their trigger functions name the totals
// tables, so they cannot simply be renamed), then every table, index and sequence
// "old$…" becomes "new$…". The caller rebuilds indexes and SIFT totals for newName
// afterwards (SyncKeys) and must run all of it, with the Company record update, in one
// transaction: tx may not be a *sql.DB.
func RenameObjects(tx database.Executor, dbType database.DBType, oldName, newName string) error {
	if _, ok := tx.(*sql.DB); ok {
		return fmt.Errorf("renaming company %s must run in a transaction", oldName)
	}
	if oldName == newName {
		return nil
	}
	// Leftovers named like the new company (e.g. of a deleted company whose objects were
	// not dropped) would collide with the renamed ones
	if existing, err := objects(tx, dbType, newName); err != nil {
		return err
	} else if len(existing) > 0 {
		return fmt.Errorf("database objects named %q already exist (e.g. %s)", newName+"$…", existing[0].name)
	}

	if err := sift.DropCompany(tx, dbType, oldName); err != nil {
		return fmt.Errorf("drop SIFT totals: %w", err)
	}
	objs, err := objects(tx, dbType, oldName)
	if err != nil {
		return err
	}
	for _, o := range objs {
		renamed := newName + strings.TrimPrefix(o.name, oldName)
		var stmt string
		switch {
		case o.kind == "table":
			stmt = fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, quote(o.name), quote(renamed))
		case dbType != database.DBTypePostgres:
			// SQLite cannot rename an index or view; indexes are created again by SyncKeys
			// for the new name, a view (none are generated) is dropped
			if o.kind == "index" && strings.HasPrefix(o.name, "sqlite_autoindex_") {
				continue
			}
			stmt = fmt.Sprintf(`DROP %s IF EXISTS %s`, strings.ToUpper(o.kind), quote(o.name))
		case o.kind == "index" && !o.constraint && len(o.name) >= 63:
			// Postgres cut the name to 63 bytes; the new name cut the same way would differ
			// from the one SyncKeys creates for the new company, so build the index again
			stmt = fmt.Sprintf(`DROP INDEX IF EXISTS %s`, quote(o.name))
		default:
			stmt = fmt.Sprintf(`ALTER %s %s RENAME TO %s`, strings.ToUpper(o.kind), quote(o.name), quote(renamed))
		}
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("rename %s %s: %w", o.kind, o.name, err)
		}
	}
	return nil
}

// quote returns a database identifier in double quotes.
func quote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
