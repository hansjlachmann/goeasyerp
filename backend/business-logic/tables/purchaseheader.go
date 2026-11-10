package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchaseHeaderLogic is the business logic of table 38 Purchase Header: quotes, orders,
// invoices, credit memos and blanket orders.
//
// Table triggers cannot ask the user (HideValidationDialog): a changed vendor, currency or
// posting group recreates the lines as when the user answers Yes. The goods are received at the
// company (Company Information's ship-to address and location). Not migrated yet: posting
// (Applies-to, Receive/Invoice, posting numbers), drop shipments (Sell-to Customer No., Ship-to
// Code), Bal. Account, jobs, Reserve, the VAT Base Discount (sales tax only), foreign trade
// statistics, 1099 codes and the exchange rate change (form 511).
type purchaseHeaderLogic struct{}

func init() {
	gtables.RegisterPurchaseHeaderLogic(purchaseHeaderLogic{})
	functions.Register("table", gtables.PurchaseHeaderTableName, "AssistEdit", functions.ForAssistEdit(AssistEditPurchaseHeader), functions.WithAssistEdit())
}

// OnInsert: the next number of the document type's series, then the dates, the ship-to address
// and the posting description (InitRecord)
func (purchaseHeaderLogic) OnInsert(rec *gtables.PurchaseHeader) error {
	purchSetup := gtables.GetPurchasesPayablesSetup(rec)
	if rec.No == "" {
		err := testPurchNoSeries(rec, purchSetup)
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(getPurchNoSeriesCode(rec, purchSetup), xRec.No_series, rec.Posting_date, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	return InitPurchaseHeaderRecord(rec)
}

// OnDelete: the document's lines (with their triggers) and comments
func (purchaseHeaderLogic) OnDelete(rec *gtables.PurchaseHeader) error {
	purchLine := gtables.NewPurchaseLine(rec)
	purchLine.LockTable()
	purchLine.SetRange("document_type", rec.Document_type)
	purchLine.SetRange("document_no", rec.No)
	for found := purchLine.FindSet(); found; found = purchLine.Next() {
		err := purchLine.DeleteOrError(true)
		if err != nil {
			return err
		}
	}
	purchCommentLine := gtables.NewPurchCommentLine(rec)
	purchCommentLine.SetRange("document_type", rec.Document_type)
	purchCommentLine.SetRange("no", rec.No)
	purchCommentLine.DeleteAll()
	return nil
}

// OnRename: a purchase document is never renamed
func (purchaseHeaderLogic) OnRename(rec *gtables.PurchaseHeader) error {
	return apperrors.RenameNotAllowed(gtables.PurchaseHeaderTableName)
}

// InitPurchaseHeaderRecord (InitRecord): the series of the posted documents (Posting/Receiving
// No. Series), order and expected receipt dates from the work date (a quote's order date too),
// the posting date (not on quotes), the document date, the company's ship-to address and the
// posting description ("Order PO000001")
func InitPurchaseHeaderRecord(rec *gtables.PurchaseHeader) error {
	purchSetup := gtables.GetPurchasesPayablesSetup(rec)
	noSeriesMgt := noseries.New(rec)
	var err error
	switch rec.Document_type {
	case gtables.PurchaseHeader_Document_type.Quote, gtables.PurchaseHeader_Document_type.Order:
		err = noSeriesMgt.SetDefaultSeries(&rec.Posting_no_series, purchSetup.Posted_invoice_nos)
		if err != nil {
			return err
		}
		err = noSeriesMgt.SetDefaultSeries(&rec.Receiving_no_series, purchSetup.Posted_receipt_nos)
	case gtables.PurchaseHeader_Document_type.Invoice:
		if rec.No_series != "" && purchSetup.Invoice_nos == purchSetup.Posted_invoice_nos {
			rec.Posting_no_series = rec.No_series
		} else {
			err = noSeriesMgt.SetDefaultSeries(&rec.Posting_no_series, purchSetup.Posted_invoice_nos)
		}
		if err == nil && purchSetup.Receipt_on_invoice {
			err = noSeriesMgt.SetDefaultSeries(&rec.Receiving_no_series, purchSetup.Posted_receipt_nos)
		}
	case gtables.PurchaseHeader_Document_type.CreditMemo:
		if rec.No_series != "" && purchSetup.Credit_memo_nos == purchSetup.Posted_credit_memo_nos {
			rec.Posting_no_series = rec.No_series
		} else {
			err = noSeriesMgt.SetDefaultSeries(&rec.Posting_no_series, purchSetup.Posted_credit_memo_nos)
		}
	}
	if err != nil {
		return err
	}

	if rec.Document_type == gtables.PurchaseHeader_Document_type.Order || rec.Document_type == gtables.PurchaseHeader_Document_type.Invoice {
		rec.Expected_receipt_date = system.WorkDate()
		rec.Order_date = system.WorkDate()
	}
	// a quote is dated too, as a sales quote (its costs are those of the order date)
	if rec.Document_type == gtables.PurchaseHeader_Document_type.Quote {
		rec.Order_date = system.WorkDate()
	}
	if rec.Document_type != gtables.PurchaseHeader_Document_type.Quote && rec.Posting_date.IsZero() {
		rec.Posting_date = system.WorkDate()
	}
	rec.Document_date = system.WorkDate()
	setPurchShipToCompany(rec)
	documentType := i18n.GetInstance().OptionCaption(gtables.PurchaseHeaderTableName, "document_type", i18n.Key(rec.Document_type.String()), fcodeunits.CurrentLanguage())
	rec.Posting_description = types.Text(system.CopyStr(documentType+" "+rec.No.String(), 1, rec.MaxStrLen("posting_description")))
	return nil
}

// setPurchShipToCompany (Sell-to Customer No. OnValidate with a blank customer): the goods come
// to the company — Company Information's ship-to address and location
func setPurchShipToCompany(rec *gtables.PurchaseHeader) {
	companyInfo := gtables.GetCompanyInformation(rec)
	rec.Ship_to_name = companyInfo.Ship_to_name
	rec.Ship_to_name_2 = companyInfo.Ship_to_name_2
	rec.Ship_to_address = companyInfo.Ship_to_address
	rec.Ship_to_address_2 = companyInfo.Ship_to_address_2
	rec.Ship_to_city = companyInfo.Ship_to_city
	rec.Ship_to_post_code = companyInfo.Ship_to_post_code
	rec.Ship_to_state = companyInfo.Ship_to_state
	rec.Ship_to_country_code = ""
	rec.Ship_to_contact = companyInfo.Ship_to_contact
	rec.Location_code = companyInfo.Location_code
}

// AssistEditPurchaseHeader (AssistEdit): No. from the document type's series or one related to
// it
func AssistEditPurchaseHeader(rec *gtables.PurchaseHeader) (bool, error) {
	purchSetup := gtables.GetPurchasesPayablesSetup(rec)
	err := testPurchNoSeries(rec, purchSetup)
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(getPurchNoSeriesCode(rec, purchSetup), xRec.No_series, &noSeries)
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

// testPurchNoSeries (TestNoSeries): the setup has the document type's number series
func testPurchNoSeries(rec *gtables.PurchaseHeader, purchSetup *gtables.PurchasesPayablesSetup) error {
	switch rec.Document_type {
	case gtables.PurchaseHeader_Document_type.Quote:
		return purchSetup.TestField("quote_nos")
	case gtables.PurchaseHeader_Document_type.Order:
		return purchSetup.TestField("order_nos")
	case gtables.PurchaseHeader_Document_type.Invoice:
		err := purchSetup.TestField("invoice_nos")
		if err != nil {
			return err
		}
		return purchSetup.TestField("posted_invoice_nos")
	case gtables.PurchaseHeader_Document_type.CreditMemo:
		err := purchSetup.TestField("credit_memo_nos")
		if err != nil {
			return err
		}
		return purchSetup.TestField("posted_credit_memo_nos")
	case gtables.PurchaseHeader_Document_type.BlanketOrder:
		return purchSetup.TestField("blanket_order_nos")
	}
	return nil
}

// getPurchNoSeriesCode (GetNoSeriesCode): the number series of the document type
func getPurchNoSeriesCode(rec *gtables.PurchaseHeader, purchSetup *gtables.PurchasesPayablesSetup) types.Code {
	switch rec.Document_type {
	case gtables.PurchaseHeader_Document_type.Quote:
		return purchSetup.Quote_nos
	case gtables.PurchaseHeader_Document_type.Order:
		return purchSetup.Order_nos
	case gtables.PurchaseHeader_Document_type.Invoice:
		return purchSetup.Invoice_nos
	case gtables.PurchaseHeader_Document_type.CreditMemo:
		return purchSetup.Credit_memo_nos
	case gtables.PurchaseHeader_Document_type.BlanketOrder:
		return purchSetup.Blanket_order_nos
	}
	return ""
}

// OnValidate_No: a number typed by hand needs Manual Nos. (one the series just issued keeps
// its series, as on the vendor)
func (purchaseHeaderLogic) OnValidate_No(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		purchSetup := gtables.GetPurchasesPayablesSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(getPurchNoSeriesCode(rec, purchSetup), rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(getPurchNoSeriesCode(rec, purchSetup))
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	return nil
}

// OnValidate_Buy_from_vendor_no: the vendor's name, address, posting and tax groups; the pay-to
// vendor (the vendor's Pay-to Vendor No. or itself); a changed vendor recreates the lines (not
// on an order that has received lines)
func (purchaseHeaderLogic) OnValidate_Buy_from_vendor_no(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if rec.Buy_from_vendor_no != xRec.Buy_from_vendor_no && xRec.Buy_from_vendor_no != "" {
		if rec.Buy_from_vendor_no == "" {
			if purchLinesExist(rec) {
				return apperrors.PurchResetVendorLines(gtables.PurchaseHeaderTableName, "buy_from_vendor_no")
			}
			rec.Init()
			err := InitPurchaseHeaderRecord(rec)
			if err != nil {
				return err
			}
			rec.No_series = xRec.No_series
			return nil
		}
		err := testPurchLinesNotReceived(rec)
		if err != nil {
			return err
		}
	}

	vend := gtables.NewVendor(rec)
	err := vend.GetOrError(rec.Buy_from_vendor_no)
	if err != nil {
		return err
	}
	err = vend.TestField("blocked", false)
	if err != nil {
		return err
	}
	rec.Buy_from_vendor_name = vend.Name
	rec.Buy_from_vendor_name_2 = vend.Name_2
	rec.Buy_from_address = vend.Address
	rec.Buy_from_address_2 = vend.Address_2
	rec.Buy_from_city = vend.City
	rec.Buy_from_post_code = vend.Post_code
	rec.Buy_from_state = vend.State
	rec.Buy_from_country_code = vend.Country_code
	rec.Vat_country_code = vend.Country_code
	rec.Buy_from_contact = vend.Contact
	rec.Gen_bus_posting_group = vend.Gen_bus_posting_group
	rec.Tax_bus_posting_group = vend.Tax_bus_posting_group
	rec.Tax_area_code = vend.Tax_area_code
	rec.Tax_liable = vend.Tax_liable
	rec.Vat_registration_no = vend.Vat_registration_no
	if vend.Pay_to_vendor_no != "" {
		err = rec.Validate("pay_to_vendor_no", vend.Pay_to_vendor_no)
	} else {
		err = rec.Validate("pay_to_vendor_no", rec.Buy_from_vendor_no)
	}
	if err != nil {
		return err
	}
	rec.Order_address_code = ""

	if xRec.Buy_from_vendor_no != rec.Buy_from_vendor_no || xRec.Currency_code != rec.Currency_code {
		return RecreatePurchLines(rec)
	}
	return nil
}

// testPurchLinesNotReceived: the vendor of an order with received lines (another document with
// lines from a receipt) stays
func testPurchLinesNotReceived(rec *gtables.PurchaseHeader) error {
	purchLine := gtables.NewPurchaseLine(rec)
	purchLine.SetRange("document_type", rec.Document_type)
	purchLine.SetRange("document_no", rec.No)
	if rec.Document_type == gtables.PurchaseHeader_Document_type.Order {
		purchLine.SetFilter("quantity_received", "<>0")
		if purchLine.FindFirst() {
			return purchLine.TestField("quantity_received", types.Decimal{})
		}
	}
	return nil
}

// OnValidate_Pay_to_vendor_no: the pay-to vendor's name and address, payment terms and method,
// shipment method, dimension 1/2, posting group, currency, invoice discounts, language,
// purchaser; a changed pay-to vendor (same buy-from) recreates the lines
func (purchaseHeaderLogic) OnValidate_Pay_to_vendor_no(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if xRec.Pay_to_vendor_no != rec.Pay_to_vendor_no && xRec.Pay_to_vendor_no != "" {
		err := testPurchLinesNotReceived(rec)
		if err != nil {
			return err
		}
	}
	vend := gtables.NewVendor(rec)
	err := vend.GetOrError(rec.Pay_to_vendor_no)
	if err != nil {
		return err
	}
	err = vend.TestField("blocked", false)
	if err != nil {
		return err
	}
	err = vend.TestField("vendor_posting_group")
	if err != nil {
		return err
	}
	rec.Pay_to_name = vend.Name
	rec.Pay_to_name_2 = vend.Name_2
	rec.Pay_to_address = vend.Address
	rec.Pay_to_address_2 = vend.Address_2
	rec.Pay_to_city = vend.City
	rec.Pay_to_post_code = vend.Post_code
	rec.Pay_to_state = vend.State
	rec.Pay_to_country_code = vend.Country_code
	rec.Pay_to_contact = vend.Contact
	rec.Payment_terms_code = vend.Payment_terms_code
	if rec.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
		rec.Payment_method_code = ""
	} else {
		rec.Payment_method_code = vend.Payment_method_code
	}
	rec.Shipment_method_code = vend.Shipment_method_code
	rec.Dimension_1_code = vend.Dimension_1_code
	rec.Dimension_2_code = vend.Dimension_2_code
	rec.Vendor_posting_group = vend.Vendor_posting_group
	rec.Reserve = gtables.PurchaseHeaderReserve(vend.Reserve)
	rec.Currency_code = vend.Currency_code
	rec.Invoice_disc_code = vend.Invoice_disc_code
	rec.Language_code = vend.Language_code
	rec.Purchaser_code = vend.Purchaser_code
	err = validatePurchPaymentTerms(rec)
	if err != nil {
		return err
	}
	// the currency validated from code (not the user's field): only the factor
	err = updatePurchCurrencyFactor(rec)
	if err != nil {
		return err
	}

	if xRec.Buy_from_vendor_no == rec.Buy_from_vendor_no && xRec.Pay_to_vendor_no != rec.Pay_to_vendor_no {
		return RecreatePurchLines(rec)
	}
	return nil
}

// OnValidate_Order_address_code: the order address's (or else the vendor's) name and address
func (purchaseHeaderLogic) OnValidate_Order_address_code(rec *gtables.PurchaseHeader) error {
	if rec.Order_address_code != "" {
		orderAddr := gtables.NewOrderAddress(rec)
		err := orderAddr.GetOrError(map[string]interface{}{"vendor_no": rec.Buy_from_vendor_no, "code": rec.Order_address_code})
		if err != nil {
			return err
		}
		rec.Buy_from_vendor_name = orderAddr.Name
		rec.Buy_from_vendor_name_2 = orderAddr.Name_2
		rec.Buy_from_address = orderAddr.Address
		rec.Buy_from_address_2 = orderAddr.Address_2
		rec.Buy_from_city = orderAddr.City
		rec.Buy_from_contact = orderAddr.Contact
		rec.Buy_from_post_code = orderAddr.Post_code
		rec.Buy_from_state = orderAddr.State
		rec.Buy_from_country_code = orderAddr.Country_code
		rec.Vat_country_code = orderAddr.Country_code
	} else {
		vend := gtables.NewVendor(rec)
		err := vend.GetOrError(rec.Buy_from_vendor_no)
		if err != nil {
			return err
		}
		rec.Buy_from_vendor_name = vend.Name
		rec.Buy_from_vendor_name_2 = vend.Name_2
		rec.Buy_from_address = vend.Address
		rec.Buy_from_address_2 = vend.Address_2
		rec.Buy_from_city = vend.City
		rec.Buy_from_contact = vend.Contact
		rec.Buy_from_post_code = vend.Post_code
		rec.Buy_from_state = vend.State
		rec.Buy_from_country_code = vend.Country_code
		rec.Vat_country_code = vend.Country_code
		rec.Vat_registration_no = vend.Vat_registration_no
		return rec.Validate("gen_bus_posting_group", vend.Gen_bus_posting_group)
	}
	return nil
}

// OnValidate_Posting_date: the document date follows; a currency's factor on the new date
func (purchaseHeaderLogic) OnValidate_Posting_date(rec *gtables.PurchaseHeader) error {
	err := rec.Validate("document_date", rec.Posting_date)
	if err != nil {
		return err
	}
	if rec.Currency_code != "" {
		xRec := rec.XRec()
		err = updatePurchCurrencyFactor(rec)
		if err != nil {
			return err
		}
		if !rec.Currency_factor.Equal(xRec.Currency_factor) {
			return rec.Validate("currency_factor", rec.Currency_factor)
		}
	}
	return nil
}

// OnValidate_Expected_receipt_date: to the lines
func (purchaseHeaderLogic) OnValidate_Expected_receipt_date(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "expected_receipt_date")
}

// OnValidate_Payment_terms_code: the due date and payment discount from the document date
func (purchaseHeaderLogic) OnValidate_Payment_terms_code(rec *gtables.PurchaseHeader) error {
	return validatePurchPaymentTerms(rec)
}

// validatePurchPaymentTerms (VALIDATE("Payment Terms Code")): due date, payment discount date
// and % from the payment terms and the document date (credit memos have none)
func validatePurchPaymentTerms(rec *gtables.PurchaseHeader) error {
	if rec.Document_type != gtables.PurchaseHeader_Document_type.CreditMemo {
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

// OnValidate_Dimension_1_code: to the lines
func (purchaseHeaderLogic) OnValidate_Dimension_1_code(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "dimension_1_code")
}

// OnValidate_Dimension_2_code: to the lines
func (purchaseHeaderLogic) OnValidate_Dimension_2_code(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "dimension_2_code")
}

// OnValidate_Currency_code: a changed currency gets its factor and recreates the lines; the
// same currency again updates the factor and the lines' costs when it changed
func (purchaseHeaderLogic) OnValidate_Currency_code(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if rec.Currency_code != xRec.Currency_code {
		err := updatePurchCurrencyFactor(rec)
		if err != nil {
			return err
		}
		return RecreatePurchLines(rec)
	} else {
		if rec.Currency_code != "" {
			err := updatePurchCurrencyFactor(rec)
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

// OnValidate_Currency_factor: the lines' costs in the new factor
func (purchaseHeaderLogic) OnValidate_Currency_factor(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if !rec.Currency_factor.Equal(xRec.Currency_factor) {
		return updatePurchLines(rec, "currency_factor")
	}
	return nil
}

// updatePurchCurrencyFactor (UpdateCurrencyFactor): the exchange rate on the posting date (a
// quote: the work date)
func updatePurchCurrencyFactor(rec *gtables.PurchaseHeader) error {
	if rec.Currency_code != "" {
		var currencyDate types.Date
		if rec.Document_type != gtables.PurchaseHeader_Document_type.Quote {
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
func (purchaseHeaderLogic) OnValidate_Document_date(rec *gtables.PurchaseHeader) error {
	return validatePurchPaymentTerms(rec)
}

// OnValidate_Gen_bus_posting_group: its default tax business posting group; the lines again
func (purchaseHeaderLogic) OnValidate_Gen_bus_posting_group(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if xRec.Gen_bus_posting_group != rec.Gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			rec.Tax_bus_posting_group = genBusPostingGroup.Def_tax_bus_posting_group
			return RecreatePurchLines(rec)
		}
	}
	return nil
}

// OnValidate_Tax_bus_posting_group: the lines again
func (purchaseHeaderLogic) OnValidate_Tax_bus_posting_group(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if xRec.Tax_bus_posting_group != rec.Tax_bus_posting_group {
		return RecreatePurchLines(rec)
	}
	return nil
}

// Post codes: a known post code gives the city
func (purchaseHeaderLogic) OnValidate_Pay_to_post_code(rec *gtables.PurchaseHeader) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Pay_to_post_code != "" && postCode.Get(rec.Pay_to_post_code) {
		rec.Pay_to_city = postCode.City
	}
	return nil
}

func (purchaseHeaderLogic) OnValidate_Buy_from_post_code(rec *gtables.PurchaseHeader) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Buy_from_post_code != "" && postCode.Get(rec.Buy_from_post_code) {
		rec.Buy_from_city = postCode.City
	}
	return nil
}

func (purchaseHeaderLogic) OnValidate_Ship_to_post_code(rec *gtables.PurchaseHeader) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Ship_to_post_code != "" && postCode.Get(rec.Ship_to_post_code) {
		rec.Ship_to_city = postCode.City
	}
	return nil
}

// purchLinesExist (PurchLinesExist): the document has lines
func purchLinesExist(rec *gtables.PurchaseHeader) bool {
	purchLine := gtables.NewPurchaseLine(rec)
	purchLine.SetRange("document_type", rec.Document_type)
	purchLine.SetRange("document_no", rec.No)
	return purchLine.FindFirst()
}

// RecreatePurchLines (RecreatePurchLines): the lines again from the header's new values — each
// line deleted and entered again with its type, number (or description) and quantity. The
// header is saved first, the lines read it. The classic asks first; table triggers cannot.
func RecreatePurchLines(rec *gtables.PurchaseHeader) error {
	if !purchLinesExist(rec) {
		return nil
	}
	purchLine := gtables.NewPurchaseLine(rec)
	purchLine.LockTable()
	err := rec.ModifyOrError(false)
	if err != nil {
		return err
	}
	purchLine.SetRange("document_type", rec.Document_type)
	purchLine.SetRange("document_no", rec.No)
	var purchLineTmp []gtables.PurchaseLine
	for found := purchLine.FindSet(); found; found = purchLine.Next() {
		purchLineTmp = append(purchLineTmp, *purchLine)
	}
	purchLine.DeleteAll()
	lineNo := 0
	extendedTextAdded := false
	for i := range purchLineTmp {
		old := &purchLineTmp[i]
		if old.Attached_to_line_no != 0 {
			// the text lines of the line before: its extended text inserted again, once
			if !extendedTextAdded {
				transferExtendedText := &TransferExtendedText{}
				_, err = transferExtendedText.PurchCheckIfAnyExtText(purchLine, true)
				if err != nil {
					return err
				}
				err = transferExtendedText.InsertPurchExtText(purchLine)
				if err != nil {
					return err
				}
				last := gtables.NewPurchaseLine(rec)
				last.SetRange("document_type", purchLine.Document_type)
				last.SetRange("document_no", rec.No)
				if last.FindLast() {
					lineNo = last.Line_no
				}
				extendedTextAdded = true
			}
			continue
		}
		extendedTextAdded = false
		purchLine = gtables.NewPurchaseLine(rec)
		lineNo += 10000
		purchLine.Document_type = gtables.PurchaseLineDocument_type(rec.Document_type)
		purchLine.Document_no = rec.No
		purchLine.Line_no = lineNo
		err = purchLine.Validate("type", old.Type)
		if err != nil {
			return err
		}
		if old.No == "" {
			purchLine.Description = old.Description
			purchLine.Description_2 = old.Description_2
		} else {
			err = purchLine.Validate("no", old.No)
			if err != nil {
				return err
			}
			if !old.Quantity.IsZero() {
				err = purchLine.Validate("quantity", old.Quantity)
				if err != nil {
					return err
				}
			}
		}
		err = purchLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Transaction_type, _Transport_method, _Entry_point, _Area, _Transaction_specification:
// the foreign trade statistics fields go to the lines
func (purchaseHeaderLogic) OnValidate_Transaction_type(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "transaction_type")
}

func (purchaseHeaderLogic) OnValidate_Transport_method(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "transport_method")
}

func (purchaseHeaderLogic) OnValidate_Entry_point(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "entry_point")
}

func (purchaseHeaderLogic) OnValidate_Area(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "area")
}

func (purchaseHeaderLogic) OnValidate_Transaction_specification(rec *gtables.PurchaseHeader) error {
	return updatePurchLines(rec, "transaction_specification")
}

// updatePurchLines (UpdatePurchLines): a header field to every line (expected receipt date,
// dimension 1, dimension 2) or the lines' costs again in a new currency factor. The header is saved
// first.
func updatePurchLines(rec *gtables.PurchaseHeader, changedField string) error {
	if !purchLinesExist(rec) {
		return nil
	}
	purchLine := gtables.NewPurchaseLine(rec)
	purchLine.LockTable()
	err := rec.ModifyOrError(false)
	if err != nil {
		return err
	}
	purchLine.SetRange("document_type", rec.Document_type)
	purchLine.SetRange("document_no", rec.No)
	for found := purchLine.FindSet(); found; found = purchLine.Next() {
		switch changedField {
		case "expected_receipt_date":
			err = purchLine.Validate("expected_receipt_date", rec.Expected_receipt_date)
		case "currency_factor":
			if purchLine.Type != gtables.PurchaseLine_Type.Blank {
				err = purchLine.Validate("direct_unit_cost", purchLine.Direct_unit_cost)
			}
		case "dimension_1_code":
			err = purchLine.Validate("dimension_1_code", rec.Dimension_1_code)
		case "dimension_2_code":
			err = purchLine.Validate("dimension_2_code", rec.Dimension_2_code)
		case "transaction_type":
			err = purchLine.Validate("transaction_type", rec.Transaction_type)
		case "transport_method":
			err = purchLine.Validate("transport_method", rec.Transport_method)
		case "entry_point":
			err = purchLine.Validate("entry_point", rec.Entry_point)
		case "area":
			err = purchLine.Validate("area", rec.Area)
		case "transaction_specification":
			err = purchLine.Validate("transaction_specification", rec.Transaction_specification)
		}
		if err != nil {
			return err
		}
		err = purchLine.ModifyOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Applies_to_id (field 118): a changed Applies-to ID takes the old one off the pay-to
// vendor's open entries marked with it (VendEntrySetApplID.SetApplId with a blank ID)
func (purchaseHeaderLogic) OnValidate_Applies_to_id(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if rec.Applies_to_id != xRec.Applies_to_id && xRec.Applies_to_id != "" {
		vendLedgEntry := gtables.NewVendorLedgerEntry(rec)
		vendLedgEntry.SetCurrentKey("vendor_no", "open", "positive")
		vendLedgEntry.SetRange("vendor_no", rec.Pay_to_vendor_no)
		vendLedgEntry.SetRange("open", true)
		vendLedgEntry.SetRange("applies_to_id", xRec.Applies_to_id)
		for found := vendLedgEntry.FindSetBuffered(); found; found = vendLedgEntry.Next() {
			vendLedgEntry.Applies_to_id = ""
			err := vendLedgEntry.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
