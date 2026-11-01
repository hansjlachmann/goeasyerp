package tables

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// bankAccReconciliationLineLogic is the business logic of table 274 Bank Acc. Reconciliation
// Line: a transaction of the bank statement — applied to bank account ledger entries or check
// ledger entries (Apply Entries, Suggest Lines), or a Difference; Difference is Statement Amount
// less Applied Amount.
type bankAccReconciliationLineLogic struct{}

func init() { gtables.RegisterBankAccReconciliationLineLogic(bankAccReconciliationLineLogic{}) }

// OnInsert: the line's reconciliation exists; a new line has nothing applied
func (bankAccReconciliationLineLogic) OnInsert(rec *gtables.BankAccReconciliationLine) error {
	bankAccRecon := gtables.NewBankAccReconciliation(rec)
	err := bankAccRecon.GetOrError(map[string]interface{}{"bank_account_no": rec.Bank_account_no, "statement_no": rec.Statement_no})
	if err != nil {
		return err
	}
	rec.Applied_entries = 0
	return rec.Validate("applied_amount", types.Decimal{})
}

// OnDelete: the entries applied to the line are free again
func (bankAccReconciliationLineLogic) OnDelete(rec *gtables.BankAccReconciliationLine) error {
	return removeApplication(rec, rec.Type)
}

func (bankAccReconciliationLineLogic) OnRename(rec *gtables.BankAccReconciliationLine) error {
	return apperrors.RenameNotAllowed(gtables.BankAccReconciliationLineTableName)
}

func (bankAccReconciliationLineLogic) OnValidate_Statement_amount(rec *gtables.BankAccReconciliationLine) error {
	rec.Difference = rec.Statement_amount.Sub(rec.Applied_amount)
	return nil
}

func (bankAccReconciliationLineLogic) OnValidate_Difference(rec *gtables.BankAccReconciliationLine) error {
	rec.Statement_amount = rec.Applied_amount.Add(rec.Difference)
	return nil
}

func (bankAccReconciliationLineLogic) OnValidate_Applied_amount(rec *gtables.BankAccReconciliationLine) error {
	rec.Difference = rec.Statement_amount.Sub(rec.Applied_amount)
	return nil
}

// OnValidate_Type: a line with applied entries loses its application (asked in a dialog)
func (bankAccReconciliationLineLogic) OnValidate_Type(rec *gtables.BankAccReconciliationLine) error {
	xRec := rec.XRec()
	if rec.Type == xRec.Type || rec.Applied_entries == 0 {
		return nil
	}
	if system.GuiAllowed() && !system.Confirm(i18n.GetInstance().Message("BANK_REC_DELETE_APPLICATION", fcodeunits.CurrentLanguage())) {
		return apperrors.BankRecUpdateCanceled()
	}
	err := removeApplication(rec, xRec.Type)
	if err != nil {
		return err
	}
	err = rec.Validate("applied_amount", types.Decimal{})
	rec.Applied_entries = 0
	rec.Check_no = ""
	return err
}

// removeApplication: the entries of the type applied to the line are free again; the line has
// nothing applied (RemoveApplication)
func removeApplication(rec *gtables.BankAccReconciliationLine, appliedType gtables.BankAccReconciliationLineType) error {
	switch appliedType {
	case gtables.BankAccReconciliationLine_Type.BankAccountLedgerEntry:
		bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(rec)
		bankAccLedgEntry.SetRange("bank_account_no", rec.Bank_account_no)
		bankAccLedgEntry.SetRange("open", true)
		bankAccLedgEntry.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied)
		bankAccLedgEntry.SetRange("statement_no", rec.Statement_no)
		bankAccLedgEntry.SetRange("statement_line_no", rec.Statement_line_no)
		bankAccLedgEntry.LockTable()
		for found := bankAccLedgEntry.FindSetBuffered(); found; found = bankAccLedgEntry.Next() {
			err := RemoveBankAccReconNo(bankAccLedgEntry, rec, true)
			if err != nil {
				return err
			}
		}
	case gtables.BankAccReconciliationLine_Type.CheckLedgerEntry:
		checkLedgEntry := gtables.NewCheckLedgerEntry(rec)
		checkLedgEntry.SetRange("bank_account_no", rec.Bank_account_no)
		checkLedgEntry.SetRange("open", true)
		checkLedgEntry.SetRange("statement_status", gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied)
		checkLedgEntry.SetRange("statement_no", rec.Statement_no)
		checkLedgEntry.SetRange("statement_line_no", rec.Statement_line_no)
		checkLedgEntry.LockTable()
		for found := checkLedgEntry.FindSetBuffered(); found; found = checkLedgEntry.Next() {
			err := RemoveCheckReconNo(checkLedgEntry, rec, true)
			if err != nil {
				return err
			}
		}
		rec.Check_no = ""
	default:
		return nil
	}
	rec.Applied_entries = 0
	err := rec.Validate("applied_amount", types.Decimal{})
	if err != nil {
		return err
	}
	rec.Modify(false) // a line being deleted or not saved yet
	return nil
}

// Codeunit 375 Bank Acc. Entry Set Recon.-No.

// ToggleBankAccReconNo applies an open bank account ledger entry to a statement line, or takes it
// off; the line's Applied Amount and Applied Entries follow — with changeAmount its Statement
// Amount too (ToggleReconNo)
func ToggleBankAccReconNo(bankAccLedgEntry *gtables.BankAccountLedgerEntry, bankAccReconLine *gtables.BankAccReconciliationLine, changeAmount bool) error {
	bankAccLedgEntry.LockTable()
	bankAccReconLine.LockTable()
	if !bankAccReconLine.Find("=") {
		return apperrors.TableRecordNotFound(gtables.BankAccReconciliationLineTableName, bankAccReconLine.Statement_no.String())
	}
	if bankAccLedgEntry.Statement_no == "" {
		err := SetBankAccReconNo(bankAccLedgEntry, bankAccReconLine)
		if err != nil {
			return err
		}
		bankAccReconLine.Applied_amount = bankAccReconLine.Applied_amount.Add(bankAccLedgEntry.Remaining_amount)
		bankAccReconLine.Applied_entries++
	} else {
		err := RemoveBankAccReconNo(bankAccLedgEntry, bankAccReconLine, true)
		if err != nil {
			return err
		}
		bankAccReconLine.Applied_amount = bankAccReconLine.Applied_amount.Sub(bankAccLedgEntry.Remaining_amount)
		bankAccReconLine.Applied_entries--
	}
	var err error
	if changeAmount {
		err = bankAccReconLine.Validate("statement_amount", bankAccReconLine.Applied_amount)
	} else {
		err = bankAccReconLine.Validate("statement_amount", bankAccReconLine.Statement_amount)
	}
	if err != nil {
		return err
	}
	return bankAccReconLine.ModifyOrError(false)
}

// openChecksOf: the open check ledger entries of a bank account ledger entry
func openChecksOf(bankAccLedgEntry *gtables.BankAccountLedgerEntry) *gtables.CheckLedgerEntry {
	checkLedgEntry := gtables.NewCheckLedgerEntry(bankAccLedgEntry)
	checkLedgEntry.SetRange("bank_account_ledger_entry_no", bankAccLedgEntry.Entry_no)
	checkLedgEntry.SetRange("open", true)
	return checkLedgEntry
}

// SetBankAccReconNo: an open entry not on a statement is applied to the line; its open checks
// are marked Bank Acc. Entry Applied (SetReconNo)
func SetBankAccReconNo(bankAccLedgEntry *gtables.BankAccountLedgerEntry, bankAccReconLine *gtables.BankAccReconciliationLine) error {
	err := testField(bankAccLedgEntry, "open", true)
	if err == nil {
		err = testField(bankAccLedgEntry, "statement_status", gtables.BankAccountLedgerEntry_Statement_status.Open)
	}
	if err == nil {
		err = testField(bankAccLedgEntry, "statement_no", types.Code(""))
	}
	if err == nil {
		err = testField(bankAccLedgEntry, "statement_line_no", 0)
	}
	if err == nil {
		err = testField(bankAccLedgEntry, "bank_account_no", bankAccReconLine.Bank_account_no)
	}
	if err != nil {
		return err
	}
	bankAccLedgEntry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied
	bankAccLedgEntry.Statement_no = bankAccReconLine.Statement_no
	bankAccLedgEntry.Statement_line_no = bankAccReconLine.Statement_line_no
	err = bankAccLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	checkLedgEntry := openChecksOf(bankAccLedgEntry)
	for found := checkLedgEntry.FindSetBuffered(); found; found = checkLedgEntry.Next() {
		err = testField(checkLedgEntry, "statement_status", gtables.CheckLedgerEntry_Statement_status.Open)
		if err == nil {
			err = testField(checkLedgEntry, "statement_no", types.Code(""))
		}
		if err == nil {
			err = testField(checkLedgEntry, "statement_line_no", 0)
		}
		if err != nil {
			return err
		}
		checkLedgEntry.Statement_status = gtables.CheckLedgerEntry_Statement_status.BankAccEntryApplied
		checkLedgEntry.Statement_no = ""
		checkLedgEntry.Statement_line_no = 0
		err = checkLedgEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveBankAccReconNo: the entry (and its open checks) is no longer applied (RemoveReconNo; test:
// it must be applied to the line)
func RemoveBankAccReconNo(bankAccLedgEntry *gtables.BankAccountLedgerEntry, bankAccReconLine *gtables.BankAccReconciliationLine, test bool) error {
	err := testField(bankAccLedgEntry, "open", true)
	if err == nil && test {
		err = testField(bankAccLedgEntry, "statement_status", gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied)
		if err == nil {
			err = testField(bankAccLedgEntry, "statement_no", bankAccReconLine.Statement_no)
		}
		if err == nil {
			err = testField(bankAccLedgEntry, "statement_line_no", bankAccReconLine.Statement_line_no)
		}
	}
	if err == nil {
		err = testField(bankAccLedgEntry, "bank_account_no", bankAccReconLine.Bank_account_no)
	}
	if err != nil {
		return err
	}
	bankAccLedgEntry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.Open
	bankAccLedgEntry.Statement_no = ""
	bankAccLedgEntry.Statement_line_no = 0
	err = bankAccLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	checkLedgEntry := openChecksOf(bankAccLedgEntry)
	for found := checkLedgEntry.FindSetBuffered(); found; found = checkLedgEntry.Next() {
		if test {
			err = testField(checkLedgEntry, "statement_status", gtables.CheckLedgerEntry_Statement_status.BankAccEntryApplied)
			if err == nil {
				err = testField(checkLedgEntry, "statement_no", types.Code(""))
			}
			if err == nil {
				err = testField(checkLedgEntry, "statement_line_no", 0)
			}
			if err != nil {
				return err
			}
		}
		checkLedgEntry.Statement_status = gtables.CheckLedgerEntry_Statement_status.Open
		checkLedgEntry.Statement_no = ""
		checkLedgEntry.Statement_line_no = 0
		err = checkLedgEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// Codeunit 376 Check Entry Set Recon.-No.

// ToggleCheckReconNo applies an open check ledger entry to a statement line, or takes it off; the
// line's Applied Amount (the check's amount reversed), Applied Entries and Check No. follow
func ToggleCheckReconNo(checkLedgEntry *gtables.CheckLedgerEntry, bankAccReconLine *gtables.BankAccReconciliationLine, changeAmount bool) error {
	checkLedgEntry.LockTable()
	bankAccReconLine.LockTable()
	if !bankAccReconLine.Find("=") {
		return apperrors.TableRecordNotFound(gtables.BankAccReconciliationLineTableName, bankAccReconLine.Statement_no.String())
	}
	if checkLedgEntry.Statement_no == "" {
		err := SetCheckReconNo(checkLedgEntry, bankAccReconLine)
		if err != nil {
			return err
		}
		bankAccReconLine.Applied_amount = bankAccReconLine.Applied_amount.Sub(checkLedgEntry.Amount)
		bankAccReconLine.Applied_entries++
	} else {
		err := RemoveCheckReconNo(checkLedgEntry, bankAccReconLine, true)
		if err != nil {
			return err
		}
		bankAccReconLine.Applied_amount = bankAccReconLine.Applied_amount.Add(checkLedgEntry.Amount)
		bankAccReconLine.Applied_entries--
	}
	if bankAccReconLine.Applied_entries == 1 {
		bankAccReconLine.Check_no = checkLedgEntry.Check_no
	} else {
		bankAccReconLine.Check_no = ""
	}
	var err error
	if changeAmount {
		err = bankAccReconLine.Validate("statement_amount", bankAccReconLine.Applied_amount)
	} else {
		err = bankAccReconLine.Validate("statement_amount", bankAccReconLine.Statement_amount)
	}
	if err != nil {
		return err
	}
	return bankAccReconLine.ModifyOrError(false)
}

// SetCheckReconNo: an open posted (or financially voided) check not on a statement is applied
// to the line; its bank account ledger entry is marked Check Entry Applied (SetReconNo)
func SetCheckReconNo(checkLedgEntry *gtables.CheckLedgerEntry, bankAccReconLine *gtables.BankAccReconciliationLine) error {
	err := testField(checkLedgEntry, "open", true)
	if err != nil {
		return err
	}
	if checkLedgEntry.Statement_status != gtables.CheckLedgerEntry_Statement_status.Open &&
		checkLedgEntry.Statement_status != gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied {
		return apperrors.JnlOptionNotAllowed(gtables.CheckLedgerEntryTableName, "statement_status", checkLedgEntry.Statement_status)
	}
	err = testField(checkLedgEntry, "statement_no", types.Code(""))
	if err == nil {
		err = testField(checkLedgEntry, "statement_line_no", 0)
	}
	if err != nil {
		return err
	}
	if checkLedgEntry.Entry_status != gtables.CheckLedgerEntry_Entry_status.Posted &&
		checkLedgEntry.Entry_status != gtables.CheckLedgerEntry_Entry_status.FinanciallyVoided {
		return apperrors.JnlOptionNotAllowed(gtables.CheckLedgerEntryTableName, "entry_status", checkLedgEntry.Entry_status)
	}
	err = testField(checkLedgEntry, "bank_account_no", bankAccReconLine.Bank_account_no)
	if err != nil {
		return err
	}
	checkLedgEntry.Statement_status = gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied
	checkLedgEntry.Statement_no = bankAccReconLine.Statement_no
	checkLedgEntry.Statement_line_no = bankAccReconLine.Statement_line_no
	err = checkLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(checkLedgEntry)
	err = bankAccLedgEntry.GetOrError(checkLedgEntry.Bank_account_ledger_entry_no)
	if err != nil {
		return err
	}
	err = testField(bankAccLedgEntry, "open", true)
	if err != nil {
		return err
	}
	if bankAccLedgEntry.Statement_status != gtables.BankAccountLedgerEntry_Statement_status.Open &&
		bankAccLedgEntry.Statement_status != gtables.BankAccountLedgerEntry_Statement_status.CheckEntryApplied {
		return apperrors.JnlOptionNotAllowed(gtables.BankAccountLedgerEntryTableName, "statement_status", bankAccLedgEntry.Statement_status)
	}
	err = testField(bankAccLedgEntry, "statement_no", types.Code(""))
	if err == nil {
		err = testField(bankAccLedgEntry, "statement_line_no", 0)
	}
	if err != nil {
		return err
	}
	bankAccLedgEntry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.CheckEntryApplied
	bankAccLedgEntry.Statement_no = ""
	bankAccLedgEntry.Statement_line_no = 0
	return bankAccLedgEntry.ModifyOrError(false)
}

// RemoveCheckReconNo: the check is no longer applied; its bank account ledger entry is free again
// when none of its checks is applied (RemoveReconNo)
func RemoveCheckReconNo(checkLedgEntry *gtables.CheckLedgerEntry, bankAccReconLine *gtables.BankAccReconciliationLine, test bool) error {
	err := testField(checkLedgEntry, "open", true)
	if err == nil && test {
		err = testField(checkLedgEntry, "statement_status", gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied)
		if err == nil {
			err = testField(checkLedgEntry, "statement_no", bankAccReconLine.Statement_no)
		}
		if err == nil {
			err = testField(checkLedgEntry, "statement_line_no", bankAccReconLine.Statement_line_no)
		}
	}
	if err == nil {
		err = testField(checkLedgEntry, "bank_account_no", bankAccReconLine.Bank_account_no)
	}
	if err != nil {
		return err
	}
	checkLedgEntry.Statement_status = gtables.CheckLedgerEntry_Statement_status.Open
	checkLedgEntry.Statement_no = ""
	checkLedgEntry.Statement_line_no = 0
	err = checkLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	checkLedgEntry2 := gtables.NewCheckLedgerEntry(checkLedgEntry)
	checkLedgEntry2.SetRange("bank_account_ledger_entry_no", checkLedgEntry.Bank_account_ledger_entry_no)
	checkLedgEntry2.SetRange("statement_status", gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied)
	if checkLedgEntry2.FindFirst() {
		return nil
	}
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(checkLedgEntry)
	err = bankAccLedgEntry.GetOrError(checkLedgEntry.Bank_account_ledger_entry_no)
	if err != nil {
		return err
	}
	err = testField(bankAccLedgEntry, "open", true)
	if err == nil && test {
		err = testField(bankAccLedgEntry, "statement_status", gtables.BankAccountLedgerEntry_Statement_status.CheckEntryApplied)
		if err == nil {
			err = testField(bankAccLedgEntry, "statement_no", types.Code(""))
		}
		if err == nil {
			err = testField(bankAccLedgEntry, "statement_line_no", 0)
		}
	}
	if err != nil {
		return err
	}
	bankAccLedgEntry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.Open
	bankAccLedgEntry.Statement_no = ""
	bankAccLedgEntry.Statement_line_no = 0
	return bankAccLedgEntry.ModifyOrError(false)
}

// testField: TESTFIELD(field, value) on any record
func testField(rec interface {
	TestField(field string, value ...interface{}) error
}, field string, value interface{}) error {
	return rec.TestField(field, value)
}
