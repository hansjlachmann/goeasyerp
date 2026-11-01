// Package purchgetreceipt is codeunit 74 Purch.-Get Receipt: an invoice gets the lines of the
// vendor's posted receipts that are not invoiced yet; posting the invoice invoices them.
package purchgetreceipt

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// GetReceiptLinesPageID is page 5709 Get Receipt Lines
const GetReceiptLinesPageID = 5709

// Run (OnRun): the receipt the user picks (SelectReceipt), its lines onto the invoice
// (CreateInvLines); false when the user picked none
func Run(purchHeader *gtables.PurchaseHeader, receiptDescription string) (bool, error) {
	receiptNo, picked, err := SelectReceipt(purchHeader)
	if err != nil || !picked {
		return false, err
	}
	return true, CreateInvLines(purchHeader, receiptNo, receiptDescription)
}

// SelectReceipt: the pay-to vendor's receipt lines in the invoice's currency that are not
// invoiced, on page 5709 in lookup mode; the receipt of the line the user picked
func SelectReceipt(purchHeader *gtables.PurchaseHeader) (types.Code, bool, error) {
	err := purchHeader.TestField("document_type", gtables.PurchaseHeader_Document_type.Invoice)
	if err != nil {
		return "", false, err
	}
	err = purchHeader.TestField("pay_to_vendor_no")
	if err != nil {
		return "", false, err
	}
	purchRcptLine := gtables.NewPurchRcptLine(purchHeader)
	purchRcptLine.SetRange("pay_to_vendor_no", purchHeader.Pay_to_vendor_no)
	purchRcptLine.SetFilter("qty_rcd_not_invoiced", "<>0")
	purchRcptLine.SetRange("currency_code", purchHeader.Currency_code)
	if system.RunModal(GetReceiptLinesPageID, purchRcptLine, true) != system.ActionLookupOK {
		return "", false, nil
	}
	return purchRcptLine.Document_no, true, nil
}

// CreateInvLines (CreateInvLines): a description line "Receipt No. …:" and an invoice line for
// each line of the receipt not yet invoiced, after the invoice's last line
func CreateInvLines(purchHeader *gtables.PurchaseHeader, receiptNo types.Code, receiptDescription string) error {
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
	purchLine.SetRange("document_no", purchHeader.No)
	nextLineNo := 10000
	if purchLine.FindLast() {
		nextLineNo = purchLine.Line_no + 10000
	}
	purchRcptLine := gtables.NewPurchRcptLine(purchHeader)
	purchRcptLine.SetRange("document_no", receiptNo)
	purchRcptLine.SetFilter("qty_rcd_not_invoiced", "<>0")
	var rcptLines []*gtables.PurchRcptLine
	for found := purchRcptLine.FindSet(); found; found = purchRcptLine.Next() {
		line := gtables.NewPurchRcptLine(purchHeader)
		line.Copy(purchRcptLine)
		rcptLines = append(rcptLines, line)
	}
	if len(rcptLines) == 0 {
		return nil
	}
	descriptionLine := gtables.NewPurchaseLine(purchHeader)
	descriptionLine.Document_type = gtables.PurchaseLineDocument_type(purchHeader.Document_type)
	descriptionLine.Document_no = purchHeader.No
	descriptionLine.Line_no = nextLineNo
	descriptionLine.Description = types.Text(system.CopyStr(system.StrSubstNo(receiptDescription, receiptNo), 1, descriptionLine.MaxStrLen("description")))
	err := descriptionLine.InsertOrError(false)
	if err != nil {
		return err
	}
	for _, rcptLine := range rcptLines {
		nextLineNo = nextLineNo + 10000
		err = insertInvLineFromRcptLine(purchHeader, rcptLine, nextLineNo)
		if err != nil {
			return err
		}
	}
	return nil
}

// insertInvLineFromRcptLine (InsertInvLineFromRcptLine): the order line the receipt line came
// from, on the invoice for the quantity received and not invoiced, with the receipt's number
func insertInvLineFromRcptLine(purchHeader *gtables.PurchaseHeader, purchRcptLine *gtables.PurchRcptLine, lineNo int) error {
	purchOrderLine := gtables.NewPurchaseLine(purchHeader)
	err := purchOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.Order), "document_no": purchRcptLine.Order_no, "line_no": purchRcptLine.Order_line_no})
	if err != nil {
		return err
	}
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.TransferFields(purchOrderLine)
	purchLine.Document_type = gtables.PurchaseLineDocument_type(purchHeader.Document_type)
	purchLine.Document_no = purchHeader.No
	purchLine.Line_no = lineNo
	purchLine.Receipt_no = purchRcptLine.Document_no
	purchLine.Receipt_line_no = purchRcptLine.Line_no
	purchLine.Quantity_received = types.Decimal{}
	purchLine.Qty_received_base = types.Decimal{}
	purchLine.Quantity_invoiced = types.Decimal{}
	purchLine.Qty_invoiced_base = types.Decimal{}
	purchLine.Qty_rcd_not_invoiced = types.Decimal{}
	purchLine.Qty_rcd_not_invd_base = types.Decimal{}
	err = purchLine.Validate("quantity", purchRcptLine.Qty_rcd_not_invoiced)
	if err != nil {
		return err
	}
	// the order's cost and discount, whatever the quantity would find now
	if !purchLine.Direct_unit_cost.Equal(purchOrderLine.Direct_unit_cost) {
		err = purchLine.Validate("direct_unit_cost", purchOrderLine.Direct_unit_cost)
		if err != nil {
			return err
		}
	}
	if !purchLine.Line_discount_percent.Equal(purchOrderLine.Line_discount_percent) {
		err = purchLine.Validate("line_discount_percent", purchOrderLine.Line_discount_percent)
		if err != nil {
			return err
		}
	}
	return purchLine.InsertOrError(false)
}
