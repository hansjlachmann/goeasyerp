package salesposting_test

import (
	"testing"

	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Applies-to ID on a credit memo (Apply Entries, codeunit 401): the customer entries marked with
// the ID are what posting applies the credit memo to; changing the ID takes the old marks off
func TestCreditMemoAppliesToID(t *testing.T) {
	src := demoCompany(t)
	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	addLine(t, invoice, 10000, gtables.SalesLine_Type.Item, "1000", "4")
	postedInvoice := post(t, invoice, false, false).InvoiceNo
	invEntry := custEntry(t, src, postedInvoice)

	creditMemo := newDocument(t, src, gtables.SalesHeader_Document_type.CreditMemo, "C00020")
	addLine(t, creditMemo, 10000, gtables.SalesLine_Type.Item, "1000", "1")

	// F9 on Apply Customer Entries: the invoice entry marked with the credit memo's ID
	invEntry.Applies_to_id = creditMemo.No
	must(t, invEntry.ModifyOrError(false))
	must(t, creditMemo.Validate("applies_to_id", creditMemo.No))
	must(t, creditMemo.ModifyOrError(true))

	// another ID takes the mark off, and back
	must(t, creditMemo.Validate("applies_to_id", "OTHER"))
	invEntry = custEntry(t, src, postedInvoice)
	if invEntry.Applies_to_id != "" {
		t.Fatalf("old mark kept: %q", invEntry.Applies_to_id)
	}
	invEntry.Applies_to_id = creditMemo.No
	must(t, invEntry.ModifyOrError(false))
	creditMemo.Applies_to_id = creditMemo.No
	must(t, creditMemo.ModifyOrError(false))

	invEntry.CalcFields("remaining_amount")
	before := invEntry.Remaining_amount
	salesPost := post(t, creditMemo, false, false)
	glBalance(t, src, salesPost.CrMemoNo)
	cmEntry := custEntry(t, src, salesPost.CrMemoNo)
	cmEntry.CalcFields("amount", "remaining_amount")
	invEntry = custEntry(t, src, postedInvoice)
	invEntry.CalcFields("remaining_amount")
	if cmEntry.Open || !invEntry.Remaining_amount.Equal(before.Add(cmEntry.Amount)) || invEntry.Applies_to_id != "" {
		t.Errorf("credit memo %s open %v; invoice remaining %s (was %s), mark %q", cmEntry.Amount, cmEntry.Open, invEntry.Remaining_amount, before, invEntry.Applies_to_id)
	}
}
