package pages

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Item Availability's Expected Inventory and Available Inventory: 10 on hand, a purchase order of
// 5 in October and 7 in December, sales orders of 3 in October and 4 in December — for October
// expected 10 + 5 − 3 = 12, available 10 − 7 + 5 = 8 (every sales order, the purchases up to the
// period's end); the Location Filter limits them
func TestItemAvailabilityExpectedInventory(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Item{}, &gtables.ItemLedgerEntry{}, &gtables.SalesLine{}, &gtables.PurchaseLine{}} {
		if err := tbl.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	item := gtables.NewItem(src)
	item.No = "I1"
	if err := item.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	entry := gtables.NewItemLedgerEntry(src)
	entry.Entry_no, entry.Item_no, entry.Posting_date, entry.Location_code = 1, "I1", types.NewDate(2026, 9, 1), "MAIN"
	entry.Quantity = types.NewDecimalFromInt(10)
	if err := entry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	for i, l := range []struct {
		date     types.Date
		purchase bool
		qty      int64
	}{
		{types.NewDate(2026, 10, 10), true, 5},
		{types.NewDate(2026, 12, 10), true, 7},
		{types.NewDate(2026, 10, 12), false, 3},
		{types.NewDate(2026, 12, 12), false, 4},
	} {
		if l.purchase {
			line := gtables.NewPurchaseLine(src)
			line.Document_type, line.Document_no, line.Line_no = gtables.PurchaseLine_Document_type.Order, "P1", (i+1)*10000
			line.Type, line.No, line.Location_code, line.Expected_receipt_date = gtables.PurchaseLine_Type.Item, "I1", "MAIN", l.date
			line.Outstanding_qty_base = types.NewDecimalFromInt(l.qty)
			if err := line.InsertOrError(false); err != nil {
				t.Fatal(err)
			}
		} else {
			line := gtables.NewSalesLine(src)
			line.Document_type, line.Document_no, line.Line_no = gtables.SalesLine_Document_type.Order, "S1", (i+1)*10000
			line.Type, line.No, line.Location_code, line.Shipment_date = gtables.SalesLine_Type.Item, "I1", "MAIN", l.date
			line.Outstanding_qty_base = types.NewDecimalFromInt(l.qty)
			if err := line.InsertOrError(false); err != nil {
				t.Fatal(err)
			}
		}
	}

	values := func(location string) (string, string) {
		rec := gtables.NewItem(src)
		if !rec.Get(types.Code("I1")) {
			t.Fatal("item I1")
		}
		if err := rec.SetFlowFilter("date_filter", "2026-10-01..2026-10-31"); err != nil {
			t.Fatal(err)
		}
		if err := rec.SetFlowFilter("location_filter", location); err != nil {
			t.Fatal(err)
		}
		control, err := pages.RunAfterGetRecordControl(157, rec)
		if err != nil {
			t.Fatal(err)
		}
		v := control.Values()
		return fmtValue(v["expected_inventory"]), fmtValue(v["available_inventory"])
	}
	expected, available := values("")
	if expected != "12" || available != "8" {
		t.Errorf("October: expected %s (want 12), available %s (want 8)", expected, available)
	}
	expected, available = values("OSLO")
	if expected != "0" || available != "0" {
		t.Errorf("October at OSLO: expected %s, available %s, want 0 and 0", expected, available)
	}
}

func fmtValue(v interface{}) string {
	switch d := v.(type) {
	case types.Decimal:
		return d.String()
	case string:
		return d
	}
	return ""
}
