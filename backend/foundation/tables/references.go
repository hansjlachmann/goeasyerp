package tables

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
)

// Reference is a field of a table that refers to the key of the record being deleted
// (generated from the table relations in the definitions).
type Reference struct {
	Table      string
	Global     bool   // the referring table has no company prefix
	Column     string // the referring field
	CondColumn string // a conditional relation: only rows whose CondColumn = CondValue
	CondValue  int
}

// DeletedRecord is the record a delete is about to remove.
type DeletedRecord struct {
	Table     string
	Global    bool
	Company   string
	KeyColumn string
	Key       any
	KeyText   string
}

// RefuseDeleteWhenUsed returns ERR_DELETE_IN_USE when a record still refers to rec: table
// YAML refuse_delete_when_used (the generated Delete calls it after OnDelete, so what OnDelete
// deleted or moved no longer counts). The references of a global table are looked for in
// every company. A global table refers to a company table's key without saying which company
// it means, so such a reference does not count. A blank key is no record anyone refers to:
// a blank field means no value.
func RefuseDeleteWhenUsed(db database.Executor, dbType database.DBType, rec DeletedRecord, references []Reference) error {
	if rec.KeyText == "" {
		return nil
	}
	companies := []string{rec.Company}
	if rec.Global {
		var err error
		companies, err = companyNames(db)
		if err != nil {
			return err
		}
	}
	for _, ref := range references {
		if ref.Global && !rec.Global {
			continue
		}
		tables := []string{ref.Table}
		if !ref.Global {
			tables = tables[:0]
			for _, company := range companies {
				tables = append(tables, company+"$"+ref.Table)
			}
		}
		for _, table := range tables {
			used, err := referenceExists(db, dbType, table, ref, rec)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", table, ref.Column, err)
			}
			if used {
				return apperrors.DeleteInUse(rec.Table, rec.KeyText, ref.Table, ref.Column)
			}
		}
	}
	return nil
}

func companyNames(db database.Executor) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM "Company"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var companies []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		companies = append(companies, name)
	}
	return companies, rows.Err()
}

// referenceExists: a company entered before a table was added does not have it yet — asked
// first, as a failed query would end a PostgreSQL transaction.
func referenceExists(db database.Executor, dbType database.DBType, table string, ref Reference, rec DeletedRecord) (bool, error) {
	exists, err := physicalTableExists(db, dbType, table)
	if err != nil || !exists {
		return false, err
	}
	where := []string{quoteColumn(ref.Column) + " = ?"}
	args := []any{rec.Key}
	if ref.CondColumn != "" {
		where = append(where, quoteColumn(ref.CondColumn)+" = ?")
		args = append(args, ref.CondValue)
	}
	if strings.EqualFold(ref.Table, rec.Table) {
		// The table refers to itself: the record referring to itself does not count
		where = append(where, quoteColumn(rec.KeyColumn)+" <> ?")
		args = append(args, rec.Key)
	}
	query := fmt.Sprintf(`SELECT 1 FROM "%s" WHERE %s LIMIT 1`, table, strings.Join(where, " AND "))
	if dbType == database.DBTypePostgres {
		for i := range args {
			query = strings.Replace(query, "?", fmt.Sprintf("$%d", i+1), 1)
		}
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}

func physicalTableExists(db database.Executor, dbType database.DBType, name string) (bool, error) {
	var found int
	var err error
	if dbType == database.DBTypePostgres {
		err = db.QueryRow(`SELECT COUNT(*) FROM pg_class WHERE relname = $1 AND relkind = 'r'`, name).Scan(&found)
	} else {
		err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	}
	return found > 0, err
}

func quoteColumn(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
