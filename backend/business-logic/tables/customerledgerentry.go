package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// customerLedgerEntryLogic is the business logic of table 21 Customer Ledger Entry
type customerLedgerEntryLogic struct{}

func init() { gtables.RegisterCustomerLedgerEntryLogic(customerLedgerEntryLogic{}) }

// SetDebitCredit fills Debit/Credit Amount (and LCY) from Amount and Amount (LCY): a positive
// amount is a debit, a negative one a credit (shown positive) — as posting does
func SetDebitCredit(e *gtables.CustomerLedgerEntry) {
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

func init() {
	functions.Register("table", "Customer_ledger_entry", "ShowDoc", functions.For(ShowCustLedgEntryDoc), functions.WithDialog())
}

// ShowCustLedgEntryDoc (Show Document): the posted invoice or credit memo of an entry opens on
// its card (an entry posted from a journal has none)
func ShowCustLedgEntryDoc(rec *gtables.CustomerLedgerEntry) (fcodeunits.Result, error) {
	switch rec.Document_type {
	case gtables.CustomerLedgerEntry_Document_type.Invoice:
		salesInvHeader := gtables.NewSalesInvoiceHeader(rec)
		salesInvHeader.SetRange("no", rec.Document_no)
		if !salesInvHeader.IsEmpty() {
			system.RunModal(132, salesInvHeader, false)
		}
	case gtables.CustomerLedgerEntry_Document_type.CreditMemo:
		salesCrMemoHeader := gtables.NewSalesCrMemoHeader(rec)
		salesCrMemoHeader.SetRange("no", rec.Document_no)
		if !salesCrMemoHeader.IsEmpty() {
			system.RunModal(134, salesCrMemoHeader, false)
		}
	}
	return fcodeunits.Result{Success: true}, nil
}
