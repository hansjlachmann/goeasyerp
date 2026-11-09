package pages

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 304 shows per entry type the latest entry (by posting date, then entry no.) over all
// locations, and the order lines with the earliest and latest dates; the Location Filter
// limits both
func TestItemEntryStatistics(t *testing.T) {
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
	day := func(d int) types.Date { return types.NewDate(2026, 10, d) }
	for i, e := range []struct {
		entryType gtables.ItemLedgerEntryEntry_type
		date      types.Date
		location  types.Code
		doc       types.Code
		qty       int64
		invoiced  int64
	}{
		{gtables.ItemLedgerEntry_Entry_type.Sale, day(5), "MAIN", "S1", -2, -2},
		{gtables.ItemLedgerEntry_Entry_type.Sale, day(9), "OSLO", "S2", -1, 0}, // shipped, not invoiced
		{gtables.ItemLedgerEntry_Entry_type.Sale, day(3), "MAIN", "S3", -4, -4},
		{gtables.ItemLedgerEntry_Entry_type.Purchase, day(1), "MAIN", "P1", 10, 10},
	} {
		entry := gtables.NewItemLedgerEntry(src)
		entry.Entry_no, entry.Item_no, entry.Entry_type, entry.Posting_date = i+1, "I1", e.entryType, e.date
		entry.Location_code, entry.Document_no = e.location, e.doc
		entry.Quantity, entry.Invoiced_quantity = types.NewDecimalFromInt(e.qty), types.NewDecimalFromInt(e.invoiced)
		if err := entry.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	for i, l := range []struct {
		doc  types.Code
		date types.Date
	}{{"SO2", day(20)}, {"SO1", day(12)}, {"SO3", day(28)}} {
		line := gtables.NewSalesLine(src)
		line.Document_type, line.Document_no, line.Line_no = gtables.SalesLine_Document_type.Order, l.doc, 10000*(i+1)
		line.Type, line.No, line.Location_code, line.Shipment_date = gtables.SalesLine_Type.Item, "I1", "MAIN", l.date
		line.Quantity, line.Outstanding_quantity = types.NewDecimalFromInt(1), types.NewDecimalFromInt(1)
		if err := line.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	values := func() map[string]interface{} {
		t.Helper()
		control, err := pages.RunAfterGetRecordControl(304, item)
		if err != nil {
			t.Fatal(err)
		}
		return control.Values()
	}
	v := values()
	if fmt.Sprint(v["sale_document_no"]) != "S1" || fmt.Sprint(v["sale_quantity"]) != "2" ||
		fmt.Sprint(v["shipment_document_no"]) != "S2" || fmt.Sprint(v["purchase_document_no"]) != "P1" ||
		fmt.Sprint(v["next_shipment_order_no"]) != "SO1" || fmt.Sprint(v["last_shipment_order_no"]) != "SO3" {
		t.Errorf("all locations: %v", v)
	}
	item.Location_filter = "MAIN"
	v = values()
	if fmt.Sprint(v["shipment_document_no"]) != "S1" || fmt.Sprint(v["sale_document_no"]) != "S1" {
		t.Errorf("location MAIN: %v", v)
	}
}
