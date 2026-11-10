package reports_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Make Orders on Req. Wksh. Names: report 493 on the worksheet of the line — its lines (two items
// of one vendor) become a purchase order (without a dialog the request page's defaults)
func TestReqWkshNameMakeOrders(t *testing.T) {
	src := listCompany(t)
	worksheet := openWorksheet(t, src, false)
	addReqLine(t, worksheet, 10000, "1000", "5")
	addReqLine(t, worksheet, 20000, "1020", "3")
	name := gtables.NewRequisitionWkshName(src)
	mustOK(t, name.GetOrError(map[string]interface{}{"worksheet_template_name": worksheet.Worksheet_template_name, "name": worksheet.Journal_batch_name}))
	lines := gtables.NewRequisitionLine(src)
	lines.SetRange("worksheet_template_name", worksheet.Worksheet_template_name)
	lines.SetRange("journal_batch_name", worksheet.Journal_batch_name)
	before := lines.Count()
	orders := gtables.NewPurchaseHeader(src)
	orders.SetRange("document_type", gtables.PurchaseHeader_Document_type.Order)
	ordersBefore := orders.Count()
	if before == 0 {
		t.Fatal("no lines in the demo worksheet REQ./DEFAULT")
	}
	_, err := codeunits.ReqWkshNameMakeOrders(name)
	mustOK(t, err)
	// the lines are gone (the emptied worksheet keeps a blank line); one vendor: one order
	lines.SetFilter("no", "<>''")
	if lines.Count() != 0 || orders.Count() != ordersBefore+1 {
		t.Errorf("worksheet lines %d → %d, purchase orders %d → %d", before, lines.Count(), ordersBefore, orders.Count())
	}
}
