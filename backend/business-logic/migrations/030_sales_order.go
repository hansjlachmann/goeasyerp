package migrations

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration030SalesOrder{})
}

// Migration030SalesOrder goes with the sales orders: Sales Line got the quantities to ship and
// invoice (17, 18, 5417, 5418). Lines entered before had none, so a quote made an order would
// ship nothing: before the table sync adds the other new columns, these are added and set to
// the outstanding quantity — as the line's Quantity sets them now (nothing is shipped yet).
// Idempotent: lines that have the columns already are left alone.
type Migration030SalesOrder struct{}

func (m *Migration030SalesOrder) Version() int {
	return 30
}

func (m *Migration030SalesOrder) Name() string {
	return "sales-order"
}

func (m *Migration030SalesOrder) Description() string {
	return "Sales Line quantities to ship and invoice from the outstanding quantity"
}

func (m *Migration030SalesOrder) Up(ctx *fmigrations.Context) error {
	columnType := "TEXT" // SQLite stores decimals as text
	if ctx.DBType == database.DBTypePostgres {
		columnType = "NUMERIC"
	}
	for _, company := range ctx.Companies {
		table := company + "$Sales_Line"
		exists, err := ctx.TableExists(table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		done, err := ctx.ColumnExists(table, "qty_to_ship")
		if err != nil {
			return err
		}
		if done {
			continue
		}
		for _, column := range []string{"qty_to_ship", "qty_to_ship_base", "qty_to_invoice", "qty_to_invoice_base"} {
			err = ctx.AddColumn(table, column, columnType)
			if err != nil {
				return err
			}
		}
		err = ctx.ExecuteSQL(fmt.Sprintf(`UPDATE "%s" SET qty_to_ship = outstanding_quantity, qty_to_ship_base = outstanding_qty_base, qty_to_invoice = outstanding_quantity, qty_to_invoice_base = outstanding_qty_base`, table))
		if err != nil {
			return err
		}
	}
	return nil
}
