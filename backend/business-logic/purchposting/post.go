// Package purchposting is codeunit 90 Purch.-Post: the posting of the purchase documents (its
// SumPurchLines, the documents' totals, is package purchpost).
package purchposting

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchpost"
	"github.com/hansjlachmann/openerp/backend/business-logic/salestax"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// PurchPost is codeunit 90 Purch.-Post: posts an order (receive and/or invoice), an invoice or
// a credit memo. Receiving writes the posted receipt and the items' ledger entries (codeunit
// 22); invoicing writes the posted invoice or credit memo, invoices the item entries (a line
// received before through its receipt's entry), posts the purchase, discount and invoice
// rounding amounts with their sales tax (use tax: the tax owed on the purchase, not on the
// invoice) to the G/L and the vendor entry (codeunit 12, applied by Applies-to Doc. No.). An
// order keeps what is left to receive and invoice and is deleted when everything is invoiced;
// an invoice or credit memo is deleted. The caller runs it in a transaction.
//
// Not migrated: the VAT calculation types (as in the journals), prepayments, drop shipments,
// reservations and item tracking, jobs, fixed asset lines, Get Receipt Lines, archive.
type PurchPost struct {
	src            ftables.Source
	purchSetup     *gtables.PurchasesPayablesSetup
	srcCode        types.Code
	genJnlPostLine *genjnl.GenJnlPostLine
	itemJnlPost    *itemjnl.ItemJnlPostLine

	purchHeader        *gtables.PurchaseHeader
	purchRcptHeader    *gtables.PurchRcptHeader
	purchInvHeader     *gtables.PurchInvHeader
	purchCrMemoHeader  *gtables.PurchCrMemoHdr
	createReceipt      bool
	genJnlLineDocType  gtables.GenJournalLineDocument_type
	genJnlLineDocNo    types.Code
	genJnlLineExtDocNo types.Code
	sign               types.Decimal // +1: an invoice, -1: a credit memo (amounts and quantities)

	totals           *purchpost.Totals // the invoiced amounts in the document's currency and in LCY
	invPostingBuffer []*invPostingBuffer

	// What was posted: the posted documents' numbers, and whether the document is gone
	ReceiptNo types.Code
	InvoiceNo types.Code
	CrMemoNo  types.Code
	Deleted   bool
}

// invPostingBuffer adds up the G/L amounts of the lines with the same account and posting
// groups (Invoice Post. Buffer, table 49)
type invPostingBuffer struct {
	glAccount          types.Code
	genPostingType     bool // a purchase amount with tax (else a discount or rounding amount without)
	genBusPostingGrp   types.Code
	genProdPostingGrp  types.Code
	taxBusPostingGrp   types.Code
	taxProdPostingGrp  types.Code
	taxCalculationType gtables.PurchaseLineTax_calculation_type
	taxAreaCode        types.Code
	taxLiable          bool
	taxGroupCode       types.Code
	useTax             bool
	departmentCode     types.Code
	projectCode        types.Code
	amount             types.Decimal // LCY
	amountFCY          types.Decimal
	taxAmount          types.Decimal // LCY
	taxAmountFCY       types.Decimal
	taxBaseAmount      types.Decimal // LCY
	taxBaseAmountFCY   types.Decimal
	quantity           types.Decimal
}

// NewPurchPost makes codeunit 90 for a database and company
func NewPurchPost(src ftables.Source) *PurchPost {
	return &PurchPost{src: src}
}

// Run posts the document (a PurchPost can post one document after the other: each run starts
// afresh)
func (p *PurchPost) Run(purchHeader *gtables.PurchaseHeader) error {
	*p = PurchPost{src: p.src}
	p.purchHeader = purchHeader
	err := p.checkAndUpdate()
	if err != nil {
		return err
	}
	err = p.insertPostedHeaders()
	if err != nil {
		return err
	}
	err = p.postLines()
	if err != nil {
		return err
	}
	if purchHeader.Invoice {
		err = p.invoiceRounding()
		if err != nil {
			return err
		}
		err = p.postInvPostingBuffer()
		if err != nil {
			return err
		}
		err = p.postVendorEntry()
		if err != nil {
			return err
		}
	}
	return p.finalize()
}

// checkAndUpdate: the document can be posted — its type, vendor, dates, the vendor's document
// number, what it receives and invoices, something to post — and gets its posted documents'
// numbers
func (p *PurchPost) checkAndUpdate() error {
	purchHeader := p.purchHeader
	switch purchHeader.Document_type {
	case gtables.PurchaseHeader_Document_type.Order:
		if !purchHeader.Receive && !purchHeader.Invoice {
			return apperrors.PurchReceiveOrInvoice(gtables.PurchaseHeaderTableName, "receive", "invoice")
		}
	case gtables.PurchaseHeader_Document_type.Invoice:
		purchHeader.Receive = true
		purchHeader.Invoice = true
	case gtables.PurchaseHeader_Document_type.CreditMemo:
		purchHeader.Receive = false
		purchHeader.Invoice = true
	default:
		return apperrors.PurchCannotPostType(gtables.PurchaseHeaderTableName, "document_type", purchHeader.Document_type)
	}
	for _, field := range []string{"buy_from_vendor_no", "pay_to_vendor_no", "posting_date", "document_date"} {
		err := purchHeader.TestField(field)
		if err != nil {
			return err
		}
	}
	p.purchSetup = gtables.GetPurchasesPayablesSetup(purchHeader)
	if purchHeader.Invoice {
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
			p.genJnlLineExtDocNo = purchHeader.Vendor_cr_memo_no
			if p.purchSetup.Ext_doc_no_mandatory {
				err := purchHeader.TestField("vendor_cr_memo_no")
				if err != nil {
					return err
				}
			}
		} else {
			p.genJnlLineExtDocNo = purchHeader.Vendor_invoice_no
			if p.purchSetup.Ext_doc_no_mandatory {
				err := purchHeader.TestField("vendor_invoice_no")
				if err != nil {
					return err
				}
			}
		}
	}
	for _, vendNo := range []types.Code{purchHeader.Buy_from_vendor_no, purchHeader.Pay_to_vendor_no} {
		vend := gtables.NewVendor(purchHeader)
		err := vend.GetOrError(vendNo)
		if err != nil {
			return err
		}
		err = vend.TestField("blocked", false)
		if err != nil {
			return err
		}
	}
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
		p.sign = types.NewDecimalFromInt(-1)
	} else {
		p.sign = types.NewDecimalFromInt(1)
	}

	// something to post
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
	purchLine.SetRange("document_no", purchHeader.No)
	somethingToPost := false
	somethingToReceive := false
	for found := purchLine.FindSet(); found; found = purchLine.Next() {
		qtyToReceive, _, qtyToInvoice, _ := p.qtysToPost(purchLine)
		if !qtyToReceive.IsZero() || !qtyToInvoice.IsZero() {
			somethingToPost = true
		}
		if !qtyToReceive.IsZero() {
			somethingToReceive = true
		}
	}
	if !somethingToPost {
		return apperrors.PurchNothingToPost()
	}

	// the posted documents' numbers
	p.createReceipt = purchHeader.Receive && somethingToReceive && (purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order ||
		purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Invoice && p.purchSetup.Receipt_on_invoice)
	noSeriesMgt := noseries.New(purchHeader)
	if p.createReceipt && purchHeader.Receiving_no == "" {
		err := purchHeader.TestField("receiving_no_series")
		if err != nil {
			return err
		}
		purchHeader.Receiving_no, err = noSeriesMgt.GetNextNo(purchHeader.Receiving_no_series, purchHeader.Posting_date, true)
		if err != nil {
			return err
		}
	}
	if purchHeader.Invoice && purchHeader.Posting_no == "" {
		if purchHeader.No_series != "" || purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
			err := purchHeader.TestField("posting_no_series")
			if err != nil {
				return err
			}
		}
		if purchHeader.No_series != purchHeader.Posting_no_series || purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
			var err error
			purchHeader.Posting_no, err = noSeriesMgt.GetNextNo(purchHeader.Posting_no_series, purchHeader.Posting_date, true)
			if err != nil {
				return err
			}
		} else {
			purchHeader.Posting_no = purchHeader.No
		}
	}
	err := purchHeader.ModifyOrError(false)
	if err != nil {
		return err
	}

	sourceCodeSetup := gtables.GetSourceCodeSetup(purchHeader)
	p.srcCode = sourceCodeSetup.Purchases
	p.genJnlPostLine = genjnl.NewGenJnlPostLine(p.src)
	p.itemJnlPost = itemjnl.NewItemJnlPostLine(p.src)
	var err2 error
	p.totals, err2 = purchpost.NewTotals(purchHeader)
	return err2
}

// qtysToPost: what a line receives and invoices now (and in base units) — an order what is set
// to receive and invoice, an invoice its whole quantity, a credit memo its whole quantity
// invoiced
func (p *PurchPost) qtysToPost(purchLine *gtables.PurchaseLine) (qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase types.Decimal) {
	purchHeader := p.purchHeader
	switch purchHeader.Document_type {
	case gtables.PurchaseHeader_Document_type.Order:
		if purchHeader.Receive {
			qtyToReceive = purchLine.Qty_to_receive
			qtyToReceiveBase = purchLine.Qty_to_receive_base
		}
		if purchHeader.Invoice {
			qtyToInvoice = purchLine.Qty_to_invoice
			qtyToInvoiceBase = purchLine.Qty_to_invoice_base
			// invoicing only: not more than is received and not invoiced
			if !purchHeader.Receive && qtyToInvoice.Abs().GreaterThan(purchLine.Qty_rcd_not_invoiced.Abs()) {
				qtyToInvoice = purchLine.Qty_rcd_not_invoiced
				qtyToInvoiceBase = purchLine.Qty_rcd_not_invd_base
			}
		}
	case gtables.PurchaseHeader_Document_type.Invoice:
		// a line got from a receipt (Get Receipt Lines) was received there
		if purchLine.Receipt_no == "" {
			qtyToReceive = purchLine.Quantity
			qtyToReceiveBase = purchLine.Quantity_base
		}
		qtyToInvoice = purchLine.Quantity
		qtyToInvoiceBase = purchLine.Quantity_base
	case gtables.PurchaseHeader_Document_type.CreditMemo:
		qtyToInvoice = purchLine.Quantity
		qtyToInvoiceBase = purchLine.Quantity_base
	}
	return qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase
}

// insertPostedHeaders writes the posted receipt and the posted invoice or credit memo headers
// (the document's fields, the posted numbers) with the document's comments
func (p *PurchPost) insertPostedHeaders() error {
	purchHeader := p.purchHeader
	isOrder := purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order
	if p.createReceipt {
		p.purchRcptHeader = gtables.NewPurchRcptHeader(purchHeader)
		p.purchRcptHeader.LockTable()
		p.purchRcptHeader.TransferFields(purchHeader)
		p.purchRcptHeader.No = purchHeader.Receiving_no
		if isOrder {
			p.purchRcptHeader.Order_no = purchHeader.No
		}
		p.purchRcptHeader.No_series = purchHeader.Receiving_no_series
		p.purchRcptHeader.Source_code = p.srcCode
		p.purchRcptHeader.User_id = types.Code(system.UserId())
		p.purchRcptHeader.No_printed = 0
		err := p.purchRcptHeader.InsertOrError(false)
		if err != nil {
			return err
		}
		err = p.copyComments(gtables.PurchCommentLine_Document_type.Receipt, p.purchRcptHeader.No)
		if err != nil {
			return err
		}
		p.ReceiptNo = p.purchRcptHeader.No
	}
	if !purchHeader.Invoice {
		return nil
	}
	p.genJnlLineDocNo = purchHeader.Posting_no
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
		p.genJnlLineDocType = gtables.GenJournalLine_Document_type.CreditMemo
		p.purchCrMemoHeader = gtables.NewPurchCrMemoHdr(purchHeader)
		p.purchCrMemoHeader.LockTable()
		p.purchCrMemoHeader.TransferFields(purchHeader)
		p.purchCrMemoHeader.No = purchHeader.Posting_no
		p.purchCrMemoHeader.Pre_assigned_no = purchHeader.No
		p.purchCrMemoHeader.No_series = purchHeader.Posting_no_series
		p.purchCrMemoHeader.Source_code = p.srcCode
		p.purchCrMemoHeader.User_id = types.Code(system.UserId())
		p.purchCrMemoHeader.No_printed = 0
		err := p.purchCrMemoHeader.InsertOrError(false)
		if err != nil {
			return err
		}
		p.CrMemoNo = p.purchCrMemoHeader.No
		return p.copyComments(gtables.PurchCommentLine_Document_type.PostedCreditMemo, p.purchCrMemoHeader.No)
	}
	p.genJnlLineDocType = gtables.GenJournalLine_Document_type.Invoice
	p.purchInvHeader = gtables.NewPurchInvHeader(purchHeader)
	p.purchInvHeader.LockTable()
	p.purchInvHeader.TransferFields(purchHeader)
	p.purchInvHeader.No = purchHeader.Posting_no
	if isOrder {
		p.purchInvHeader.Order_no = purchHeader.No
	} else {
		p.purchInvHeader.Pre_assigned_no = purchHeader.No
	}
	p.purchInvHeader.No_series = purchHeader.Posting_no_series
	p.purchInvHeader.Source_code = p.srcCode
	p.purchInvHeader.User_id = types.Code(system.UserId())
	p.purchInvHeader.No_printed = 0
	err := p.purchInvHeader.InsertOrError(false)
	if err != nil {
		return err
	}
	p.InvoiceNo = p.purchInvHeader.No
	return p.copyComments(gtables.PurchCommentLine_Document_type.PostedInvoice, p.purchInvHeader.No)
}

// copyComments: the document's comment lines for a posted document
func (p *PurchPost) copyComments(toDocType gtables.PurchCommentLineDocument_type, toNo types.Code) error {
	purchCommentLine := gtables.NewPurchCommentLine(p.purchHeader)
	purchCommentLine.SetRange("document_type", gtables.PurchCommentLineDocument_type(p.purchHeader.Document_type))
	purchCommentLine.SetRange("no", p.purchHeader.No)
	for found := purchCommentLine.FindSet(); found; found = purchCommentLine.Next() {
		purchCommentLine2 := gtables.NewPurchCommentLine(p.purchHeader)
		purchCommentLine2.TransferFields(purchCommentLine)
		purchCommentLine2.Document_type = toDocType
		purchCommentLine2.No = toNo
		err := purchCommentLine2.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// postLines posts each line: its item entries, its share of the G/L amounts, its posted
// receipt and invoice/credit memo lines; an order line keeps the quantities posted
func (p *PurchPost) postLines() error {
	purchHeader := p.purchHeader
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
	purchLine.SetRange("document_no", purchHeader.No)
	var postedLines []*gtables.PurchaseLine
	for found := purchLine.FindSet(); found; found = purchLine.Next() {
		line := gtables.NewPurchaseLine(purchHeader)
		line.Copy(purchLine)
		postedLines = append(postedLines, line)
	}
	for _, purchLine := range postedLines {
		qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase := p.qtysToPost(purchLine)
		if purchLine.Type == gtables.PurchaseLine_Type.Blank {
			// a text line goes on the posted documents as it is
			err := p.insertPostedLines(purchLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase, nil, nil, 0)
			if err != nil {
				return err
			}
			continue
		}
		if qtyToReceive.IsZero() && qtyToInvoice.IsZero() {
			continue
		}
		// the invoiced share of the line's amounts, rounded so the totals stay exact
		var lineFCY, lineLCY *gtables.PurchaseLine
		if !qtyToInvoice.IsZero() {
			lineFCY, lineLCY = p.totals.InvoicedShare(purchLine, qtyToInvoice)
		}
		itemRcptEntryNo := 0
		fromReceipt := purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Invoice && purchLine.Receipt_no != ""
		if fromReceipt {
			err := p.invoiceReceiptLine(purchLine, qtyToInvoice, qtyToInvoiceBase, lineLCY)
			if err != nil {
				return err
			}
		} else if purchLine.Type == gtables.PurchaseLine_Type.Item {
			var err error
			itemRcptEntryNo, err = p.postItemLines(purchLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase, lineLCY)
			if err != nil {
				return err
			}
		}
		if !qtyToInvoice.IsZero() {
			err := p.fillInvPostingBuffer(purchLine, lineFCY, lineLCY)
			if err != nil {
				return err
			}
		}
		err := p.insertPostedLines(purchLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase, lineFCY, lineLCY, itemRcptEntryNo)
		if err != nil {
			return err
		}
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
			err = p.updateOrderLine(purchLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// postItemLines posts the item entries of a line: first the invoicing of what was received
// before (an entry without quantity on the receipt's entry), then what is received now with the
// rest invoiced; returns the entry of the receipt
func (p *PurchPost) postItemLines(purchLine *gtables.PurchaseLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineLCY *gtables.PurchaseLine) (int, error) {
	invoicedBefore := types.Decimal{}
	if !qtyToInvoice.IsZero() && p.purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order && !purchLine.Qty_rcd_not_invoiced.IsZero() {
		left := qtyToInvoice.Sub(qtyToReceive)
		if left.GreaterThan(purchLine.Qty_rcd_not_invoiced) {
			left = purchLine.Qty_rcd_not_invoiced
		}
		purchRcptLine := gtables.NewPurchRcptLine(purchLine)
		purchRcptLine.SetRange("order_no", purchLine.Document_no)
		purchRcptLine.SetRange("order_line_no", purchLine.Line_no)
		var rcptLines []*gtables.PurchRcptLine
		for found := purchRcptLine.FindSet(); found; found = purchRcptLine.Next() {
			if !purchRcptLine.Qty_rcd_not_invoiced.IsZero() {
				line := gtables.NewPurchRcptLine(purchLine)
				line.Copy(purchRcptLine)
				rcptLines = append(rcptLines, line)
			}
		}
		for _, rcptLine := range rcptLines {
			if !left.IsPositive() {
				break
			}
			qty := left
			if qty.GreaterThan(rcptLine.Qty_rcd_not_invoiced) {
				qty = rcptLine.Qty_rcd_not_invoiced
			}
			qtyBase := system.Round(qty.Mul(purchLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001"))
			_, err := p.postItemJnlLine(purchLine, types.Decimal{}, types.Decimal{}, qty, qtyBase, p.amountPart(lineLCY, qty, qtyToInvoice), rcptLine.Item_rcpt_entry_no)
			if err != nil {
				return 0, err
			}
			rcptLine.Quantity_invoiced = rcptLine.Quantity_invoiced.Add(qty)
			rcptLine.Qty_invoiced_base = rcptLine.Qty_invoiced_base.Add(qtyBase)
			rcptLine.Qty_rcd_not_invoiced = rcptLine.Qty_rcd_not_invoiced.Sub(qty)
			rcptLine.Qty_rcd_not_invd_base = rcptLine.Qty_rcd_not_invd_base.Sub(qtyBase)
			err = rcptLine.ModifyOrError(false)
			if err != nil {
				return 0, err
			}
			left = left.Sub(qty)
			invoicedBefore = invoicedBefore.Add(qty)
		}
	}
	if qtyToReceive.IsZero() && p.purchHeader.Document_type != gtables.PurchaseHeader_Document_type.CreditMemo {
		return 0, nil
	}
	invoicedNow := qtyToInvoice.Sub(invoicedBefore)
	invoicedNowBase := qtyToInvoiceBase.Sub(system.Round(invoicedBefore.Mul(purchLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001")))
	if p.purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
		// a credit memo sends the items back: a purchase of the negative quantity, invoiced
		qtyToReceive = qtyToInvoice
		qtyToReceiveBase = qtyToInvoiceBase
	}
	return p.postItemJnlLine(purchLine, qtyToReceive, qtyToReceiveBase, invoicedNow, invoicedNowBase, p.amountPart(lineLCY, invoicedNow, qtyToInvoice), 0)
}

// invoiceReceiptLine invoices a line got from a receipt: an item's receipt entry is invoiced (an
// entry without quantity on it), the receipt line and the order line it came from count the
// quantity invoiced
func (p *PurchPost) invoiceReceiptLine(purchLine *gtables.PurchaseLine, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineLCY *gtables.PurchaseLine) error {
	purchRcptLine := gtables.NewPurchRcptLine(purchLine)
	err := purchRcptLine.GetOrError(map[string]interface{}{"document_no": purchLine.Receipt_no, "line_no": purchLine.Receipt_line_no})
	if err != nil {
		return err
	}
	if purchLine.Type == gtables.PurchaseLine_Type.Item {
		_, err = p.postItemJnlLine(purchLine, types.Decimal{}, types.Decimal{}, qtyToInvoice, qtyToInvoiceBase, p.amountPart(lineLCY, qtyToInvoice, qtyToInvoice), purchRcptLine.Item_rcpt_entry_no)
		if err != nil {
			return err
		}
	}
	purchRcptLine.Quantity_invoiced = purchRcptLine.Quantity_invoiced.Add(qtyToInvoice)
	purchRcptLine.Qty_invoiced_base = purchRcptLine.Qty_invoiced_base.Add(qtyToInvoiceBase)
	purchRcptLine.Qty_rcd_not_invoiced = purchRcptLine.Qty_rcd_not_invoiced.Sub(qtyToInvoice)
	purchRcptLine.Qty_rcd_not_invd_base = purchRcptLine.Qty_rcd_not_invd_base.Sub(qtyToInvoiceBase)
	err = purchRcptLine.ModifyOrError(false)
	if err != nil {
		return err
	}
	purchOrderLine := gtables.NewPurchaseLine(purchLine)
	if !purchOrderLine.Get(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.Order), "document_no": purchRcptLine.Order_no, "line_no": purchRcptLine.Order_line_no}) {
		return nil
	}
	return p.updateOrderLine(purchOrderLine, types.Decimal{}, types.Decimal{}, qtyToInvoice, qtyToInvoiceBase)
}

// amountPart: the LCY amounts of part of the invoiced quantity
func (p *PurchPost) amountPart(lineLCY *gtables.PurchaseLine, qty, qtyToInvoice types.Decimal) [2]types.Decimal {
	if lineLCY == nil || qty.IsZero() || qtyToInvoice.IsZero() {
		return [2]types.Decimal{}
	}
	if qty.Equal(qtyToInvoice) {
		return [2]types.Decimal{lineLCY.Amount, lineLCY.Line_discount_amount.Add(lineLCY.Inv_discount_amount)}
	}
	amount := system.Round(lineLCY.Amount.Mul(qty).Div(qtyToInvoice))
	discount := system.Round(lineLCY.Line_discount_amount.Add(lineLCY.Inv_discount_amount).Mul(qty).Div(qtyToInvoice))
	return [2]types.Decimal{amount, discount}
}

// postItemJnlLine posts an item journal line of the purchase (codeunit 22): the quantity
// received and invoiced with its amount and discount in LCY, the line's unit cost (with the
// indirect cost); returns the item ledger entry of a receipt
func (p *PurchPost) postItemJnlLine(purchLine *gtables.PurchaseLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase types.Decimal, amounts [2]types.Decimal, itemRcptEntryNo int) (int, error) {
	purchHeader := p.purchHeader
	itemJnlLine := gtables.NewItemJournalLine(purchHeader)
	itemJnlLine.Init()
	itemJnlLine.Posting_date = purchHeader.Posting_date
	itemJnlLine.Document_date = purchHeader.Document_date
	if !qtyToInvoice.IsZero() {
		itemJnlLine.Document_no = p.genJnlLineDocNo
		itemJnlLine.External_document_no = p.genJnlLineExtDocNo
	} else {
		itemJnlLine.Document_no = purchHeader.Receiving_no
		itemJnlLine.External_document_no = purchHeader.Vendor_shipment_no
	}
	if itemJnlLine.Document_no == "" {
		itemJnlLine.Document_no = p.genJnlLineDocNo
	}
	itemJnlLine.Source_posting_group = purchHeader.Vendor_posting_group
	itemJnlLine.Salespers_purch_code = purchHeader.Purchaser_code
	itemJnlLine.Reason_code = purchHeader.Reason_code
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.Purchase
	itemJnlLine.Source_type = gtables.ItemJournalLine_Source_type.Vendor
	itemJnlLine.Source_no = purchLine.Buy_from_vendor_no
	itemJnlLine.Item_no = purchLine.No
	itemJnlLine.Description = purchLine.Description
	itemJnlLine.Location_code = purchLine.Location_code
	itemJnlLine.Variant_code = purchLine.Variant_code
	itemJnlLine.Inventory_posting_group = purchLine.Posting_group
	itemJnlLine.Department_code = purchLine.Department_code
	itemJnlLine.Project_code = purchLine.Project_code
	itemJnlLine.Gen_bus_posting_group = purchLine.Gen_bus_posting_group
	itemJnlLine.Gen_prod_posting_group = purchLine.Gen_prod_posting_group
	itemJnlLine.Unit_of_measure_code = purchLine.Unit_of_measure_code
	itemJnlLine.Qty_per_unit_of_measure = purchLine.Qty_per_unit_of_measure
	itemJnlLine.Source_code = p.srcCode
	itemJnlLine.Posting_no_series = purchHeader.Posting_no_series
	itemJnlLine.Quantity = qtyToReceive.Mul(p.sign)
	itemJnlLine.Quantity_base = qtyToReceiveBase.Mul(p.sign)
	itemJnlLine.Invoiced_quantity = qtyToInvoice.Mul(p.sign)
	itemJnlLine.Invoiced_qty_base = qtyToInvoiceBase.Mul(p.sign)
	itemJnlLine.Unit_cost = purchLine.Unit_cost_lcy
	itemJnlLine.Indirect_cost_percent = purchLine.Indirect_cost_percent
	itemJnlLine.Item_shpt_entry_no = itemRcptEntryNo
	itemJnlLine.Amount = amounts[0].Mul(p.sign)
	itemJnlLine.Discount_amount = amounts[1].Mul(p.sign)
	if !qtyToInvoice.IsZero() {
		itemJnlLine.Unit_amount = system.Round(amounts[0].Div(qtyToInvoice), types.MustDecimal("0.00001"))
	} else {
		itemJnlLine.Unit_amount = purchLine.Unit_cost_lcy
	}
	err := p.itemJnlPost.Run(itemJnlLine)
	if err != nil {
		return 0, err
	}
	return itemJnlLine.Item_shpt_entry_no, nil
}

// fillInvPostingBuffer adds a line's invoiced amounts to the G/L amounts: the purchase (credit
// memo) account of its General Posting Setup — a G/L account line its own account — with its
// sales tax (use tax: calculated on the amount, the vendor is not charged it), and, as the
// setup's Discount Posting says, the line and invoice discounts on their accounts
func (p *PurchPost) fillInvPostingBuffer(purchLine, lineFCY, lineLCY *gtables.PurchaseLine) error {
	var glAccount types.Code
	genPostingSetup := gtables.NewGeneralPostingSetup(purchLine)
	if purchLine.Type == gtables.PurchaseLine_Type.GLAccount {
		glAccount = purchLine.No
		genPostingSetup.Get(map[string]interface{}{"gen_bus_posting_group": purchLine.Gen_bus_posting_group, "gen_prod_posting_group": purchLine.Gen_prod_posting_group})
	} else {
		err := genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": purchLine.Gen_bus_posting_group, "gen_prod_posting_group": purchLine.Gen_prod_posting_group})
		if err != nil {
			return err
		}
		if p.purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
			err = genPostingSetup.TestField("purch_credit_memo_account")
			glAccount = genPostingSetup.Purch_credit_memo_account
		} else {
			err = genPostingSetup.TestField("purch_account")
			glAccount = genPostingSetup.Purch_account
		}
		if err != nil {
			return err
		}
	}
	amount := lineLCY.Amount
	amountFCY := lineFCY.Amount
	discountPosting := p.purchSetup.Discount_posting
	postInvDisc := discountPosting == gtables.PurchasesPayablesSetup_Discount_posting.InvoiceDiscounts || discountPosting == gtables.PurchasesPayablesSetup_Discount_posting.AllDiscounts
	postLineDisc := discountPosting == gtables.PurchasesPayablesSetup_Discount_posting.LineDiscounts || discountPosting == gtables.PurchasesPayablesSetup_Discount_posting.AllDiscounts
	if postInvDisc && !lineLCY.Inv_discount_amount.IsZero() && purchLine.Type != gtables.PurchaseLine_Type.GLAccount {
		err := genPostingSetup.TestField("purch_inv_disc_account")
		if err != nil {
			return err
		}
		p.addToBuffer(&invPostingBuffer{glAccount: genPostingSetup.Purch_inv_disc_account, departmentCode: purchLine.Department_code, projectCode: purchLine.Project_code,
			amount: lineLCY.Inv_discount_amount.Neg(), amountFCY: lineFCY.Inv_discount_amount.Neg()})
		amount = amount.Add(lineLCY.Inv_discount_amount)
		amountFCY = amountFCY.Add(lineFCY.Inv_discount_amount)
	}
	if postLineDisc && !lineLCY.Line_discount_amount.IsZero() && purchLine.Type != gtables.PurchaseLine_Type.GLAccount {
		err := genPostingSetup.TestField("purch_line_disc_account")
		if err != nil {
			return err
		}
		p.addToBuffer(&invPostingBuffer{glAccount: genPostingSetup.Purch_line_disc_account, departmentCode: purchLine.Department_code, projectCode: purchLine.Project_code,
			amount: lineLCY.Line_discount_amount.Neg(), amountFCY: lineFCY.Line_discount_amount.Neg()})
		amount = amount.Add(lineLCY.Line_discount_amount)
		amountFCY = amountFCY.Add(lineFCY.Line_discount_amount)
	}
	taxAmount := lineLCY.Amount_including_tax.Sub(lineLCY.Amount)
	taxAmountFCY := lineFCY.Amount_including_tax.Sub(lineFCY.Amount)
	if purchLine.Tax_calculation_type == gtables.PurchaseLine_Tax_calculation_type.SalesTax && purchLine.Use_tax {
		// use tax: owed on the purchase, not charged by the vendor
		salesTaxCalculate := salestax.New(purchLine)
		var err error
		taxAmount, err = salesTaxCalculate.CalculateTax(purchLine.Tax_area_code, purchLine.Tax_group_code, purchLine.Tax_liable,
			p.purchHeader.Posting_date, lineLCY.Amount, lineLCY.Quantity, types.Decimal{})
		if err != nil {
			return err
		}
		taxAmountFCY, err = salesTaxCalculate.CalculateTax(purchLine.Tax_area_code, purchLine.Tax_group_code, purchLine.Tax_liable,
			p.purchHeader.Posting_date, lineFCY.Amount, lineFCY.Quantity, p.purchHeader.Currency_factor)
		if err != nil {
			return err
		}
	}
	p.addToBuffer(&invPostingBuffer{
		glAccount: glAccount, genPostingType: true,
		genBusPostingGrp: purchLine.Gen_bus_posting_group, genProdPostingGrp: purchLine.Gen_prod_posting_group,
		taxBusPostingGrp: purchLine.Tax_bus_posting_group, taxProdPostingGrp: purchLine.Tax_prod_posting_group,
		taxCalculationType: purchLine.Tax_calculation_type, taxAreaCode: purchLine.Tax_area_code, taxLiable: purchLine.Tax_liable, taxGroupCode: purchLine.Tax_group_code,
		useTax:         purchLine.Use_tax,
		departmentCode: purchLine.Department_code, projectCode: purchLine.Project_code,
		amount: amount, amountFCY: amountFCY,
		taxAmount: taxAmount, taxAmountFCY: taxAmountFCY,
		taxBaseAmount: lineLCY.Amount, taxBaseAmountFCY: lineFCY.Amount,
		quantity: lineLCY.Quantity,
	})
	return nil
}

// addToBuffer adds an amount to the buffer line of its account and groups
func (p *PurchPost) addToBuffer(add *invPostingBuffer) {
	for _, b := range p.invPostingBuffer {
		if b.glAccount == add.glAccount && b.genPostingType == add.genPostingType &&
			b.genBusPostingGrp == add.genBusPostingGrp && b.genProdPostingGrp == add.genProdPostingGrp &&
			b.taxBusPostingGrp == add.taxBusPostingGrp && b.taxProdPostingGrp == add.taxProdPostingGrp &&
			b.taxAreaCode == add.taxAreaCode && b.taxLiable == add.taxLiable && b.taxGroupCode == add.taxGroupCode && b.useTax == add.useTax &&
			b.departmentCode == add.departmentCode && b.projectCode == add.projectCode {
			b.amount = b.amount.Add(add.amount)
			b.amountFCY = b.amountFCY.Add(add.amountFCY)
			b.taxAmount = b.taxAmount.Add(add.taxAmount)
			b.taxAmountFCY = b.taxAmountFCY.Add(add.taxAmountFCY)
			b.taxBaseAmount = b.taxBaseAmount.Add(add.taxBaseAmount)
			b.taxBaseAmountFCY = b.taxBaseAmountFCY.Add(add.taxBaseAmountFCY)
			b.quantity = b.quantity.Add(add.quantity)
			return
		}
	}
	p.invPostingBuffer = append(p.invPostingBuffer, add)
}

// invoiceRounding (InvoiceRounding): with the setup's Invoice Rounding the total including tax
// is rounded to the currency's (else the General Ledger Setup's) invoice rounding precision; the
// difference goes to the vendor posting group's Invoice Rounding Account and onto the posted
// document as a line
func (p *PurchPost) invoiceRounding() error {
	if !p.purchSetup.Invoice_rounding {
		return nil
	}
	purchHeader := p.purchHeader
	var precision types.Decimal
	var direction string
	if purchHeader.Currency_code == "" {
		glSetup := gtables.GetGeneralLedgerSetup(purchHeader)
		precision = glSetup.Inv_rounding_precision_lcy
		direction = roundingDirection(int(glSetup.Inv_rounding_type_lcy))
	} else {
		precision = p.totals.Currency().Invoice_rounding_precision
		direction = roundingDirection(int(p.totals.Currency().Invoice_rounding_type))
	}
	if precision.IsZero() {
		return nil
	}
	total := p.totals.FCY().Amount_including_tax
	diff := system.Round(total, precision, direction).Sub(total)
	if diff.IsZero() {
		return nil
	}
	vendPostingGr := gtables.NewVendorPostingGroup(purchHeader)
	err := vendPostingGr.GetOrError(purchHeader.Vendor_posting_group)
	if err != nil {
		return err
	}
	err = vendPostingGr.TestField("invoice_rounding_account")
	if err != nil {
		return err
	}
	diffLCY := diff
	if purchHeader.Currency_code != "" {
		currExchRate := tables.ExchangeRates(purchHeader)
		amount, err := currExchRate.ExchangeAmtFCYToLCY(purchHeader.Posting_date, purchHeader.Currency_code, total.Add(diff), purchHeader.Currency_factor)
		if err != nil {
			return err
		}
		diffLCY = system.Round(amount).Sub(p.totals.LCY().Amount_including_tax)
	}
	p.addToBuffer(&invPostingBuffer{glAccount: vendPostingGr.Invoice_rounding_account, amount: diffLCY, amountFCY: diff})
	p.totals.FCY().Amount = p.totals.FCY().Amount.Add(diff)
	p.totals.FCY().Amount_including_tax = p.totals.FCY().Amount_including_tax.Add(diff)
	p.totals.LCY().Amount = p.totals.LCY().Amount.Add(diffLCY)
	p.totals.LCY().Amount_including_tax = p.totals.LCY().Amount_including_tax.Add(diffLCY)

	// the rounding line on the posted document
	roundingLine := gtables.NewPurchaseLine(purchHeader)
	roundingLine.Document_type = gtables.PurchaseLineDocument_type(purchHeader.Document_type)
	roundingLine.Document_no = purchHeader.No
	roundingLine.Line_no = p.lastLineNo() + 10000
	roundingLine.Type = gtables.PurchaseLine_Type.GLAccount
	roundingLine.No = vendPostingGr.Invoice_rounding_account
	glAcc := gtables.NewGLAccount(purchHeader)
	if glAcc.Get(roundingLine.No) {
		roundingLine.Description = glAcc.Name
	}
	roundingLine.Buy_from_vendor_no = purchHeader.Buy_from_vendor_no
	roundingLine.Pay_to_vendor_no = purchHeader.Pay_to_vendor_no
	roundingLine.Currency_code = purchHeader.Currency_code
	roundingLine.Quantity = types.NewDecimalFromInt(1)
	roundingLine.Quantity_base = roundingLine.Quantity
	roundingLine.Qty_per_unit_of_measure = roundingLine.Quantity
	roundingLine.Direct_unit_cost = diff
	roundingLine.Amount = diff
	roundingLine.Amount_including_tax = diff
	return p.insertPostedLines(roundingLine, types.Decimal{}, types.Decimal{}, roundingLine.Quantity, roundingLine.Quantity, roundingLine, roundingLine, 0)
}

// roundingDirection: an invoice rounding type (Nearest, Up, Down) as a Round direction
func roundingDirection(roundingType int) string {
	switch roundingType {
	case 1:
		return ">"
	case 2:
		return "<"
	}
	return "="
}

// lastLineNo: the highest line number of the document
func (p *PurchPost) lastLineNo() int {
	purchLine := gtables.NewPurchaseLine(p.purchHeader)
	purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(p.purchHeader.Document_type))
	purchLine.SetRange("document_no", p.purchHeader.No)
	if purchLine.FindLast() {
		return purchLine.Line_no
	}
	return 0
}

// postInvPostingBuffer posts the G/L amounts (codeunit 12): debited on an invoice, credited on
// a credit memo; a purchase amount with its sales tax (Manual VAT Entry; use tax: the tax owed,
// posted against the jurisdiction's reverse charge account)
func (p *PurchPost) postInvPostingBuffer() error {
	purchHeader := p.purchHeader
	for _, b := range p.invPostingBuffer {
		genJnlLine := p.newGenJnlLine()
		genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
		genJnlLine.Account_no = b.glAccount
		genJnlLine.Amount = b.amountFCY.Mul(p.sign)
		genJnlLine.Amount_lcy = b.amount.Mul(p.sign)
		genJnlLine.Source_currency_code = purchHeader.Currency_code
		genJnlLine.Source_currency_amount = genJnlLine.Amount
		genJnlLine.Department_code = b.departmentCode
		genJnlLine.Project_code = b.projectCode
		genJnlLine.Quantity = b.quantity.Mul(p.sign)
		if b.genPostingType {
			genJnlLine.Gen_posting_type = gtables.GenJournalLine_Gen_posting_type.Purchase
			genJnlLine.Gen_bus_posting_group = b.genBusPostingGrp
			genJnlLine.Gen_prod_posting_group = b.genProdPostingGrp
			genJnlLine.Tax_bus_posting_group = b.taxBusPostingGrp
			genJnlLine.Tax_prod_posting_group = b.taxProdPostingGrp
			genJnlLine.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(b.taxCalculationType)
			genJnlLine.Tax_area_code = b.taxAreaCode
			genJnlLine.Tax_liable = b.taxLiable
			genJnlLine.Tax_group_code = b.taxGroupCode
			genJnlLine.Use_tax = b.useTax
			genJnlLine.Vat_posting = gtables.GenJournalLine_Vat_posting.ManualVATEntry
			genJnlLine.Tax_amount = b.taxAmountFCY.Mul(p.sign)
			genJnlLine.Tax_amount_lcy = b.taxAmount.Mul(p.sign)
			genJnlLine.Tax_base_amount = b.taxBaseAmountFCY.Mul(p.sign)
			genJnlLine.Tax_base_amount_lcy = b.taxBaseAmount.Mul(p.sign)
			genJnlLine.Source_curr_tax_amount = genJnlLine.Tax_amount
			genJnlLine.Source_curr_tax_base_amount = genJnlLine.Tax_base_amount
		}
		err := p.genJnlPostLine.RunWithoutCheck(genJnlLine)
		if err != nil {
			return err
		}
	}
	return nil
}

// newGenJnlLine: a journal line of the document (dates, document and the vendor's document, the
// vendor, source code)
func (p *PurchPost) newGenJnlLine() *gtables.GenJournalLine {
	purchHeader := p.purchHeader
	genJnlLine := gtables.NewGenJournalLine(purchHeader)
	genJnlLine.Init()
	genJnlLine.Posting_date = purchHeader.Posting_date
	genJnlLine.Document_date = purchHeader.Document_date
	genJnlLine.Description = purchHeader.Posting_description
	genJnlLine.Reason_code = purchHeader.Reason_code
	genJnlLine.Document_type = p.genJnlLineDocType
	genJnlLine.Document_no = p.genJnlLineDocNo
	genJnlLine.External_document_no = p.genJnlLineExtDocNo
	genJnlLine.System_created_entry = true
	genJnlLine.Currency_code = purchHeader.Currency_code
	genJnlLine.Currency_factor = purchHeader.Currency_factor
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Vendor
	genJnlLine.Source_no = purchHeader.Pay_to_vendor_no
	genJnlLine.Source_code = p.srcCode
	genJnlLine.Posting_no_series = purchHeader.Posting_no_series
	genJnlLine.Sell_to_buy_from_no = purchHeader.Buy_from_vendor_no
	return genJnlLine
}

// postVendorEntry posts the vendor's entry of the document (codeunit 12): the total including
// tax (no use tax) credited, its purchase and invoice discount in LCY, its due and discount
// dates, applied to the entry of Applies-to Doc. No.
func (p *PurchPost) postVendorEntry() error {
	purchHeader := p.purchHeader
	totalFCY := p.totals.FCY()
	totalLCY := p.totals.LCY()
	genJnlLine := p.newGenJnlLine()
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Vendor
	genJnlLine.Account_no = purchHeader.Pay_to_vendor_no
	genJnlLine.Amount = totalFCY.Amount_including_tax.Mul(p.sign).Neg()
	genJnlLine.Amount_lcy = totalLCY.Amount_including_tax.Mul(p.sign).Neg()
	genJnlLine.Source_currency_code = purchHeader.Currency_code
	genJnlLine.Source_currency_amount = genJnlLine.Amount
	genJnlLine.Sales_purch_lcy = totalLCY.Amount.Mul(p.sign).Neg()
	genJnlLine.Inv_discount_lcy = totalLCY.Inv_discount_amount.Mul(p.sign).Neg()
	genJnlLine.Salespers_purch_code = purchHeader.Purchaser_code
	genJnlLine.Posting_group = purchHeader.Vendor_posting_group
	genJnlLine.Department_code = purchHeader.Department_code
	genJnlLine.Project_code = purchHeader.Project_code
	genJnlLine.Payment_terms_code = purchHeader.Payment_terms_code
	genJnlLine.Due_date = purchHeader.Due_date
	genJnlLine.Pmt_discount_date = purchHeader.Pmt_discount_date
	genJnlLine.Payment_discount_percent = purchHeader.Payment_discount_percent
	genJnlLine.Applies_to_doc_type = gtables.GenJournalLineApplies_to_doc_type(purchHeader.Applies_to_doc_type)
	genJnlLine.Applies_to_doc_no = purchHeader.Applies_to_doc_no
	if onVendorEntry != nil {
		onVendorEntry(genJnlLine)
	}
	return p.genJnlPostLine.RunWithoutCheck(genJnlLine)
}

// onVendorEntry (tests only) changes the vendor entry's journal line before it is posted: a
// posting whose G/L entries do not balance, which the balance check must refuse
var onVendorEntry func(genJnlLine *gtables.GenJournalLine)

// insertPostedLines writes a line's posted receipt line (what was received, what of it is
// invoiced) and posted invoice or credit memo line (the invoiced quantity and amounts)
func (p *PurchPost) insertPostedLines(purchLine *gtables.PurchaseLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineFCY, lineLCY *gtables.PurchaseLine, itemRcptEntryNo int) error {
	isText := purchLine.Type == gtables.PurchaseLine_Type.Blank
	if p.purchRcptHeader != nil && (!qtyToReceive.IsZero() || isText) {
		purchRcptLine := gtables.NewPurchRcptLine(purchLine)
		purchRcptLine.TransferFields(purchLine)
		purchRcptLine.Document_no = p.purchRcptHeader.No
		purchRcptLine.Quantity = qtyToReceive
		purchRcptLine.Quantity_base = qtyToReceiveBase
		invoicedNow := types.Decimal{}
		if !qtyToInvoice.IsZero() {
			invoicedNow = qtyToInvoice.Sub(purchLine.Qty_rcd_not_invoiced)
			if p.purchHeader.Document_type != gtables.PurchaseHeader_Document_type.Order {
				invoicedNow = qtyToInvoice
			}
			if invoicedNow.IsNegative() {
				invoicedNow = types.Decimal{}
			}
			if invoicedNow.GreaterThan(qtyToReceive) {
				invoicedNow = qtyToReceive
			}
		}
		invoicedNowBase := system.Round(invoicedNow.Mul(purchLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001"))
		purchRcptLine.Quantity_invoiced = invoicedNow
		purchRcptLine.Qty_invoiced_base = invoicedNowBase
		purchRcptLine.Qty_rcd_not_invoiced = qtyToReceive.Sub(invoicedNow)
		purchRcptLine.Qty_rcd_not_invd_base = qtyToReceiveBase.Sub(invoicedNowBase)
		if p.purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
			purchRcptLine.Order_no = purchLine.Document_no
			purchRcptLine.Order_line_no = purchLine.Line_no
		}
		purchRcptLine.Item_rcpt_entry_no = itemRcptEntryNo
		purchRcptLine.Amount = types.Decimal{}
		purchRcptLine.Amount_including_tax = types.Decimal{}
		purchRcptLine.Tax_base_amount = types.Decimal{}
		purchRcptLine.Line_discount_amount = types.Decimal{}
		purchRcptLine.Inv_discount_amount = types.Decimal{}
		err := purchRcptLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if !p.purchHeader.Invoice || qtyToInvoice.IsZero() && !isText {
		return nil
	}
	if p.purchCrMemoHeader != nil {
		purchCrMemoLine := gtables.NewPurchCrMemoLine(purchLine)
		purchCrMemoLine.TransferFields(purchLine)
		purchCrMemoLine.Document_no = p.purchCrMemoHeader.No
		purchCrMemoLine.Quantity = qtyToInvoice
		purchCrMemoLine.Quantity_base = qtyToInvoiceBase
		if lineFCY != nil {
			purchCrMemoLine.Amount = lineFCY.Amount
			purchCrMemoLine.Amount_including_tax = lineFCY.Amount_including_tax
			purchCrMemoLine.Tax_base_amount = lineFCY.Tax_base_amount
			purchCrMemoLine.Line_discount_amount = lineFCY.Line_discount_amount
			purchCrMemoLine.Inv_discount_amount = lineFCY.Inv_discount_amount
		}
		return purchCrMemoLine.InsertOrError(false)
	}
	purchInvLine := gtables.NewPurchInvLine(purchLine)
	purchInvLine.TransferFields(purchLine)
	purchInvLine.Document_no = p.purchInvHeader.No
	purchInvLine.Quantity = qtyToInvoice
	purchInvLine.Quantity_base = qtyToInvoiceBase
	if lineFCY != nil {
		purchInvLine.Amount = lineFCY.Amount
		purchInvLine.Amount_including_tax = lineFCY.Amount_including_tax
		purchInvLine.Tax_base_amount = lineFCY.Tax_base_amount
		purchInvLine.Line_discount_amount = lineFCY.Line_discount_amount
		purchInvLine.Inv_discount_amount = lineFCY.Inv_discount_amount
	}
	if p.purchRcptHeader != nil && !qtyToReceive.IsZero() {
		purchInvLine.Receipt_no = p.purchRcptHeader.No
		purchInvLine.Receipt_line_no = purchLine.Line_no
	}
	return purchInvLine.InsertOrError(false)
}

// updateOrderLine: the order line's received and invoiced quantities grow by what was posted;
// what is left is outstanding, to receive and to invoice
func (p *PurchPost) updateOrderLine(purchLine *gtables.PurchaseLine, qtyToReceive, qtyToReceiveBase, qtyToInvoice, qtyToInvoiceBase types.Decimal) error {
	purchLine.Quantity_received = purchLine.Quantity_received.Add(qtyToReceive)
	purchLine.Qty_received_base = purchLine.Qty_received_base.Add(qtyToReceiveBase)
	purchLine.Quantity_invoiced = purchLine.Quantity_invoiced.Add(qtyToInvoice)
	purchLine.Qty_invoiced_base = purchLine.Qty_invoiced_base.Add(qtyToInvoiceBase)
	err := tables.InitPurchLineOutstanding(purchLine)
	if err != nil {
		return err
	}
	tables.InitPurchLineQtyToReceive(purchLine)
	return purchLine.ModifyOrError(false)
}

// finalize: an order with everything invoiced, an invoice and a credit memo are deleted with
// their lines and comments; an order that goes on keeps the numbers last posted
func (p *PurchPost) finalize() error {
	purchHeader := p.purchHeader
	everythingInvoiced := true
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
		purchLine := gtables.NewPurchaseLine(purchHeader)
		purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
		purchLine.SetRange("document_no", purchHeader.No)
		for found := purchLine.FindSet(); found; found = purchLine.Next() {
			if !purchLine.Quantity_invoiced.Equal(purchLine.Quantity) {
				everythingInvoiced = false
			}
		}
	}
	if everythingInvoiced {
		purchLine := gtables.NewPurchaseLine(purchHeader)
		purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
		purchLine.SetRange("document_no", purchHeader.No)
		purchLine.DeleteAll()
		purchCommentLine := gtables.NewPurchCommentLine(purchHeader)
		purchCommentLine.SetRange("document_type", gtables.PurchCommentLineDocument_type(purchHeader.Document_type))
		purchCommentLine.SetRange("no", purchHeader.No)
		purchCommentLine.DeleteAll()
		p.Deleted = true
		return purchHeader.DeleteOrError(false)
	}
	if purchHeader.Receiving_no != "" {
		purchHeader.Last_receiving_no = purchHeader.Receiving_no
		purchHeader.Receiving_no = ""
	}
	if purchHeader.Posting_no != "" {
		purchHeader.Last_posting_no = purchHeader.Posting_no
		purchHeader.Posting_no = ""
	}
	purchHeader.Receive = false
	purchHeader.Invoice = false
	return purchHeader.ModifyOrError(false)
}
