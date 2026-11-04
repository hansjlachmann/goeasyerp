package migrations

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration007DropCompanyIndexesOnGlobalTables{})
}

// Migration007DropCompanyIndexesOnGlobalTables drops the per-company copies of the indexes
// of global tables. Table sync named every index "company$Table$key", also for global
// tables (User, Company, User Member, …), so each company that was initialized added its
// own copy of each of their indexes (and a deleted or renamed company left its copies).
// Global table indexes are now named "Table$key" and created once by the startup sync.
type Migration007DropCompanyIndexesOnGlobalTables struct{}

func (m *Migration007DropCompanyIndexesOnGlobalTables) Version() int {
	return 7
}

func (m *Migration007DropCompanyIndexesOnGlobalTables) Name() string {
	return "drop_company_indexes_on_global_tables"
}

func (m *Migration007DropCompanyIndexesOnGlobalTables) Description() string {
	return "Drop the per-company copies (company$Table$key) of the indexes of global tables"
}

func (m *Migration007DropCompanyIndexesOnGlobalTables) Up(ctx *fmigrations.Context) error {
	query := `SELECT tbl_name, name FROM sqlite_master WHERE type = 'index'`
	if ctx.DBType == database.DBTypePostgres {
		query = `SELECT tablename, indexname FROM pg_indexes WHERE schemaname = current_schema()`
	}
	rows, err := ctx.QuerySQL(query)
	if err != nil {
		return err
	}
	var drop []string
	for rows.Next() {
		var table, index string
		err = rows.Scan(&table, &index)
		if err != nil {
			_ = rows.Close()
			return err
		}
		// A global table has no "$" in its name; its company copies are "<company>$<table>$<key>"
		if !strings.Contains(table, "$") && isCompanyCopy(index, table) {
			drop = append(drop, index)
		}
	}
	err = rows.Err()
	if err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	for _, index := range drop {
		err := ctx.DropIndex(index)
		if err != nil {
			return fmt.Errorf("drop index %s: %w", index, err)
		}
	}
	return nil
}

// isCompanyCopy reports whether index is named "<company>$<table>$<key>" (non-empty company).
func isCompanyCopy(index, table string) bool {
	i := strings.Index(index, "$"+table+"$")
	return i > 0 && !strings.HasSuffix(index, "$"+table+"$")
}
