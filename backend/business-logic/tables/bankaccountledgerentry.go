package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bankAccountLedgerEntryLogic is the business logic of table 271 Bank Account Ledger Entry
type bankAccountLedgerEntryLogic struct{}

func init() { gtables.RegisterBankAccountLedgerEntryLogic(bankAccountLedgerEntryLogic{}) }

// SetBankDebitCredit fills Debit/Credit Amount (and LCY) from Amount and Amount (LCY): a
// positive amount is a debit, a negative one a credit (shown positive) — as posting does
func SetBankDebitCredit(e *gtables.BankAccountLedgerEntry) {
	zero := types.ZeroDecimal()
	e.Debit_amount, e.Credit_amount = zero, zero
	e.Debit_amount_lcy, e.Credit_amount_lcy = zero, zero
	if e.Amount.IsPositive() {
		e.Debit_amount = e.Amount
	} else {
		e.Credit_amount = e.Amount.Abs()
	}
	if e.Amount_lcy.IsPositive() {
		e.Debit_amount_lcy = e.Amount_lcy
	} else {
		e.Credit_amount_lcy = e.Amount_lcy.Abs()
	}
}
