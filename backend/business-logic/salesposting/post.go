// Package salesposting is codeunit 80 Sales-Post: the posting of the sales documents (its
// SumSalesLines, the documents' totals, is package salespost).
package salesposting

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/openerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/salespost"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// SalesPost is codeunit 80 Sales-Post: posts an order (ship and/or invoice), an invoice or a
// credit memo. Shipping writes the posted shipment and the items' ledger entries (codeunit
// 22); invoicing writes the posted invoice or credit memo, invoices the item entries (a line
// shipped before through its shipment's entry), posts the resources (codeunit 212), the sales,
// discount and invoice rounding amounts and their sales tax to the G/L and the customer entry
// (codeunit 12, applied by Applies-to Doc. No.). An order keeps what is left to ship and invoice
// and is deleted when everything is invoiced; an invoice or credit memo is deleted. The caller
// runs it in a transaction.
//
// Not migrated: the VAT calculation types (as in the journals), prepayments, drop shipments,
// reservations and item tracking, combined shipments, the credit limit and archive.
type SalesPost struct {
	src            ftables.Source
	salesSetup     *gtables.SalesReceivablesSetup
	srcCode        types.Code
	genJnlPostLine *genjnl.GenJnlPostLine
	itemJnlPost    *itemjnl.ItemJnlPostLine
	resJnlPostLine *resjnl.ResJnlPostLine

	salesHeader       *gtables.SalesHeader
	salesShptHeader   *gtables.SalesShipmentHeader
	salesInvHeader    *gtables.SalesInvoiceHeader
	salesCrMemoHeader *gtables.SalesCrMemoHeader
	createShipment    bool
	genJnlLineDocType gtables.GenJournalLineDocument_type
	genJnlLineDocNo   types.Code
	sign              types.Decimal // +1: an invoice, -1: a credit memo (amounts and quantities)

	totals           *salespost.Totals // the invoiced amounts in the document's currency and in LCY
	invPostingBuffer []*invPostingBuffer

	// What was posted: the posted documents' numbers, and whether the document is gone
	ShipmentNo types.Code
	InvoiceNo  types.Code
	CrMemoNo   types.Code
	Deleted    bool
}

// invPostingBuffer adds up the G/L amounts of the lines with the same account and posting
// groups (Invoice Post. Buffer, table 49)
type invPostingBuffer struct {
	glAccount          types.Code
	genPostingType     bool // a sales amount with tax (else a discount or rounding amount without)
	genBusPostingGrp   types.Code
	genProdPostingGrp  types.Code
	taxBusPostingGrp   types.Code
	taxProdPostingGrp  types.Code
	taxCalculationType gtables.SalesLineTax_calculation_type
	taxAreaCode        types.Code
	taxLiable          bool
	taxGroupCode       types.Code
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

// NewSalesPost makes codeunit 80 for a database and company
func NewSalesPost(src ftables.Source) *SalesPost {
	return &SalesPost{src: src}
}

// Run posts the document (a SalesPost can post one document after the other: each run starts
// afresh)
func (p *SalesPost) Run(salesHeader *gtables.SalesHeader) error {
	*p = SalesPost{src: p.src}
	p.salesHeader = salesHeader
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
	if salesHeader.Invoice {
		err = p.invoiceRounding()
		if err != nil {
			return err
		}
		err = p.postInvPostingBuffer()
		if err != nil {
			return err
		}
		err = p.postCustomerEntry()
		if err != nil {
			return err
		}
	}
	return p.finalize()
}

// checkAndUpdate: the document can be posted — its type, customer, dates, what it ships and
// invoices, something to post — and gets its posted documents' numbers
func (p *SalesPost) checkAndUpdate() error {
	salesHeader := p.salesHeader
	switch salesHeader.Document_type {
	case gtables.SalesHeader_Document_type.Order:
		if !salesHeader.Ship && !salesHeader.Invoice {
			return apperrors.SalesShipOrInvoice(gtables.SalesHeaderTableName, "ship", "invoice")
		}
	case gtables.SalesHeader_Document_type.Invoice:
		salesHeader.Ship = true
		salesHeader.Invoice = true
	case gtables.SalesHeader_Document_type.CreditMemo:
		salesHeader.Ship = false
		salesHeader.Invoice = true
	default:
		return apperrors.SalesCannotPostType(gtables.SalesHeaderTableName, "document_type", salesHeader.Document_type)
	}
	for _, field := range []string{"sell_to_customer_no", "bill_to_customer_no", "posting_date", "document_date"} {
		err := salesHeader.TestField(field)
		if err != nil {
			return err
		}
	}
	p.salesSetup = gtables.GetSalesReceivablesSetup(salesHeader)
	if p.salesSetup.Ext_doc_no_mandatory {
		err := salesHeader.TestField("external_document_no")
		if err != nil {
			return err
		}
	}
	for _, custNo := range []types.Code{salesHeader.Sell_to_customer_no, salesHeader.Bill_to_customer_no} {
		cust := gtables.NewCustomer(salesHeader)
		err := cust.GetOrError(custNo)
		if err != nil {
			return err
		}
		err = cust.TestField("blocked", false)
		if err != nil {
			return err
		}
	}
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
		p.sign = types.NewDecimalFromInt(-1)
	} else {
		p.sign = types.NewDecimalFromInt(1)
	}

	// something to post
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
	salesLine.SetRange("document_no", salesHeader.No)
	somethingToPost := false
	somethingToShip := false
	for found := salesLine.FindSet(); found; found = salesLine.Next() {
		qtyToShip, _, qtyToInvoice, _ := p.qtysToPost(salesLine)
		if !qtyToShip.IsZero() || !qtyToInvoice.IsZero() {
			somethingToPost = true
		}
		if !qtyToShip.IsZero() {
			somethingToShip = true
		}
	}
	if !somethingToPost {
		return apperrors.SalesNothingToPost()
	}

	// the posted documents' numbers
	p.createShipment = salesHeader.Ship && somethingToShip && (salesHeader.Document_type == gtables.SalesHeader_Document_type.Order ||
		salesHeader.Document_type == gtables.SalesHeader_Document_type.Invoice && p.salesSetup.Shipment_on_invoice)
	noSeriesMgt := noseries.New(salesHeader)
	if p.createShipment && salesHeader.Shipping_no == "" {
		err := salesHeader.TestField("shipping_no_series")
		if err != nil {
			return err
		}
		salesHeader.Shipping_no, err = noSeriesMgt.GetNextNo(salesHeader.Shipping_no_series, salesHeader.Posting_date, true)
		if err != nil {
			return err
		}
	}
	if salesHeader.Invoice && salesHeader.Posting_no == "" {
		if salesHeader.No_series != "" || salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
			err := salesHeader.TestField("posting_no_series")
			if err != nil {
				return err
			}
		}
		if salesHeader.No_series != salesHeader.Posting_no_series || salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
			var err error
			salesHeader.Posting_no, err = noSeriesMgt.GetNextNo(salesHeader.Posting_no_series, salesHeader.Posting_date, true)
			if err != nil {
				return err
			}
		} else {
			salesHeader.Posting_no = salesHeader.No
		}
	}
	err := salesHeader.ModifyOrError(false)
	if err != nil {
		return err
	}

	sourceCodeSetup := gtables.GetSourceCodeSetup(salesHeader)
	p.srcCode = sourceCodeSetup.Sales
	p.genJnlPostLine = genjnl.NewGenJnlPostLine(p.src)
	p.itemJnlPost = itemjnl.NewItemJnlPostLine(p.src)
	p.resJnlPostLine = resjnl.NewResJnlPostLine(p.src)
	var err2 error
	p.totals, err2 = salespost.NewTotals(salesHeader)
	return err2
}

// qtysToPost: what a line ships and invoices now (and in base units) — an order what is set to
// ship and invoice, an invoice its whole quantity, a credit memo its whole quantity invoiced
func (p *SalesPost) qtysToPost(salesLine *gtables.SalesLine) (qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase types.Decimal) {
	salesHeader := p.salesHeader
	switch salesHeader.Document_type {
	case gtables.SalesHeader_Document_type.Order:
		if salesHeader.Ship {
			qtyToShip = salesLine.Qty_to_ship
			qtyToShipBase = salesLine.Qty_to_ship_base
		}
		if salesHeader.Invoice {
			qtyToInvoice = salesLine.Qty_to_invoice
			qtyToInvoiceBase = salesLine.Qty_to_invoice_base
			// invoicing only: not more than is shipped and not invoiced
			if !salesHeader.Ship && qtyToInvoice.Abs().GreaterThan(salesLine.Qty_shipped_not_invoiced.Abs()) {
				qtyToInvoice = salesLine.Qty_shipped_not_invoiced
				qtyToInvoiceBase = salesLine.Qty_shpd_not_invd_base
			}
		}
	case gtables.SalesHeader_Document_type.Invoice:
		// a line got from a shipment (Get Shipment Lines) was shipped there
		if salesLine.Shipment_no == "" {
			qtyToShip = salesLine.Quantity
			qtyToShipBase = salesLine.Quantity_base
		}
		qtyToInvoice = salesLine.Quantity
		qtyToInvoiceBase = salesLine.Quantity_base
	case gtables.SalesHeader_Document_type.CreditMemo:
		qtyToInvoice = salesLine.Quantity
		qtyToInvoiceBase = salesLine.Quantity_base
	}
	return qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase
}

// insertPostedHeaders writes the posted shipment and the posted invoice or credit memo headers
// (the document's fields, the posted numbers) with the document's comments
func (p *SalesPost) insertPostedHeaders() error {
	salesHeader := p.salesHeader
	isOrder := salesHeader.Document_type == gtables.SalesHeader_Document_type.Order
	if p.createShipment {
		p.salesShptHeader = gtables.NewSalesShipmentHeader(salesHeader)
		p.salesShptHeader.LockTable()
		p.salesShptHeader.TransferFields(salesHeader)
		p.salesShptHeader.No = salesHeader.Shipping_no
		if isOrder {
			p.salesShptHeader.Order_no = salesHeader.No
		}
		p.salesShptHeader.No_series = salesHeader.Shipping_no_series
		p.salesShptHeader.Source_code = p.srcCode
		p.salesShptHeader.User_id = types.Code(system.UserId())
		p.salesShptHeader.No_printed = 0
		err := p.salesShptHeader.InsertOrError(false)
		if err != nil {
			return err
		}
		err = p.copyComments(gtables.SalesCommentLine_Document_type.Shipment, p.salesShptHeader.No)
		if err != nil {
			return err
		}
		p.ShipmentNo = p.salesShptHeader.No
	}
	if !salesHeader.Invoice {
		return nil
	}
	p.genJnlLineDocNo = salesHeader.Posting_no
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
		p.genJnlLineDocType = gtables.GenJournalLine_Document_type.CreditMemo
		p.salesCrMemoHeader = gtables.NewSalesCrMemoHeader(salesHeader)
		p.salesCrMemoHeader.LockTable()
		p.salesCrMemoHeader.TransferFields(salesHeader)
		p.salesCrMemoHeader.No = salesHeader.Posting_no
		p.salesCrMemoHeader.Pre_assigned_no = salesHeader.No
		p.salesCrMemoHeader.No_series = salesHeader.Posting_no_series
		p.salesCrMemoHeader.Source_code = p.srcCode
		p.salesCrMemoHeader.User_id = types.Code(system.UserId())
		p.salesCrMemoHeader.No_printed = 0
		err := p.salesCrMemoHeader.InsertOrError(false)
		if err != nil {
			return err
		}
		p.CrMemoNo = p.salesCrMemoHeader.No
		return p.copyComments(gtables.SalesCommentLine_Document_type.PostedCreditMemo, p.salesCrMemoHeader.No)
	}
	p.genJnlLineDocType = gtables.GenJournalLine_Document_type.Invoice
	p.salesInvHeader = gtables.NewSalesInvoiceHeader(salesHeader)
	p.salesInvHeader.LockTable()
	p.salesInvHeader.TransferFields(salesHeader)
	p.salesInvHeader.No = salesHeader.Posting_no
	if isOrder {
		p.salesInvHeader.Order_no = salesHeader.No
	} else {
		p.salesInvHeader.Pre_assigned_no = salesHeader.No
	}
	p.salesInvHeader.No_series = salesHeader.Posting_no_series
	p.salesInvHeader.Source_code = p.srcCode
	p.salesInvHeader.User_id = types.Code(system.UserId())
	p.salesInvHeader.No_printed = 0
	err := p.salesInvHeader.InsertOrError(false)
	if err != nil {
		return err
	}
	p.InvoiceNo = p.salesInvHeader.No
	return p.copyComments(gtables.SalesCommentLine_Document_type.PostedInvoice, p.salesInvHeader.No)
}

// copyComments: the document's comment lines for a posted document
func (p *SalesPost) copyComments(toDocType gtables.SalesCommentLineDocument_type, toNo types.Code) error {
	salesCommentLine := gtables.NewSalesCommentLine(p.salesHeader)
	salesCommentLine.SetRange("document_type", gtables.SalesCommentLineDocument_type(p.salesHeader.Document_type))
	salesCommentLine.SetRange("no", p.salesHeader.No)
	for found := salesCommentLine.FindSet(); found; found = salesCommentLine.Next() {
		salesCommentLine2 := gtables.NewSalesCommentLine(p.salesHeader)
		salesCommentLine2.TransferFields(salesCommentLine)
		salesCommentLine2.Document_type = toDocType
		salesCommentLine2.No = toNo
		err := salesCommentLine2.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// postLines posts each line: its item or resource entries, its share of the G/L amounts, its
// posted shipment and invoice/credit memo lines; an order line keeps the quantities posted
func (p *SalesPost) postLines() error {
	salesHeader := p.salesHeader
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
	salesLine.SetRange("document_no", salesHeader.No)
	var postedLines []*gtables.SalesLine
	for found := salesLine.FindSet(); found; found = salesLine.Next() {
		line := gtables.NewSalesLine(salesHeader)
		line.Copy(salesLine)
		postedLines = append(postedLines, line)
	}
	for _, salesLine := range postedLines {
		qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase := p.qtysToPost(salesLine)
		if salesLine.Type == gtables.SalesLine_Type.Blank {
			// a text line goes on the posted documents as it is
			err := p.insertPostedLines(salesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase, nil, nil, 0)
			if err != nil {
				return err
			}
			continue
		}
		if qtyToShip.IsZero() && qtyToInvoice.IsZero() {
			continue
		}
		// the invoiced share of the line's amounts, rounded so the totals stay exact
		var lineFCY, lineLCY *gtables.SalesLine
		if !qtyToInvoice.IsZero() {
			lineFCY, lineLCY = p.invoicedShare(salesLine, qtyToInvoice)
		}
		itemShptEntryNo := 0
		fromShipment := salesHeader.Document_type == gtables.SalesHeader_Document_type.Invoice && salesLine.Shipment_no != ""
		switch {
		case fromShipment:
			err := p.invoiceShipmentLine(salesLine, qtyToInvoice, qtyToInvoiceBase, lineLCY)
			if err != nil {
				return err
			}
			if salesLine.Type == gtables.SalesLine_Type.Resource {
				err = p.postResJnlLine(salesLine, qtyToInvoice, lineLCY)
				if err != nil {
					return err
				}
			}
		case salesLine.Type == gtables.SalesLine_Type.Item:
			var err error
			itemShptEntryNo, err = p.postItemLines(salesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase, lineLCY)
			if err != nil {
				return err
			}
		case salesLine.Type == gtables.SalesLine_Type.Resource:
			if !qtyToInvoice.IsZero() {
				err := p.postResJnlLine(salesLine, qtyToInvoice, lineLCY)
				if err != nil {
					return err
				}
			}
		}
		if !qtyToInvoice.IsZero() {
			err := p.fillInvPostingBuffer(salesLine, lineFCY, lineLCY)
			if err != nil {
				return err
			}
		}
		err := p.insertPostedLines(salesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase, lineFCY, lineLCY, itemShptEntryNo)
		if err != nil {
			return err
		}
		err = p.updateBlanketOrderLine(salesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase)
		if err != nil {
			return err
		}
		if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
			err = p.updateOrderLine(salesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// invoicedShare: the line's amounts for the quantity invoiced, in the document's currency and in
// LCY (with the cost of the quantity in LCY)
func (p *SalesPost) invoicedShare(salesLine *gtables.SalesLine, qtyToInvoice types.Decimal) (*gtables.SalesLine, *gtables.SalesLine) {
	return p.totals.InvoicedShare(salesLine, qtyToInvoice)
}

// postItemLines posts the item entries of a line: first the invoicing of what was shipped
// before (an entry without quantity on the shipment's entry), then what ships now with the
// rest invoiced; returns the entry of the shipment
func (p *SalesPost) postItemLines(salesLine *gtables.SalesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineLCY *gtables.SalesLine) (int, error) {
	invoicedBefore := types.Decimal{}
	if !qtyToInvoice.IsZero() && p.salesHeader.Document_type == gtables.SalesHeader_Document_type.Order && !salesLine.Qty_shipped_not_invoiced.IsZero() {
		left := qtyToInvoice.Sub(qtyToShip)
		if left.GreaterThan(salesLine.Qty_shipped_not_invoiced) {
			left = salesLine.Qty_shipped_not_invoiced
		}
		salesShptLine := gtables.NewSalesShipmentLine(salesLine)
		salesShptLine.SetRange("order_no", salesLine.Document_no)
		salesShptLine.SetRange("order_line_no", salesLine.Line_no)
		var shptLines []*gtables.SalesShipmentLine
		for found := salesShptLine.FindSet(); found; found = salesShptLine.Next() {
			if !salesShptLine.Qty_shipped_not_invoiced.IsZero() {
				line := gtables.NewSalesShipmentLine(salesLine)
				line.Copy(salesShptLine)
				shptLines = append(shptLines, line)
			}
		}
		for _, shptLine := range shptLines {
			if !left.IsPositive() {
				break
			}
			err := checkShipmentLine(shptLine, salesLine)
			if err != nil {
				return 0, err
			}
			qty := left
			if qty.GreaterThan(shptLine.Qty_shipped_not_invoiced) {
				qty = shptLine.Qty_shipped_not_invoiced
			}
			qtyBase := system.Round(qty.Mul(salesLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001"))
			_, err = p.postItemJnlLine(salesLine, types.Decimal{}, types.Decimal{}, qty, qtyBase, p.amountPart(lineLCY, qty, qtyToInvoice), shptLine.Item_shpt_entry_no)
			if err != nil {
				return 0, err
			}
			shptLine.Quantity_invoiced = shptLine.Quantity_invoiced.Add(qty)
			shptLine.Qty_invoiced_base = shptLine.Qty_invoiced_base.Add(qtyBase)
			shptLine.Qty_shipped_not_invoiced = shptLine.Qty_shipped_not_invoiced.Sub(qty)
			shptLine.Qty_shpd_not_invd_base = shptLine.Qty_shpd_not_invd_base.Sub(qtyBase)
			err = shptLine.ModifyOrError(false)
			if err != nil {
				return 0, err
			}
			left = left.Sub(qty)
			invoicedBefore = invoicedBefore.Add(qty)
		}
	}
	if qtyToShip.IsZero() && p.salesHeader.Document_type != gtables.SalesHeader_Document_type.CreditMemo {
		return 0, nil
	}
	invoicedNow := qtyToInvoice.Sub(invoicedBefore)
	invoicedNowBase := qtyToInvoiceBase.Sub(system.Round(invoicedBefore.Mul(salesLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001")))
	if p.salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
		// a credit memo takes the items back: a sale of the negative quantity, invoiced
		qtyToShip = qtyToInvoice
		qtyToShipBase = qtyToInvoiceBase
	}
	return p.postItemJnlLine(salesLine, qtyToShip, qtyToShipBase, invoicedNow, invoicedNowBase, p.amountPart(lineLCY, invoicedNow, qtyToInvoice), 0)
}

// invoiceShipmentLine invoices a line got from a shipment: an item's shipment entry is invoiced
// (an entry without quantity on it), the shipment line and the order line it came from count the
// quantity invoiced
func (p *SalesPost) invoiceShipmentLine(salesLine *gtables.SalesLine, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineLCY *gtables.SalesLine) error {
	salesShptLine := gtables.NewSalesShipmentLine(salesLine)
	err := salesShptLine.GetOrError(map[string]interface{}{"document_no": salesLine.Shipment_no, "line_no": salesLine.Shipment_line_no})
	if err == nil {
		err = checkShipmentLine(salesShptLine, salesLine)
	}
	if err != nil {
		return err
	}
	if salesLine.Type == gtables.SalesLine_Type.Item {
		_, err = p.postItemJnlLine(salesLine, types.Decimal{}, types.Decimal{}, qtyToInvoice, qtyToInvoiceBase, p.amountPart(lineLCY, qtyToInvoice, qtyToInvoice), salesShptLine.Item_shpt_entry_no)
		if err != nil {
			return err
		}
	}
	salesShptLine.Quantity_invoiced = salesShptLine.Quantity_invoiced.Add(qtyToInvoice)
	salesShptLine.Qty_invoiced_base = salesShptLine.Qty_invoiced_base.Add(qtyToInvoiceBase)
	salesShptLine.Qty_shipped_not_invoiced = salesShptLine.Qty_shipped_not_invoiced.Sub(qtyToInvoice)
	salesShptLine.Qty_shpd_not_invd_base = salesShptLine.Qty_shpd_not_invd_base.Sub(qtyToInvoiceBase)
	err = salesShptLine.ModifyOrError(false)
	if err != nil {
		return err
	}
	salesOrderLine := gtables.NewSalesLine(salesLine)
	if !salesOrderLine.Get(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order), "document_no": salesShptLine.Order_no, "line_no": salesShptLine.Order_line_no}) {
		return nil
	}
	return p.updateOrderLine(salesOrderLine, types.Decimal{}, types.Decimal{}, qtyToInvoice, qtyToInvoiceBase)
}

// amountPart: the LCY amounts of part of the invoiced quantity
func (p *SalesPost) amountPart(lineLCY *gtables.SalesLine, qty, qtyToInvoice types.Decimal) [2]types.Decimal {
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

// postItemJnlLine posts an item journal line of the sale (codeunit 22): the quantity shipped and
// invoiced with its amount and discount in LCY; returns the item ledger entry of a shipment
func (p *SalesPost) postItemJnlLine(salesLine *gtables.SalesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase types.Decimal, amounts [2]types.Decimal, itemShptEntryNo int) (int, error) {
	salesHeader := p.salesHeader
	itemJnlLine := gtables.NewItemJournalLine(salesHeader)
	itemJnlLine.Init()
	itemJnlLine.Posting_date = salesHeader.Posting_date
	itemJnlLine.Document_date = salesHeader.Document_date
	if !qtyToInvoice.IsZero() {
		itemJnlLine.Document_no = p.genJnlLineDocNo
	} else {
		itemJnlLine.Document_no = salesHeader.Shipping_no
	}
	if itemJnlLine.Document_no == "" {
		itemJnlLine.Document_no = p.genJnlLineDocNo
	}
	itemJnlLine.External_document_no = salesHeader.External_document_no
	itemJnlLine.Source_posting_group = salesHeader.Customer_posting_group
	itemJnlLine.Salespers_purch_code = salesHeader.Salesperson_code
	itemJnlLine.Reason_code = salesHeader.Reason_code
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.Sale
	itemJnlLine.Source_type = gtables.ItemJournalLine_Source_type.Customer
	itemJnlLine.Source_no = salesHeader.Sell_to_customer_no
	itemJnlLine.Item_no = salesLine.No
	itemJnlLine.Description = salesLine.Description
	itemJnlLine.Location_code = salesLine.Location_code
	itemJnlLine.Country_code = salesHeader.Vat_country_code
	itemJnlLine.Transaction_type = salesLine.Transaction_type
	itemJnlLine.Transport_method = salesLine.Transport_method
	itemJnlLine.Entry_exit_point = salesLine.Exit_point
	itemJnlLine.Area = salesLine.Area
	itemJnlLine.Transaction_specification = salesLine.Transaction_specification
	itemJnlLine.Variant_code = salesLine.Variant_code
	itemJnlLine.Bin_code = salesLine.Bin_code
	itemJnlLine.Serial_no = salesLine.Serial_no
	itemJnlLine.Lot_no = salesLine.Lot_no
	itemJnlLine.Inventory_posting_group = salesLine.Posting_group
	itemJnlLine.Department_code = salesLine.Department_code
	itemJnlLine.Project_code = salesLine.Project_code
	itemJnlLine.Gen_bus_posting_group = salesLine.Gen_bus_posting_group
	itemJnlLine.Gen_prod_posting_group = salesLine.Gen_prod_posting_group
	itemJnlLine.Unit_of_measure_code = salesLine.Unit_of_measure_code
	itemJnlLine.Qty_per_unit_of_measure = salesLine.Qty_per_unit_of_measure
	itemJnlLine.Source_code = p.srcCode
	itemJnlLine.Posting_no_series = salesHeader.Posting_no_series
	itemJnlLine.Quantity = qtyToShip.Mul(p.sign)
	itemJnlLine.Quantity_base = qtyToShipBase.Mul(p.sign)
	itemJnlLine.Invoiced_quantity = qtyToInvoice.Mul(p.sign)
	itemJnlLine.Invoiced_qty_base = qtyToInvoiceBase.Mul(p.sign)
	itemJnlLine.Unit_cost = salesLine.Unit_cost_lcy
	itemJnlLine.Source_currency_code = salesHeader.Currency_code
	itemJnlLine.Source_currency_unit_cost = salesLine.Unit_cost
	itemJnlLine.Item_shpt_entry_no = itemShptEntryNo
	itemJnlLine.Amount = amounts[0].Mul(p.sign)
	itemJnlLine.Discount_amount = amounts[1].Mul(p.sign)
	if !qtyToInvoice.IsZero() {
		itemJnlLine.Unit_amount = system.Round(amounts[0].Div(qtyToInvoice), types.MustDecimal("0.00001"))
	} else {
		itemJnlLine.Unit_amount = salesLine.Unit_price
	}
	itemJnlLine.Reserve = gtables.ItemJournalLineReserve(salesLine.Reserve)

	// the line's reservations go with the quantity shipped to the item journal line (codeunit 22
	// applies the reserved entries)
	if !salesLine.Reserved_qty_base.IsZero() && !qtyToShipBase.IsZero() {
		if !salesLine.Shipment_date.Equal(itemJnlLine.Posting_date) {
			return 0, apperrors.ReservShipmentDateNotPostingDate(gtables.SalesLineTableName, "shipment_date")
		}
		err := reservmgt.New(p.src).TransferSalesLineToItemJnlLine(salesLine, itemJnlLine, qtyToShipBase)
		if err != nil {
			return 0, err
		}
	}
	err := p.itemJnlPost.Run(itemJnlLine)
	if err != nil {
		return 0, err
	}
	return itemJnlLine.Item_shpt_entry_no, nil
}

// postResJnlLine posts a resource line's sale (Res. Jnl.-Post Line): the quantity invoiced, its
// cost and price in LCY
func (p *SalesPost) postResJnlLine(salesLine *gtables.SalesLine, qtyToInvoice types.Decimal, lineLCY *gtables.SalesLine) error {
	salesHeader := p.salesHeader
	resJnlLine := gtables.NewResJournalLine(salesHeader)
	resJnlLine.Init()
	resJnlLine.Posting_date = salesHeader.Posting_date
	resJnlLine.Document_date = salesHeader.Document_date
	resJnlLine.Reason_code = salesHeader.Reason_code
	resJnlLine.Entry_type = gtables.ResJournalLine_Entry_type.Sale
	resJnlLine.Document_no = p.genJnlLineDocNo
	resJnlLine.External_document_no = salesHeader.External_document_no
	resJnlLine.Resource_no = salesLine.No
	resJnlLine.Description = salesLine.Description
	resJnlLine.Work_type_code = salesLine.Work_type_code
	resJnlLine.Unit_of_measure_code = salesLine.Unit_of_measure_code
	resJnlLine.Department_code = salesLine.Department_code
	resJnlLine.Project_code = salesLine.Project_code
	resJnlLine.Gen_bus_posting_group = salesLine.Gen_bus_posting_group
	resJnlLine.Gen_prod_posting_group = salesLine.Gen_prod_posting_group
	resJnlLine.Source_code = p.srcCode
	resJnlLine.Posting_no_series = salesHeader.Posting_no_series
	resJnlLine.Chargeable = true
	resJnlLine.Quantity = qtyToInvoice.Mul(p.sign) // a sale: codeunit 212 turns the signs
	resJnlLine.Unit_cost = salesLine.Unit_cost_lcy
	resJnlLine.Total_cost = system.Round(resJnlLine.Quantity.Mul(resJnlLine.Unit_cost))
	resJnlLine.Total_price = lineLCY.Amount.Mul(p.sign)
	resJnlLine.Unit_price = system.Round(lineLCY.Amount.Div(qtyToInvoice), types.MustDecimal("0.00001"))
	return p.resJnlPostLine.Run(resJnlLine)
}

// fillInvPostingBuffer adds a line's invoiced amounts to the G/L amounts: the sales (credit
// memo) account of its General Posting Setup — a G/L account line its own account — with its
// sales tax, and, as the setup's Discount Posting says, the line and invoice discounts on their
// accounts
func (p *SalesPost) fillInvPostingBuffer(salesLine, lineFCY, lineLCY *gtables.SalesLine) error {
	var glAccount types.Code
	genPostingSetup := gtables.NewGeneralPostingSetup(salesLine)
	if salesLine.Type == gtables.SalesLine_Type.GLAccount {
		glAccount = salesLine.No
		genPostingSetup.Get(map[string]interface{}{"gen_bus_posting_group": salesLine.Gen_bus_posting_group, "gen_prod_posting_group": salesLine.Gen_prod_posting_group})
	} else {
		err := genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": salesLine.Gen_bus_posting_group, "gen_prod_posting_group": salesLine.Gen_prod_posting_group})
		if err != nil {
			return err
		}
		if p.salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
			err = genPostingSetup.TestField("sales_credit_memo_account")
			glAccount = genPostingSetup.Sales_credit_memo_account
		} else {
			err = genPostingSetup.TestField("sales_account")
			glAccount = genPostingSetup.Sales_account
		}
		if err != nil {
			return err
		}
	}
	amount := lineLCY.Amount
	amountFCY := lineFCY.Amount
	discountPosting := p.salesSetup.Discount_posting
	postInvDisc := discountPosting == gtables.SalesReceivablesSetup_Discount_posting.InvoiceDiscounts || discountPosting == gtables.SalesReceivablesSetup_Discount_posting.AllDiscounts
	postLineDisc := discountPosting == gtables.SalesReceivablesSetup_Discount_posting.LineDiscounts || discountPosting == gtables.SalesReceivablesSetup_Discount_posting.AllDiscounts
	if postInvDisc && !lineLCY.Inv_discount_amount.IsZero() && salesLine.Type != gtables.SalesLine_Type.GLAccount {
		err := genPostingSetup.TestField("sales_inv_disc_account")
		if err != nil {
			return err
		}
		p.addToBuffer(&invPostingBuffer{glAccount: genPostingSetup.Sales_inv_disc_account, departmentCode: salesLine.Department_code, projectCode: salesLine.Project_code,
			amount: lineLCY.Inv_discount_amount.Neg(), amountFCY: lineFCY.Inv_discount_amount.Neg()})
		amount = amount.Add(lineLCY.Inv_discount_amount)
		amountFCY = amountFCY.Add(lineFCY.Inv_discount_amount)
	}
	if postLineDisc && !lineLCY.Line_discount_amount.IsZero() && salesLine.Type != gtables.SalesLine_Type.GLAccount {
		err := genPostingSetup.TestField("sales_line_disc_account")
		if err != nil {
			return err
		}
		p.addToBuffer(&invPostingBuffer{glAccount: genPostingSetup.Sales_line_disc_account, departmentCode: salesLine.Department_code, projectCode: salesLine.Project_code,
			amount: lineLCY.Line_discount_amount.Neg(), amountFCY: lineFCY.Line_discount_amount.Neg()})
		amount = amount.Add(lineLCY.Line_discount_amount)
		amountFCY = amountFCY.Add(lineFCY.Line_discount_amount)
	}
	p.addToBuffer(&invPostingBuffer{
		glAccount: glAccount, genPostingType: true,
		genBusPostingGrp: salesLine.Gen_bus_posting_group, genProdPostingGrp: salesLine.Gen_prod_posting_group,
		taxBusPostingGrp: salesLine.Tax_bus_posting_group, taxProdPostingGrp: salesLine.Tax_prod_posting_group,
		taxCalculationType: salesLine.Tax_calculation_type, taxAreaCode: salesLine.Tax_area_code, taxLiable: salesLine.Tax_liable, taxGroupCode: salesLine.Tax_group_code,
		departmentCode: salesLine.Department_code, projectCode: salesLine.Project_code,
		amount: amount, amountFCY: amountFCY,
		taxAmount: lineLCY.Amount_including_tax.Sub(lineLCY.Amount), taxAmountFCY: lineFCY.Amount_including_tax.Sub(lineFCY.Amount),
		taxBaseAmount: lineLCY.Amount, taxBaseAmountFCY: lineFCY.Amount,
		quantity: lineLCY.Quantity,
	})
	return nil
}

// addToBuffer adds an amount to the buffer line of its account and groups
func (p *SalesPost) addToBuffer(add *invPostingBuffer) {
	for _, b := range p.invPostingBuffer {
		if b.glAccount == add.glAccount && b.genPostingType == add.genPostingType &&
			b.genBusPostingGrp == add.genBusPostingGrp && b.genProdPostingGrp == add.genProdPostingGrp &&
			b.taxBusPostingGrp == add.taxBusPostingGrp && b.taxProdPostingGrp == add.taxProdPostingGrp &&
			b.taxAreaCode == add.taxAreaCode && b.taxLiable == add.taxLiable && b.taxGroupCode == add.taxGroupCode &&
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
// difference goes to the customer posting group's Invoice Rounding Account and onto the posted
// document as a line
func (p *SalesPost) invoiceRounding() error {
	if !p.salesSetup.Invoice_rounding {
		return nil
	}
	salesHeader := p.salesHeader
	var precision types.Decimal
	var direction string
	if salesHeader.Currency_code == "" {
		glSetup := gtables.GetGeneralLedgerSetup(salesHeader)
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
	custPostingGr := gtables.NewCustomerPostingGroup(salesHeader)
	err := custPostingGr.GetOrError(salesHeader.Customer_posting_group)
	if err != nil {
		return err
	}
	err = custPostingGr.TestField("invoice_rounding_account")
	if err != nil {
		return err
	}
	diffLCY := diff
	if salesHeader.Currency_code != "" {
		currExchRate := tables.ExchangeRates(salesHeader)
		amount, err := currExchRate.ExchangeAmtFCYToLCY(salesHeader.Posting_date, salesHeader.Currency_code, total.Add(diff), salesHeader.Currency_factor)
		if err != nil {
			return err
		}
		diffLCY = system.Round(amount).Sub(p.totals.LCY().Amount_including_tax)
	}
	p.addToBuffer(&invPostingBuffer{glAccount: custPostingGr.Invoice_rounding_account, amount: diffLCY, amountFCY: diff})
	p.totals.FCY().Amount = p.totals.FCY().Amount.Add(diff)
	p.totals.FCY().Amount_including_tax = p.totals.FCY().Amount_including_tax.Add(diff)
	p.totals.LCY().Amount = p.totals.LCY().Amount.Add(diffLCY)
	p.totals.LCY().Amount_including_tax = p.totals.LCY().Amount_including_tax.Add(diffLCY)

	// the rounding line on the posted document
	roundingLine := gtables.NewSalesLine(salesHeader)
	roundingLine.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
	roundingLine.Document_no = salesHeader.No
	roundingLine.Line_no = p.lastLineNo() + 10000
	roundingLine.Type = gtables.SalesLine_Type.GLAccount
	roundingLine.No = custPostingGr.Invoice_rounding_account
	glAcc := gtables.NewGLAccount(salesHeader)
	if glAcc.Get(roundingLine.No) {
		roundingLine.Description = glAcc.Name
	}
	roundingLine.Sell_to_customer_no = salesHeader.Sell_to_customer_no
	roundingLine.Bill_to_customer_no = salesHeader.Bill_to_customer_no
	roundingLine.Currency_code = salesHeader.Currency_code
	roundingLine.Quantity = types.NewDecimalFromInt(1)
	roundingLine.Quantity_base = roundingLine.Quantity
	roundingLine.Qty_per_unit_of_measure = roundingLine.Quantity
	roundingLine.Unit_price = diff
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
func (p *SalesPost) lastLineNo() int {
	salesLine := gtables.NewSalesLine(p.salesHeader)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(p.salesHeader.Document_type))
	salesLine.SetRange("document_no", p.salesHeader.No)
	if salesLine.FindLast() {
		return salesLine.Line_no
	}
	return 0
}

// postInvPostingBuffer posts the G/L amounts (codeunit 12): credited on an invoice, debited on
// a credit memo; a sales amount with its sales tax (Manual VAT Entry)
func (p *SalesPost) postInvPostingBuffer() error {
	salesHeader := p.salesHeader
	for _, b := range p.invPostingBuffer {
		genJnlLine := p.newGenJnlLine()
		genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
		genJnlLine.Account_no = b.glAccount
		genJnlLine.Amount = b.amountFCY.Mul(p.sign).Neg()
		genJnlLine.Amount_lcy = b.amount.Mul(p.sign).Neg()
		genJnlLine.Source_currency_code = salesHeader.Currency_code
		genJnlLine.Source_currency_amount = genJnlLine.Amount
		genJnlLine.Department_code = b.departmentCode
		genJnlLine.Project_code = b.projectCode
		genJnlLine.Quantity = b.quantity.Mul(p.sign).Neg()
		if b.genPostingType {
			genJnlLine.Gen_posting_type = gtables.GenJournalLine_Gen_posting_type.Sale
			genJnlLine.Gen_bus_posting_group = b.genBusPostingGrp
			genJnlLine.Gen_prod_posting_group = b.genProdPostingGrp
			genJnlLine.Tax_bus_posting_group = b.taxBusPostingGrp
			genJnlLine.Tax_prod_posting_group = b.taxProdPostingGrp
			genJnlLine.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(b.taxCalculationType)
			genJnlLine.Tax_area_code = b.taxAreaCode
			genJnlLine.Tax_liable = b.taxLiable
			genJnlLine.Tax_group_code = b.taxGroupCode
			genJnlLine.Vat_posting = gtables.GenJournalLine_Vat_posting.ManualVATEntry
			genJnlLine.Tax_amount = b.taxAmountFCY.Mul(p.sign).Neg()
			genJnlLine.Tax_amount_lcy = b.taxAmount.Mul(p.sign).Neg()
			genJnlLine.Tax_base_amount = b.taxBaseAmountFCY.Mul(p.sign).Neg()
			genJnlLine.Tax_base_amount_lcy = b.taxBaseAmount.Mul(p.sign).Neg()
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

// newGenJnlLine: a journal line of the document (dates, document, customer, source code)
func (p *SalesPost) newGenJnlLine() *gtables.GenJournalLine {
	salesHeader := p.salesHeader
	genJnlLine := gtables.NewGenJournalLine(salesHeader)
	genJnlLine.Init()
	genJnlLine.Posting_date = salesHeader.Posting_date
	genJnlLine.Document_date = salesHeader.Document_date
	genJnlLine.Description = salesHeader.Posting_description
	genJnlLine.Reason_code = salesHeader.Reason_code
	genJnlLine.Document_type = p.genJnlLineDocType
	genJnlLine.Document_no = p.genJnlLineDocNo
	genJnlLine.External_document_no = salesHeader.External_document_no
	genJnlLine.System_created_entry = true
	genJnlLine.Currency_code = salesHeader.Currency_code
	genJnlLine.Currency_factor = salesHeader.Currency_factor
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Customer
	genJnlLine.Source_no = salesHeader.Bill_to_customer_no
	genJnlLine.Source_code = p.srcCode
	genJnlLine.Posting_no_series = salesHeader.Posting_no_series
	genJnlLine.Sell_to_buy_from_no = salesHeader.Sell_to_customer_no
	return genJnlLine
}

// postCustomerEntry posts the customer's entry of the document (codeunit 12): the total
// including tax, its sales, profit and invoice discount in LCY, its due and discount dates,
// applied to the entry of Applies-to Doc. No.
func (p *SalesPost) postCustomerEntry() error {
	salesHeader := p.salesHeader
	totalFCY := p.totals.FCY()
	totalLCY := p.totals.LCY()
	genJnlLine := p.newGenJnlLine()
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Customer
	genJnlLine.Account_no = salesHeader.Bill_to_customer_no
	genJnlLine.Amount = totalFCY.Amount_including_tax.Mul(p.sign)
	genJnlLine.Amount_lcy = totalLCY.Amount_including_tax.Mul(p.sign)
	genJnlLine.Source_currency_code = salesHeader.Currency_code
	genJnlLine.Source_currency_amount = genJnlLine.Amount
	genJnlLine.Sales_purch_lcy = totalLCY.Amount.Mul(p.sign)
	genJnlLine.Profit_lcy = totalLCY.Amount.Sub(totalLCY.Unit_cost_lcy).Mul(p.sign)
	genJnlLine.Inv_discount_lcy = totalLCY.Inv_discount_amount.Mul(p.sign)
	genJnlLine.Salespers_purch_code = salesHeader.Salesperson_code
	genJnlLine.Posting_group = salesHeader.Customer_posting_group
	genJnlLine.Department_code = salesHeader.Department_code
	genJnlLine.Project_code = salesHeader.Project_code
	genJnlLine.Payment_terms_code = salesHeader.Payment_terms_code
	genJnlLine.Due_date = salesHeader.Due_date
	genJnlLine.Pmt_discount_date = salesHeader.Pmt_discount_date
	genJnlLine.Payment_discount_percent = salesHeader.Payment_discount_percent
	genJnlLine.Applies_to_doc_type = gtables.GenJournalLineApplies_to_doc_type(salesHeader.Applies_to_doc_type)
	genJnlLine.Applies_to_doc_no = salesHeader.Applies_to_doc_no
	if onCustomerEntry != nil {
		onCustomerEntry(genJnlLine)
	}
	return p.genJnlPostLine.RunWithoutCheck(genJnlLine)
}

// onCustomerEntry (tests only) changes the customer entry's journal line before it is posted:
// a posting whose G/L entries do not balance, which the balance check must refuse
var onCustomerEntry func(genJnlLine *gtables.GenJournalLine)

// insertPostedLines writes a line's posted shipment line (what shipped, what of it is invoiced)
// and posted invoice or credit memo line (the invoiced quantity and amounts)
func (p *SalesPost) insertPostedLines(salesLine *gtables.SalesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineFCY, lineLCY *gtables.SalesLine, itemShptEntryNo int) error {
	isText := salesLine.Type == gtables.SalesLine_Type.Blank
	if p.salesShptHeader != nil && (!qtyToShip.IsZero() || isText) {
		salesShptLine := gtables.NewSalesShipmentLine(salesLine)
		salesShptLine.TransferFields(salesLine)
		salesShptLine.Document_no = p.salesShptHeader.No
		salesShptLine.Quantity = qtyToShip
		salesShptLine.Quantity_base = qtyToShipBase
		invoicedNow := types.Decimal{}
		if !qtyToInvoice.IsZero() {
			invoicedNow = qtyToInvoice.Sub(salesLine.Qty_shipped_not_invoiced)
			if p.salesHeader.Document_type != gtables.SalesHeader_Document_type.Order {
				invoicedNow = qtyToInvoice
			}
			if invoicedNow.IsNegative() {
				invoicedNow = types.Decimal{}
			}
			if invoicedNow.GreaterThan(qtyToShip) {
				invoicedNow = qtyToShip
			}
		}
		invoicedNowBase := system.Round(invoicedNow.Mul(salesLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001"))
		salesShptLine.Quantity_invoiced = invoicedNow
		salesShptLine.Qty_invoiced_base = invoicedNowBase
		salesShptLine.Qty_shipped_not_invoiced = qtyToShip.Sub(invoicedNow)
		salesShptLine.Qty_shpd_not_invd_base = qtyToShipBase.Sub(invoicedNowBase)
		if p.salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
			salesShptLine.Order_no = salesLine.Document_no
			salesShptLine.Order_line_no = salesLine.Line_no
		}
		salesShptLine.Item_shpt_entry_no = itemShptEntryNo
		salesShptLine.Amount = types.Decimal{}
		salesShptLine.Amount_including_tax = types.Decimal{}
		salesShptLine.Tax_base_amount = types.Decimal{}
		salesShptLine.Line_discount_amount = types.Decimal{}
		salesShptLine.Inv_discount_amount = types.Decimal{}
		err := salesShptLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	if !p.salesHeader.Invoice || qtyToInvoice.IsZero() && !isText {
		return nil
	}
	if p.salesCrMemoHeader != nil {
		salesCrMemoLine := gtables.NewSalesCrMemoLine(salesLine)
		salesCrMemoLine.TransferFields(salesLine)
		salesCrMemoLine.Document_no = p.salesCrMemoHeader.No
		salesCrMemoLine.Quantity = qtyToInvoice
		salesCrMemoLine.Quantity_base = qtyToInvoiceBase
		if lineFCY != nil {
			salesCrMemoLine.Amount = lineFCY.Amount
			salesCrMemoLine.Amount_including_tax = lineFCY.Amount_including_tax
			salesCrMemoLine.Tax_base_amount = lineFCY.Tax_base_amount
			salesCrMemoLine.Line_discount_amount = lineFCY.Line_discount_amount
			salesCrMemoLine.Inv_discount_amount = lineFCY.Inv_discount_amount
		}
		return salesCrMemoLine.InsertOrError(false)
	}
	salesInvLine := gtables.NewSalesInvoiceLine(salesLine)
	salesInvLine.TransferFields(salesLine)
	salesInvLine.Document_no = p.salesInvHeader.No
	salesInvLine.Quantity = qtyToInvoice
	salesInvLine.Quantity_base = qtyToInvoiceBase
	if lineFCY != nil {
		salesInvLine.Amount = lineFCY.Amount
		salesInvLine.Amount_including_tax = lineFCY.Amount_including_tax
		salesInvLine.Tax_base_amount = lineFCY.Tax_base_amount
		salesInvLine.Line_discount_amount = lineFCY.Line_discount_amount
		salesInvLine.Inv_discount_amount = lineFCY.Inv_discount_amount
	}
	if p.salesShptHeader != nil && !qtyToShip.IsZero() {
		salesInvLine.Shipment_no = p.salesShptHeader.No
		salesInvLine.Shipment_line_no = salesLine.Line_no
	}
	return salesInvLine.InsertOrError(false)
}

// updateBlanketOrderLine (UpdateBlanketOrderLine): a line taken from a blanket order line adds
// what it ships and invoices (a credit memo: takes back) to the blanket order line, in the
// blanket line's unit of measure; the blanket line cannot ship more than its quantity
func (p *SalesPost) updateBlanketOrderLine(salesLine *gtables.SalesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase types.Decimal) error {
	if salesLine.Blanket_order_no == "" || salesLine.Blanket_order_line_no == 0 || (qtyToShip.IsZero() && qtyToInvoice.IsZero()) {
		return nil
	}
	blanketOrderSalesLine := gtables.NewSalesLine(salesLine)
	if !blanketOrderSalesLine.Get(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.BlanketOrder),
		"document_no": salesLine.Blanket_order_no, "line_no": salesLine.Blanket_order_line_no}) {
		return nil
	}
	err := blanketOrderSalesLine.TestField("type", salesLine.Type)
	if err == nil {
		err = blanketOrderSalesLine.TestField("no", salesLine.No)
	}
	if err == nil {
		err = blanketOrderSalesLine.TestField("sell_to_customer_no", salesLine.Sell_to_customer_no)
	}
	if err != nil {
		return err
	}
	sign := types.NewDecimalFromInt(1)
	if salesLine.Document_type == gtables.SalesLine_Document_type.CreditMemo {
		sign = sign.Neg()
	}
	// a quantity in the blanket line's unit of measure
	inBlanketUnit := func(qty types.Decimal) types.Decimal {
		if blanketOrderSalesLine.Qty_per_unit_of_measure.Equal(salesLine.Qty_per_unit_of_measure) || blanketOrderSalesLine.Qty_per_unit_of_measure.IsZero() {
			return qty
		}
		return system.Round(salesLine.Qty_per_unit_of_measure.Div(blanketOrderSalesLine.Qty_per_unit_of_measure).Mul(qty), types.MustDecimal("0.00001"))
	}
	if !qtyToShip.IsZero() {
		blanketOrderSalesLine.Quantity_shipped = blanketOrderSalesLine.Quantity_shipped.Add(sign.Mul(inBlanketUnit(qtyToShip)))
		blanketOrderSalesLine.Qty_shipped_base = blanketOrderSalesLine.Qty_shipped_base.Add(sign.Mul(qtyToShipBase))
	}
	if !qtyToInvoice.IsZero() {
		blanketOrderSalesLine.Quantity_invoiced = blanketOrderSalesLine.Quantity_invoiced.Add(sign.Mul(inBlanketUnit(qtyToInvoice)))
		blanketOrderSalesLine.Qty_invoiced_base = blanketOrderSalesLine.Qty_invoiced_base.Add(sign.Mul(qtyToInvoiceBase))
	}
	err = tables.InitSalesLineOutstanding(blanketOrderSalesLine)
	if err != nil {
		return err
	}
	if blanketOrderSalesLine.Quantity.Mul(blanketOrderSalesLine.Quantity_shipped).IsNegative() ||
		blanketOrderSalesLine.Quantity.Abs().LessThan(blanketOrderSalesLine.Quantity_shipped.Abs()) {
		return apperrors.BlanketOrderQtyTooHigh(gtables.SalesLineTableName, "quantity_shipped", "quantity")
	}
	if blanketOrderSalesLine.Quantity_base.Mul(blanketOrderSalesLine.Qty_shipped_base).IsNegative() ||
		blanketOrderSalesLine.Quantity_base.Abs().LessThan(blanketOrderSalesLine.Qty_shipped_base.Abs()) {
		return apperrors.BlanketOrderQtyTooHigh(gtables.SalesLineTableName, "qty_shipped_base", "quantity_base")
	}
	return blanketOrderSalesLine.ModifyOrError(false)
}

// updateOrderLine: the order line's shipped and invoiced quantities grow by what was posted;
// what is left is outstanding, to ship and to invoice
func (p *SalesPost) updateOrderLine(salesLine *gtables.SalesLine, qtyToShip, qtyToShipBase, qtyToInvoice, qtyToInvoiceBase types.Decimal) error {
	salesLine.Quantity_shipped = salesLine.Quantity_shipped.Add(qtyToShip)
	salesLine.Qty_shipped_base = salesLine.Qty_shipped_base.Add(qtyToShipBase)
	salesLine.Quantity_invoiced = salesLine.Quantity_invoiced.Add(qtyToInvoice)
	salesLine.Qty_invoiced_base = salesLine.Qty_invoiced_base.Add(qtyToInvoiceBase)
	err := tables.InitSalesLineOutstanding(salesLine)
	if err != nil {
		return err
	}
	tables.InitSalesLineQtyToShip(salesLine)
	return salesLine.ModifyOrError(false)
}

// finalize: an order with everything invoiced, an invoice and a credit memo are deleted with
// their lines and comments; an order that goes on keeps the numbers last posted
func (p *SalesPost) finalize() error {
	salesHeader := p.salesHeader
	everythingInvoiced := true
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
		salesLine := gtables.NewSalesLine(salesHeader)
		salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
		salesLine.SetRange("document_no", salesHeader.No)
		for found := salesLine.FindSet(); found; found = salesLine.Next() {
			if !salesLine.Quantity_invoiced.Equal(salesLine.Quantity) {
				everythingInvoiced = false
			}
		}
	}
	if everythingInvoiced {
		salesLine := gtables.NewSalesLine(salesHeader)
		salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
		salesLine.SetRange("document_no", salesHeader.No)
		salesLine.DeleteAll()
		salesCommentLine := gtables.NewSalesCommentLine(salesHeader)
		salesCommentLine.SetRange("document_type", gtables.SalesCommentLineDocument_type(salesHeader.Document_type))
		salesCommentLine.SetRange("no", salesHeader.No)
		salesCommentLine.DeleteAll()
		p.Deleted = true
		return salesHeader.DeleteOrError(false)
	}
	if salesHeader.Shipping_no != "" {
		salesHeader.Last_shipping_no = salesHeader.Shipping_no
		salesHeader.Shipping_no = ""
	}
	if salesHeader.Posting_no != "" {
		salesHeader.Last_posting_no = salesHeader.Posting_no
		salesHeader.Posting_no = ""
	}
	salesHeader.Ship = false
	salesHeader.Invoice = false
	return salesHeader.ModifyOrError(false)
}

// checkShipmentLine: a shipment line invoiced now is still the line's — customer, type, number,
// posting groups, location, department, project, serial number, unit of measure, bin, lot
// number and variant (classic TESTFIELD(…, SalesLine.…) before invoicing a shipment)
func checkShipmentLine(salesShptLine *gtables.SalesShipmentLine, salesLine *gtables.SalesLine) error {
	for _, field := range []string{"sell_to_customer_no", "type", "no", "gen_bus_posting_group", "gen_prod_posting_group",
		"tax_bus_posting_group", "tax_prod_posting_group", "location_code", "department_code", "project_code",
		"serial_no", "unit_of_measure_code", "bin_code", "lot_no", "variant_code"} {
		value, _ := salesLine.FieldValue(salesLine.FieldNo(field))
		err := salesShptLine.TestField(field, value)
		if err != nil {
			return err
		}
	}
	return nil
}
