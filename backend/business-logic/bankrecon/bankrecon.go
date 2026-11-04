// Package bankrecon is bank account reconciliation: codeunit 370 Bank Acc. Reconciliation Post
// (the statement's lines close the bank account and check ledger entries applied to them; the
// reconciliation becomes a Bank Account Statement) and codeunit 374 Bank Acc. Recon. Apply
// Entries (pages 381/382 over the entries a statement line can be applied to; F9 applies the
// selected entries or takes them off).
package bankrecon

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 381 Apply Bank Acc. Ledger Entries and 382 Apply Check Ledger Entries, opened from a
// statement line with its key and Change Amount as page parameters
const (
	ApplyBankAccLedgEntriesPageID = 381
	ApplyCheckLedgEntriesPageID   = 382
)

func init() {
	functions.Register("table", gtables.BankAccReconciliationLineTableName, "ApplyEntries", functions.For(ApplyEntries), functions.WithDialog())
	// (the ledger entry tables by their API names: a function finds its record type by it)
	functions.Register("table", "Bank_Account_Ledger_Entry", "ToggleReconNo", functions.For(toggleBankAccReconNo))
	functions.Register("table", "Check_Ledger_Entry", "ToggleReconNo", functions.For(toggleCheckReconNo))
	pages.OnAfterGetRecord(ApplyBankAccLedgEntriesPageID, func(rec *gtables.BankAccountLedgerEntry, page *pages.Control) {
		line := paramStmtLine(rec)
		page.SetValue("applied", rec.Statement_status == gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied &&
			rec.Statement_no == line.Statement_no && rec.Statement_line_no == line.Statement_line_no)
		setStmtLineValues(page, line)
	})
	pages.OnAfterGetRecord(ApplyCheckLedgEntriesPageID, func(rec *gtables.CheckLedgerEntry, page *pages.Control) {
		line := paramStmtLine(rec)
		page.SetValue("applied", rec.Statement_status == gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied &&
			rec.Statement_no == line.Statement_no && rec.Statement_line_no == line.Statement_line_no)
		setStmtLineValues(page, line)
	})
}

// paramStmtLine: the statement line of the page's parameters (as stored)
func paramStmtLine(src ftables.Source) *gtables.BankAccReconciliationLine {
	line := gtables.NewBankAccReconciliationLine(src)
	lineNo, _ := strconv.Atoi(system.PageParam("statement_line_no"))
	line.Get(map[string]interface{}{"bank_account_no": types.Code(system.PageParam("bank_account_no")),
		"statement_no": types.Code(system.PageParam("statement_no")), "statement_line_no": lineNo})
	return line
}

// setStmtLineValues: the footer of the apply pages — the line's statement, applied amount and
// difference
func setStmtLineValues(page *pages.Control, line *gtables.BankAccReconciliationLine) {
	page.SetValue("statement_amount", line.Statement_amount)
	page.SetValue("applied_amount", line.Applied_amount)
	page.SetValue("difference", line.Difference)
}

// ApplyEntries is codeunit 374 Bank Acc. Recon. Apply Entries (Apply Entries on the
// reconciliation): the line is Ready for Application; the open entries of its type not applied to
// another line open in a page (bank account ledger entries 381, checks 382) where F9 applies the
// selected ones to the line or takes them off — a line without a statement amount takes the
// applied amount
func ApplyEntries(rec *gtables.BankAccReconciliationLine) (fcodeunits.Result, error) {
	bankAccReconLine := rec
	bankAccReconLine.Ready_for_application = true
	err := bankAccReconLine.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	var pageID int
	var view system.PageRecord
	switch bankAccReconLine.Type {
	case gtables.BankAccReconciliationLine_Type.BankAccountLedgerEntry:
		bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(rec)
		bankAccLedgEntry.SetRange("bank_account_no", bankAccReconLine.Bank_account_no)
		bankAccLedgEntry.SetRange("open", true)
		bankAccLedgEntry.SetFilter("statement_status", "0|1") // Open | Bank Acc. Entry Applied
		bankAccLedgEntry.SetFilter("statement_no", "''|"+bankAccReconLine.Statement_no.String())
		bankAccLedgEntry.SetFilter("statement_line_no", "0|"+itoa(bankAccReconLine.Statement_line_no))
		pageID, view = ApplyBankAccLedgEntriesPageID, bankAccLedgEntry
	case gtables.BankAccReconciliationLine_Type.CheckLedgerEntry:
		checkLedgEntry := gtables.NewCheckLedgerEntry(rec)
		checkLedgEntry.SetRange("bank_account_no", bankAccReconLine.Bank_account_no)
		checkLedgEntry.SetRange("open", true)
		checkLedgEntry.SetFilter("entry_status", "3|4")     // Posted | Financially Voided
		checkLedgEntry.SetFilter("statement_status", "0|2") // Open | Check Entry Applied
		checkLedgEntry.SetFilter("statement_no", "''|"+bankAccReconLine.Statement_no.String())
		checkLedgEntry.SetFilter("statement_line_no", "0|"+itoa(bankAccReconLine.Statement_line_no))
		pageID, view = ApplyCheckLedgEntriesPageID, checkLedgEntry
	default:
		return fcodeunits.Result{Success: true}, nil
	}
	page := system.NewPage(pageID)
	page.LookupMode(true)
	page.SetParam("bank_account_no", bankAccReconLine.Bank_account_no.String())
	page.SetParam("statement_no", bankAccReconLine.Statement_no.String())
	page.SetParam("statement_line_no", itoa(bankAccReconLine.Statement_line_no))
	page.SetParam("change_amount", boolText(bankAccReconLine.Statement_amount.IsZero()))
	page.SetTableView(view)
	page.RunModal()
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// toggleBankAccReconNo: F9 on page 381 — the selected entries applied to the page's statement
// line or taken off (codeunit 375 ToggleReconNo)
func toggleBankAccReconNo(rec *gtables.BankAccountLedgerEntry) (fcodeunits.Result, error) {
	line := paramStmtLine(rec)
	changeAmount := system.PageParam("change_amount") == "true"
	rec.SetSelectionFilter()
	rec.LockTable()
	for found := rec.FindSetBuffered(); found; found = rec.Next() {
		err := tables.ToggleBankAccReconNo(rec, line, changeAmount)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// toggleCheckReconNo: F9 on page 382 (codeunit 376 ToggleReconNo)
func toggleCheckReconNo(rec *gtables.CheckLedgerEntry) (fcodeunits.Result, error) {
	line := paramStmtLine(rec)
	changeAmount := system.PageParam("change_amount") == "true"
	rec.SetSelectionFilter()
	rec.LockTable()
	for found := rec.FindSetBuffered(); found; found = rec.Next() {
		err := tables.ToggleCheckReconNo(rec, line, changeAmount)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// Post is codeunit 370 Bank Acc. Reconciliation Post: the statement date is filled in, the
// lines add up to the statement ending balance less the last statement's balance; each line's
// applied entries are closed (a check closes its bank account ledger entry when it settles the
// remaining amount) and must give the line's Applied Amount; the bank account gets the statement
// as its last and the new balance; the reconciliation and its lines become a Bank Account
// Statement. It reports false when there are no lines (nothing posted).
func Post(bankAccRecon *gtables.BankAccReconciliation) (bool, error) {
	language := fcodeunits.CurrentLanguage()
	window := system.OpenWindow(i18n.GetInstance().Message("BANK_REC_POST_WINDOW", language))
	defer window.Close()
	window.Update(1, system.StrSubstNo("%1 %2", bankAccRecon.Bank_account_no, bankAccRecon.Statement_no))
	err := bankAccRecon.TestField("statement_date")
	if err != nil {
		return false, err
	}
	bankAccReconLine2 := reconLines(bankAccRecon)
	err = bankAccReconLine2.CalcSums("statement_amount", "difference")
	if err != nil {
		return false, err
	}
	if !bankAccReconLine2.Statement_amount.Equal(bankAccRecon.Statement_ending_balance.Sub(bankAccRecon.Balance_last_statement)) {
		return false, apperrors.BankRecEndingBalance(gtables.BankAccReconciliationTableName)
	}

	// Run through lines
	var lines []*gtables.BankAccReconciliationLine
	bankAccReconLine := reconLines(bankAccRecon)
	for found := bankAccReconLine.FindSet(); found; found = bankAccReconLine.Next() {
		l := gtables.NewBankAccReconciliationLine(bankAccRecon)
		l.Copy(bankAccReconLine)
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		return false, nil
	}
	gtables.NewBankAccountLedgerEntry(bankAccRecon).LockTable()
	gtables.NewCheckLedgerEntry(bankAccRecon).LockTable()
	var totalAmount, totalAppliedAmount, totalDiff types.Decimal
	for i, line := range lines {
		window.Update(2, i+1)
		var appliedAmount types.Decimal
		switch line.Type {
		case gtables.BankAccReconciliationLine_Type.BankAccountLedgerEntry:
			appliedAmount, err = closeBankAccLedgEntries(line)
		case gtables.BankAccReconciliationLine_Type.CheckLedgerEntry:
			appliedAmount, err = closeCheckLedgEntries(line)
		case gtables.BankAccReconciliationLine_Type.Difference:
			totalDiff = totalDiff.Add(line.Statement_amount)
		}
		if err != nil {
			return false, err
		}
		err = line.TestField("applied_amount", appliedAmount)
		if err != nil {
			return false, err
		}
		totalAmount = totalAmount.Add(line.Statement_amount)
		totalAppliedAmount = totalAppliedAmount.Add(appliedAmount)
	}
	// Test amount
	if !totalAmount.Equal(totalAppliedAmount.Add(totalDiff)) {
		return false, apperrors.BankRecApplication(totalAppliedAmount.Add(totalDiff).String(), totalAmount.String())
	}
	if !bankAccReconLine2.Difference.Equal(totalDiff) {
		return false, apperrors.BankRecTotalDifference(bankAccReconLine2.Difference.String(), totalDiff.String())
	}
	// Get bank
	bankAcc := gtables.NewBankAccount(bankAccRecon)
	bankAcc.LockTable()
	err = bankAcc.GetOrError(bankAccRecon.Bank_account_no)
	if err == nil {
		err = bankAcc.TestField("blocked", false)
	}
	if err != nil {
		return false, err
	}
	bankAcc.Last_statement_no = bankAccRecon.Statement_no
	bankAcc.Balance_last_statement = bankAccRecon.Balance_last_statement.Add(totalAmount)
	err = bankAcc.ModifyOrError(false)
	if err != nil {
		return false, err
	}
	// Copy and delete statement
	for _, line := range lines {
		bankAccStmtLine := gtables.NewBankAccountStatementLine(bankAccRecon)
		bankAccStmtLine.TransferFields(line)
		err = bankAccStmtLine.InsertOrError(false)
		if err != nil {
			return false, err
		}
	}
	reconLines(bankAccRecon).DeleteAll()
	bankAccStmt := gtables.NewBankAccountStatement(bankAccRecon)
	bankAccStmt.TransferFields(bankAccRecon)
	err = bankAccStmt.InsertOrError(false)
	if err != nil {
		return false, err
	}
	return true, bankAccRecon.DeleteOrError(false)
}

func reconLines(bankAccRecon *gtables.BankAccReconciliation) *gtables.BankAccReconciliationLine {
	line := gtables.NewBankAccReconciliationLine(bankAccRecon)
	line.SetRange("bank_account_no", bankAccRecon.Bank_account_no)
	line.SetRange("statement_no", bankAccRecon.Statement_no)
	return line
}

// closeBankAccLedgEntries: the bank account ledger entries applied to the line close (and their
// open checks); their remaining amounts are the applied amount
func closeBankAccLedgEntries(line *gtables.BankAccReconciliationLine) (types.Decimal, error) {
	var appliedAmount types.Decimal
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(line)
	bankAccLedgEntry.SetRange("bank_account_no", line.Bank_account_no)
	bankAccLedgEntry.SetRange("open", true)
	bankAccLedgEntry.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied)
	bankAccLedgEntry.SetRange("statement_no", line.Statement_no)
	bankAccLedgEntry.SetRange("statement_line_no", line.Statement_line_no)
	for found := bankAccLedgEntry.FindSetBuffered(); found; found = bankAccLedgEntry.Next() {
		appliedAmount = appliedAmount.Add(bankAccLedgEntry.Remaining_amount)
		bankAccLedgEntry.Remaining_amount = types.Decimal{}
		bankAccLedgEntry.Open = false
		bankAccLedgEntry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.Closed
		err := bankAccLedgEntry.ModifyOrError(false)
		if err != nil {
			return appliedAmount, err
		}
		checkLedgEntry := gtables.NewCheckLedgerEntry(line)
		checkLedgEntry.SetRange("bank_account_ledger_entry_no", bankAccLedgEntry.Entry_no)
		checkLedgEntry.SetRange("open", true)
		for found := checkLedgEntry.FindSetBuffered(); found; found = checkLedgEntry.Next() {
			err = checkLedgEntry.TestField("open", true)
			if err == nil {
				err = checkLedgEntry.TestField("statement_status", gtables.CheckLedgerEntry_Statement_status.BankAccEntryApplied)
			}
			if err == nil {
				err = checkLedgEntry.TestField("statement_no", types.Code(""))
			}
			if err == nil {
				err = checkLedgEntry.TestField("statement_line_no", 0)
			}
			if err != nil {
				return appliedAmount, err
			}
			checkLedgEntry.Open = false
			checkLedgEntry.Statement_status = gtables.CheckLedgerEntry_Statement_status.Closed
			err = checkLedgEntry.ModifyOrError(false)
			if err != nil {
				return appliedAmount, err
			}
		}
	}
	return appliedAmount, nil
}

// closeCheckLedgEntries: the checks applied to the line close; each settles its bank account
// ledger entry by its amount — the entry closes when nothing remains, else it is open again
// unless another partial check of it is applied
func closeCheckLedgEntries(line *gtables.BankAccReconciliationLine) (types.Decimal, error) {
	var appliedAmount types.Decimal
	checkLedgEntry := gtables.NewCheckLedgerEntry(line)
	checkLedgEntry.SetRange("bank_account_no", line.Bank_account_no)
	checkLedgEntry.SetRange("open", true)
	checkLedgEntry.SetRange("statement_status", gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied)
	checkLedgEntry.SetRange("statement_no", line.Statement_no)
	checkLedgEntry.SetRange("statement_line_no", line.Statement_line_no)
	for found := checkLedgEntry.FindSetBuffered(); found; found = checkLedgEntry.Next() {
		appliedAmount = appliedAmount.Sub(checkLedgEntry.Amount)
		checkLedgEntry.Open = false
		checkLedgEntry.Statement_status = gtables.CheckLedgerEntry_Statement_status.Closed
		err := checkLedgEntry.ModifyOrError(false)
		if err != nil {
			return appliedAmount, err
		}
		bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(line)
		err = bankAccLedgEntry.GetOrError(checkLedgEntry.Bank_account_ledger_entry_no)
		if err != nil {
			return appliedAmount, err
		}
		if !checkLedgEntry.Amount.IsZero() {
			err = bankAccLedgEntry.TestField("open", true)
			if err == nil {
				err = bankAccLedgEntry.TestField("statement_status", gtables.BankAccountLedgerEntry_Statement_status.CheckEntryApplied)
			}
			if err == nil {
				err = bankAccLedgEntry.TestField("statement_no", types.Code(""))
			}
			if err == nil {
				err = bankAccLedgEntry.TestField("statement_line_no", 0)
			}
			if err != nil {
				return appliedAmount, err
			}
			bankAccLedgEntry.Remaining_amount = bankAccLedgEntry.Remaining_amount.Add(checkLedgEntry.Amount)
		}
		if bankAccLedgEntry.Remaining_amount.IsZero() {
			bankAccLedgEntry.Open = false
			bankAccLedgEntry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.Closed
		} else {
			checkLedgEntry2 := gtables.NewCheckLedgerEntry(line)
			checkLedgEntry2.SetRange("bank_account_ledger_entry_no", bankAccLedgEntry.Entry_no)
			checkLedgEntry2.SetRange("open", true)
			checkLedgEntry2.SetRange("check_type", gtables.CheckLedgerEntry_Check_type.PartialCheck)
			checkLedgEntry2.SetRange("statement_status", gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied)
			if !checkLedgEntry2.FindFirst() {
				bankAccLedgEntry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.Open
			}
		}
		err = bankAccLedgEntry.ModifyOrError(false)
		if err != nil {
			return appliedAmount, err
		}
	}
	return appliedAmount, nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
