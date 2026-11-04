package reports_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 294 on an invoice of the project's customer: the project's open usage becomes invoice
// lines per resource/item, priced at the usage's remaining amount, with the project, its phase,
// task and step and a Project Applies-to ID the entries are marked with; posting the invoice
// closes the usage
func TestGetProjectUsage(t *testing.T) {
	src := projectCompany(t)
	project := gtables.NewProject(src)
	project.SetRange("status", gtables.Project_Status.Order)
	if !project.FindFirst() {
		t.Fatal("no demo project on Order")
	}
	usage := gtables.NewProjectLedgerEntry(src)
	usage.SetRange("project_no", project.No)
	usage.SetRange("entry_type", gtables.ProjectLedgerEntry_Entry_type.Usage)
	usage.SetRange("open", true)
	openUsage := usage.Count()
	mustOK(t, usage.CalcSums("remaining_amount"))
	remaining := usage.Remaining_amount
	if openUsage == 0 || remaining.IsZero() {
		t.Fatalf("no open usage: %d entries, %s", openUsage, remaining)
	}

	invoice := gtables.NewSalesHeader(src)
	invoice.Document_type = gtables.SalesHeader_Document_type.Invoice
	mustOK(t, invoice.InsertOrError(true))
	mustOK(t, invoice.Validate("sell_to_customer_no", project.Bill_to_customer_no))
	mustOK(t, invoice.Validate("project_no", project.No))
	mustOK(t, invoice.ModifyOrError(true))

	_, err := reports.Execute(reports.GetProjectUsageID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Context: invoice})
	mustOK(t, err)

	line := gtables.NewSalesLine(src)
	line.SetRange("document_type", gtables.SalesLine_Document_type.Invoice)
	line.SetRange("document_no", invoice.No)
	if line.Count() == 0 {
		t.Fatal("no invoice lines")
	}
	var total types.Decimal
	for found := line.FindSetBuffered(); found; found = line.Next() {
		if line.Project_no != project.No || line.Project_applies_to_id == "" || line.Quantity.IsZero() {
			t.Errorf("line %d: %+v", line.Line_no, line.ToMap())
		}
		total = total.Add(line.Unit_price.Mul(line.Quantity))
		marked := gtables.NewProjectLedgerEntry(src)
		marked.SetRange("applies_to_id", line.Project_applies_to_id)
		if marked.Count() == 0 {
			t.Errorf("line %d: no entries marked %s", line.Line_no, line.Project_applies_to_id)
		}
	}
	if total.Sub(remaining).Abs().GreaterThan(types.MustDecimal("0.10")) {
		t.Errorf("the lines amount to %s, the open usage to %s", total, remaining)
	}

	mustOK(t, invoice.GetOrError(map[string]interface{}{"document_type": invoice.Document_type, "no": invoice.No}))
	invoice.Ship, invoice.Invoice = true, true
	mustOK(t, salesposting.NewSalesPost(src).Run(invoice))
	// the resources' usage is closed; an item's is applied less the customer's line discount
	// (validating the item gives the line its discount, as in the classic report)
	usage.SetRange("open")
	for found := usage.FindSetBuffered(); found; found = usage.Next() {
		if usage.Type == gtables.ProjectLedgerEntry_Type.Resource && usage.Open {
			t.Errorf("resource usage %d still open (remaining %s)", usage.Entry_no, usage.Remaining_amount)
		}
		if usage.Open && !usage.Remaining_amount.LessThan(usage.Total_price) {
			t.Errorf("usage %d not applied: remaining %s of %s", usage.Entry_no, usage.Remaining_amount, usage.Total_price)
		}
	}
}
