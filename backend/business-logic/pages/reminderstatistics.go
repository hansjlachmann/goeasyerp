package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 437 Reminder Statistics, 441 Issued Reminder Statistics, 449 Finance Charge Memo
// Statistics and 453 Issued Fin. Charge Memo Stat.: the document's total (a reminder's remaining
// amount, additional fee, interest and tax; a memo's without the remaining amount) and its
// customer's balance, credit limit and how much of it the balance uses (OnAfterGetRecord)
func init() {
	pages.OnAfterGetRecord(437, func(rec *gtables.ReminderHeader, page *pages.Control) {
		reminderHeader := gtables.NewReminderHeader(rec)
		reminderHeader.No = rec.No
		reminderHeader.CalcFields("remaining_amount", "additional_fee", "interest_amount", "tax_amount")
		reminderTotal := reminderHeader.Remaining_amount.Add(reminderHeader.Additional_fee).
			Add(reminderHeader.Interest_amount).Add(reminderHeader.Tax_amount)
		page.SetValue("reminder_total", reminderTotal)
		setDocumentCustomerCredit(rec, rec.Customer_no, page)
	})
	pages.OnAfterGetRecord(441, func(rec *gtables.IssuedReminderHeader, page *pages.Control) {
		issuedReminderHeader := gtables.NewIssuedReminderHeader(rec)
		issuedReminderHeader.No = rec.No
		issuedReminderHeader.CalcFields("remaining_amount", "additional_fee", "interest_amount", "tax_amount")
		reminderTotal := issuedReminderHeader.Remaining_amount.Add(issuedReminderHeader.Additional_fee).
			Add(issuedReminderHeader.Interest_amount).Add(issuedReminderHeader.Tax_amount)
		page.SetValue("reminder_total", reminderTotal)
		setDocumentCustomerCredit(rec, rec.Customer_no, page)
	})
	pages.OnAfterGetRecord(449, func(rec *gtables.FinanceChargeMemoHeader, page *pages.Control) {
		finChrgMemoHeader := gtables.NewFinanceChargeMemoHeader(rec)
		finChrgMemoHeader.No = rec.No
		finChrgMemoHeader.CalcFields("additional_fee", "interest_amount", "tax_amount")
		finChrgMemoTotal := finChrgMemoHeader.Additional_fee.Add(finChrgMemoHeader.Interest_amount).
			Add(finChrgMemoHeader.Tax_amount)
		page.SetValue("fin_chrg_memo_total", finChrgMemoTotal)
		setDocumentCustomerCredit(rec, rec.Customer_no, page)
	})
	pages.OnAfterGetRecord(453, func(rec *gtables.IssuedFinChargeMemoHeader, page *pages.Control) {
		issuedFinChrgMemoHeader := gtables.NewIssuedFinChargeMemoHeader(rec)
		issuedFinChrgMemoHeader.No = rec.No
		issuedFinChrgMemoHeader.CalcFields("additional_fee", "interest_amount", "tax_amount")
		finChrgMemoTotal := issuedFinChrgMemoHeader.Additional_fee.Add(issuedFinChrgMemoHeader.Interest_amount).
			Add(issuedFinChrgMemoHeader.Tax_amount)
		page.SetValue("fin_chrg_memo_total", finChrgMemoTotal)
		setDocumentCustomerCredit(rec, rec.Customer_no, page)
	})
}

// setDocumentCustomerCredit: the customer's Balance (LCY), Credit Limit (LCY) and the share of
// the credit limit the balance uses (classic CreditLimitLCYExpendedPct, a bar of 0–10000; here
// the percentage) — nothing when the customer does not exist
func setDocumentCustomerCredit(src ftables.Source, customerNo types.Code, page *pages.Control) {
	cust := gtables.NewCustomer(src)
	balanceLCY := types.ZeroDecimal()
	creditLimitLCY := types.ZeroDecimal()
	found := cust.Get(customerNo)
	if found {
		cust.CalcFields("balance_lcy")
		balanceLCY = cust.Balance_lcy
		creditLimitLCY = cust.Credit_limit_lcy
	}
	creditLimitLCYExpendedPct := types.ZeroDecimal()
	if !creditLimitLCY.IsZero() {
		creditLimitLCYExpendedPct = system.Round(balanceLCY.Div(creditLimitLCY).Mul(types.NewDecimal(100)), types.MustDecimal("0.01"))
	}
	page.SetValue("balance_lcy", balanceLCY)
	page.SetValue("credit_limit_lcy", creditLimitLCY)
	page.SetValue("credit_limit_lcy_expended_pct", creditLimitLCYExpendedPct)
}
