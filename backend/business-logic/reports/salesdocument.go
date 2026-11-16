package reports

import (
	"strings"
	"unicode/utf8"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salespost"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesprinted"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesDocument is the printout of a sales document of one type — report 204 Sales - Quote
// (salesquote.go), 205 Order Confirmation (orderconfirmation.go), 210 Blanket Sales Order
// (blanketorders.go): the company (with its logo), the bill-to and ship-to addresses, the
// document's terms, its lines, totals and comments, in copies; a print that is not a preview
// counts in the document's No. Printed (codeunit 313). Run from a document it prints that
// document. Each document prints in its Language Code (the user's language when blank). The
// totals name the tax as VAT or sales tax by the lines' tax calculation, with the amounts
// subject to and exempt from it; with Prices Including Tax the prices, line amounts and totals
// include the tax. Each report has its layout and texts.
type salesDocument struct {
	documentType  gtables.SalesHeaderDocument_type
	salesHeader   *gtables.SalesHeader
	pricesInclTax bool
	noOfLoops     int
	printed       int // documents printed
}

func (r *salesDocument) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "no_of_copies", Type: "number", Default: "0"},
			{Name: "print_company_address", Type: "bool", Default: "true"},
			{Name: "print_comments", Type: "bool", Default: "true"},
		},
		ContextTable: gtables.SalesHeaderTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "SalesHeader",
			Table:               gtables.SalesHeaderTableName,
			Key:                 []string{"document_type", "no"},
			RequestFilterFields: []string{"no", "sell_to_customer_no", "bill_to_customer_no", "no_printed"},
			OnPreDataItem:       Trigger(r.salesHeaderOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.salesHeaderOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.salesHeaderOnPostDataItem),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem:   Trigger(r.copyLoopOnPostDataItem),
				DataItems: []*DataItem{{
					Name:             "SalesLine",
					Table:            gtables.SalesLineTableName,
					Key:              []string{"document_type", "document_no", "line_no"},
					OnPreDataItem:    Trigger(r.salesLineOnPreDataItem),
					OnAfterGetRecord: Trigger(r.salesLineOnAfterGetRecord),
				}, {
					Name:          "SalesCommentLine",
					Table:         gtables.SalesCommentLineTableName,
					Key:           []string{"document_type", "no", "line_no"},
					OnPreDataItem: Trigger(r.salesCommentLineOnPreDataItem),
				}},
			}},
		}},
	}
}

// onInitReport: run from a document (the card or the list), the request page starts filtered
// to it
func (r *salesDocument) onInitReport(run *Run) error {
	salesHeader, ok := run.Context().(*gtables.SalesHeader)
	if ok && salesHeader.No != "" {
		run.SetRequestFilter("SalesHeader", "no", salesHeader.No.String())
	}
	return nil
}

func (r *salesDocument) onPreReport(run *Run) error {
	return setCompanyValues(run)
}

// setCompanyValues: with Print Company Address the company's address, logo, phone, e-mail and
// VAT registration no. for the document printouts (CompanyInformation.GET, FormatAddr.Company)
func setCompanyValues(run *Run) error {
	companyInfo := gtables.GetCompanyInformation(run)
	if run.OptionBool("print_company_address") {
		companyAddress, err := formataddress.Company(companyInfo)
		if err != nil {
			return err
		}
		run.SetValue("company_address", companyAddress)
		run.SetValue("company_picture", companyInfo.Picture)
		run.SetValue("company_phone_no", companyInfo.Phone_no.String())
		run.SetValue("company_e_mail", companyInfo.E_mail.String())
		run.SetValue("company_vat_registration_no", companyInfo.Vat_registration_no.String())
	}
	return nil
}

func (r *salesDocument) salesHeaderOnPreDataItem(rec *gtables.SalesHeader, run *Run) error {
	rec.FilterGroup(2)
	rec.SetRange("document_type", r.documentType)
	rec.FilterGroup(0)
	return nil
}

func (r *salesDocument) salesHeaderOnAfterGetRecord(rec *gtables.SalesHeader, run *Run) error {
	r.salesHeader = rec
	r.pricesInclTax = rec.Prices_including_tax
	run.SetLanguage(rec.Language_code.String())
	billToAddress, err := formataddress.SalesHeaderBillTo(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.SalesHeaderShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("bill_to_address", billToAddress)
	run.SetValue("ship_to_address", shipToAddress)

	setTermsValues(run, rec, rec.Salesperson_code, rec.Shipment_method_code, rec.Payment_terms_code)

	totals, err := salespost.SumSalesLines(rec, salespost.General)
	if err != nil {
		return err
	}
	totalSalesLine := totals.TotalSalesLine
	run.SetValue("subtotal", totalSalesLine.Amount.Add(totalSalesLine.Inv_discount_amount))
	// the invoice discount is without the tax: printed as such with Prices Including Tax
	if r.pricesInclTax {
		run.SetValue("invoice_discount", types.Decimal{})
		run.SetValue("invoice_discount_excl", totalSalesLine.Inv_discount_amount.Neg())
	} else {
		run.SetValue("invoice_discount", totalSalesLine.Inv_discount_amount.Neg())
		run.SetValue("invoice_discount_excl", types.Decimal{})
	}
	run.SetValue("tax_amount", totals.TaxAmount)
	run.SetValue("total", totalSalesLine.Amount_including_tax)
	run.SetValue("total_excl_tax", totalSalesLine.Amount)
	r.setTaxValues(run, rec)
	setCurrencyValue(run, rec, rec.Currency_code)
	r.noOfLoops = noOfLoops(run)
	r.printed++
	run.SetFileName(run.Text("FILE_NAME", rec.No))
	return nil
}

// salesHeaderOnPostDataItem: several documents go into one file named as the report
func (r *salesDocument) salesHeaderOnPostDataItem(rec *gtables.SalesHeader, run *Run) error {
	if r.printed > 1 {
		run.SetFileName("")
	}
	return nil
}

func (r *salesDocument) copyLoopOnPreDataItem(rec *Integer, run *Run) error {
	rec.SetRange("number", 1, r.noOfLoops)
	return nil
}

func (r *salesDocument) copyLoopOnAfterGetRecord(rec *Integer, run *Run) error {
	run.SetPageNo(1)
	if rec.Number > 1 {
		run.SetValue("copy_text", run.Text("COPY"))
	} else {
		run.SetValue("copy_text", "")
	}
	return nil
}

// copyLoopOnPostDataItem: the document is printed (not when only previewed)
func (r *salesDocument) copyLoopOnPostDataItem(rec *Integer, run *Run) error {
	if !run.Preview() {
		return salesprinted.Run(r.salesHeader)
	}
	return nil
}

func (r *salesDocument) salesLineOnPreDataItem(rec *gtables.SalesLine, run *Run) error {
	rec.SetRange("document_type", gtables.SalesLineDocument_type(r.salesHeader.Document_type))
	rec.SetRange("document_no", r.salesHeader.No)
	return nil
}

// salesLineOnAfterGetRecord: the line's amount before the invoice discount — with Prices
// Including Tax the amount including the tax (after the invoice discount: the tax is on it)
func (r *salesDocument) salesLineOnAfterGetRecord(rec *gtables.SalesLine, run *Run) error {
	if r.pricesInclTax {
		run.SetValue("line_amount", rec.Amount_including_tax)
	} else {
		run.SetValue("line_amount", rec.Amount.Add(rec.Inv_discount_amount))
	}
	run.SetValue("description_2", rec.Description_2.String())
	return nil
}

// salesCommentLineOnPreDataItem: the document's comments, with Print Comments
func (r *salesDocument) salesCommentLineOnPreDataItem(rec *gtables.SalesCommentLine, run *Run) error {
	if !run.OptionBool("print_comments") {
		return Break()
	}
	rec.SetRange("document_type", gtables.SalesCommentLineDocument_type(r.salesHeader.Document_type))
	rec.SetRange("no", r.salesHeader.No)
	return nil
}

// setTaxValues: the captions of the prices, the line amounts and the totals in the document's
// language, and the amounts subject to and exempt from the tax. The tax is VAT when a line is
// calculated with VAT, sales tax otherwise. A line is subject to the tax when it has tax; the
// amounts are after the invoice discount, without the tax. They print with sales tax, and with
// VAT when part of the document is exempt.
func (r *salesDocument) setTaxValues(run *Run, salesHeader *gtables.SalesHeader) {
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
	salesLine.SetRange("document_no", salesHeader.No)
	salesLine.SetFilter("type", "<>0")
	isVAT := false
	var amountSubject, amountExempt types.Decimal
	for found := salesLine.FindSet(); found; found = salesLine.Next() {
		if salesLine.Tax_calculation_type != gtables.SalesLine_Tax_calculation_type.SalesTax {
			isVAT = true
		}
		if salesLine.Amount_including_tax.Equal(salesLine.Amount) {
			amountExempt = amountExempt.Add(salesLine.Amount)
		} else {
			amountSubject = amountSubject.Add(salesLine.Amount)
		}
	}
	taxName, taxShortName := run.Text("TAX_SALES_TAX"), run.Text("TAX_SALES_TAX_SHORT")
	if isVAT {
		taxName, taxShortName = run.Text("TAX_VAT"), run.Text("TAX_VAT_SHORT")
	}
	run.SetValue("prices_incl_tax", r.pricesInclTax)
	run.SetValue("tax_caption", upperFirst(taxName))
	if r.pricesInclTax {
		run.SetValue("unit_price_caption", run.Text("DOC_PRICE_INCL", taxShortName))
		run.SetValue("line_amount_caption", run.Text("DOC_AMOUNT_INCL", taxShortName))
	} else {
		run.SetValue("unit_price_caption", run.Text("UNIT_PRICE"))
		run.SetValue("line_amount_caption", run.Text("TOTAL_PRICE"))
	}
	run.SetValue("total_incl_tax_caption", run.Text("DOC_TOTAL_INCL", taxName))
	run.SetValue("total_excl_tax_caption", run.Text("DOC_TOTAL_EXCL", taxName))
	run.SetValue("tax_included_caption", run.Text("DOC_TAX_INCLUDED", taxName))
	run.SetValue("invoice_discount_excl_caption", run.Text("DOC_INV_DISCOUNT_EXCL", taxName))
	run.SetValue("amount_subject_caption", run.Text("DOC_AMOUNT_SUBJECT_TO", taxName))
	run.SetValue("amount_exempt_caption", run.Text("DOC_AMOUNT_EXEMPT_FROM", taxName))
	run.SetValue("amount_subject", amountSubject)
	run.SetValue("amount_exempt", amountExempt)
	run.SetValue("show_tax_base", !isVAT || !amountExempt.IsZero())
}

// setTermsValues: the names of the salesperson, the shipment method and the payment terms
// (blank when they do not exist)
func setTermsValues(run *Run, src ftables.Source, salespersonCode, shipmentMethodCode, paymentTermsCode types.Code) {
	salesPurchPerson := gtables.NewSalespersonPurchaser(src)
	if salespersonCode == "" {
		salesPurchPerson.Init()
	} else {
		if !salesPurchPerson.Get(salespersonCode) {
			salesPurchPerson.Init()
		}
	}
	run.SetValue("salesperson", salesPurchPerson.Name.String())
	shipmentMethod := gtables.NewShipmentMethod(src)
	if shipmentMethodCode == "" {
		shipmentMethod.Init()
	} else {
		if !shipmentMethod.Get(shipmentMethodCode) {
			shipmentMethod.Init()
		}
	}
	run.SetValue("ship_via", shipmentMethod.Description.String())
	paymentTerms := gtables.NewPaymentTerms(src)
	if paymentTermsCode == "" {
		paymentTerms.Init()
	} else {
		if !paymentTerms.Get(paymentTermsCode) {
			paymentTerms.Init()
		}
	}
	run.SetValue("terms", paymentTerms.Description.String())
}

// setCurrencyValue: the document's currency, or the local currency code
func setCurrencyValue(run *Run, src ftables.Source, currencyCode types.Code) {
	if currencyCode == "" {
		glSetup := gtables.GetGeneralLedgerSetup(src)
		run.SetValue("currency", glSetup.Lcy_code.String())
	} else {
		run.SetValue("currency", currencyCode.String())
	}
}

// noOfLoops: the original and the No. of Copies (NoLoops := 1 + ABS(NoCopies))
func noOfLoops(run *Run) int {
	noOfCopies := run.OptionInt("no_of_copies")
	if noOfCopies < 0 {
		noOfCopies = -noOfCopies
	}
	return 1 + noOfCopies
}

// upperFirst: a text with its first letter in upper case (a tax's name as a caption of its own)
func upperFirst(text string) string {
	r, size := utf8.DecodeRuneInString(text)
	if size == 0 {
		return text
	}
	return strings.ToUpper(string(r)) + text[size:]
}
