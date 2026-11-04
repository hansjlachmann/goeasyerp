package salesquotetoorder_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salespost"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Invoices and credit memos: their numbers, a posting date, lines that ship and invoice their
// whole quantity, the statistics; they are not outstanding orders of the customer
func TestSalesInvoiceAndCreditMemo(t *testing.T) {
	src := orderCompany(t)
	for _, c := range []struct {
		documentType gtables.SalesHeaderDocument_type
		no           types.Code
	}{
		{gtables.SalesHeader_Document_type.Invoice, "SI000001"},
		{gtables.SalesHeader_Document_type.CreditMemo, "SCM000001"},
	} {
		salesHeader := newDocument(t, src, c.documentType)
		if salesHeader.No != c.no || salesHeader.Posting_date.IsZero() || salesHeader.Document_date.IsZero() {
			t.Errorf("%v: no. %s, posting date %v, document date %v", c.documentType, salesHeader.No, salesHeader.Posting_date, salesHeader.Document_date)
		}
		salesLine := addLine(t, salesHeader, 10000, "BOX", "2")
		wantDec(t, "Qty. to Ship", salesLine.Qty_to_ship, "2")
		wantDec(t, "Qty. to Invoice", salesLine.Qty_to_invoice, "2")
		wantDec(t, "Qty. to Invoice (Base)", salesLine.Qty_to_invoice_base, "20")
		must(t, salesLine.Validate("quantity", dec("3")))
		wantDec(t, "Qty. to Invoice after a new quantity", salesLine.Qty_to_invoice, "3")
		must(t, salesLine.ModifyOrError(true))

		totals, err := salespost.SumSalesLines(salesHeader, salespost.General)
		must(t, err)
		wantDec(t, "Amount", totals.TotalSalesLine.Amount, "300")
		wantDec(t, "Quantity", totals.TotalSalesLine.Quantity, "3")
	}
	cust := gtables.NewCustomer(src)
	must(t, cust.GetOrError(types.Code("C1")))
	cust.CalcFields("outstanding_orders")
	wantDec(t, "Outstanding Orders of invoices and credit memos", cust.Outstanding_orders, "0")
}
