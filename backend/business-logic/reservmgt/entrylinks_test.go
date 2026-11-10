package reservmgt_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Insert Requisition Line (the Reservation page): what the sales line still has to reserve
// becomes a line of the setup's requisition worksheet, reserved for the sales line
func TestInsertReqLine(t *testing.T) {
	src := demoCompany(t)
	reservSetup := gtables.GetReservationSetup(src)
	reservSetup.Auto_req_wksh_template = "REQ."
	reservSetup.Auto_req_wksh_name = "DEFAULT"
	must(t, reservSetup.ModifyOrError(false))

	line := salesOrderLine(t, src, "5")
	m := reservmgt.New(src)
	must(t, m.SalesLineSetReservFor(line))
	must(t, m.InsertReqLine(line.Quantity_base, line.Unit_of_measure_code, line.Shipment_date, "", line.Shipment_date))

	reqLine := gtables.NewRequisitionLine(src)
	reqLine.SetRange("worksheet_template_name", "REQ.")
	reqLine.SetRange("journal_batch_name", "DEFAULT")
	if !reqLine.FindLast() {
		t.Fatal("no requisition line")
	}
	if reqLine.No != "1000" || reqLine.Location_code != "MAIN" || !reqLine.Quantity_base.Equal(line.Quantity_base) ||
		!reqLine.Reserved_qty_base.Equal(line.Quantity_base.Neg()) || reqLine.Expected_receipt_date != line.Shipment_date {
		t.Errorf("requisition line %s at %s: %s, reserved %s, expected %s", reqLine.No, reqLine.Location_code, reqLine.Quantity_base, reqLine.Reserved_qty_base, reqLine.Expected_receipt_date)
	}
	reload(t, line)
	line.CalcFields("reserved_qty_base")
	if !line.Reserved_qty_base.Equal(line.Quantity_base) {
		t.Errorf("sales line reserved %s of %s", line.Reserved_qty_base, line.Quantity_base)
	}
	entries := gtables.NewReservationEntry(src)
	entries.SetRange("from_type", reservmgt.TypeRequisitionLine)
	entries.SetRange("from_ref_no", reqLine.Line_no)
	must(t, entries.CalcSums("quantity"))
	if !entries.Quantity.Equal(line.Quantity_base) {
		t.Errorf("reserved from the requisition line %s", entries.Quantity)
	}
}

// Reservation Entries of a line: its open entries; Show History drops the status filter, Hide
// History keeps the open ones — also opened from the menu
func TestReservationEntriesHistory(t *testing.T) {
	src := demoCompany(t)
	line := salesOrderLine(t, src, "1")
	hasStatusFilter := func(params map[string]string) bool {
		open, err := pages.RunOpenPage(reservmgt.ReservationEntriesPageID, gtables.NewReservationEntry(src), params)
		must(t, err)
		for _, f := range open.Filters() {
			if f.Field == "reservation_status" {
				return true
			}
		}
		return false
	}
	params := func(history string) map[string]string {
		p := reservmgt.Params(reservmgt.SalesLineSource(line))
		if history != "" {
			p[reservmgt.ParamHistory] = history
		}
		return p
	}
	if !hasStatusFilter(params("")) || hasStatusFilter(params("1")) || !hasStatusFilter(params("0")) {
		t.Error("a line's entries: open, Show History all, Hide History open")
	}
	if hasStatusFilter(map[string]string{}) || !hasStatusFilter(map[string]string{reservmgt.ParamHistory: "0"}) {
		t.Error("from the menu: all, Hide History open")
	}
}
