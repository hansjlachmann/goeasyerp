package reports

import (
	"sort"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The documents' test reports (Posting › Test Report on the sales and purchase orders, invoices
// and credit memos; Report Selections usage S.Test / P.Test):
//   - 202 Sales Document - Test: per document (not quotes) what posting would refuse — the
//     customers, the posting and document dates, Ship/Invoice and whether there is anything to
//     post, the due date, the shipping and posting no. series, the external document no. — and
//     its lines with theirs: type and no., quantities to ship and invoice, the shipment lines an
//     invoice takes, the posting setups, the account, item or resource (blocked, direct posting,
//     specific costing, Reserve Always, serial nos.) — with the totals and the tax per rate;
//   - 402 Purchase Document - Test: the same for purchases (receive instead of ship, the
//     vendor's invoice or credit memo no. — mandatory and not used before —, fixed assets).
//
// Drop shipments are not part of this application (their checks are left out).
const (
	SalesDocumentTestID = 202
	PurchDocumentTestID = 402
)

func init() {
	Register(SalesDocumentTestID, "sales-document-test", func() Report { return &salesDocumentTest{} })
	Register(PurchDocumentTestID, "purchase-document-test", func() Report { return &purchDocumentTest{} })
	DefaultReports[gtables.ReportSelections_Usage.STest] = SalesDocumentTestID
	DefaultReports[gtables.ReportSelections_Usage.PTest] = PurchDocumentTestID
}

// documentTest is what the two test reports share: the warnings of the head and of the current
// line, the allowed posting dates and the tax amount per rate
type documentTest struct {
	headerErrors, lineErrors         []string
	allowPostingFrom, allowPostingTo types.Date
	taxLines                         map[string]*taxAmountLine
	language                         string
}

// taxAmountLine is one tax rate's base and tax (classic temporary Tax Amount Line)
type taxAmountLine struct {
	taxPct, taxBase, amountInclTax types.Decimal
}

func (t *documentTest) errorLoop(name string, errors *[]string) *DataItem {
	return &DataItem{
		Name: name, Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			rec.SetRange("number", 1, len(*errors))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			run.SetValue("error_text", (*errors)[rec.Number-1])
			return nil
		}),
	}
}

// taxCounter prints the tax per rate when the document has more than one rate
func (t *documentTest) taxCounter() *DataItem {
	var lines []*taxAmountLine
	return &DataItem{
		Name: "Tax_Counter", Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			lines = lines[:0]
			for _, l := range t.taxLines {
				lines = append(lines, l)
			}
			sort.Slice(lines, func(i, j int) bool { return lines[i].taxPct.LessThan(lines[j].taxPct) })
			if len(lines) <= 1 {
				return Break()
			}
			rec.SetRange("number", 1, len(lines))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			l := lines[rec.Number-1]
			run.SetValue("tax_pct", l.taxPct)
			run.SetValue("tax_base", l.taxBase)
			run.SetValue("tax_amount", l.amountInclTax.Sub(l.taxBase))
			return nil
		}),
	}
}

func (t *documentTest) addTaxLine(taxPct, taxBase, amountInclTax types.Decimal) {
	key := taxPct.String()
	l, ok := t.taxLines[key]
	if !ok {
		l = &taxAmountLine{taxPct: taxPct}
		t.taxLines[key] = l
	}
	l.taxBase = l.taxBase.Add(taxBase)
	l.amountInclTax = l.amountInclTax.Add(amountInclTax)
}

// checkPostingDate: specified, not a closing date, within the user's allowed posting dates
// (User Setup, else General Ledger Setup)
func (t *documentTest) checkPostingDate(run *Run, postingDate types.Date, caption string) {
	if postingDate.IsZero() {
		t.headerErrors = append(t.headerErrors, run.Text("MUST_BE_SPECIFIED", caption))
		return
	}
	if postingDate.IsClosing() {
		t.headerErrors = append(t.headerErrors, run.Text("NOT_CLOSING_DATE", caption))
		return
	}
	if t.allowPostingFrom.IsZero() && t.allowPostingTo.IsZero() {
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(run)
			if userSetup.Get(types.Code(system.UserId())) {
				t.allowPostingFrom = userSetup.Allow_posting_from
				t.allowPostingTo = userSetup.Allow_posting_to
			}
		}
		if t.allowPostingFrom.IsZero() && t.allowPostingTo.IsZero() {
			glSetup := gtables.GetGeneralLedgerSetup(run)
			t.allowPostingFrom = glSetup.Allow_posting_from
			t.allowPostingTo = glSetup.Allow_posting_to
		}
		if t.allowPostingTo.IsZero() {
			t.allowPostingTo, _ = system.DMY2Date(31, 12, 9999)
		}
	}
	if postingDate.Before(t.allowPostingFrom) || t.allowPostingTo.Before(postingDate) {
		t.headerErrors = append(t.headerErrors, run.Text("DATE_NOT_ALLOWED", caption))
	}
}

// mustBeFor is "%1 must be %2 for %3 %4." (a blocked account, item, resource …)
func (t *documentTest) mustBeFor(run *Run, fieldCaption string, value bool, tableCaption string, no types.Code) string {
	return run.Text("MUST_BE_FOR", fieldCaption, formatValue(value, "", t.language), tableCaption, no)
}

// Report 202 Sales Document - Test
type salesDocumentTest struct {
	documentTest
	salesHeader           *gtables.SalesHeader
	ship, invoice         bool
	vatNoError, applError bool
}

func (r *salesDocumentTest) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "ship", Type: "bool", Default: "true"},
			{Name: "invoice", Type: "bool", Default: "true"},
		},
		ContextTable: gtables.SalesHeaderTableName,
		OnInitReport: func(run *Run) error {
			salesHeader, ok := run.Context().(*gtables.SalesHeader)
			if ok && salesHeader.No != "" {
				run.SetRequestFilter("Sales_Header", "document_type", system.Format(int(salesHeader.Document_type)))
				run.SetRequestFilter("Sales_Header", "no", salesHeader.No.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			r.language = fcodeunits.CurrentLanguage()
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Sales_Header", Table: gtables.SalesHeaderTableName, Key: []string{"document_type", "no"},
			TableView:           []ftables.FieldFilter{{Field: "document_type", Expression: "<>0"}}, // not quotes
			RequestFilterFields: []string{"document_type", "no"},
			OnPreDataItem: Trigger(func(salesHeader *gtables.SalesHeader, run *Run) error {
				setFilterText(run, salesHeader)
				return nil
			}),
			OnAfterGetRecord: Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{
				r.errorLoop("Header_Error_Loop", &r.headerErrors),
				{
					Name: "Sales_Line", Table: gtables.SalesLineTableName, Key: []string{"document_type", "document_no", "line_no"},
					Link: map[string]string{"document_type": "document_type", "document_no": "no"},
					OnPreDataItem: Trigger(func(salesLine *gtables.SalesLine, run *Run) error {
						r.taxLines = map[string]*taxAmountLine{}
						r.vatNoError, r.applError = false, false
						return nil
					}),
					OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
					DataItems:        []*DataItem{r.errorLoop("Line_Error_Loop", &r.lineErrors)},
				},
				r.taxCounter(),
			},
		}},
	}
}

// headerOnAfterGetRecord: the head's addresses and checks
func (r *salesDocumentTest) headerOnAfterGetRecord(salesHeader *gtables.SalesHeader, run *Run) error {
	r.salesHeader = salesHeader
	r.headerErrors = nil
	run.SetPageNo(1)
	caption := func(field string) string { return salesHeader.GetFieldCaption(field, r.language) }
	sellToAddr, err := formataddress.SalesHeaderSellTo(salesHeader)
	if err != nil {
		return err
	}
	shipToAddr, err := formataddress.SalesHeaderShipTo(salesHeader)
	if err != nil {
		return err
	}
	run.SetValue("sell_to_address", sellToAddr)
	run.SetValue("ship_to_address", shipToAddr)
	run.SetValue("document_heading", run.Text("SALES_DOCUMENT",
		optionCaption(gtables.SalesHeaderTableName, "document_type", salesHeader.Document_type.String(), r.language)))
	if salesHeader.Currency_code == "" {
		run.SetValue("total_text", run.Text("TOTAL"))
		run.SetValue("total_incl_tax_text", run.Text("TOTAL_INCL_TAX"))
	} else {
		run.SetValue("total_text", run.Text("TOTAL_CURRENCY", salesHeader.Currency_code))
		run.SetValue("total_incl_tax_text", run.Text("TOTAL_CURRENCY_INCL_TAX", salesHeader.Currency_code))
	}

	r.ship = run.OptionBool("ship")
	r.invoice = run.OptionBool("invoice")
	shipInvText := ""
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
		switch {
		case r.ship && r.invoice:
			shipInvText = run.Text("SHIP_AND_INVOICE")
		case r.ship:
			shipInvText = run.Text("SHIP")
		case r.invoice:
			shipInvText = run.Text("INVOICE")
		}
		shipInvText = run.Text("ORDER_POSTING", shipInvText)
	}
	run.SetValue("ship_inv_text", shipInvText)

	for _, party := range []struct {
		field string
		no    types.Code
	}{{"sell_to_customer_no", salesHeader.Sell_to_customer_no}, {"bill_to_customer_no", salesHeader.Bill_to_customer_no}} {
		if party.no == "" {
			r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption(party.field)))
			continue
		}
		cust := gtables.NewCustomer(run)
		if cust.Get(party.no) {
			if cust.Blocked {
				r.headerErrors = append(r.headerErrors, r.mustBeFor(run, cust.GetFieldCaption("blocked", r.language), false, cust.TableCaption(r.language), party.no))
			}
		} else {
			r.headerErrors = append(r.headerErrors, run.Text("NOT_EXIST", cust.TableCaption(r.language), party.no))
		}
	}
	salesSetup := gtables.GetSalesReceivablesSetup(run)
	r.checkPostingDate(run, salesHeader.Posting_date, caption("posting_date"))
	if !salesHeader.Document_date.IsZero() && salesHeader.Document_date.IsClosing() {
		r.headerErrors = append(r.headerErrors, run.Text("NOT_CLOSING_DATE", caption("document_date")))
	}
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.Invoice || salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
		r.ship = true
		r.invoice = true
	}
	if !r.ship && !r.invoice {
		r.headerErrors = append(r.headerErrors, run.Text("ENTER_YES_IN", caption("ship"), caption("invoice")))
	}
	if r.invoice {
		salesLine := gtables.NewSalesLine(run)
		salesLine.SetRange("document_type", int(salesHeader.Document_type))
		salesLine.SetRange("document_no", salesHeader.No)
		salesLine.SetFilter("quantity", "<>0")
		if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
			salesLine.SetFilter("qty_to_invoice", "<>0")
		}
		r.invoice = false
		for found := salesLine.FindSetBuffered(); found && !r.invoice; found = salesLine.Next() {
			r.invoice = r.ship || !salesLine.Quantity_shipped.Sub(salesLine.Quantity_invoiced).IsZero()
		}
	}
	if r.ship {
		salesLine := gtables.NewSalesLine(run)
		salesLine.SetRange("document_type", int(salesHeader.Document_type))
		salesLine.SetRange("document_no", salesHeader.No)
		salesLine.SetFilter("quantity", "<>0")
		if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
			salesLine.SetFilter("qty_to_ship", "<>0")
		}
		salesLine.SetRange("shipment_no", "")
		r.ship = salesLine.FindFirst()
	}
	if !r.ship && !r.invoice {
		r.headerErrors = append(r.headerErrors, run.Text("NOTHING_TO_POST"))
	}
	if r.invoice && salesHeader.Document_type != gtables.SalesHeader_Document_type.CreditMemo && salesHeader.Due_date.IsZero() {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("due_date")))
	}
	if r.ship && salesHeader.Shipping_no == "" {
		if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order ||
			salesHeader.Document_type == gtables.SalesHeader_Document_type.Invoice && salesSetup.Shipment_on_invoice {
			if salesHeader.Shipping_no_series == "" {
				r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("shipping_no_series")))
			}
		}
	}
	if r.invoice && salesHeader.Posting_no == "" && salesHeader.Document_type == gtables.SalesHeader_Document_type.Order && salesHeader.Posting_no_series == "" {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("posting_no_series")))
	}
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order && salesSetup.Ext_doc_no_mandatory && salesHeader.External_document_no == "" {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("external_document_no")))
	}
	return nil
}

// lineOnAfterGetRecord: the line's checks; its amounts go into the tax per rate
func (r *salesDocumentTest) lineOnAfterGetRecord(salesLine *gtables.SalesLine, run *Run) error {
	r.lineErrors = nil
	salesHeader := r.salesHeader
	caption := func(field string) string { return salesLine.GetFieldCaption(field, r.language) }
	r.addTaxLine(salesLine.Tax_percent, salesLine.Amount, salesLine.Amount_including_tax)

	if salesLine.Document_type != gtables.SalesLine_Document_type.Order {
		if !salesLine.Qty_to_ship.Equal(salesLine.Quantity) {
			r.lineErrors = append(r.lineErrors, run.Text("MUST_BE", caption("qty_to_ship"), formatValue(salesLine.Quantity, "0:5", r.language)))
		}
		if !salesLine.Qty_to_invoice.Equal(salesLine.Quantity) {
			r.lineErrors = append(r.lineErrors, run.Text("MUST_BE", caption("qty_to_invoice"), formatValue(salesLine.Quantity, "0:5", r.language)))
		}
	}
	if !r.ship {
		salesLine.Qty_to_ship = types.ZeroDecimal()
	}
	if salesLine.Document_type == gtables.SalesLine_Document_type.Invoice && salesLine.Shipment_no != "" {
		salesLine.Quantity_shipped = salesLine.Quantity
		salesLine.Qty_to_ship = types.ZeroDecimal()
	}
	if r.invoice {
		maxQtyToBeInvoiced := salesLine.Qty_to_ship.Add(salesLine.Quantity_shipped).Sub(salesLine.Quantity_invoiced)
		if salesLine.Qty_to_invoice.Abs().GreaterThan(maxQtyToBeInvoiced.Abs()) {
			salesLine.Qty_to_invoice = maxQtyToBeInvoiced
		}
	} else {
		salesLine.Qty_to_invoice = types.ZeroDecimal()
	}

	if salesLine.Gen_prod_posting_group != "" {
		if salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo &&
			salesHeader.Applies_to_doc_type == gtables.SalesHeader_Applies_to_doc_type.Invoice && salesHeader.Applies_to_doc_no != "" {
			custLedgEntry := gtables.NewCustomerLedgerEntry(run)
			custLedgEntry.SetRange("customer_no", salesHeader.Bill_to_customer_no)
			custLedgEntry.SetRange("document_type", int(gtables.CustomerLedgerEntry_Document_type.Invoice))
			custLedgEntry.SetRange("document_no", salesHeader.Applies_to_doc_no)
			if !custLedgEntry.FindLast() && !r.applError {
				r.applError = true
				r.lineErrors = append(r.lineErrors, run.Text("APPLIES_TO_NOT_ON_ENTRIES", salesHeader.GetFieldCaption("applies_to_doc_no", r.language), salesHeader.Applies_to_doc_no))
			}
		}
		taxPostingSetup := gtables.NewTaxPostingSetup(run)
		if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": salesLine.Tax_bus_posting_group, "tax_prod_posting_group": salesLine.Tax_prod_posting_group}) {
			r.lineErrors = append(r.lineErrors, run.Text("SETUP_NOT_EXIST", taxPostingSetup.TableCaption(r.language), salesLine.Tax_bus_posting_group, salesLine.Tax_prod_posting_group))
		}
	}

	if !salesLine.Quantity.IsZero() {
		if salesLine.No == "" {
			r.lineErrors = append(r.lineErrors, run.Text("TYPE_NO_MUST_BE_SPECIFIED",
				optionCaption(gtables.SalesLineTableName, "type", salesLine.Type.String(), r.language), caption("no")))
		}
		if salesLine.Type == gtables.SalesLine_Type.Blank {
			r.lineErrors = append(r.lineErrors, run.Text("MUST_BE_SPECIFIED", caption("type")))
		}
	} else {
		if !salesLine.Amount.IsZero() {
			r.lineErrors = append(r.lineErrors, run.Text("AMOUNT_ZERO_WHEN_NO_QUANTITY", caption("amount"), caption("quantity")))
		}
	}

	r.checkShipmentLines(run, salesLine)

	if salesLine.Type != gtables.SalesLine_Type.Blank && !salesLine.Qty_to_invoice.IsZero() {
		genPostingSetup := gtables.NewGeneralPostingSetup(run)
		if !genPostingSetup.Get(map[string]interface{}{"gen_bus_posting_group": salesLine.Gen_bus_posting_group, "gen_prod_posting_group": salesLine.Gen_prod_posting_group}) {
			r.lineErrors = append(r.lineErrors, run.Text("SETUP_NOT_EXIST", genPostingSetup.TableCaption(r.language), salesLine.Gen_bus_posting_group, salesLine.Gen_prod_posting_group))
		}
	}

	switch salesLine.Type {
	case gtables.SalesLine_Type.GLAccount:
		if salesLine.No == "" && salesLine.Amount.IsZero() {
			return nil
		}
		if salesLine.No != "" {
			glAcc := gtables.NewGLAccount(run)
			if glAcc.Get(salesLine.No) {
				if glAcc.Blocked {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, glAcc.GetFieldCaption("blocked", r.language), false, glAcc.TableCaption(r.language), salesLine.No))
				}
				if !glAcc.Direct_posting {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, glAcc.GetFieldCaption("direct_posting", r.language), true, glAcc.TableCaption(r.language), salesLine.No))
				}
			} else {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_EXIST", glAcc.TableCaption(r.language), salesLine.No))
			}
		}
	case gtables.SalesLine_Type.Item:
		if salesLine.No == "" && salesLine.Quantity.IsZero() {
			return nil
		}
		if salesLine.No != "" {
			item := gtables.NewItem(run)
			if item.Get(salesLine.No) {
				if item.Blocked {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, item.GetFieldCaption("blocked", r.language), false, item.TableCaption(r.language), salesLine.No))
				}
				if item.Costing_method == gtables.Item_Costing_method.Specific && salesLine.Serial_no == "" {
					r.lineErrors = append(r.lineErrors, run.Text("MUST_BE_FILLED_WHEN", caption("serial_no"), item.GetFieldCaption("costing_method", r.language),
						optionCaption(gtables.ItemTableName, "costing_method", item.Costing_method.String(), r.language)))
				}
				signedQuantity := salesLine.Quantity
				if salesLine.Document_type != gtables.SalesLine_Document_type.CreditMemo {
					signedQuantity = signedQuantity.Neg()
				}
				if item.Reserve == gtables.Item_Reserve.Always && signedQuantity.IsNegative() && salesLine.Reserved_quantity.Abs().LessThan(salesLine.Qty_to_ship.Abs()) {
					r.lineErrors = append(r.lineErrors, run.Text("MUST_BE", caption("reserved_quantity"), formatValue(salesLine.Qty_to_ship.Neg(), "0:5", r.language)))
				}
			} else {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_EXIST", item.TableCaption(r.language), salesLine.No))
			}
		}
		r.checkSerialNo(run, salesLine.Serial_no, salesLine.Quantity, salesLine.Quantity_invoiced, caption)
	case gtables.SalesLine_Type.Resource:
		if salesLine.No == "" && salesLine.Quantity.IsZero() {
			return nil
		}
		if salesLine.No != "" {
			res := gtables.NewResource(run)
			if res.Get(salesLine.No) {
				if res.Blocked {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, res.GetFieldCaption("blocked", r.language), false, res.TableCaption(r.language), salesLine.No))
				}
			} else {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_EXIST", res.TableCaption(r.language), salesLine.No))
			}
		}
	}
	return nil
}

// checkSerialNo: a line with a serial no. moves one piece
func (t *documentTest) checkSerialNo(run *Run, serialNo types.Code, quantity, quantityInvoiced types.Decimal, caption func(string) string) {
	if serialNo == "" {
		return
	}
	one := types.NewDecimalFromInt(1)
	if quantity.Equal(quantityInvoiced) || quantityInvoiced.IsZero() {
		if !quantity.Abs().Equal(one) {
			t.lineErrors = append(t.lineErrors, run.Text("MUST_BE_ONE_WHEN", caption("quantity"), caption("serial_no")))
		}
	} else {
		if !quantityInvoiced.Abs().Equal(one) {
			t.lineErrors = append(t.lineErrors, run.Text("MUST_BE_ONE_WHEN", caption("quantity_invoiced"), caption("serial_no")))
		}
	}
}

// checkShipmentLines: what is invoiced beyond what is shipped now comes from the line's shipment
// lines, which must still be the line's
func (r *salesDocumentTest) checkShipmentLines(run *Run, line *gtables.SalesLine) {
	caption := func(field string) string { return line.GetFieldCaption(field, r.language) }
	qtyToShip := line.Qty_to_ship
	remQtyToBeInvoiced := line.Qty_to_invoice
	if line.Document_type != gtables.SalesLine_Document_type.CreditMemo {
		qtyToShip = qtyToShip.Neg()
		remQtyToBeInvoiced = remQtyToBeInvoiced.Neg()
	}
	if !remQtyToBeInvoiced.Abs().GreaterThan(line.Qty_to_ship.Abs()) {
		return
	}
	salesShptLine := gtables.NewSalesShipmentLine(run)
	switch line.Document_type {
	case gtables.SalesLine_Document_type.Order:
		salesShptLine.SetRange("order_no", line.Document_no)
		salesShptLine.SetRange("order_line_no", line.Line_no)
	case gtables.SalesLine_Document_type.Invoice:
		salesShptLine.SetRange("document_no", line.Shipment_no)
		salesShptLine.SetRange("line_no", line.Shipment_line_no)
	}
	for found := salesShptLine.FindSetBuffered(); found && remQtyToBeInvoiced.Abs().GreaterThan(line.Qty_to_ship.Abs()); found = salesShptLine.Next() {
		if salesShptLine.Quantity_invoiced.Equal(salesShptLine.Quantity) {
			continue
		}
		notSame := func(same bool, field string) {
			if !same {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_SAME_ON_SHIPMENT", caption(field)))
			}
		}
		notSame(salesShptLine.Sell_to_customer_no == line.Sell_to_customer_no, "sell_to_customer_no")
		notSame(int(salesShptLine.Type) == int(line.Type), "type")
		notSame(salesShptLine.No == line.No, "no")
		notSame(salesShptLine.Gen_bus_posting_group == line.Gen_bus_posting_group, "gen_bus_posting_group")
		notSame(salesShptLine.Gen_prod_posting_group == line.Gen_prod_posting_group, "gen_prod_posting_group")
		notSame(salesShptLine.Location_code == line.Location_code, "location_code")
		notSame(salesShptLine.Dimension_1_code == line.Dimension_1_code, "dimension_1_code")
		notSame(salesShptLine.Dimension_2_code == line.Dimension_2_code, "dimension_2_code")
		notSame(salesShptLine.Project_no == line.Project_no, "project_no")
		notSame(salesShptLine.Serial_no == line.Serial_no, "serial_no")
		if remQtyToBeInvoiced.Neg().Mul(salesShptLine.Quantity).IsNegative() {
			r.lineErrors = append(r.lineErrors, run.Text("SAME_SIGN_AS_SHIPMENT", caption("qty_to_invoice")))
		}
		qtyToBeInvoiced := remQtyToBeInvoiced.Sub(qtyToShip)
		if qtyToBeInvoiced.Abs().GreaterThan(salesShptLine.Quantity.Sub(salesShptLine.Quantity_invoiced).Abs()) {
			qtyToBeInvoiced = salesShptLine.Quantity.Sub(salesShptLine.Quantity_invoiced).Neg()
		}
		remQtyToBeInvoiced = remQtyToBeInvoiced.Sub(qtyToBeInvoiced)
	}
	if remQtyToBeInvoiced.Abs().GreaterThan(line.Qty_to_ship.Abs()) {
		if line.Document_type == gtables.SalesLine_Document_type.Invoice {
			r.lineErrors = append(r.lineErrors, run.Text("SHIPMENT_INVOICED", line.Shipment_no))
		} else {
			r.lineErrors = append(r.lineErrors, run.Text("SHIPMENT_LINES_DELETED"))
		}
	}
}

// Report 402 Purchase Document - Test
type purchDocumentTest struct {
	documentTest
	purchHeader      *gtables.PurchaseHeader
	receive, invoice bool
	applError        bool
}

func (r *purchDocumentTest) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "receive", Type: "bool", Default: "true"},
			{Name: "invoice", Type: "bool", Default: "true"},
		},
		ContextTable: gtables.PurchaseHeaderTableName,
		OnInitReport: func(run *Run) error {
			purchHeader, ok := run.Context().(*gtables.PurchaseHeader)
			if ok && purchHeader.No != "" {
				run.SetRequestFilter("Purchase_Header", "document_type", system.Format(int(purchHeader.Document_type)))
				run.SetRequestFilter("Purchase_Header", "no", purchHeader.No.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			r.language = fcodeunits.CurrentLanguage()
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Purchase_Header", Table: gtables.PurchaseHeaderTableName, Key: []string{"document_type", "no"},
			TableView:           []ftables.FieldFilter{{Field: "document_type", Expression: "<>0"}}, // not quotes
			RequestFilterFields: []string{"document_type", "no"},
			OnPreDataItem: Trigger(func(purchHeader *gtables.PurchaseHeader, run *Run) error {
				setFilterText(run, purchHeader)
				return nil
			}),
			OnAfterGetRecord: Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{
				r.errorLoop("Header_Error_Loop", &r.headerErrors),
				{
					Name: "Purchase_Line", Table: gtables.PurchaseLineTableName, Key: []string{"document_type", "document_no", "line_no"},
					Link: map[string]string{"document_type": "document_type", "document_no": "no"},
					OnPreDataItem: Trigger(func(purchLine *gtables.PurchaseLine, run *Run) error {
						r.taxLines = map[string]*taxAmountLine{}
						r.applError = false
						return nil
					}),
					OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
					DataItems:        []*DataItem{r.errorLoop("Line_Error_Loop", &r.lineErrors)},
				},
				r.taxCounter(),
			},
		}},
	}
}

func (r *purchDocumentTest) headerOnAfterGetRecord(purchHeader *gtables.PurchaseHeader, run *Run) error {
	r.purchHeader = purchHeader
	r.headerErrors = nil
	run.SetPageNo(1)
	caption := func(field string) string { return purchHeader.GetFieldCaption(field, r.language) }
	buyFromAddr, err := formataddress.PurchHeaderBuyFrom(purchHeader)
	if err != nil {
		return err
	}
	shipToAddr, err := formataddress.PurchHeaderShipTo(purchHeader)
	if err != nil {
		return err
	}
	run.SetValue("buy_from_address", buyFromAddr)
	run.SetValue("ship_to_address", shipToAddr)
	run.SetValue("document_heading", run.Text("PURCHASE_DOCUMENT",
		optionCaption(gtables.PurchaseHeaderTableName, "document_type", purchHeader.Document_type.String(), r.language)))
	if purchHeader.Currency_code == "" {
		run.SetValue("total_text", run.Text("TOTAL"))
		run.SetValue("total_incl_tax_text", run.Text("TOTAL_INCL_TAX"))
	} else {
		run.SetValue("total_text", run.Text("TOTAL_CURRENCY", purchHeader.Currency_code))
		run.SetValue("total_incl_tax_text", run.Text("TOTAL_CURRENCY_INCL_TAX", purchHeader.Currency_code))
	}

	r.receive = run.OptionBool("receive")
	r.invoice = run.OptionBool("invoice")
	recInvText := ""
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
		switch {
		case r.receive && r.invoice:
			recInvText = run.Text("RECEIVE_AND_INVOICE")
		case r.receive:
			recInvText = run.Text("RECEIVE")
		case r.invoice:
			recInvText = run.Text("INVOICE")
		}
		recInvText = run.Text("ORDER_POSTING", recInvText)
	}
	run.SetValue("ship_inv_text", recInvText)

	for _, party := range []struct {
		field string
		no    types.Code
	}{{"buy_from_vendor_no", purchHeader.Buy_from_vendor_no}, {"pay_to_vendor_no", purchHeader.Pay_to_vendor_no}} {
		if party.no == "" {
			r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption(party.field)))
			continue
		}
		vend := gtables.NewVendor(run)
		if vend.Get(party.no) {
			if vend.Blocked {
				r.headerErrors = append(r.headerErrors, r.mustBeFor(run, vend.GetFieldCaption("blocked", r.language), false, vend.TableCaption(r.language), party.no))
			}
		} else {
			r.headerErrors = append(r.headerErrors, run.Text("NOT_EXIST", vend.TableCaption(r.language), party.no))
		}
	}
	purchSetup := gtables.GetPurchasesPayablesSetup(run)
	r.checkPostingDate(run, purchHeader.Posting_date, caption("posting_date"))
	if !purchHeader.Document_date.IsZero() && purchHeader.Document_date.IsClosing() {
		r.headerErrors = append(r.headerErrors, run.Text("NOT_CLOSING_DATE", caption("document_date")))
	}
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Invoice || purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
		r.receive = true
		r.invoice = true
	}
	if !r.receive && !r.invoice {
		r.headerErrors = append(r.headerErrors, run.Text("ENTER_YES_IN", caption("receive"), caption("invoice")))
	}
	if r.invoice {
		purchLine := gtables.NewPurchaseLine(run)
		purchLine.SetRange("document_type", int(purchHeader.Document_type))
		purchLine.SetRange("document_no", purchHeader.No)
		purchLine.SetFilter("quantity", "<>0")
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
			purchLine.SetFilter("qty_to_invoice", "<>0")
		}
		r.invoice = false
		for found := purchLine.FindSetBuffered(); found && !r.invoice; found = purchLine.Next() {
			r.invoice = r.receive || !purchLine.Quantity_received.Sub(purchLine.Quantity_invoiced).IsZero()
		}
	}
	if r.receive {
		purchLine := gtables.NewPurchaseLine(run)
		purchLine.SetRange("document_type", int(purchHeader.Document_type))
		purchLine.SetRange("document_no", purchHeader.No)
		purchLine.SetFilter("quantity", "<>0")
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
			purchLine.SetFilter("qty_to_receive", "<>0")
		}
		purchLine.SetRange("receipt_no", "")
		r.receive = purchLine.FindFirst()
	}
	if !r.receive && !r.invoice {
		r.headerErrors = append(r.headerErrors, run.Text("NOTHING_TO_POST"))
	}
	if r.invoice && purchHeader.Document_type != gtables.PurchaseHeader_Document_type.CreditMemo && purchHeader.Due_date.IsZero() {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("due_date")))
	}
	if r.receive && purchHeader.Receiving_no == "" {
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order ||
			purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Invoice && purchSetup.Receipt_on_invoice {
			if purchHeader.Receiving_no_series == "" {
				r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_ENTERED", caption("receiving_no_series")))
			}
		}
	}
	if r.invoice && purchHeader.Posting_no == "" && purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order && purchHeader.Posting_no_series == "" {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_ENTERED", caption("posting_no_series")))
	}
	if r.invoice {
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order || purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Invoice {
			if purchSetup.Ext_doc_no_mandatory && purchHeader.Vendor_invoice_no == "" {
				r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("vendor_invoice_no")))
			}
		} else {
			if purchSetup.Ext_doc_no_mandatory && purchHeader.Vendor_cr_memo_no == "" {
				r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("vendor_cr_memo_no")))
			}
		}
	}
	if purchHeader.Vendor_invoice_no != "" {
		vendLedgEntry := gtables.NewVendorLedgerEntry(run)
		vendLedgEntry.SetRange("document_type", int(purchHeader.Document_type))
		vendLedgEntry.SetRange("document_no", purchHeader.Vendor_invoice_no)
		vendLedgEntry.SetRange("vendor_no", purchHeader.Pay_to_vendor_no)
		if vendLedgEntry.FindFirst() {
			r.headerErrors = append(r.headerErrors, run.Text("PURCHASE_ALREADY_EXISTS",
				optionCaption(gtables.PurchaseHeaderTableName, "document_type", purchHeader.Document_type.String(), r.language), purchHeader.Vendor_invoice_no))
		}
	}
	return nil
}

func (r *purchDocumentTest) lineOnAfterGetRecord(purchLine *gtables.PurchaseLine, run *Run) error {
	r.lineErrors = nil
	purchHeader := r.purchHeader
	caption := func(field string) string { return purchLine.GetFieldCaption(field, r.language) }
	r.addTaxLine(purchLine.Tax_percent, purchLine.Amount, purchLine.Amount_including_tax)

	if purchLine.Document_type != gtables.PurchaseLine_Document_type.Order {
		if !purchLine.Qty_to_receive.Equal(purchLine.Quantity) {
			r.lineErrors = append(r.lineErrors, run.Text("MUST_BE", caption("qty_to_receive"), formatValue(purchLine.Quantity, "0:5", r.language)))
		}
		if !purchLine.Qty_to_invoice.Equal(purchLine.Quantity) {
			r.lineErrors = append(r.lineErrors, run.Text("MUST_BE", caption("qty_to_invoice"), formatValue(purchLine.Quantity, "0:5", r.language)))
		}
	}
	if !r.receive {
		purchLine.Qty_to_receive = types.ZeroDecimal()
	}
	if purchLine.Document_type == gtables.PurchaseLine_Document_type.Invoice && purchLine.Receipt_no != "" {
		purchLine.Quantity_received = purchLine.Quantity
		purchLine.Qty_to_receive = types.ZeroDecimal()
	}
	if r.invoice {
		maxQtyToBeInvoiced := purchLine.Qty_to_receive.Add(purchLine.Quantity_received).Sub(purchLine.Quantity_invoiced)
		if purchLine.Qty_to_invoice.Abs().GreaterThan(maxQtyToBeInvoiced.Abs()) {
			purchLine.Qty_to_invoice = maxQtyToBeInvoiced
		}
	} else {
		purchLine.Qty_to_invoice = types.ZeroDecimal()
	}

	if purchLine.Gen_prod_posting_group != "" {
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo &&
			purchHeader.Applies_to_doc_type == gtables.PurchaseHeader_Applies_to_doc_type.Invoice && purchHeader.Applies_to_doc_no != "" {
			vendLedgEntry := gtables.NewVendorLedgerEntry(run)
			vendLedgEntry.SetRange("vendor_no", purchHeader.Pay_to_vendor_no)
			vendLedgEntry.SetRange("document_type", int(gtables.VendorLedgerEntry_Document_type.Invoice))
			vendLedgEntry.SetRange("document_no", purchHeader.Applies_to_doc_no)
			if !vendLedgEntry.FindLast() && !r.applError {
				r.applError = true
				r.lineErrors = append(r.lineErrors, run.Text("APPLIES_TO_NOT_ON_VENDOR_ENTRIES", purchHeader.GetFieldCaption("applies_to_doc_no", r.language), purchHeader.Applies_to_doc_no))
			}
		}
		taxPostingSetup := gtables.NewTaxPostingSetup(run)
		if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": purchLine.Tax_bus_posting_group, "tax_prod_posting_group": purchLine.Tax_prod_posting_group}) {
			r.lineErrors = append(r.lineErrors, run.Text("SETUP_NOT_EXIST", taxPostingSetup.TableCaption(r.language), purchLine.Tax_bus_posting_group, purchLine.Tax_prod_posting_group))
		}
	}

	if !purchLine.Quantity.IsZero() {
		if purchLine.No == "" {
			r.lineErrors = append(r.lineErrors, run.Text("TYPE_NO_MUST_BE_SPECIFIED",
				optionCaption(gtables.PurchaseLineTableName, "type", purchLine.Type.String(), r.language), caption("no")))
		}
		if purchLine.Type == gtables.PurchaseLine_Type.Blank {
			r.lineErrors = append(r.lineErrors, run.Text("MUST_BE_SPECIFIED", caption("type")))
		}
	} else {
		if !purchLine.Amount.IsZero() {
			r.lineErrors = append(r.lineErrors, run.Text("AMOUNT_ZERO_WHEN_NO_QUANTITY", caption("amount"), caption("quantity")))
		}
	}

	r.checkReceiptLines(run, purchLine)

	if purchLine.Type != gtables.PurchaseLine_Type.Blank && !purchLine.Qty_to_invoice.IsZero() {
		genPostingSetup := gtables.NewGeneralPostingSetup(run)
		if !genPostingSetup.Get(map[string]interface{}{"gen_bus_posting_group": purchLine.Gen_bus_posting_group, "gen_prod_posting_group": purchLine.Gen_prod_posting_group}) {
			r.lineErrors = append(r.lineErrors, run.Text("SETUP_NOT_EXIST", genPostingSetup.TableCaption(r.language), purchLine.Gen_bus_posting_group, purchLine.Gen_prod_posting_group))
		}
	}

	switch purchLine.Type {
	case gtables.PurchaseLine_Type.GLAccount:
		if purchLine.No == "" && purchLine.Amount.IsZero() {
			return nil
		}
		if purchLine.No != "" {
			glAcc := gtables.NewGLAccount(run)
			if glAcc.Get(purchLine.No) {
				if glAcc.Blocked {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, glAcc.GetFieldCaption("blocked", r.language), false, glAcc.TableCaption(r.language), purchLine.No))
				}
				if !glAcc.Direct_posting {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, glAcc.GetFieldCaption("direct_posting", r.language), true, glAcc.TableCaption(r.language), purchLine.No))
				}
			} else {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_EXIST", glAcc.TableCaption(r.language), purchLine.No))
			}
		}
	case gtables.PurchaseLine_Type.Item:
		if purchLine.No == "" && purchLine.Quantity.IsZero() {
			return nil
		}
		if purchLine.No != "" {
			item := gtables.NewItem(run)
			if item.Get(purchLine.No) {
				if item.Blocked {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, item.GetFieldCaption("blocked", r.language), false, item.TableCaption(r.language), purchLine.No))
				}
				if item.Costing_method == gtables.Item_Costing_method.Specific && purchLine.Serial_no == "" {
					r.lineErrors = append(r.lineErrors, run.Text("MUST_BE_FILLED_WHEN", caption("serial_no"), item.GetFieldCaption("costing_method", r.language),
						optionCaption(gtables.ItemTableName, "costing_method", item.Costing_method.String(), r.language)))
				}
			} else {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_EXIST", item.TableCaption(r.language), purchLine.No))
			}
		}
		r.checkSerialNo(run, purchLine.Serial_no, purchLine.Quantity, purchLine.Quantity_invoiced, caption)
	case gtables.PurchaseLine_Type.FixedAsset:
		if purchLine.No == "" && purchLine.Quantity.IsZero() {
			return nil
		}
		if purchLine.No != "" {
			fa := gtables.NewFixedAsset(run)
			if fa.Get(purchLine.No) {
				if fa.Blocked {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, fa.GetFieldCaption("blocked", r.language), false, fa.TableCaption(r.language), purchLine.No))
				}
				if fa.Inactive {
					r.lineErrors = append(r.lineErrors, r.mustBeFor(run, fa.GetFieldCaption("inactive", r.language), false, fa.TableCaption(r.language), purchLine.No))
				}
			} else {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_EXIST", fa.TableCaption(r.language), purchLine.No))
			}
		}
	}
	return nil
}

// checkReceiptLines: what is invoiced beyond what is received now comes from the line's receipt
// lines, which must still be the line's
func (r *purchDocumentTest) checkReceiptLines(run *Run, line *gtables.PurchaseLine) {
	caption := func(field string) string { return line.GetFieldCaption(field, r.language) }
	qtyToReceive := line.Qty_to_receive
	remQtyToBeInvoiced := line.Qty_to_invoice
	if line.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
		qtyToReceive = qtyToReceive.Neg()
		remQtyToBeInvoiced = remQtyToBeInvoiced.Neg()
	}
	if !remQtyToBeInvoiced.Abs().GreaterThan(line.Qty_to_receive.Abs()) {
		return
	}
	purchRcptLine := gtables.NewPurchRcptLine(run)
	switch line.Document_type {
	case gtables.PurchaseLine_Document_type.Order:
		purchRcptLine.SetRange("order_no", line.Document_no)
		purchRcptLine.SetRange("order_line_no", line.Line_no)
	case gtables.PurchaseLine_Document_type.Invoice:
		purchRcptLine.SetRange("document_no", line.Receipt_no)
		purchRcptLine.SetRange("line_no", line.Receipt_line_no)
	}
	for found := purchRcptLine.FindSetBuffered(); found && remQtyToBeInvoiced.Abs().GreaterThan(line.Qty_to_receive.Abs()); found = purchRcptLine.Next() {
		if purchRcptLine.Quantity_invoiced.Equal(purchRcptLine.Quantity) {
			continue
		}
		notSame := func(same bool, field string) {
			if !same {
				r.lineErrors = append(r.lineErrors, run.Text("NOT_SAME_ON_RECEIPT", caption(field)))
			}
		}
		notSame(purchRcptLine.Buy_from_vendor_no == line.Buy_from_vendor_no, "buy_from_vendor_no")
		notSame(int(purchRcptLine.Type) == int(line.Type), "type")
		notSame(purchRcptLine.No == line.No, "no")
		notSame(purchRcptLine.Gen_bus_posting_group == line.Gen_bus_posting_group, "gen_bus_posting_group")
		notSame(purchRcptLine.Gen_prod_posting_group == line.Gen_prod_posting_group, "gen_prod_posting_group")
		notSame(purchRcptLine.Location_code == line.Location_code, "location_code")
		notSame(purchRcptLine.Dimension_1_code == line.Dimension_1_code, "dimension_1_code")
		notSame(purchRcptLine.Dimension_2_code == line.Dimension_2_code, "dimension_2_code")
		notSame(purchRcptLine.Serial_no == line.Serial_no, "serial_no")
		if remQtyToBeInvoiced.Mul(purchRcptLine.Quantity).IsNegative() {
			r.lineErrors = append(r.lineErrors, run.Text("SAME_SIGN_AS_RECEIPT", caption("qty_to_invoice")))
		}
		qtyToBeInvoiced := remQtyToBeInvoiced.Sub(qtyToReceive)
		if qtyToBeInvoiced.Abs().GreaterThan(purchRcptLine.Quantity.Sub(purchRcptLine.Quantity_invoiced).Abs()) {
			qtyToBeInvoiced = purchRcptLine.Quantity.Sub(purchRcptLine.Quantity_invoiced)
		}
		remQtyToBeInvoiced = remQtyToBeInvoiced.Sub(qtyToBeInvoiced)
	}
	if remQtyToBeInvoiced.Abs().GreaterThan(line.Qty_to_receive.Abs()) {
		if line.Document_type == gtables.PurchaseLine_Document_type.Invoice {
			r.lineErrors = append(r.lineErrors, run.Text("RECEIPT_INVOICED", line.Receipt_no))
		} else {
			r.lineErrors = append(r.lineErrors, run.Text("RECEIPT_LINES_DELETED"))
		}
	}
}
