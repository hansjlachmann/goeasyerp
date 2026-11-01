package pages

import (
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Page 379 Bank Acc. Reconciliation: Total Balance (the last statement's balance with the
// lines' statement amounts) and Total Difference of its lines (form 380's CalcBalance)
func init() {
	pages.OnAfterGetRecord(379, func(rec *gtables.BankAccReconciliation, page *pages.Control) {
		line := gtables.NewBankAccReconciliationLine(rec)
		line.SetRange("bank_account_no", rec.Bank_account_no)
		line.SetRange("statement_no", rec.Statement_no)
		var statementAmount, difference types.Decimal
		if line.CalcSums("statement_amount", "difference") == nil {
			statementAmount, difference = line.Statement_amount, line.Difference
		}
		page.SetValue("total_balance", rec.Balance_last_statement.Add(statementAmount))
		page.SetValue("total_difference", difference)
	})
}
