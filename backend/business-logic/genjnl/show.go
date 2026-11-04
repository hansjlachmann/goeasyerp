package genjnl

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The journal pages' Account menu: Card (Shift+F5, codeunit 15 Gen. Jnl.-Show Card) and
// Ledger Entries (Ctrl+F5, codeunit 14 Gen. Jnl.-Show Entries) open the card or the entries of
// the line's account
func init() {
	functions.Register("table", gtables.GenJournalLineTableName, "ShowCard", functions.For(ShowCard), functions.WithDialog())
	functions.Register("table", gtables.GenJournalLineTableName, "ShowEntries", functions.For(ShowEntries), functions.WithDialog())
}

// ShowCard opens the card of the line's account (codeunit 15)
func ShowCard(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	if rec.Account_no == "" {
		return fcodeunits.Result{Success: true}, nil
	}
	switch rec.Account_type {
	case gtables.GenJournalLine_Account_type.GLAccount:
		glAcc := gtables.NewGLAccount(rec)
		glAcc.SetRange("no", rec.Account_no)
		system.RunModal(17, glAcc, false)
	case gtables.GenJournalLine_Account_type.Customer:
		cust := gtables.NewCustomer(rec)
		cust.SetRange("no", rec.Account_no)
		system.RunModal(21, cust, false)
	case gtables.GenJournalLine_Account_type.Vendor:
		vend := gtables.NewVendor(rec)
		vend.SetRange("no", rec.Account_no)
		system.RunModal(26, vend, false)
	case gtables.GenJournalLine_Account_type.BankAccount:
		bankAcc := gtables.NewBankAccount(rec)
		bankAcc.SetRange("no", rec.Account_no)
		system.RunModal(370, bankAcc, false)
	case gtables.GenJournalLine_Account_type.FixedAsset:
		fa := gtables.NewFixedAsset(rec)
		fa.SetRange("no", rec.Account_no)
		system.RunModal(5600, fa, false)
	}
	return fcodeunits.Result{Success: true}, nil
}

// ShowEntries opens the ledger entries of the line's account (codeunit 14)
func ShowEntries(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	if rec.Account_no == "" {
		return fcodeunits.Result{Success: true}, nil
	}
	switch rec.Account_type {
	case gtables.GenJournalLine_Account_type.GLAccount:
		glEntry := gtables.NewGLEntry(rec)
		glEntry.SetCurrentKey("gl_account_no", "posting_date")
		glEntry.SetRange("gl_account_no", rec.Account_no)
		system.RunModal(20, glEntry, false)
	case gtables.GenJournalLine_Account_type.Customer:
		custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
		custLedgEntry.SetCurrentKey("customer_no", "posting_date")
		custLedgEntry.SetRange("customer_no", rec.Account_no)
		system.RunModal(25, custLedgEntry, false)
	case gtables.GenJournalLine_Account_type.Vendor:
		vendLedgEntry := gtables.NewVendorLedgerEntry(rec)
		vendLedgEntry.SetCurrentKey("vendor_no", "posting_date")
		vendLedgEntry.SetRange("vendor_no", rec.Account_no)
		system.RunModal(29, vendLedgEntry, false)
	case gtables.GenJournalLine_Account_type.BankAccount:
		bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(rec)
		bankAccLedgEntry.SetCurrentKey("bank_account_no", "posting_date")
		bankAccLedgEntry.SetRange("bank_account_no", rec.Account_no)
		system.RunModal(372, bankAccLedgEntry, false)
	case gtables.GenJournalLine_Account_type.FixedAsset:
		faLedgEntry := gtables.NewFALedgerEntry(rec)
		faLedgEntry.SetRange("fa_no", rec.Account_no)
		system.RunModal(5604, faLedgEntry, false)
	}
	return fcodeunits.Result{Success: true}, nil
}
