package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Sales Reservation Avail. (209): a sales order line reserved from the inventory is a Full
// Shipment with its quantity on hand, its reservation entry under it and the order's line
// after it; 409 and 723 print their documents and batches
func TestReservationAvailReports(t *testing.T) {
	src := listCompany(t)
	reservSetup := gtables.GetReservationSetup(src)
	reservSetup.Reserve_sales_order = true
	reservSetup.Show_item_ledger_entry = gtables.ReservationSetup_Show_item_ledger_entry.Always
	reservSetup.Item_ledger_entry_priority = 1
	mustOK(t, reservSetup.ModifyOrError(false))
	cust := gtables.NewCustomer(src)
	if !cust.FindFirst() {
		t.Fatal("no demo customer")
	}
	header := gtables.NewSalesHeader(src)
	header.Document_type = gtables.SalesHeader_Document_type.Order
	mustOK(t, header.InsertOrError(true))
	mustOK(t, header.Validate("sell_to_customer_no", cust.No))
	mustOK(t, header.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Order
	line.Document_no = header.No
	line.Line_no = 10000
	mustOK(t, line.Validate("type", gtables.SalesLine_Type.Item))
	mustOK(t, line.Validate("no", types.Code("1000")))
	mustOK(t, line.Validate("location_code", types.Code("MAIN")))
	mustOK(t, line.Validate("quantity", types.NewDecimalFromInt(3)))
	mustOK(t, line.Validate("shipment_date", types.NewDate(2026, 10, 20)))
	mustOK(t, line.Validate("reserve", gtables.SalesLine_Reserve.Optional))
	mustOK(t, line.InsertOrError(true))
	m := reservmgt.New(src)
	mustOK(t, m.SalesLineSetReservFor(line))
	full, err := m.AutoReserve(line.Shipment_date, "", line.Shipment_date)
	mustOK(t, err)
	if !full {
		t.Fatal("not reserved")
	}

	texts := runList(t, src, reports.SalesReservationAvailID, map[string]string{
		"show_sales_lines": "true", "show_reservation_entries": "true", "modify_qty_to_ship": "true"})
	for _, want := range []string{"Sales Reservation Availability", "Order " + string(header.No), "Full Shipment", "Item Ledger Entry"} {
		if !strings.Contains(texts, want) {
			t.Errorf("209 does not print %q:\n%s", want, texts)
		}
	}
	mustOK(t, line.GetOrError(map[string]interface{}{"document_type": int(line.Document_type), "document_no": line.Document_no, "line_no": line.Line_no}))
	if !line.Qty_to_ship_base.Equal(types.NewDecimalFromInt(3)) {
		t.Errorf("Qty. to Ship (Base) %s, want the quantity on hand 3", line.Qty_to_ship_base)
	}

	runList(t, src, reports.PurchaseReservationAvailID, map[string]string{"show_purchase_lines": "true"})
	runList(t, src, reports.ItemJnlReservationAvailID, map[string]string{"show_item_jnl_lines": "true"})
}
