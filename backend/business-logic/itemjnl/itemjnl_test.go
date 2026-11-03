package itemjnl

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

const testCompany = "demo"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

func date(t *testing.T, day, month, year int) types.Date {
	t.Helper()
	d, err := system.DMY2Date(day, month, year)
	must(t, err)
	return d
}

func openPoster(src ftables.Source) demodata.ItemJnlPoster { return NewItemJnlPostLine(src) }

// demoCompany is a company with the demo data, the demo items' year of postings included
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, testCompany, database.DBTypeSQLite))
	_, err = demodata.Create(db, testCompany, database.DBTypeSQLite, demodata.Options{
		Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local), ItemJnlPostLine: openPoster})
	must(t, err)
	return ftables.DBSource(db, testCompany, database.DBTypeSQLite)
}

// glBalance is the balance of a G/L account
func glBalance(src ftables.Source, account string) types.Decimal {
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("gl_account_no", account)
	_ = glEntry.CalcSums("amount")
	return glEntry.Amount.Round(2)
}

// The demo items' year: opening stock, purchases and sales posted through codeunit 22 (and the
// office chairs with serial numbers: three in, one out) — every item's inventory adds up, the inventory value is in the G/L, the G/L balances
func TestDemoInventory(t *testing.T) {
	src := demoCompany(t)
	itemLedgEntry := gtables.NewItemLedgerEntry(src)
	if n := itemLedgEntry.Count(); n != 19*13+4 {
		t.Fatalf("%d item ledger entries, want %d", n, 19*13+4)
	}
	var invValue types.Decimal
	for found := itemLedgEntry.FindSet(); found; found = itemLedgEntry.Next() {
		invValue = invValue.Add(itemLedgEntry.Adjusted_cost_invoiced_qty)
	}
	if !glBalance(src, "1333").Add(glBalance(src, "1335")).Equal(invValue) {
		t.Errorf("inventory accounts %s + %s, item ledger value %s", glBalance(src, "1333"), glBalance(src, "1335"), invValue)
	}
	glEntry := gtables.NewGLEntry(src)
	_ = glEntry.CalcSums("amount")
	if !glEntry.Amount.Round(2).IsZero() { // SQLite adds the stored decimals up as floats
		t.Errorf("G/L entries add up to %s", glEntry.Amount)
	}
	item := gtables.NewItem(src)
	must(t, item.GetOrError(types.Code("1000")))
	item.CalcFields("quantity_on_hand", "net_invoiced_qty", "purchases_qty", "sales_qty")
	open := gtables.NewItemLedgerEntry(src)
	open.SetRange("item_no", "1000")
	_ = open.CalcSums("remaining_quantity")
	if item.Quantity_on_hand.IsZero() || !item.Quantity_on_hand.Equal(open.Remaining_quantity) || item.Sales_qty.IsNegative() {
		t.Errorf("item 1000: on hand %s, remaining %s, sales %s", item.Quantity_on_hand, open.Remaining_quantity, item.Sales_qty)
	}
}
