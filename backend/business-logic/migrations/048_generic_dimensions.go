package migrations

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
	"github.com/hansjlachmann/openerp/backend/foundation/sift"
)

func init() {
	Register(&Migration048GenericDimensions{})
}

// Migration048GenericDimensions gives the two posting dimensions generic names: table 11
// Department becomes Dimension_1 and table 12 Project becomes Dimension_2 (the name Project is
// free for the projects module). Per company the tables are renamed with their recorded field
// numbers; their index is built again by the table sync. The dimension fields of the other
// tables keep their numbers, so the table sync renames their columns (department_code →
// dimension_1_code …); the SIFT totals are dropped first — their triggers name the columns —
// and the sync builds them again. Existing companies keep their captions: General Ledger
// Setup Dimension-1 is "Department" and Dimension-2 "Project".
type Migration048GenericDimensions struct{}

func (m *Migration048GenericDimensions) Version() int {
	return 48
}

func (m *Migration048GenericDimensions) Name() string {
	return "generic_dimensions"
}

func (m *Migration048GenericDimensions) Description() string {
	return "Department and Project become Dimension 1 and Dimension 2, named in General Ledger Setup"
}

var genericDimensionTables = []struct{ old, new, label string }{
	{"Department", "Dimension_1", "Department"},
	{"Project", "Dimension_2", "Project"},
}

func (m *Migration048GenericDimensions) Up(ctx *fmigrations.Context) error {
	err := sift.DropAll(ctx.Tx(), ctx.DBType)
	if err != nil {
		return fmt.Errorf("drop SIFT totals: %w", err)
	}
	hasFieldNumbers, err := ctx.TableExists("_field_definition")
	if err != nil {
		return err
	}
	err = genericDimensionPermissions(ctx)
	if err != nil {
		return err
	}
	for _, company := range ctx.Companies {
		renamed := false
		for _, t := range genericDimensionTables {
			oldTable := company + "$" + t.old
			newTable := company + "$" + t.new
			oldExists, err := ctx.TableExists(oldTable)
			if err != nil {
				return err
			}
			newExists, err := ctx.TableExists(newTable)
			if err != nil {
				return err
			}
			if !oldExists || newExists {
				continue
			}
			err = ctx.ExecuteSQL(fmt.Sprintf(`DROP INDEX IF EXISTS "%s$Primary"`, oldTable))
			if err != nil {
				return err
			}
			err = ctx.ExecuteSQL(fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, oldTable, newTable))
			if err != nil {
				return fmt.Errorf("rename %s: %w", oldTable, err)
			}
			if ctx.DBType == database.DBTypePostgres {
				// the primary key keeps its name "company$Department_pkey": it would collide
				// with a new table of the old name
				var n int
				err = ctx.Tx().QueryRow(`SELECT COUNT(*) FROM pg_constraint WHERE conname = $1`, oldTable+"_pkey").Scan(&n)
				if err != nil {
					return err
				}
				if n > 0 {
					err = ctx.ExecuteSQL(fmt.Sprintf(`ALTER TABLE "%s" RENAME CONSTRAINT "%s_pkey" TO "%s_pkey"`, newTable, oldTable, newTable))
					if err != nil {
						return fmt.Errorf("rename the primary key of %s: %w", oldTable, err)
					}
				}
			}
			if hasFieldNumbers {
				err = ctx.ExecuteSQL(placeholders(ctx.DBType, `UPDATE "_field_definition" SET physical_table = ? WHERE physical_table = ?`), newTable, oldTable)
				if err != nil {
					return err
				}
			}
			renamed = true
		}
		if renamed {
			err = genericDimensionLabels(ctx, company)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// genericDimensionPermissions: the roles' permissions on the tables go with the new names
func genericDimensionPermissions(ctx *fmigrations.Context) error {
	exists, err := ctx.TableExists("Permission")
	if err != nil || !exists {
		return err
	}
	for _, t := range genericDimensionTables {
		oldName, newName := strings.ToUpper(t.old), strings.ToUpper(t.new)
		err = ctx.ExecuteSQL(placeholders(ctx.DBType, `UPDATE "Permission" SET table_name = ? WHERE table_name = ?
			AND NOT EXISTS (SELECT 1 FROM "Permission" p WHERE p.role_id = "Permission".role_id AND p.table_name = ?)`), newName, oldName, newName)
		if err != nil {
			return fmt.Errorf("permissions on %s: %w", t.old, err)
		}
	}
	return nil
}

// genericDimensionLabels: the company's General Ledger Setup names the dimensions as before
// (the columns come before the table sync adds them)
func genericDimensionLabels(ctx *fmigrations.Context, company string) error {
	table := company + "$General_Ledger_Setup"
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	for i, t := range genericDimensionTables {
		column := fmt.Sprintf("dimension_%d", i+1)
		err = ctx.AddColumnIfNotExists(table, column, "VARCHAR(30)")
		if err != nil {
			return err
		}
		err = ctx.ExecuteSQL(placeholders(ctx.DBType, fmt.Sprintf(`UPDATE "%s" SET %s = ? WHERE %s IS NULL OR %s = ''`, table, column, column, column)), t.label)
		if err != nil {
			return err
		}
	}
	return nil
}

// placeholders turns the ? of a statement into $1, $2 … on PostgreSQL
func placeholders(dbType database.DBType, stmt string) string {
	if dbType != database.DBTypePostgres {
		return stmt
	}
	out := make([]byte, 0, len(stmt)+8)
	n := 0
	for i := 0; i < len(stmt); i++ {
		if stmt[i] == '?' {
			n++
			out = append(out, fmt.Sprintf("$%d", n)...)
			continue
		}
		out = append(out, stmt[i])
	}
	return string(out)
}
