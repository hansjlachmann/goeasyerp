package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// salesHeaderLogic is the business logic of table 36 Sales Header: quotes, orders, invoices,
// credit memos and blanket orders.
//
// Table triggers cannot ask the user (HideValidationDialog): a changed customer, currency or
// posting group recreates the lines as when the user answers Yes, and the "has not been changed
// on the existing sales lines" messages are not shown. Not migrated yet: the credit limit check
// (codeunit 312), Applies-to, Bal. Account, jobs, shipping/posting numbers, Reserve, the VAT
// Base Discount (sales tax only), foreign trade statistics, extended texts and the exchange rate
// change (form 511).
type salesHeaderLogic struct{}

func init() {
	gtables.RegisterSalesHeaderLogic(salesHeaderLogic{})
	functions.Register("table", gtables.SalesHeaderTableName, "AssistEdit", functions.ForAssistEdit(AssistEditSalesHeader), functions.WithAssistEdit())
}

// OnInsert: the next number of the document type's series, then the dates and the posting
// description (InitRecord)
func (salesHeaderLogic) OnInsert(rec *gtables.SalesHeader) error {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	if rec.No == "" {
		err := testSalesNoSeries(rec, salesSetup)
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(getSalesNoSeriesCode(rec, salesSetup), xRec.No_series, rec.Posting_date, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	return InitSalesHeaderRecord(rec)
}

// OnDelete: the document's lines (with their triggers) and comments
func (salesHeaderLogic) OnDelete(rec *gtables.SalesHeader) error {
	salesLine := gtables.NewSalesLine(rec)
	salesLine.LockTable()
	salesLine.SetRange("document_type", rec.Document_type)
	salesLine.SetRange("document_no", rec.No)
	for found := salesLine.FindSet(); found; found = salesLine.Next() {
		err := salesLine.DeleteOrError(true)
		if err != nil {
			return err
		}
	}
	salesCommentLine := gtables.NewSalesCommentLine(rec)
	salesCommentLine.SetRange("document_type", rec.Document_type)
	salesCommentLine.SetRange("no", rec.No)
	salesCommentLine.DeleteAll()
	return nil
}

// OnRename: a sales document is never renamed
func (salesHeaderLogic) OnRename(rec *gtables.SalesHeader) error {
	return apperrors.RenameNotAllowed(gtables.SalesHeaderTableName)
}

// InitSalesHeaderRecord (InitRecord): the series of the posted documents (Posting/Shipping No.
// Series), order and invoice dates from the work date (a quote's order date too), the posting
// date (not on quotes), the document date and the posting description ("Quote SQ000001")
func InitSalesHeaderRecord(rec *gtables.SalesHeader) error {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	noSeriesMgt := noseries.New(rec)
	var err error
	switch rec.Document_type {
	case gtables.SalesHeader_Document_type.Quote, gtables.SalesHeader_Document_type.Order:
		err = noSeriesMgt.SetDefaultSeries(&rec.Posting_no_series, salesSetup.Posted_invoice_nos)
		if err != nil {
			return err
		}
		err = noSeriesMgt.SetDefaultSeries(&rec.Shipping_no_series, salesSetup.Posted_shipment_nos)
	case gtables.SalesHeader_Document_type.Invoice:
		if rec.No_series != "" && salesSetup.Invoice_nos == salesSetup.Posted_invoice_nos {
			rec.Posting_no_series = rec.No_series
		} else {
			err = noSeriesMgt.SetDefaultSeries(&rec.Posting_no_series, salesSetup.Posted_invoice_nos)
		}
		if err == nil && salesSetup.Shipment_on_invoice {
			err = noSeriesMgt.SetDefaultSeries(&rec.Shipping_no_series, salesSetup.Posted_shipment_nos)
		}
	case gtables.SalesHeader_Document_type.CreditMemo:
		if rec.No_series != "" && salesSetup.Credit_memo_nos == salesSetup.Posted_credit_memo_nos {
			rec.Posting_no_series = rec.No_series
		} else {
			err = noSeriesMgt.SetDefaultSeries(&rec.Posting_no_series, salesSetup.Posted_credit_memo_nos)
		}
	}
	if err != nil {
		return err
	}

	if rec.Document_type == gtables.SalesHeader_Document_type.Order || rec.Document_type == gtables.SalesHeader_Document_type.Invoice {
		rec.Shipment_date = system.WorkDate()
		rec.Order_date = system.WorkDate()
	}
	// a quote is dated too (decided 2026-10-28; the classic leaves its Order Date blank): its
	// prices are those of the order date
	if rec.Document_type == gtables.SalesHeader_Document_type.Quote {
		rec.Order_date = system.WorkDate()
	}
	if rec.Document_type != gtables.SalesHeader_Document_type.Quote && rec.Posting_date.IsZero() {
		rec.Posting_date = system.WorkDate()
	}
	rec.Document_date = system.WorkDate()
	documentType := i18n.GetInstance().OptionCaption(gtables.SalesHeaderTableName, "document_type", i18n.Key(rec.Document_type.String()), fcodeunits.CurrentLanguage())
	rec.Posting_description = types.Text(system.CopyStr(documentType+" "+rec.No.String(), 1, rec.MaxStrLen("posting_description")))
	return nil
}

// AssistEditSalesHeader (AssistEdit): No. from the document type's series or one related to it
func AssistEditSalesHeader(rec *gtables.SalesHeader) (bool, error) {
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	err := testSalesNoSeries(rec, salesSetup)
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(getSalesNoSeriesCode(rec, salesSetup), xRec.No_series, &noSeries)
	if err != nil {
		return false, err
	}
	if picked {
		err = noSeriesMgt.SetSeries(&rec.No)
		if err != nil {
			return false, err
		}
		rec.No_series = noSeries
	}
	return picked, nil
}

// testSalesNoSeries (TestNoSeries): the setup has the document type's number series
func testSalesNoSeries(rec *gtables.SalesHeader, salesSetup *gtables.SalesReceivablesSetup) error {
	switch rec.Document_type {
	case gtables.SalesHeader_Document_type.Quote:
		return salesSetup.TestField("quote_nos")
	case gtables.SalesHeader_Document_type.Order:
		return salesSetup.TestField("order_nos")
	case gtables.SalesHeader_Document_type.Invoice:
		err := salesSetup.TestField("invoice_nos")
		if err != nil {
			return err
		}
		return salesSetup.TestField("posted_invoice_nos")
	case gtables.SalesHeader_Document_type.CreditMemo:
		err := salesSetup.TestField("credit_memo_nos")
		if err != nil {
			return err
		}
		return salesSetup.TestField("posted_credit_memo_nos")
	case gtables.SalesHeader_Document_type.BlanketOrder:
		return salesSetup.TestField("blanket_order_nos")
	}
	return nil
}

// getSalesNoSeriesCode (GetNoSeriesCode): the number series of the document type
func getSalesNoSeriesCode(rec *gtables.SalesHeader, salesSetup *gtables.SalesReceivablesSetup) types.Code {
	switch rec.Document_type {
	case gtables.SalesHeader_Document_type.Quote:
		return salesSetup.Quote_nos
	case gtables.SalesHeader_Document_type.Order:
		return salesSetup.Order_nos
	case gtables.SalesHeader_Document_type.Invoice:
		return salesSetup.Invoice_nos
	case gtables.SalesHeader_Document_type.CreditMemo:
		return salesSetup.Credit_memo_nos
	case gtables.SalesHeader_Document_type.BlanketOrder:
		return salesSetup.Blanket_order_nos
	}
	return ""
}

// OnValidate_No: a number typed by hand needs Manual Nos. (one the series just issued keeps
// its series, as on the customer)
func (salesHeaderLogic) OnValidate_No(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		salesSetup := gtables.GetSalesReceivablesSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(getSalesNoSeriesCode(rec, salesSetup), rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(getSalesNoSeriesCode(rec, salesSetup))
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	return nil
}

// OnValidate_Sell_to_customer_no: the customer's name, address, posting and tax groups; the
// bill-to customer (the customer's Bill-to Customer No. or itself) and the ship-to address;
// a changed customer recreates the lines
func (salesHeaderLogic) OnValidate_Sell_to_customer_no(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	if rec.Sell_to_customer_no != xRec.Sell_to_customer_no && xRec.Sell_to_customer_no != "" {
		if rec.Sell_to_customer_no == "" {
			if salesLinesExist(rec) {
				return apperrors.SalesResetCustomerLines(gtables.SalesHeaderTableName, "sell_to_customer_no")
			}
			rec.Init()
			err := InitSalesHeaderRecord(rec)
			if err != nil {
				return err
			}
			rec.No_series = xRec.No_series
			return nil
		}
	}

	cust := gtables.NewCustomer(rec)
	err := cust.GetOrError(rec.Sell_to_customer_no)
	if err != nil {
		return err
	}
	err = cust.TestField("blocked", false)
	if err != nil {
		return err
	}
	rec.Sell_to_customer_name = cust.Name
	rec.Sell_to_customer_name_2 = cust.Name_2
	rec.Sell_to_address = cust.Address
	rec.Sell_to_address_2 = cust.Address_2
	rec.Sell_to_city = cust.City
	rec.Sell_to_post_code = cust.Post_code
	rec.Sell_to_state = cust.State
	rec.Sell_to_country_code = cust.Country_code
	rec.Sell_to_contact = cust.Contact
	rec.Gen_bus_posting_group = cust.Gen_bus_posting_group
	rec.Tax_bus_posting_group = cust.Tax_bus_posting_group
	rec.Tax_area_code = cust.Tax_area_code
	rec.Tax_liable = cust.Tax_liable
	rec.Vat_registration_no = cust.Vat_registration_no
	if cust.Bill_to_customer_no != "" {
		err = rec.Validate("bill_to_customer_no", cust.Bill_to_customer_no)
	} else {
		err = rec.Validate("bill_to_customer_no", rec.Sell_to_customer_no)
	}
	if err != nil {
		return err
	}
	err = rec.Validate("ship_to_code", types.Code(""))
	if err != nil {
		return err
	}

	if xRec.Sell_to_customer_no != rec.Sell_to_customer_no || xRec.Currency_code != rec.Currency_code {
		return RecreateSalesLines(rec)
	}
	return nil
}

// OnValidate_Bill_to_customer_no: the bill-to customer's name and address, payment terms and
// method, department/project, posting group, currency, prices and discounts, language,
// salesperson; a changed bill-to customer (same sell-to) recreates the lines
func (salesHeaderLogic) OnValidate_Bill_to_customer_no(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	cust := gtables.NewCustomer(rec)
	err := cust.GetOrError(rec.Bill_to_customer_no)
	if err != nil {
		return err
	}
	err = cust.TestField("blocked", false)
	if err != nil {
		return err
	}
	err = cust.TestField("customer_posting_group")
	if err != nil {
		return err
	}
	rec.Bill_to_name = cust.Name
	rec.Bill_to_name_2 = cust.Name_2
	rec.Bill_to_address = cust.Address
	rec.Bill_to_address_2 = cust.Address_2
	rec.Bill_to_city = cust.City
	rec.Bill_to_post_code = cust.Post_code
	rec.Bill_to_state = cust.State
	rec.Bill_to_country_code = cust.Country_code
	rec.Bill_to_contact = cust.Contact
	rec.Payment_terms_code = cust.Payment_terms_code
	if rec.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
		rec.Payment_method_code = ""
	} else {
		rec.Payment_method_code = cust.Payment_method_code
	}
	rec.Department_code = cust.Department_code
	rec.Project_code = cust.Project_code
	rec.Customer_posting_group = cust.Customer_posting_group
	rec.Currency_code = cust.Currency_code
	rec.Price_group_code = cust.Price_group_code
	rec.Prices_including_tax = cust.Prices_including_tax
	rec.Allow_quantity_disc = cust.Allow_quantity_disc
	rec.Invoice_disc_code = cust.Invoice_disc_code
	rec.Cust_item_disc_gr = cust.Cust_item_disc_gr
	rec.Language_code = cust.Language_code
	rec.Salesperson_code = cust.Salesperson_code
	err = validateSalesPaymentTerms(rec)
	if err != nil {
		return err
	}
	// the currency validated from code (not the user's field): only the factor
	err = updateSalesCurrencyFactor(rec)
	if err != nil {
		return err
	}

	if xRec.Sell_to_customer_no == rec.Sell_to_customer_no && xRec.Bill_to_customer_no != rec.Bill_to_customer_no {
		return RecreateSalesLines(rec)
	}
	return nil
}

// OnValidate_Ship_to_code: the ship-to address's (or else the customer's) address, shipment
// method, location, shipping agent and tax area; a changed tax area recreates the lines
func (salesHeaderLogic) OnValidate_Ship_to_code(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	if rec.Ship_to_code != "" {
		shipToAddr := gtables.NewShipToAddress(rec)
		err := shipToAddr.GetOrError(map[string]interface{}{"customer_no": rec.Sell_to_customer_no, "code": rec.Ship_to_code})
		if err != nil {
			return err
		}
		rec.Ship_to_name = shipToAddr.Name
		rec.Ship_to_name_2 = shipToAddr.Name_2
		rec.Ship_to_address = shipToAddr.Address
		rec.Ship_to_address_2 = shipToAddr.Address_2
		rec.Ship_to_city = shipToAddr.City
		rec.Ship_to_post_code = shipToAddr.Post_code
		rec.Ship_to_state = shipToAddr.State
		rec.Ship_to_country_code = shipToAddr.Country_code
		rec.Ship_to_contact = shipToAddr.Contact
		rec.Shipment_method_code = shipToAddr.Shipment_method_code
		rec.Location_code = shipToAddr.Location_code
		rec.Vat_country_code = shipToAddr.Country_code
		rec.Shipping_agent_code = shipToAddr.Shipping_agent_code
		if shipToAddr.Tax_area_code != "" {
			rec.Tax_area_code = shipToAddr.Tax_area_code
		}
	} else {
		cust := gtables.NewCustomer(rec)
		err := cust.GetOrError(rec.Sell_to_customer_no)
		if err != nil {
			return err
		}
		rec.Ship_to_name = cust.Name
		rec.Ship_to_name_2 = cust.Name_2
		rec.Ship_to_address = cust.Address
		rec.Ship_to_address_2 = cust.Address_2
		rec.Ship_to_city = cust.City
		rec.Ship_to_post_code = cust.Post_code
		rec.Ship_to_state = cust.State
		rec.Ship_to_country_code = cust.Country_code
		rec.Ship_to_contact = cust.Contact
		rec.Shipment_method_code = cust.Shipment_method_code
		rec.Location_code = cust.Location_code
		rec.Vat_country_code = cust.Country_code
		rec.Shipping_agent_code = cust.Shipping_agent_code
	}

	if xRec.Sell_to_customer_no == rec.Sell_to_customer_no && xRec.Ship_to_code != rec.Ship_to_code {
		if xRec.Vat_country_code != rec.Vat_country_code || xRec.Tax_area_code != rec.Tax_area_code {
			return RecreateSalesLines(rec)
		}
	}
	return nil
}

// OnValidate_Posting_date: the document date follows; a currency's factor on the new date
func (salesHeaderLogic) OnValidate_Posting_date(rec *gtables.SalesHeader) error {
	err := rec.Validate("document_date", rec.Posting_date)
	if err != nil {
		return err
	}
	if rec.Currency_code != "" {
		xRec := rec.XRec()
		err = updateSalesCurrencyFactor(rec)
		if err != nil {
			return err
		}
		if !rec.Currency_factor.Equal(xRec.Currency_factor) {
			return rec.Validate("currency_factor", rec.Currency_factor)
		}
	}
	return nil
}

// OnValidate_Transaction_type, _Transport_method, _Exit_point, _Area, _Transaction_specification:
// the foreign trade statistics fields go to the lines
func (salesHeaderLogic) OnValidate_Transaction_type(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "transaction_type")
}

func (salesHeaderLogic) OnValidate_Transport_method(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "transport_method")
}

func (salesHeaderLogic) OnValidate_Exit_point(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "exit_point")
}

func (salesHeaderLogic) OnValidate_Area(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "area")
}

func (salesHeaderLogic) OnValidate_Transaction_specification(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "transaction_specification")
}

// OnValidate_Shipment_date: to the lines
func (salesHeaderLogic) OnValidate_Shipment_date(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "shipment_date")
}

// OnValidate_Payment_terms_code: the due date and payment discount from the document date
func (salesHeaderLogic) OnValidate_Payment_terms_code(rec *gtables.SalesHeader) error {
	return validateSalesPaymentTerms(rec)
}

// validateSalesPaymentTerms (VALIDATE("Payment Terms Code")): due date, payment discount date
// and % from the payment terms and the document date (credit memos have none)
func validateSalesPaymentTerms(rec *gtables.SalesHeader) error {
	if rec.Document_type != gtables.SalesHeader_Document_type.CreditMemo {
		if rec.Payment_terms_code != "" && !rec.Document_date.IsZero() {
			paymentTerms := gtables.NewPaymentTerms(rec)
			err := paymentTerms.GetOrError(rec.Payment_terms_code)
			if err != nil {
				return err
			}
			rec.Due_date, err = CalcDateFormula(paymentTerms.Due_date_calculation, rec.Document_date)
			if err != nil {
				return err
			}
			rec.Pmt_discount_date, err = CalcDateFormula(paymentTerms.Discount_date_calculation, rec.Document_date)
			if err != nil {
				return err
			}
			return rec.Validate("payment_discount_percent", paymentTerms.Discount_percent)
		} else {
			rec.Due_date = rec.Document_date
			rec.Pmt_discount_date = rec.Document_date
			return rec.Validate("payment_discount_percent", types.Decimal{})
		}
	}
	return nil
}

// OnValidate_Department_code: to the lines
func (salesHeaderLogic) OnValidate_Department_code(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "department_code")
}

// OnValidate_Project_code: to the lines
func (salesHeaderLogic) OnValidate_Project_code(rec *gtables.SalesHeader) error {
	return updateSalesLines(rec, "project_code")
}

// OnValidate_Currency_code: a changed currency gets its factor and recreates the lines; the
// same currency again updates the factor and the lines' prices when it changed
func (salesHeaderLogic) OnValidate_Currency_code(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	if rec.Currency_code != xRec.Currency_code {
		err := updateSalesCurrencyFactor(rec)
		if err != nil {
			return err
		}
		return RecreateSalesLines(rec)
	} else {
		if rec.Currency_code != "" {
			err := updateSalesCurrencyFactor(rec)
			if err != nil {
				return err
			}
			if !rec.Currency_factor.Equal(xRec.Currency_factor) {
				return rec.Validate("currency_factor", rec.Currency_factor)
			}
		}
	}
	return nil
}

// OnValidate_Currency_factor: the lines' prices in the new factor
func (salesHeaderLogic) OnValidate_Currency_factor(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	if !rec.Currency_factor.Equal(xRec.Currency_factor) {
		return updateSalesLines(rec, "currency_factor")
	}
	return nil
}

// updateSalesCurrencyFactor (UpdateCurrencyFactor): the exchange rate on the posting date (a
// quote: the work date)
func updateSalesCurrencyFactor(rec *gtables.SalesHeader) error {
	if rec.Currency_code != "" {
		var currencyDate types.Date
		if rec.Document_type != gtables.SalesHeader_Document_type.Quote {
			currencyDate = rec.Posting_date
		} else {
			currencyDate = system.WorkDate()
		}
		factor, err := ExchangeRates(rec).ExchangeRate(currencyDate, rec.Currency_code)
		if err != nil {
			return err
		}
		rec.Currency_factor = factor
	} else {
		rec.Currency_factor = types.Decimal{}
	}
	return nil
}

// OnValidate_Document_date: the due date again
func (salesHeaderLogic) OnValidate_Document_date(rec *gtables.SalesHeader) error {
	return validateSalesPaymentTerms(rec)
}

// OnValidate_Gen_bus_posting_group: its default tax business posting group; the lines again
func (salesHeaderLogic) OnValidate_Gen_bus_posting_group(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	if xRec.Gen_bus_posting_group != rec.Gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			rec.Tax_bus_posting_group = genBusPostingGroup.Def_tax_bus_posting_group
			return RecreateSalesLines(rec)
		}
	}
	return nil
}

// OnValidate_Tax_bus_posting_group: the lines again
func (salesHeaderLogic) OnValidate_Tax_bus_posting_group(rec *gtables.SalesHeader) error {
	xRec := rec.XRec()
	if xRec.Tax_bus_posting_group != rec.Tax_bus_posting_group {
		return RecreateSalesLines(rec)
	}
	return nil
}

// Post codes: a known post code gives the city
func (salesHeaderLogic) OnValidate_Bill_to_post_code(rec *gtables.SalesHeader) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Bill_to_post_code != "" && postCode.Get(rec.Bill_to_post_code) {
		rec.Bill_to_city = postCode.City
	}
	return nil
}

func (salesHeaderLogic) OnValidate_Sell_to_post_code(rec *gtables.SalesHeader) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Sell_to_post_code != "" && postCode.Get(rec.Sell_to_post_code) {
		rec.Sell_to_city = postCode.City
	}
	return nil
}

func (salesHeaderLogic) OnValidate_Ship_to_post_code(rec *gtables.SalesHeader) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Ship_to_post_code != "" && postCode.Get(rec.Ship_to_post_code) {
		rec.Ship_to_city = postCode.City
	}
	return nil
}

// salesLinesExist (SalesLinesExist): the document has lines
func salesLinesExist(rec *gtables.SalesHeader) bool {
	salesLine := gtables.NewSalesLine(rec)
	salesLine.SetRange("document_type", rec.Document_type)
	salesLine.SetRange("document_no", rec.No)
	return salesLine.FindFirst()
}

// RecreateSalesLines (RecreateSalesLines): the lines again from the header's new values — each
// line deleted and entered again with its type, number (or description) and quantity. The
// header is saved first, the lines read it. The classic asks first; table triggers cannot.
func RecreateSalesLines(rec *gtables.SalesHeader) error {
	if !salesLinesExist(rec) {
		return nil
	}
	salesLine := gtables.NewSalesLine(rec)
	salesLine.LockTable()
	err := rec.ModifyOrError(false)
	if err != nil {
		return err
	}
	salesLine.SetRange("document_type", rec.Document_type)
	salesLine.SetRange("document_no", rec.No)
	var salesLineTmp []gtables.SalesLine
	for found := salesLine.FindSet(); found; found = salesLine.Next() {
		salesLineTmp = append(salesLineTmp, *salesLine)
	}
	salesLine.DeleteAll()
	lineNo := 0
	for i := range salesLineTmp {
		old := &salesLineTmp[i]
		salesLine = gtables.NewSalesLine(rec)
		lineNo += 10000
		salesLine.Document_type = gtables.SalesLineDocument_type(rec.Document_type)
		salesLine.Document_no = rec.No
		salesLine.Line_no = lineNo
		err = salesLine.Validate("type", old.Type)
		if err != nil {
			return err
		}
		if old.No == "" {
			salesLine.Description = old.Description
			salesLine.Description_2 = old.Description_2
		} else {
			err = salesLine.Validate("no", old.No)
			if err != nil {
				return err
			}
			if !old.Quantity.IsZero() {
				err = salesLine.Validate("quantity", old.Quantity)
				if err != nil {
					return err
				}
			}
		}
		err = salesLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// updateSalesLines (UpdateSalesLines): a header field to every line (shipment date, department,
// project) or the lines' prices again in a new currency factor. The header is saved first.
func updateSalesLines(rec *gtables.SalesHeader, changedField string) error {
	if !salesLinesExist(rec) {
		return nil
	}
	salesLine := gtables.NewSalesLine(rec)
	salesLine.LockTable()
	err := rec.ModifyOrError(false)
	if err != nil {
		return err
	}
	salesLine.SetRange("document_type", rec.Document_type)
	salesLine.SetRange("document_no", rec.No)
	for found := salesLine.FindSet(); found; found = salesLine.Next() {
		switch changedField {
		case "shipment_date":
			err = salesLine.Validate("shipment_date", rec.Shipment_date)
		case "currency_factor":
			if salesLine.Type != gtables.SalesLine_Type.Blank {
				err = salesLine.Validate("unit_price", salesLine.Unit_price)
			}
		case "department_code":
			err = salesLine.Validate("department_code", rec.Department_code)
		case "project_code":
			err = salesLine.Validate("project_code", rec.Project_code)
		case "transaction_type":
			err = salesLine.Validate("transaction_type", rec.Transaction_type)
		case "transport_method":
			err = salesLine.Validate("transport_method", rec.Transport_method)
		case "exit_point":
			err = salesLine.Validate("exit_point", rec.Exit_point)
		case "area":
			err = salesLine.Validate("area", rec.Area)
		case "transaction_specification":
			err = salesLine.Validate("transaction_specification", rec.Transaction_specification)
		}
		if err != nil {
			return err
		}
		err = salesLine.ModifyOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}
