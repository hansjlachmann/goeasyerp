package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 312 Cust-Check Cr. Limit with the warning of form 343 Check Credit Limit: by the
// Sales & Receivables Setup's Credit Warnings, an amount that takes the customer over the credit
// limit (balance + shipped not invoiced + outstanding orders + this amount), or a customer with an
// overdue balance, gives a warning. With a dialog (a codeunit or report the user runs) the user
// is asked whether to go on — No stops with "The update has been interrupted"; entered on a page
// (an API request, no dialog to ask in) the warning is shown to the user and the value is kept.
// Table triggers call it only for a value the user entered (CurrFieldNo).

// creditLimitCheck is the form's state: the customer and the amounts of the warning
type creditLimitCheck struct {
	src                     ftables.Source
	salesSetup              *gtables.SalesReceivablesSetup
	cust                    *gtables.Customer
	newOrderAmountLCY       types.Decimal
	oldOrderAmountLCY       types.Decimal
	orderAmountThisOrderLCY types.Decimal
	orderAmountTotalLCY     types.Decimal
	custCreditAmountLCY     types.Decimal
	heading                 string
}

func newCreditLimitCheck(src ftables.Source) *creditLimitCheck {
	return &creditLimitCheck{src: src, salesSetup: gtables.GetSalesReceivablesSetup(src)}
}

// CustCheckCrLimitGenJnlLine (GenJnlLineCheck): a journal line's amount to a customer
func CustCheckCrLimitGenJnlLine(genJnlLine *gtables.GenJournalLine) error {
	c := newCreditLimitCheck(genJnlLine)
	if c.salesSetup.Credit_warnings == gtables.SalesReceivablesSetup_Credit_warnings.NoWarning {
		return nil
	}
	var show bool
	var err error
	if genJnlLine.Account_type == gtables.GenJournalLine_Account_type.Customer {
		show, err = c.showWarning(genJnlLine.Account_no, genJnlLine.Amount, types.Decimal{}, true)
	} else {
		show, err = c.showWarning(genJnlLine.Bal_account_no, genJnlLine.Amount.Neg(), types.Decimal{}, true)
	}
	if err != nil || !show {
		return err
	}
	return c.ask()
}

// CustCheckCrLimitSalesHeader (SalesHeaderCheck): the document's Amount Including Tax (set by the
// caller), with the lines of a document that is not an order (used when lines are added)
func CustCheckCrLimitSalesHeader(salesHeader *gtables.SalesHeader) error {
	c := newCreditLimitCheck(salesHeader)
	if c.salesSetup.Credit_warnings == gtables.SalesReceivablesSetup_Credit_warnings.NoWarning {
		return nil
	}
	if salesHeader.Currency_code == "" {
		c.newOrderAmountLCY = salesHeader.Amount_including_tax
	} else {
		amountLCY, err := ExchangeRates(salesHeader).ExchangeAmtFCYToLCY(system.WorkDate(), salesHeader.Currency_code,
			salesHeader.Amount_including_tax, salesHeader.Currency_factor)
		if err != nil {
			return err
		}
		c.newOrderAmountLCY = system.Round(amountLCY)
	}
	if salesHeader.Document_type != gtables.SalesHeader_Document_type.Order {
		c.newOrderAmountLCY = c.newOrderAmountLCY.Add(c.salesLineAmount(salesHeader.Document_type, salesHeader.No))
	}
	show, err := c.showWarning(salesHeader.Bill_to_customer_no, c.newOrderAmountLCY, types.Decimal{}, true)
	if err != nil || !show {
		return err
	}
	return c.ask()
}

// CustCheckCrLimitSalesLine (SalesLineCheck): the line's outstanding and shipped-not-invoiced
// amount instead of what it had
func CustCheckCrLimitSalesLine(salesLine *gtables.SalesLine) error {
	c := newCreditLimitCheck(salesLine)
	if c.salesSetup.Credit_warnings == gtables.SalesReceivablesSetup_Credit_warnings.NoWarning {
		return nil
	}
	salesHeader := gtables.NewSalesHeader(salesLine)
	err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(salesLine.Document_type), "no": salesLine.Document_no})
	if err != nil {
		return err
	}
	c.newOrderAmountLCY = salesLine.Outstanding_amount_lcy.Add(salesLine.Shipped_not_invoiced_lcy)
	storedLine := gtables.NewSalesLine(salesLine)
	if storedLine.Get(map[string]interface{}{"document_type": int(salesLine.Document_type), "document_no": salesLine.Document_no, "line_no": salesLine.Line_no}) {
		c.oldOrderAmountLCY = storedLine.Outstanding_amount_lcy.Add(storedLine.Shipped_not_invoiced_lcy)
	}
	if salesLine.Document_type != gtables.SalesLine_Document_type.Order {
		c.newOrderAmountLCY = c.newOrderAmountLCY.Sub(c.oldOrderAmountLCY).Add(
			c.salesLineAmount(gtables.SalesHeaderDocument_type(salesLine.Document_type), salesLine.Document_no))
		c.oldOrderAmountLCY = types.Decimal{}
	}
	show, err := c.showWarning(salesHeader.Bill_to_customer_no, c.newOrderAmountLCY, c.oldOrderAmountLCY, false)
	if err != nil || !show {
		return err
	}
	return c.ask()
}

// salesLineAmount (SalesLineAmount): the document's outstanding and shipped-not-invoiced amounts
func (c *creditLimitCheck) salesLineAmount(docType gtables.SalesHeaderDocument_type, docNo types.Code) types.Decimal {
	salesLine := gtables.NewSalesLine(c.src)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(docType))
	salesLine.SetRange("document_no", docNo)
	err := salesLine.CalcSums("outstanding_amount_lcy", "shipped_not_invoiced_lcy")
	if err != nil {
		return types.Decimal{}
	}
	return salesLine.Outstanding_amount_lcy.Add(salesLine.Shipped_not_invoiced_lcy)
}

// showWarning (ShowWarning): whether the customer goes over the credit limit, or has an overdue
// balance (checkOverdueBalance), with the setup's warnings
func (c *creditLimitCheck) showWarning(custNo types.Code, newOrderAmountLCY, oldOrderAmountLCY types.Decimal, checkOverdueBalance bool) (bool, error) {
	if custNo == "" {
		return false, nil
	}
	c.newOrderAmountLCY = newOrderAmountLCY
	c.oldOrderAmountLCY = oldOrderAmountLCY
	c.cust = gtables.NewCustomer(c.src)
	err := c.cust.GetOrError(custNo)
	if err != nil {
		return false, err
	}
	err = c.cust.SetFlowFilter("date_filter", ".."+system.WorkDate().String())
	if err != nil {
		return false, err
	}
	warnings := c.salesSetup.Credit_warnings
	language := fcodeunits.CurrentLanguage()
	if warnings == gtables.SalesReceivablesSetup_Credit_warnings.BothWarnings || warnings == gtables.SalesReceivablesSetup_Credit_warnings.CreditLimit {
		c.calcCreditLimitLCY()
		if c.custCreditAmountLCY.GreaterThan(c.cust.Credit_limit_lcy) && !c.cust.Credit_limit_lcy.IsZero() {
			c.heading = i18n.GetInstance().Message("CREDIT_LIMIT_EXCEEDED", language)
			return true, nil
		}
	}
	if checkOverdueBalance &&
		(warnings == gtables.SalesReceivablesSetup_Credit_warnings.BothWarnings || warnings == gtables.SalesReceivablesSetup_Credit_warnings.OverdueBalance) {
		c.cust.CalcFields("balance_due_lcy")
		if c.cust.Balance_due_lcy.IsPositive() {
			c.heading = i18n.GetInstance().Message("CREDIT_LIMIT_OVERDUE_BALANCE", language)
			return true, nil
		}
	}
	return false, nil
}

// calcCreditLimitLCY (CalcCreditLimitLCY): balance + shipped not invoiced + the outstanding orders
// without the old amount of this order + this amount
func (c *creditLimitCheck) calcCreditLimitLCY() {
	c.cust.CalcFields("balance_lcy", "outstanding_orders_lcy", "shipped_not_invoiced_lcy")
	c.orderAmountThisOrderLCY = c.newOrderAmountLCY
	c.orderAmountTotalLCY = c.cust.Outstanding_orders_lcy.Sub(c.oldOrderAmountLCY)
	c.custCreditAmountLCY = c.cust.Balance_lcy.Add(c.cust.Shipped_not_invoiced_lcy).Add(c.orderAmountTotalLCY).Add(c.orderAmountThisOrderLCY)
}

// ask (RUNMODAL = ACTION::Yes): the warning with the customer's amounts — with a dialog the
// user decides (No: interrupted), else it is shown and the code goes on
func (c *creditLimitCheck) ask() error {
	language := fcodeunits.CurrentLanguage()
	ts := i18n.GetInstance()
	amount := func(d types.Decimal) string { return format.Decimal(d, "2:2", language) }
	c.calcCreditLimitLCY()
	c.cust.CalcFields("balance_due_lcy")
	details := system.StrSubstNo(ts.Message("CREDIT_LIMIT_DETAILS", language), c.cust.No, c.cust.Name,
		amount(c.cust.Balance_lcy), amount(c.orderAmountTotalLCY), amount(c.cust.Shipped_not_invoiced_lcy),
		amount(c.orderAmountThisOrderLCY), amount(c.custCreditAmountLCY), amount(c.cust.Credit_limit_lcy), amount(c.cust.Balance_due_lcy))
	if !system.GuiAllowed() {
		fcodeunits.Notify(c.heading + " " + details)
		return nil
	}
	if !system.Confirm(system.StrSubstNo(ts.Message("CREDIT_LIMIT_QUESTION", language), c.heading, details)) {
		return apperrors.JnlUpdateInterrupted()
	}
	return nil
}
