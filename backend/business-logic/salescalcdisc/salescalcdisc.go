// Package salescalcdisc is codeunit 60 Sales-Calc. Discount: the invoice discount and service
// charge of a sales document from the customer's invoice discounts (Cust. Invoice Disc. 19).
package salescalcdisc

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

var hundred = types.NewDecimalFromInt(100)

// Run (OnRun with a Sales Line): the invoice discount of the line's document
func Run(salesLine *gtables.SalesLine) error {
	salesHeader := gtables.NewSalesHeader(salesLine)
	err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(salesLine.Document_type), "no": salesLine.Document_no})
	if err != nil {
		return err
	}
	return CalculateInvoiceDiscount(salesHeader)
}

// CalculateInvoiceDiscount (CalculateInvoiceDiscount): the customer's invoice discount code and
// currency (or else local currency, converted) give the highest minimum amount up to the lines'
// amounts — its service charge on a line of the customer posting group's Service Charge Acc.
// (from the amount of all lines but that one), its Discount % on the lines allowing invoice
// discount (from their amount before invoice discount)
func CalculateInvoiceDiscount(salesHeader *gtables.SalesHeader) error {
	salesLine2 := gtables.NewSalesLine(salesHeader)
	salesLine2.LockTable()
	err := salesHeader.TestField("customer_posting_group")
	if err != nil {
		return err
	}
	custPostingGr := gtables.NewCustomerPostingGroup(salesHeader)
	err = custPostingGr.GetOrError(salesHeader.Customer_posting_group)
	if err != nil {
		return err
	}
	salesLine2.SetRange("document_type", salesHeader.Document_type)
	salesLine2.SetRange("document_no", salesHeader.No)
	salesLine2.SetFilter("type", "<>0")
	invDiscBase := types.Decimal{}
	chargeBase := types.Decimal{}
	for found := salesLine2.FindSet(); found; found = salesLine2.Next() {
		if salesLine2.Allow_invoice_disc {
			invDiscBase = invDiscBase.Add(salesLine2.Amount).Add(salesLine2.Inv_discount_amount)
		}
		// every line but the service charge line itself
		serviceChargeLine := !salesLine2.Allow_invoice_disc && salesLine2.Type == gtables.SalesLine_Type.GLAccount && salesLine2.No == custPostingGr.Service_charge_acc
		if !serviceChargeLine {
			chargeBase = chargeBase.Add(salesLine2.Amount).Add(salesLine2.Inv_discount_amount)
		}
	}
	err = salesHeader.ModifyOrError(false)
	if err != nil {
		return err
	}

	custInvDisc := gtables.NewCustInvoiceDisc(salesHeader)
	found, err := findCustInvDisc(salesHeader, custInvDisc, chargeBase)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if !custInvDisc.Service_charge.IsZero() {
		err = custPostingGr.TestField("service_charge_acc")
		if err != nil {
			return err
		}
		salesLine2.SetRange("allow_invoice_disc", false)
		salesLine2.SetRange("type", gtables.SalesLine_Type.GLAccount)
		salesLine2.SetRange("no", custPostingGr.Service_charge_acc)
		if salesLine2.FindLast() {
			err = salesLine2.Validate("unit_price", custInvDisc.Service_charge)
			if err != nil {
				return err
			}
			err = salesLine2.ModifyOrError(false)
			if err != nil {
				return err
			}
		} else {
			salesLine2.Reset()
			salesLine2.SetRange("document_type", salesHeader.Document_type)
			salesLine2.SetRange("document_no", salesHeader.No)
			lineNo := 0
			if salesLine2.FindLast() {
				lineNo = salesLine2.Line_no
			}
			salesLine2 = gtables.NewSalesLine(salesHeader)
			salesLine2.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
			salesLine2.Document_no = salesHeader.No
			salesLine2.Line_no = lineNo + 10000
			salesLine2.Type = gtables.SalesLine_Type.GLAccount
			err = salesLine2.Validate("no", custPostingGr.Service_charge_acc)
			if err != nil {
				return err
			}
			salesLine2.Description = types.Text(i18n.GetInstance().Message("SALES_SERVICE_CHARGE", fcodeunits.CurrentLanguage()))
			err = salesLine2.Validate("quantity", types.NewDecimalFromInt(1))
			if err != nil {
				return err
			}
			err = salesLine2.Validate("unit_price", custInvDisc.Service_charge)
			if err != nil {
				return err
			}
			salesLine2.Allow_invoice_disc = false
			err = salesLine2.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}

	custInvDisc = gtables.NewCustInvoiceDisc(salesHeader)
	found, err = findCustInvDisc(salesHeader, custInvDisc, invDiscBase)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	salesLine2 = gtables.NewSalesLine(salesHeader)
	salesLine2.SetRange("document_type", salesHeader.Document_type)
	salesLine2.SetRange("document_no", salesHeader.No)
	salesLine2.SetFilter("type", "<>0")
	salesLine2.SetRange("allow_invoice_disc", true)
	for found := salesLine2.FindSet(); found; found = salesLine2.Next() {
		if !salesLine2.Quantity.IsZero() {
			salesLine2.Amount = salesLine2.Amount.Add(salesLine2.Inv_discount_amount)
			err = salesLine2.Validate("inv_discount_amount", system.Round(salesLine2.Amount.Mul(custInvDisc.Discount_percent).Div(hundred), types.MustDecimal("0.00001")))
			if err != nil {
				return err
			}
			err = salesLine2.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// findCustInvDisc finds the invoice discount with the highest minimum amount up to the base,
// in the document's currency or else in local currency (the base converted at the posting date,
// a quote's at the work date; the service charge converted back). false: none applies.
func findCustInvDisc(salesHeader *gtables.SalesHeader, custInvDisc *gtables.CustInvoiceDisc, base types.Decimal) (bool, error) {
	custInvDisc.SetRange("code", salesHeader.Invoice_disc_code)
	custInvDisc.SetRange("currency_code", salesHeader.Currency_code)
	custInvDisc.SetRange("minimum_amount", types.Decimal{}, base)
	if custInvDisc.FindLast() {
		return true, nil
	}
	if salesHeader.Currency_code != "" {
		var currencyDate types.Date
		if salesHeader.Document_type != gtables.SalesHeader_Document_type.Quote {
			currencyDate = salesHeader.Posting_date
		} else {
			currencyDate = system.WorkDate()
		}
		currExchRate := tables.ExchangeRates(salesHeader)
		currencyFactor, err := currExchRate.ExchangeRate(currencyDate, salesHeader.Currency_code)
		if err != nil {
			return false, err
		}
		baseLCY, err := currExchRate.ExchangeAmtFCYToLCY(currencyDate, salesHeader.Currency_code, base, currencyFactor)
		if err != nil {
			return false, err
		}
		custInvDisc.SetRange("currency_code", types.Code(""))
		custInvDisc.SetRange("minimum_amount", types.Decimal{}, baseLCY)
		if !custInvDisc.FindLast() {
			return false, nil
		}
		currency := gtables.NewCurrency(salesHeader)
		err = currency.GetOrError(salesHeader.Currency_code)
		if err != nil {
			return false, err
		}
		serviceCharge, err := currExchRate.ExchangeAmtLCYToFCY(currencyDate, salesHeader.Currency_code, custInvDisc.Service_charge, currencyFactor)
		if err != nil {
			return false, err
		}
		custInvDisc.Service_charge = system.Round(serviceCharge, currency.Amount_rounding_precision)
		return true, nil
	} else {
		return false, nil
	}
}
