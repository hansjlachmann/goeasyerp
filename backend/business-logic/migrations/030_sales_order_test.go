package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// Lines entered before get their quantities to ship and invoice from the outstanding quantity;
// running it again changes nothing; a company without sales lines is skipped
func TestSalesOrderMigration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE "c1$Sales_Line" (document_no TEXT, line_no INTEGER, outstanding_quantity TEXT, outstanding_qty_base TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO "c1$Sales_Line" VALUES ('SQ1', 10000, '3', '30')`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"c1", "c2"}}
		ctx.SetTx(tx)
		if err := (&Migration030SalesOrder{}).Up(ctx); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			_, err = db.Exec(`UPDATE "c1$Sales_Line" SET qty_to_ship = '1'`)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var toShip, toShipBase, toInvoice, toInvoiceBase string
	err = db.QueryRow(`SELECT qty_to_ship, qty_to_ship_base, qty_to_invoice, qty_to_invoice_base FROM "c1$Sales_Line"`).Scan(&toShip, &toShipBase, &toInvoice, &toInvoiceBase)
	if err != nil {
		t.Fatal(err)
	}
	if toShip != "1" || toShipBase != "30" || toInvoice != "3" || toInvoiceBase != "30" {
		t.Errorf("to ship %s/%s, to invoice %s/%s", toShip, toShipBase, toInvoice, toInvoiceBase)
	}
}
