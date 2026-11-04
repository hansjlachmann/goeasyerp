// Package purchcalcdisc is codeunit 70 Purch.-Calc.Discount: the invoice discount and service
// charge of a purchase document from the vendor's invoice discounts (Vendor Invoice Disc. 24).
package purchcalcdisc

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

var hundred = types.NewDecimalFromInt(100)

// Run (OnRun with a Purchase Line): the invoice discount of the line's document
func Run(purchLine *gtables.PurchaseLine) error {
	purchHeader := gtables.NewPurchaseHeader(purchLine)
	err := purchHeader.GetOrError(map[string]interface{}{"document_type": int(purchLine.Document_type), "no": purchLine.Document_no})
	if err != nil {
		return err
	}
	return CalculateInvoiceDiscount(purchHeader)
}

// CalculateInvoiceDiscount (CalculateInvoiceDiscount): the vendor's invoice discount code and
// currency (or else local currency, converted) give the highest minimum amount up to the lines'
// amounts — its service charge on a line of the vendor posting group's Service Charge Acc.
// (from the amount of all lines but that one), its Discount % on the lines allowing invoice
// discount (from their amount before invoice discount)
func CalculateInvoiceDiscount(purchHeader *gtables.PurchaseHeader) error {
	purchLine2 := gtables.NewPurchaseLine(purchHeader)
	purchLine2.LockTable()
	err := purchHeader.TestField("vendor_posting_group")
	if err != nil {
		return err
	}
	vendPostingGr := gtables.NewVendorPostingGroup(purchHeader)
	err = vendPostingGr.GetOrError(purchHeader.Vendor_posting_group)
	if err != nil {
		return err
	}
	purchLine2.SetRange("document_type", purchHeader.Document_type)
	purchLine2.SetRange("document_no", purchHeader.No)
	purchLine2.SetFilter("type", "<>0")
	invDiscBase := types.Decimal{}
	chargeBase := types.Decimal{}
	for found := purchLine2.FindSet(); found; found = purchLine2.Next() {
		if purchLine2.Allow_invoice_disc {
			invDiscBase = invDiscBase.Add(purchLine2.Amount).Add(purchLine2.Inv_discount_amount)
		}
		// every line but the service charge line itself
		serviceChargeLine := !purchLine2.Allow_invoice_disc && purchLine2.Type == gtables.PurchaseLine_Type.GLAccount && purchLine2.No == vendPostingGr.Service_charge_acc
		if !serviceChargeLine {
			chargeBase = chargeBase.Add(purchLine2.Amount).Add(purchLine2.Inv_discount_amount)
		}
	}
	err = purchHeader.ModifyOrError(false)
	if err != nil {
		return err
	}

	vendInvDisc := gtables.NewVendorInvoiceDisc(purchHeader)
	found, err := findVendInvDisc(purchHeader, vendInvDisc, chargeBase)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if !vendInvDisc.Service_charge.IsZero() {
		err = vendPostingGr.TestField("service_charge_acc")
		if err != nil {
			return err
		}
		purchLine2.SetRange("allow_invoice_disc", false)
		purchLine2.SetRange("type", gtables.PurchaseLine_Type.GLAccount)
		purchLine2.SetRange("no", vendPostingGr.Service_charge_acc)
		if purchLine2.FindLast() {
			err = purchLine2.Validate("direct_unit_cost", vendInvDisc.Service_charge)
			if err != nil {
				return err
			}
			err = purchLine2.ModifyOrError(false)
			if err != nil {
				return err
			}
		} else {
			purchLine2.Reset()
			purchLine2.SetRange("document_type", purchHeader.Document_type)
			purchLine2.SetRange("document_no", purchHeader.No)
			lineNo := 0
			if purchLine2.FindLast() {
				lineNo = purchLine2.Line_no
			}
			purchLine2 = gtables.NewPurchaseLine(purchHeader)
			purchLine2.Document_type = gtables.PurchaseLineDocument_type(purchHeader.Document_type)
			purchLine2.Document_no = purchHeader.No
			purchLine2.Line_no = lineNo + 10000
			purchLine2.Type = gtables.PurchaseLine_Type.GLAccount
			err = purchLine2.Validate("no", vendPostingGr.Service_charge_acc)
			if err != nil {
				return err
			}
			purchLine2.Description = types.Text(i18n.GetInstance().Message("PURCH_SERVICE_CHARGE", fcodeunits.CurrentLanguage()))
			err = purchLine2.Validate("quantity", types.NewDecimalFromInt(1))
			if err != nil {
				return err
			}
			err = purchLine2.Validate("direct_unit_cost", vendInvDisc.Service_charge)
			if err != nil {
				return err
			}
			purchLine2.Allow_invoice_disc = false
			err = purchLine2.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}

	vendInvDisc = gtables.NewVendorInvoiceDisc(purchHeader)
	found, err = findVendInvDisc(purchHeader, vendInvDisc, invDiscBase)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	purchLine2 = gtables.NewPurchaseLine(purchHeader)
	purchLine2.SetRange("document_type", purchHeader.Document_type)
	purchLine2.SetRange("document_no", purchHeader.No)
	purchLine2.SetFilter("type", "<>0")
	purchLine2.SetRange("allow_invoice_disc", true)
	for found := purchLine2.FindSet(); found; found = purchLine2.Next() {
		if !purchLine2.Quantity.IsZero() {
			purchLine2.Amount = purchLine2.Amount.Add(purchLine2.Inv_discount_amount)
			err = purchLine2.Validate("inv_discount_amount", system.Round(purchLine2.Amount.Mul(vendInvDisc.Discount_percent).Div(hundred), types.MustDecimal("0.00001")))
			if err != nil {
				return err
			}
			err = purchLine2.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// findVendInvDisc finds the invoice discount with the highest minimum amount up to the base,
// in the document's currency or else in local currency (the base converted at the posting date,
// a quote's at the work date; the service charge converted back). false: none applies.
func findVendInvDisc(purchHeader *gtables.PurchaseHeader, vendInvDisc *gtables.VendorInvoiceDisc, base types.Decimal) (bool, error) {
	vendInvDisc.SetRange("code", purchHeader.Invoice_disc_code)
	vendInvDisc.SetRange("currency_code", purchHeader.Currency_code)
	vendInvDisc.SetRange("minimum_amount", types.Decimal{}, base)
	if vendInvDisc.FindLast() {
		return true, nil
	}
	if purchHeader.Currency_code != "" {
		var currencyDate types.Date
		if purchHeader.Document_type != gtables.PurchaseHeader_Document_type.Quote {
			currencyDate = purchHeader.Posting_date
		} else {
			currencyDate = system.WorkDate()
		}
		currExchRate := tables.ExchangeRates(purchHeader)
		currencyFactor, err := currExchRate.ExchangeRate(currencyDate, purchHeader.Currency_code)
		if err != nil {
			return false, err
		}
		baseLCY, err := currExchRate.ExchangeAmtFCYToLCY(currencyDate, purchHeader.Currency_code, base, currencyFactor)
		if err != nil {
			return false, err
		}
		vendInvDisc.SetRange("currency_code", types.Code(""))
		vendInvDisc.SetRange("minimum_amount", types.Decimal{}, baseLCY)
		if !vendInvDisc.FindLast() {
			return false, nil
		}
		currency := gtables.NewCurrency(purchHeader)
		err = currency.GetOrError(purchHeader.Currency_code)
		if err != nil {
			return false, err
		}
		serviceCharge, err := currExchRate.ExchangeAmtLCYToFCY(currencyDate, purchHeader.Currency_code, vendInvDisc.Service_charge, currencyFactor)
		if err != nil {
			return false, err
		}
		vendInvDisc.Service_charge = system.Round(serviceCharge, currency.Amount_rounding_precision)
		return true, nil
	} else {
		return false, nil
	}
}
