package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Reports 292 Copy Sales Document and 492 Copy Purchase Document (Copy Document on the document
// cards): the lines — with Include Header also the header — of another document, open (quote,
// blanket order, order, invoice, credit memo) or posted (shipment/receipt, invoice, credit memo),
// copied onto the document the report runs from. Recalculate Lines enters the lines again from
// their type, number, quantity and unit (prices and discounts of the customer or vendor; lines of
// G/L accounts without Direct Posting are not copied); otherwise the lines are copied as they are,
// with nothing shipped/received or invoiced. A posted shipment/receipt is always recalculated.
// With Include Header the document takes the other's header fields (its own number and series
// stay) — after deleting its lines, when the user agrees —, and a credit memo made from a posted
// invoice (or a document from a posted credit memo) applies to it while it is open. The sales
// copy checks the customer's credit limit (codeunit 312).
const (
	CopySalesDocumentID = 292
	CopyPurchDocumentID = 492
)

func init() {
	Register(CopySalesDocumentID, "copy-sales-document", func() Report { return &copySalesDocument{} })
	Register(CopyPurchDocumentID, "copy-purchase-document", func() Report { return &copyPurchDocument{} })
}

// The request page's Document Type (DocType): the open document types in the header's order,
// then the posted documents
const (
	copyDocQuote = iota
	copyDocBlanketOrder
	copyDocOrder
	copyDocInvoice
	copyDocCreditMemo
	copyDocPostedShipment // Posted Receipt on purchases
	copyDocPostedInvoice
	copyDocPostedCreditMemo
)

func copyDocumentOptions(postedShipment string) []Option {
	return []Option{
		{Name: "document_type", Type: "option", Default: "1",
			Options: []string{"QUOTE", "BLANKET_ORDER", "ORDER", "INVOICE", "CREDIT_MEMO", postedShipment, "POSTED_INVOICE", "POSTED_CREDIT_MEMO"}},
		{Name: "document_no", Type: "text", Required: true},
		{Name: "include_header", Type: "bool", Default: "false"},
		{Name: "recalculate_lines", Type: "bool", Default: "true"},
	}
}

// headerDocType (SalesHeaderDocType): the open document type of the request page's Document Type
func headerDocType(docType int) int {
	switch docType {
	case copyDocQuote:
		return 0
	case copyDocBlanketOrder:
		return 4
	case copyDocOrder:
		return 1
	case copyDocInvoice:
		return 2
	case copyDocCreditMemo:
		return 3
	}
	return -1
}

// copyDocumentRequest: the request page's choices
type copyDocumentRequest struct {
	docType          int
	docNo            types.Code
	includeHeader    bool
	recalculateLines bool
}

func readCopyDocumentRequest(run *Run) (copyDocumentRequest, error) {
	req := copyDocumentRequest{
		docType:          run.OptionInt("document_type") - 1,
		docNo:            types.Code(system.UpperCase(run.Option("document_no"))),
		includeHeader:    run.OptionBool("include_header"),
		recalculateLines: run.OptionBool("recalculate_lines"),
	}
	if req.docNo == "" {
		return req, system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	// ValidateIncludeHeader / the Recalculate Lines check box: a posted shipment is recalculated
	if req.docType == copyDocPostedShipment {
		req.recalculateLines = true
	}
	return req, nil
}

// ---- Report 292 Copy Sales Document ----

type copySalesDocument struct {
	salesHeader     *gtables.SalesHeader
	fromSalesHeader *gtables.SalesHeader
	fromSalesLines  []*gtables.SalesLine
	req             copyDocumentRequest
	nextLineNo      int
	linesNotCopied  int
	oldExtTextLines transferOldExtTextLines
}

func (r *copySalesDocument) Definition() *Definition {
	return &Definition{
		Options:      copyDocumentOptions("POSTED_SHIPMENT"),
		Transaction:  true,
		ContextTable: gtables.SalesHeaderTableName,
		OnPreReport:  r.onPreReport,
	}
}

// fromDocument: the document to copy from as a sales header and its lines (a posted document's
// fields by their numbers, TRANSFERFIELDS)
func (r *copySalesDocument) fromDocument(run *Run) error {
	r.fromSalesHeader = gtables.NewSalesHeader(run)
	switch r.req.docType {
	case copyDocQuote, copyDocBlanketOrder, copyDocOrder, copyDocInvoice, copyDocCreditMemo:
		docType := headerDocType(r.req.docType)
		err := r.fromSalesHeader.GetOrError(map[string]interface{}{"document_type": docType, "no": r.req.docNo})
		if err != nil {
			return err
		}
		fromSalesLine := gtables.NewSalesLine(run)
		fromSalesLine.SetRange("document_type", docType)
		fromSalesLine.SetRange("document_no", r.req.docNo)
		for found := fromSalesLine.FindSetBuffered(); found; found = fromSalesLine.Next() {
			line := gtables.NewSalesLine(run)
			line.Copy(fromSalesLine)
			r.fromSalesLines = append(r.fromSalesLines, line)
		}
	case copyDocPostedShipment:
		fromSalesShptHeader := gtables.NewSalesShipmentHeader(run)
		err := fromSalesShptHeader.GetOrError(r.req.docNo)
		if err != nil {
			return err
		}
		r.fromSalesHeader.TransferFields(fromSalesShptHeader)
		fromSalesShptLine := gtables.NewSalesShipmentLine(run)
		fromSalesShptLine.SetRange("document_no", r.req.docNo)
		for found := fromSalesShptLine.FindSetBuffered(); found; found = fromSalesShptLine.Next() {
			line := gtables.NewSalesLine(run)
			line.TransferFields(fromSalesShptLine)
			r.fromSalesLines = append(r.fromSalesLines, line)
		}
	case copyDocPostedInvoice:
		fromSalesInvHeader := gtables.NewSalesInvoiceHeader(run)
		err := fromSalesInvHeader.GetOrError(r.req.docNo)
		if err != nil {
			return err
		}
		r.fromSalesHeader.TransferFields(fromSalesInvHeader)
		fromSalesInvLine := gtables.NewSalesInvoiceLine(run)
		fromSalesInvLine.SetRange("document_no", r.req.docNo)
		for found := fromSalesInvLine.FindSetBuffered(); found; found = fromSalesInvLine.Next() {
			line := gtables.NewSalesLine(run)
			line.TransferFields(fromSalesInvLine)
			r.fromSalesLines = append(r.fromSalesLines, line)
		}
	case copyDocPostedCreditMemo:
		fromSalesCrMemoHeader := gtables.NewSalesCrMemoHeader(run)
		err := fromSalesCrMemoHeader.GetOrError(r.req.docNo)
		if err != nil {
			return err
		}
		r.fromSalesHeader.TransferFields(fromSalesCrMemoHeader)
		fromSalesCrMemoLine := gtables.NewSalesCrMemoLine(run)
		fromSalesCrMemoLine.SetRange("document_no", r.req.docNo)
		for found := fromSalesCrMemoLine.FindSetBuffered(); found; found = fromSalesCrMemoLine.Next() {
			line := gtables.NewSalesLine(run)
			line.TransferFields(fromSalesCrMemoLine)
			r.fromSalesLines = append(r.fromSalesLines, line)
		}
	}
	return nil
}

func (r *copySalesDocument) onPreReport(run *Run) error {
	contextHeader, ok := run.Context().(*gtables.SalesHeader)
	if !ok {
		return system.Error(run.Text("NO_DOCUMENT"))
	}
	var err error
	r.req, err = readCopyDocumentRequest(run)
	if err != nil {
		return err
	}
	salesHeader := gtables.NewSalesHeader(run)
	err = salesHeader.GetOrError(map[string]interface{}{"document_type": int(contextHeader.Document_type), "no": contextHeader.No})
	if err != nil {
		return err
	}
	r.salesHeader = salesHeader
	if r.req.docType <= copyDocCreditMemo && headerDocType(r.req.docType) == int(salesHeader.Document_type) && r.req.docNo == salesHeader.No {
		return system.Error(run.Text("COPY_ONTO_ITSELF"), salesHeader.Document_type, salesHeader.No)
	}
	err = r.fromDocument(run)
	if err != nil {
		return err
	}

	// the customer's credit limit with the other document's amount (not for a shipment)
	if r.req.docType != copyDocPostedShipment && salesHeader.Document_type <= gtables.SalesHeader_Document_type.Invoice {
		var amountIncludingTax types.Decimal
		for _, fromSalesLine := range r.fromSalesLines {
			amountIncludingTax = amountIncludingTax.Add(fromSalesLine.Amount_including_tax)
		}
		creditCheckHeader := gtables.NewSalesHeader(run)
		creditCheckHeader.Copy(salesHeader)
		creditCheckHeader.Amount_including_tax = amountIncludingTax
		err = tables.CustCheckCrLimitSalesHeader(creditCheckHeader)
		if err != nil {
			return err
		}
	}
	if !r.req.includeHeader && !r.req.recalculateLines {
		for _, field := range []string{"sell_to_customer_no", "bill_to_customer_no", "customer_posting_group", "gen_bus_posting_group", "currency_code", "prices_including_tax"} {
			value, _ := salesHeader.FieldValue(salesHeader.FieldNo(field))
			err = r.fromSalesHeader.TestField(field, value)
			if err != nil {
				return err
			}
		}
	}

	salesLine := gtables.NewSalesLine(run)
	salesLine.LockTable()
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
	salesLine.SetRange("document_no", salesHeader.No)
	if r.req.includeHeader && salesLine.FindFirst() {
		if !system.Confirm(run.Text("DELETE_EXISTING_LINES"), salesHeader.Document_type, salesHeader.No) {
			return Quit()
		}
		// DELETEALL(TRUE): each line with its OnDelete trigger
		for found := salesLine.FindSetBuffered(); found; found = salesLine.Next() {
			err = salesLine.DeleteOrError(true)
			if err != nil {
				return err
			}
		}
	}
	if salesLine.FindLast() {
		r.nextLineNo = salesLine.Line_no
	}

	if r.req.includeHeader {
		err = r.copyHeader(run)
		if err != nil {
			return err
		}
	}
	for _, fromSalesLine := range r.fromSalesLines {
		err = r.copySalesLine(run, fromSalesLine)
		if err != nil {
			return err
		}
	}
	if r.linesNotCopied > 0 {
		run.Message(run.Text("LINES_NOT_COPIED"))
	}
	run.Refresh()
	return nil
}

// copyHeader (IncludeHeader): the other document's header fields, the document keeping its own
// numbers and series; a credit memo of a posted invoice — or a document of a posted credit memo —
// applies to it while it is open
func (r *copySalesDocument) copyHeader(run *Run) error {
	salesHeader := r.salesHeader
	oldSalesHeader := gtables.NewSalesHeader(run)
	oldSalesHeader.Copy(salesHeader)
	salesHeader.TransferFields(r.fromSalesHeader, false)
	salesHeader.No_series = oldSalesHeader.No_series
	salesHeader.Posting_description = oldSalesHeader.Posting_description
	salesHeader.Posting_no = oldSalesHeader.Posting_no
	salesHeader.Posting_no_series = oldSalesHeader.Posting_no_series
	salesHeader.Shipping_no = oldSalesHeader.Shipping_no
	salesHeader.Shipping_no_series = oldSalesHeader.Shipping_no_series
	salesHeader.No_printed = 0
	if r.req.docType == copyDocPostedInvoice && salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo ||
		r.req.docType == copyDocPostedCreditMemo && salesHeader.Document_type != gtables.SalesHeader_Document_type.CreditMemo {
		custLedgEntry := gtables.NewCustomerLedgerEntry(run)
		if r.req.docType == copyDocPostedInvoice {
			custLedgEntry.SetRange("document_type", gtables.CustomerLedgerEntry_Document_type.Invoice)
		} else {
			custLedgEntry.SetRange("document_type", gtables.CustomerLedgerEntry_Document_type.CreditMemo)
		}
		custLedgEntry.SetRange("document_no", r.req.docNo)
		custLedgEntry.SetRange("customer_no", salesHeader.Bill_to_customer_no)
		custLedgEntry.SetRange("open", true)
		if custLedgEntry.FindFirst() {
			if r.req.docType == copyDocPostedInvoice {
				salesHeader.Applies_to_doc_type = gtables.SalesHeader_Applies_to_doc_type.Invoice
			} else {
				salesHeader.Applies_to_doc_type = gtables.SalesHeader_Applies_to_doc_type.CreditMemo
			}
			salesHeader.Applies_to_doc_no = r.req.docNo
		}
	}
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
		salesHeader.Shipment_date = types.Date{}
		salesHeader.Order_date = types.Date{}
		salesHeader.Payment_terms_code = ""
		salesHeader.Due_date = types.Date{}
		salesHeader.Payment_discount_percent = types.Decimal{}
		salesHeader.Pmt_discount_date = types.Date{}
	}
	return salesHeader.ModifyOrError(false)
}

// copySalesLine (CopySalesLine): a line onto the document, entered again or copied as it is
func (r *copySalesDocument) copySalesLine(run *Run, fromSalesLine *gtables.SalesLine) error {
	copyThisLine := true
	salesLine := gtables.NewSalesLine(run)
	if !r.req.recalculateLines {
		salesLine.Copy(fromSalesLine)
	}
	r.nextLineNo += 10000
	salesLine.Document_type = gtables.SalesLineDocument_type(r.salesHeader.Document_type)
	salesLine.Document_no = r.salesHeader.No
	salesLine.Line_no = r.nextLineNo
	err := salesLine.Validate("currency_code", r.fromSalesHeader.Currency_code)
	if err != nil {
		return err
	}
	if r.req.recalculateLines {
		err = salesLine.Validate("type", fromSalesLine.Type)
		if err == nil {
			err = salesLine.Validate("description", fromSalesLine.Description)
		}
		if err == nil {
			err = salesLine.Validate("description_2", fromSalesLine.Description_2)
		}
		if err != nil {
			return err
		}
		if fromSalesLine.Type != gtables.SalesLine_Type.Blank && fromSalesLine.No != "" {
			if salesLine.Type == gtables.SalesLine_Type.GLAccount {
				salesLine.No = fromSalesLine.No
				glAcc := gtables.NewGLAccount(run)
				err = glAcc.GetOrError(fromSalesLine.No)
				if err != nil {
					return err
				}
				copyThisLine = glAcc.Direct_posting
				if copyThisLine {
					err = salesLine.Validate("no", fromSalesLine.No)
				}
			} else {
				err = salesLine.Validate("no", fromSalesLine.No)
			}
			if err != nil {
				return err
			}
			if copyThisLine {
				for _, step := range []struct {
					field string
					value any
				}{
					{"location_code", fromSalesLine.Location_code},
					{"unit_of_measure", fromSalesLine.Unit_of_measure},
					{"unit_of_measure_code", fromSalesLine.Unit_of_measure_code},
					{"quantity", fromSalesLine.Quantity},
				} {
					err = salesLine.Validate(step.field, step.value)
					if err != nil {
						return err
					}
				}
				if fromSalesLine.Type != gtables.SalesLine_Type.Item && fromSalesLine.Type != gtables.SalesLine_Type.Resource {
					err = r.fromSalesHeader.TestField("currency_code", r.salesHeader.Currency_code)
					if err == nil {
						err = r.fromSalesHeader.TestField("prices_including_tax", r.salesHeader.Prices_including_tax)
					}
					if err == nil {
						err = salesLine.Validate("unit_price", fromSalesLine.Unit_price)
					}
					if err == nil {
						err = salesLine.Validate("line_discount_percent", fromSalesLine.Line_discount_percent)
					}
					if err == nil && !salesLine.Quantity.IsZero() {
						err = salesLine.Validate("line_discount_amount", fromSalesLine.Line_discount_amount)
					}
					if err != nil {
						return err
					}
				}
				err = salesLine.Validate("work_type_code", fromSalesLine.Work_type_code)
				if err != nil {
					return err
				}
			}
		}
	} else {
		salesLine.Quantity_shipped = types.Decimal{}
		salesLine.Qty_shipped_base = types.Decimal{}
		salesLine.Quantity_invoiced = types.Decimal{}
		salesLine.Qty_invoiced_base = types.Decimal{}
		salesLine.Reserved_quantity = types.Decimal{}
		salesLine.Reserved_qty_base = types.Decimal{}
		err = tables.InitSalesLineOutstanding(salesLine)
		if err != nil {
			return err
		}
		tables.InitSalesLineQtyToShip(salesLine)
		salesLine.Shipment_date = r.salesHeader.Shipment_date
		salesLine.Appl_to_project_entry = 0
		salesLine.Project_applies_to_id = ""
		// a drop shipment's link to its purchase order line is not copied
		salesLine.Purchase_order_no = ""
		salesLine.Purch_order_line_no = 0
		salesLine.Drop_shipment = false
		// a line of an invoice made from shipments does not invoice them again
		salesLine.Shipment_no = ""
		salesLine.Shipment_line_no = 0
	}
	salesLine.Attached_to_line_no = r.oldExtTextLines.transferExtendedText(fromSalesLine.Line_no, r.nextLineNo, fromSalesLine.Attached_to_line_no)
	if !copyThisLine {
		r.linesNotCopied++
		return nil
	}
	return salesLine.InsertOrError(false)
}

// ---- Report 492 Copy Purchase Document ----

type copyPurchDocument struct {
	purchHeader     *gtables.PurchaseHeader
	fromPurchHeader *gtables.PurchaseHeader
	fromPurchLines  []*gtables.PurchaseLine
	req             copyDocumentRequest
	nextLineNo      int
	linesNotCopied  int
	oldExtTextLines transferOldExtTextLines
}

func (r *copyPurchDocument) Definition() *Definition {
	return &Definition{
		Options:      copyDocumentOptions("POSTED_RECEIPT"),
		Transaction:  true,
		ContextTable: gtables.PurchaseHeaderTableName,
		OnPreReport:  r.onPreReport,
	}
}

func (r *copyPurchDocument) fromDocument(run *Run) error {
	r.fromPurchHeader = gtables.NewPurchaseHeader(run)
	switch r.req.docType {
	case copyDocQuote, copyDocBlanketOrder, copyDocOrder, copyDocInvoice, copyDocCreditMemo:
		docType := headerDocType(r.req.docType)
		err := r.fromPurchHeader.GetOrError(map[string]interface{}{"document_type": docType, "no": r.req.docNo})
		if err != nil {
			return err
		}
		fromPurchLine := gtables.NewPurchaseLine(run)
		fromPurchLine.SetRange("document_type", docType)
		fromPurchLine.SetRange("document_no", r.req.docNo)
		for found := fromPurchLine.FindSetBuffered(); found; found = fromPurchLine.Next() {
			line := gtables.NewPurchaseLine(run)
			line.Copy(fromPurchLine)
			r.fromPurchLines = append(r.fromPurchLines, line)
		}
	case copyDocPostedShipment:
		fromPurchRcptHeader := gtables.NewPurchRcptHeader(run)
		err := fromPurchRcptHeader.GetOrError(r.req.docNo)
		if err != nil {
			return err
		}
		r.fromPurchHeader.TransferFields(fromPurchRcptHeader)
		fromPurchRcptLine := gtables.NewPurchRcptLine(run)
		fromPurchRcptLine.SetRange("document_no", r.req.docNo)
		for found := fromPurchRcptLine.FindSetBuffered(); found; found = fromPurchRcptLine.Next() {
			line := gtables.NewPurchaseLine(run)
			line.TransferFields(fromPurchRcptLine)
			r.fromPurchLines = append(r.fromPurchLines, line)
		}
	case copyDocPostedInvoice:
		fromPurchInvHeader := gtables.NewPurchInvHeader(run)
		err := fromPurchInvHeader.GetOrError(r.req.docNo)
		if err != nil {
			return err
		}
		r.fromPurchHeader.TransferFields(fromPurchInvHeader)
		fromPurchInvLine := gtables.NewPurchInvLine(run)
		fromPurchInvLine.SetRange("document_no", r.req.docNo)
		for found := fromPurchInvLine.FindSetBuffered(); found; found = fromPurchInvLine.Next() {
			line := gtables.NewPurchaseLine(run)
			line.TransferFields(fromPurchInvLine)
			r.fromPurchLines = append(r.fromPurchLines, line)
		}
	case copyDocPostedCreditMemo:
		fromPurchCrMemoHeader := gtables.NewPurchCrMemoHdr(run)
		err := fromPurchCrMemoHeader.GetOrError(r.req.docNo)
		if err != nil {
			return err
		}
		r.fromPurchHeader.TransferFields(fromPurchCrMemoHeader)
		fromPurchCrMemoLine := gtables.NewPurchCrMemoLine(run)
		fromPurchCrMemoLine.SetRange("document_no", r.req.docNo)
		for found := fromPurchCrMemoLine.FindSetBuffered(); found; found = fromPurchCrMemoLine.Next() {
			line := gtables.NewPurchaseLine(run)
			line.TransferFields(fromPurchCrMemoLine)
			r.fromPurchLines = append(r.fromPurchLines, line)
		}
	}
	return nil
}

func (r *copyPurchDocument) onPreReport(run *Run) error {
	contextHeader, ok := run.Context().(*gtables.PurchaseHeader)
	if !ok {
		return system.Error(run.Text("NO_DOCUMENT"))
	}
	var err error
	r.req, err = readCopyDocumentRequest(run)
	if err != nil {
		return err
	}
	purchHeader := gtables.NewPurchaseHeader(run)
	err = purchHeader.GetOrError(map[string]interface{}{"document_type": int(contextHeader.Document_type), "no": contextHeader.No})
	if err != nil {
		return err
	}
	r.purchHeader = purchHeader
	if r.req.docType <= copyDocCreditMemo && headerDocType(r.req.docType) == int(purchHeader.Document_type) && r.req.docNo == purchHeader.No {
		return system.Error(run.Text("COPY_ONTO_ITSELF"), purchHeader.Document_type, purchHeader.No)
	}
	err = r.fromDocument(run)
	if err != nil {
		return err
	}
	if !r.req.includeHeader && !r.req.recalculateLines {
		for _, field := range []string{"buy_from_vendor_no", "pay_to_vendor_no", "vendor_posting_group", "gen_bus_posting_group", "currency_code"} {
			value, _ := purchHeader.FieldValue(purchHeader.FieldNo(field))
			err = r.fromPurchHeader.TestField(field, value)
			if err != nil {
				return err
			}
		}
	}

	purchLine := gtables.NewPurchaseLine(run)
	purchLine.LockTable()
	purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
	purchLine.SetRange("document_no", purchHeader.No)
	if r.req.includeHeader && purchLine.FindFirst() {
		if !system.Confirm(run.Text("DELETE_EXISTING_LINES"), purchHeader.Document_type, purchHeader.No) {
			return Quit()
		}
		// DELETEALL(TRUE): each line with its OnDelete trigger
		for found := purchLine.FindSetBuffered(); found; found = purchLine.Next() {
			err = purchLine.DeleteOrError(true)
			if err != nil {
				return err
			}
		}
	}
	if purchLine.FindLast() {
		r.nextLineNo = purchLine.Line_no
	}

	if r.req.includeHeader {
		err = r.copyHeader(run)
		if err != nil {
			return err
		}
	}
	for _, fromPurchLine := range r.fromPurchLines {
		err = r.copyPurchLine(run, fromPurchLine)
		if err != nil {
			return err
		}
	}
	if r.linesNotCopied > 0 {
		run.Message(run.Text("LINES_NOT_COPIED"))
	}
	run.Refresh()
	return nil
}

func (r *copyPurchDocument) copyHeader(run *Run) error {
	purchHeader := r.purchHeader
	oldPurchHeader := gtables.NewPurchaseHeader(run)
	oldPurchHeader.Copy(purchHeader)
	purchHeader.TransferFields(r.fromPurchHeader, false)
	purchHeader.No_series = oldPurchHeader.No_series
	purchHeader.Posting_description = oldPurchHeader.Posting_description
	purchHeader.Posting_no = oldPurchHeader.Posting_no
	purchHeader.Posting_no_series = oldPurchHeader.Posting_no_series
	purchHeader.Receiving_no = oldPurchHeader.Receiving_no
	purchHeader.Receiving_no_series = oldPurchHeader.Receiving_no_series
	purchHeader.No_printed = 0
	if r.req.docType == copyDocPostedInvoice && purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo ||
		r.req.docType == copyDocPostedCreditMemo && purchHeader.Document_type != gtables.PurchaseHeader_Document_type.CreditMemo {
		vendLedgEntry := gtables.NewVendorLedgerEntry(run)
		if r.req.docType == copyDocPostedInvoice {
			vendLedgEntry.SetRange("document_type", gtables.VendorLedgerEntry_Document_type.Invoice)
		} else {
			vendLedgEntry.SetRange("document_type", gtables.VendorLedgerEntry_Document_type.CreditMemo)
		}
		vendLedgEntry.SetRange("document_no", r.req.docNo)
		vendLedgEntry.SetRange("vendor_no", purchHeader.Pay_to_vendor_no)
		vendLedgEntry.SetRange("open", true)
		if vendLedgEntry.FindFirst() {
			if r.req.docType == copyDocPostedInvoice {
				purchHeader.Applies_to_doc_type = gtables.PurchaseHeader_Applies_to_doc_type.Invoice
			} else {
				purchHeader.Applies_to_doc_type = gtables.PurchaseHeader_Applies_to_doc_type.CreditMemo
			}
			purchHeader.Applies_to_doc_no = r.req.docNo
		}
	}
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
		purchHeader.Expected_receipt_date = types.Date{}
		purchHeader.Order_date = types.Date{}
		purchHeader.Payment_terms_code = ""
		purchHeader.Due_date = types.Date{}
		purchHeader.Payment_discount_percent = types.Decimal{}
		purchHeader.Pmt_discount_date = types.Date{}
	}
	return purchHeader.ModifyOrError(false)
}

// copyPurchLine (CopyPurchLine)
func (r *copyPurchDocument) copyPurchLine(run *Run, fromPurchLine *gtables.PurchaseLine) error {
	copyThisLine := true
	purchLine := gtables.NewPurchaseLine(run)
	if !r.req.recalculateLines {
		purchLine.Copy(fromPurchLine)
	}
	r.nextLineNo += 10000
	purchLine.Document_type = gtables.PurchaseLineDocument_type(r.purchHeader.Document_type)
	purchLine.Document_no = r.purchHeader.No
	purchLine.Line_no = r.nextLineNo
	err := purchLine.Validate("currency_code", r.fromPurchHeader.Currency_code)
	if err != nil {
		return err
	}
	if r.req.recalculateLines {
		err = purchLine.Validate("type", fromPurchLine.Type)
		if err == nil {
			err = purchLine.Validate("description", fromPurchLine.Description)
		}
		if err == nil {
			err = purchLine.Validate("description_2", fromPurchLine.Description_2)
		}
		if err != nil {
			return err
		}
		if fromPurchLine.Type != gtables.PurchaseLine_Type.Blank && fromPurchLine.No != "" {
			if purchLine.Type == gtables.PurchaseLine_Type.GLAccount {
				purchLine.No = fromPurchLine.No
				glAcc := gtables.NewGLAccount(run)
				err = glAcc.GetOrError(fromPurchLine.No)
				if err != nil {
					return err
				}
				copyThisLine = glAcc.Direct_posting
				if copyThisLine {
					err = purchLine.Validate("no", fromPurchLine.No)
				}
			} else {
				err = purchLine.Validate("no", fromPurchLine.No)
			}
			if err != nil {
				return err
			}
			if copyThisLine {
				for _, step := range []struct {
					field string
					value any
				}{
					{"location_code", fromPurchLine.Location_code},
					{"unit_of_measure", fromPurchLine.Unit_of_measure},
					{"unit_of_measure_code", fromPurchLine.Unit_of_measure_code},
					{"quantity", fromPurchLine.Quantity},
				} {
					err = purchLine.Validate(step.field, step.value)
					if err != nil {
						return err
					}
				}
				if fromPurchLine.Type != gtables.PurchaseLine_Type.Item {
					err = r.fromPurchHeader.TestField("currency_code", r.purchHeader.Currency_code)
					if err == nil {
						err = purchLine.Validate("direct_unit_cost", fromPurchLine.Direct_unit_cost)
					}
					if err == nil {
						err = purchLine.Validate("line_discount_percent", fromPurchLine.Line_discount_percent)
					}
					if err == nil && !purchLine.Quantity.IsZero() {
						err = purchLine.Validate("line_discount_amount", fromPurchLine.Line_discount_amount)
					}
					if err != nil {
						return err
					}
				}
			}
		}
	} else {
		purchLine.Quantity_received = types.Decimal{}
		purchLine.Qty_received_base = types.Decimal{}
		purchLine.Quantity_invoiced = types.Decimal{}
		purchLine.Qty_invoiced_base = types.Decimal{}
		purchLine.Reserved_quantity = types.Decimal{}
		purchLine.Reserved_qty_base = types.Decimal{}
		err = tables.InitPurchLineOutstanding(purchLine)
		if err != nil {
			return err
		}
		tables.InitPurchLineQtyToReceive(purchLine)
		// a drop shipment's link to its sales order line is not copied
		purchLine.Sales_order_no = ""
		purchLine.Sales_order_line_no = 0
		purchLine.Drop_shipment = false
		purchLine.Receipt_no = ""
		purchLine.Receipt_line_no = 0
		purchLine.Expected_receipt_date = r.purchHeader.Expected_receipt_date
	}
	purchLine.Attached_to_line_no = r.oldExtTextLines.transferExtendedText(fromPurchLine.Line_no, r.nextLineNo, fromPurchLine.Attached_to_line_no)
	if !copyThisLine {
		r.linesNotCopied++
		return nil
	}
	return purchLine.InsertOrError(false)
}

// transferOldExtTextLines is codeunit 379 Transfer Old Ext. Text Lines: the line numbers a copy
// gave the lines, so a copied text line is attached to the copy of its line
type transferOldExtTextLines map[int]int

// transferExtendedText (TransferExtendedText): remembers the line's new number and returns the
// new number of the line it is attached to (0: not attached, or that line was not copied)
func (t *transferOldExtTextLines) transferExtendedText(oldLineNo, newLineNo, oldAttachedToLineNo int) int {
	if *t == nil {
		*t = transferOldExtTextLines{}
	}
	(*t)[oldLineNo] = newLineNo
	return (*t)[oldAttachedToLineNo]
}
