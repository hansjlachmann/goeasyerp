package tables

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// vendorLedgerEntryLogic is the business logic of table 25 Vendor Ledger Entry
type vendorLedgerEntryLogic struct{}

func init() { gtables.RegisterVendorLedgerEntryLogic(vendorLedgerEntryLogic{}) }

// SetVendDebitCredit fills Debit/Credit Amount (and LCY) from Amount and Amount (LCY): a
// positive amount is a debit, a negative one a credit (shown positive) — as posting does
func SetVendDebitCredit(e *gtables.VendorLedgerEntry) {
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
	functions.Register("table", "Vendor_ledger_entry", "ShowDoc", functions.For(ShowVendLedgEntryDoc), functions.WithDialog())
}

// ShowVendLedgEntryDoc (Show Document): the posted purchase invoice or credit memo of an entry
// opens on its card (an entry posted from a journal has none)
func ShowVendLedgEntryDoc(rec *gtables.VendorLedgerEntry) (fcodeunits.Result, error) {
	switch rec.Document_type {
	case gtables.VendorLedgerEntry_Document_type.Invoice:
		purchInvHeader := gtables.NewPurchInvHeader(rec)
		purchInvHeader.SetRange("no", rec.Document_no)
		if !purchInvHeader.IsEmpty() {
			system.RunModal(138, purchInvHeader, false)
		}
	case gtables.VendorLedgerEntry_Document_type.CreditMemo:
		purchCrMemoHeader := gtables.NewPurchCrMemoHdr(rec)
		purchCrMemoHeader.SetRange("no", rec.Document_no)
		if !purchCrMemoHeader.IsEmpty() {
			system.RunModal(140, purchCrMemoHeader, false)
		}
	}
	return fcodeunits.Result{Success: true}, nil
}
