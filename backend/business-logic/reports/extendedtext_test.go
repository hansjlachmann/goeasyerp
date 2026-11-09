package reports_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// An item's extended text (codeunit 378) becomes text lines attached to the item's sales line;
// Copy Document keeps them attached to the copy of the line (codeunit 379); deleting the line
// deletes them
func TestExtendedTexts(t *testing.T) {
	src := documentsCompany(t)
	header := gtables.NewExtendedTextHeader(src)
	header.Table_name = gtables.ExtendedTextHeader_Table_name.Item
	header.No = "1000"
	mustOK(t, header.InsertOrError(true))
	if header.Text_no != 1 || !header.Sales_order {
		t.Fatalf("new extended text: text no. %d, sales order %v", header.Text_no, header.Sales_order)
	}
	for i, text := range []string{"Acid-free paper,", "certified for archiving."} {
		line := gtables.NewExtendedTextLine(src)
		line.Table_name = gtables.ExtendedTextLine_Table_name.Item
		line.No, line.Text_no, line.Line_no, line.Text = "1000", 1, (i+1)*10000, types.Text(text)
		mustOK(t, line.InsertOrError(true))
	}

	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	mustOK(t, order.InsertOrError(true))
	mustOK(t, order.Validate("sell_to_customer_no", types.Code("C00010")))
	mustOK(t, order.ModifyOrError(true))
	itemLine := gtables.NewSalesLine(src)
	itemLine.Document_type, itemLine.Document_no, itemLine.Line_no = gtables.SalesLine_Document_type.Order, order.No, 10000
	mustOK(t, itemLine.Validate("type", gtables.SalesLine_Type.Item))
	mustOK(t, itemLine.Validate("no", types.Code("1000")))
	mustOK(t, itemLine.Validate("quantity", types.NewDecimalFromInt(2)))
	mustOK(t, itemLine.InsertOrError(true))

	// the item has no Automatic Ext. Text: entering it inserts nothing, Insert Ext. Texts does
	transfer := &tables.TransferExtendedText{}
	found, err := transfer.SalesCheckIfAnyExtText(itemLine, false)
	mustOK(t, err)
	if found {
		t.Error("an item without Automatic Ext. Text inserted its text")
	}
	found, err = transfer.SalesCheckIfAnyExtText(itemLine, true)
	mustOK(t, err)
	if !found {
		t.Fatal("no extended text found")
	}
	mustOK(t, transfer.InsertSalesExtText(itemLine))
	attached := gtables.NewSalesLine(src)
	attached.SetRange("document_type", gtables.SalesLine_Document_type.Order)
	attached.SetRange("document_no", order.No)
	attached.SetRange("attached_to_line_no", 10000)
	if attached.Count() != 2 || !attached.FindFirst() || attached.Description != "Acid-free paper," || attached.Line_no <= 10000 {
		t.Fatalf("attached text lines: %d, first %q at %d", attached.Count(), attached.Description, attached.Line_no)
	}

	// copied as they are onto an invoice: attached to the invoice's item line
	invoice := gtables.NewSalesHeader(src)
	invoice.Document_type = gtables.SalesHeader_Document_type.Invoice
	mustOK(t, invoice.InsertOrError(true))
	mustOK(t, invoice.Validate("sell_to_customer_no", types.Code("C00010")))
	mustOK(t, invoice.ModifyOrError(true))
	_, err = reports.Execute(reports.CopySalesDocumentID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: invoice, Options: map[string]string{"document_type": "3", "document_no": order.No.String(), "recalculate_lines": "false"}})
	mustOK(t, err)
	invItem := gtables.NewSalesLine(src)
	invItem.SetRange("document_type", gtables.SalesLine_Document_type.Invoice)
	invItem.SetRange("document_no", invoice.No)
	invItem.SetRange("no", types.Code("1000"))
	if !invItem.FindFirst() {
		t.Fatal("the item line was not copied")
	}
	invText := gtables.NewSalesLine(src)
	invText.SetRange("document_type", gtables.SalesLine_Document_type.Invoice)
	invText.SetRange("document_no", invoice.No)
	invText.SetRange("attached_to_line_no", invItem.Line_no)
	if invText.Count() != 2 {
		t.Errorf("copied text lines attached to the invoice's item line: %d, want 2", invText.Count())
	}

	// deleting the item line deletes its text lines
	mustOK(t, itemLine.DeleteOrError(true))
	if attached.Count() != 0 {
		t.Errorf("%d text lines left after the item line was deleted", attached.Count())
	}
	// deleting the item's extended text deletes its lines
	mustOK(t, header.DeleteOrError(true))
	textLines := gtables.NewExtendedTextLine(src)
	textLines.SetRange("no", types.Code("1000"))
	if textLines.Count() != 0 {
		t.Errorf("%d extended text lines left after the text was deleted", textLines.Count())
	}
}
