package reservmgt_test

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

// demoCompany: the demo data with its posted item entries (the inventory), in a transaction;
// sales orders and the inventory reserve, the inventory first
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
	})
	must(t, err)
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	src := ftables.DBSource(tx, "demo", database.DBTypeSQLite)
	reservSetup := gtables.GetReservationSetup(src)
	reservSetup.Reserve_sales_order = true
	reservSetup.Show_item_ledger_entry = gtables.ReservationSetup_Show_item_ledger_entry.Always
	reservSetup.Item_ledger_entry_priority = 1
	reservSetup.Show_purchase_order = gtables.ReservationSetup_Show_purchase_order.Always
	reservSetup.Purchase_order_priority = 2
	must(t, reservSetup.ModifyOrError(false)) // the demo data's setup
	return src
}

// salesOrderLine: a sales order of the first demo customer with a line of item 1000 at MAIN
func salesOrderLine(t *testing.T, src ftables.Source, qty string) *gtables.SalesLine {
	t.Helper()
	cust := gtables.NewCustomer(src)
	if !cust.FindFirst() {
		t.Fatal("no demo customer")
	}
	header := gtables.NewSalesHeader(src)
	header.Document_type = gtables.SalesHeader_Document_type.Order
	must(t, header.InsertOrError(true))
	must(t, header.Validate("sell_to_customer_no", cust.No))
	must(t, header.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Order
	line.Document_no = header.No
	line.Line_no = 10000
	must(t, line.Validate("type", gtables.SalesLine_Type.Item))
	must(t, line.Validate("no", types.Code("1000")))
	must(t, line.Validate("location_code", types.Code("MAIN")))
	must(t, line.Validate("quantity", dec(qty)))
	must(t, line.Validate("shipment_date", types.NewDate(2026, 10, 20)))
	must(t, line.InsertOrError(true))
	return line
}

func reload(t *testing.T, line *gtables.SalesLine) {
	t.Helper()
	must(t, line.GetOrError(map[string]interface{}{"document_type": int(line.Document_type), "document_no": line.Document_no, "line_no": line.Line_no}))
}

// A sales order line reserves from the inventory: Auto Reserve takes the open item entries by
// the setup's priority, the line and the entries keep the reserved quantity; the summary shows
// it; the line keeps its item and location while reserved and is not deleted; canceling gives
// the inventory back
func TestSalesLineReservesInventory(t *testing.T) {
	src := demoCompany(t)
	line := salesOrderLine(t, src, "3")

	m := reservmgt.New(src)
	must(t, m.SalesLineSetReservFor(line))
	full, err := m.AutoReserve(line.Shipment_date, "", line.Shipment_date)
	must(t, err)
	if !full {
		t.Fatal("not fully reserved from the inventory")
	}
	reload(t, line)
	if !line.Reserved_qty_base.Equal(dec("3")) || !line.Reserved_quantity.Equal(dec("3")) || !line.Planned {
		t.Errorf("line reserved %s (%s), planned %v; want 3", line.Reserved_qty_base, line.Reserved_quantity, line.Planned)
	}
	entries := gtables.NewReservationEntry(src)
	reservmgt.FilterFor(entries, reservmgt.SalesLineSource(line))
	must(t, entries.CalcSums("quantity"))
	if !entries.Quantity.Equal(dec("3")) || !entries.FindFirst() || int(entries.From_type) != reservmgt.TypeItemLedgerEntry {
		t.Fatalf("entries %s from %v", entries.Quantity, entries.From_type)
	}
	ile := gtables.NewItemLedgerEntry(src)
	ile.SetRange("item_no", types.Code("1000"))
	must(t, ile.CalcSums("reserved_quantity"))
	if !ile.Reserved_quantity.Equal(dec("-3")) {
		t.Errorf("item entries reserved %s, want -3", ile.Reserved_quantity)
	}

	summary := gtables.NewEntrySummary(src)
	must(t, summary.InitTemporary(src.GetDB(), src.GetCompany(), src.GetDBType()))
	m2 := reservmgt.New(src)
	must(t, m2.SalesLineSetReservFor(line))
	qtyReserved, err := m2.UpdateStatistics(summary, false, line.Shipment_date, func(key string) string { return key })
	must(t, err)
	if !qtyReserved.Equal(dec("3")) || !summary.Get(reservmgt.SummaryItemLedgerEntry) || !summary.Current_reserved_quantity.Equal(dec("3")) || !summary.Total_reserved_quantity.Equal(dec("3")) {
		t.Errorf("summary: reserved %s, inventory line %+v", qtyReserved, summary.ToMap())
	}

	// changes that do not fit the reservation are refused
	must(t, line.Validate("location_code", types.Code("")))
	if err := line.ModifyOrError(true); err == nil || !strings.Contains(err.Error(), "must not be changed") {
		t.Errorf("location changed while reserved: %v", err)
	}
	reload(t, line)
	if err := line.DeleteOrError(true); err == nil {
		t.Error("a reserved line was deleted")
	}
	// a later shipment date goes to the entries
	reload(t, line)
	must(t, line.Validate("shipment_date", types.NewDate(2026, 10, 25)))
	must(t, line.ModifyOrError(true))
	if !entries.FindFirst() || !entries.Shipment_date.Equal(types.NewDate(2026, 10, 25)) {
		t.Errorf("entry shipment date %s", entries.Shipment_date)
	}

	// cancel: the line and the inventory are free again
	for found := entries.FindSet(); found; found = entries.Next() {
		must(t, m.CloseReservEntry(entries.Entry_no, gtables.ReservationEntry_Reservation_status.CanceledOrChanged, true, true))
	}
	reload(t, line)
	must(t, ile.CalcSums("reserved_quantity"))
	if !line.Reserved_qty_base.IsZero() || !ile.Reserved_quantity.IsZero() {
		t.Errorf("after canceling: line %s, item entries %s", line.Reserved_qty_base, ile.Reserved_quantity)
	}
}

// The Reservation page (498) lists the sources of a sales order line, the inventory with its
// reserved quantity, and shows the line in its footer; Available - Item Ledg. Entries (504)
// shows the open inbound entries of the item at the location with what is available and
// what the line reserved from each; Reservation Entries (497) the line's open entries
func TestReservationPages(t *testing.T) {
	src := demoCompany(t)
	line := salesOrderLine(t, src, "2")
	m := reservmgt.New(src)
	must(t, m.SalesLineSetReservFor(line))
	_, err := m.AutoReserve(line.Shipment_date, "", line.Shipment_date)
	must(t, err)
	params := reservmgt.Params(reservmgt.SalesLineSource(line))
	fcodeunits.SetCurrentPageParams(params)
	t.Cleanup(func() { fcodeunits.SetCurrentPageParams(nil) })

	summary := gtables.NewEntrySummary(src)
	must(t, summary.InitTemporary(src.GetDB(), src.GetCompany(), src.GetDBType()))
	must(t, pages.RunFindRecords(reservmgt.ReservationPageID, summary, params))
	if !summary.Get(reservmgt.SummaryItemLedgerEntry) || !summary.Current_reserved_quantity.Equal(dec("2")) ||
		summary.Available_page_id != reservmgt.AvailableItemLedgEntriesPageID {
		t.Fatalf("inventory line %+v", summary.ToMap())
	}
	control, err := pages.RunAfterGetRecordControl(reservmgt.ReservationPageID, summary)
	must(t, err)
	values := control.Values()
	if fmt.Sprint(values["item_no"]) != "1000" || fmt.Sprint(values["qty_reserved"]) != "2" || fmt.Sprint(values["unreserved_quantity"]) != "0" {
		t.Errorf("footer %v", values)
	}

	ile := gtables.NewItemLedgerEntry(src)
	params[reservmgt.ParamSummary] = strconv.Itoa(reservmgt.SummaryItemLedgerEntry)
	open, err := pages.RunOpenPage(reservmgt.AvailableItemLedgEntriesPageID, ile, params)
	must(t, err)
	applyFilters(t, ile, open.Filters())
	var reservedThisLine types.Decimal
	n := 0
	for found := ile.FindSet(); found; found = ile.Next() {
		n++
		if ile.Item_no != "1000" || ile.Location_code != "MAIN" || !ile.Remaining_quantity.IsPositive() {
			t.Fatalf("entry %d shown: %s %s %s", ile.Entry_no, ile.Item_no, ile.Location_code, ile.Remaining_quantity)
		}
		control, err := pages.RunAfterGetRecordControl(reservmgt.AvailableItemLedgEntriesPageID, ile)
		must(t, err)
		v, _ := types.NewDecimalFromString(fmt.Sprint(control.Values()["current_reserved_quantity"]))
		reservedThisLine = reservedThisLine.Add(v)
	}
	if n == 0 || !reservedThisLine.Equal(dec("2")) {
		t.Errorf("%d entries, reserved from them %s", n, reservedThisLine)
	}

	entries := gtables.NewReservationEntry(src)
	open, err = pages.RunOpenPage(reservmgt.ReservationEntriesPageID, entries, reservmgt.Params(reservmgt.SalesLineSource(line)))
	must(t, err)
	applyFilters(t, entries, open.Filters())
	must(t, entries.CalcSums("quantity"))
	if !entries.Quantity.Equal(dec("2")) || !entries.FindFirst() {
		t.Fatalf("line's entries %s", entries.Quantity)
	}
	control, err = pages.RunAfterGetRecordControl(reservmgt.ReservationEntriesPageID, entries)
	must(t, err)
	if text := fmt.Sprint(control.Values()["reserved_for"]); !strings.Contains(text, string(line.Document_no)) {
		t.Errorf("reserved for %q", text)
	}
}

// applyFilters: a page's OnOpenPage filters on a record, as the list API applies them (the values
// as text, Yes/No as a filter expression)
func applyFilters(t *testing.T, rec ftables.Table, filters []pages.FixedFilter) {
	t.Helper()
	ranger := rec.(interface {
		SetRange(string, ...interface{})
	})
	for _, f := range filters {
		if f.Expression {
			must(t, ftables.ApplyFilter(rec, ftables.FieldFilter{Field: f.Field, Expression: f.Value.(string)}))
		} else if b, ok := f.Value.(bool); ok {
			must(t, ftables.ApplyFilter(rec, ftables.FieldFilter{Field: f.Field, Expression: strconv.FormatBool(b)}))
		} else {
			ranger.SetRange(f.Field, fmt.Sprint(f.Value))
		}
	}
}
