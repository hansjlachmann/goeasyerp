package checkmgt

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The payment journal's Void Check and Void All Checks (form 256, Payments menu): after asking,
// the line's printed check — or every printed computer check of the journal batch — is voided
// (VoidCheck), in one transaction; Void Check on the Check Ledger Entries voids a posted check
// financially (FinancialVoidCheck)
func init() {
	functions.Register("table", gtables.GenJournalLineTableName, "VoidCheck", functions.For(VoidCheckOfLine), functions.WithDialog())
	functions.Register("table", gtables.GenJournalLineTableName, "VoidAllChecks", functions.For(VoidAllChecks), functions.WithDialog())
	functions.Register("table", gtables.CheckLedgerEntryTableName, "FinancialVoidCheck", functions.For(FinancialVoidCheckOfEntry), functions.WithDialog())
}

// VoidCheckOfLine voids the line's printed computer check after asking
func VoidCheckOfLine(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	language := fcodeunits.CurrentLanguage()
	err := rec.TestField("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.ComputerCheck)
	if err == nil {
		err = rec.TestField("check_printed", true)
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(system.StrSubstNo(i18n.GetInstance().Message("CHECK_VOID_CONFIRM", language), rec.Document_no)) {
		return fcodeunits.Result{Success: true}, nil
	}
	return inTransaction(rec, func(src ftables.Source) error {
		genJnlLine := gtables.NewGenJournalLine(src)
		err := genJnlLine.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "journal_batch_name": rec.Journal_batch_name, "line_no": rec.Line_no})
		if err != nil {
			return err
		}
		return New(src).VoidCheck(genJnlLine)
	})
}

// VoidAllChecks voids every printed computer check of the line's journal batch after asking
func VoidAllChecks(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	language := fcodeunits.CurrentLanguage()
	if !system.Confirm(i18n.GetInstance().Message("CHECK_VOID_ALL_CONFIRM", language)) {
		return fcodeunits.Result{Success: true}, nil
	}
	return inTransaction(rec, func(src ftables.Source) error {
		checkManagement := New(src)
		genJnlLine := gtables.NewGenJournalLine(src)
		genJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		genJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
		genJnlLine.SetRange("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.ComputerCheck)
		genJnlLine.SetRange("check_printed", true)
		var lineNos []int
		for found := genJnlLine.FindSetBuffered(); found; found = genJnlLine.Next() {
			lineNos = append(lineNos, genJnlLine.Line_no)
		}
		for _, lineNo := range lineNos {
			genJnlLine2 := gtables.NewGenJournalLine(src)
			if !genJnlLine2.Get(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "journal_batch_name": rec.Journal_batch_name, "line_no": lineNo}) || !genJnlLine2.Check_printed {
				continue // voided with another line of its check
			}
			err := checkManagement.VoidCheck(genJnlLine2)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// inTransaction runs fn in a transaction of its own and reloads the page afterwards
func inTransaction(rec *gtables.GenJournalLine, fn func(src ftables.Source) error) (fcodeunits.Result, error) {
	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = fn(ftables.DBSource(tx, rec.GetCompany(), rec.GetDBType()))
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// FinancialVoidCheckOfEntry (Check Ledger Entries › Check › Void Check, form 10113 Void
// Confirmation): asks for the void date (not before the check date) and the type of void —
// Unapply and void check, offered while no entry of the check is voided or on a statement, or
// Void check only — and voids the posted check (FinancialVoidCheck) in one transaction
func FinancialVoidCheckOfEntry(rec *gtables.CheckLedgerEntry) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	language := fcodeunits.CurrentLanguage()
	err := rec.TestField("entry_status", gtables.CheckLedgerEntry_Entry_status.Posted)
	if err == nil {
		err = rec.TestField("statement_status", gtables.CheckLedgerEntry_Statement_status.Open)
	}
	if err == nil {
		err = rec.TestField("bal_account_no")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	reverseAllowed := ReverseAllowed(rec)
	defaultType := "1"
	if !reverseAllowed {
		defaultType = "2"
	}
	balAccountType := (&apperrors.AppError{Table: gtables.CheckLedgerEntryTableName, Field: "bal_account_type", Value: rec.Bal_account_type}).Message(language)
	title := system.StrSubstNo(ts.Message("CHECK_FIN_VOID_TITLE", language), rec.Check_no, rec.Bank_account_no,
		balAccountType, rec.Bal_account_no, format.Decimal(rec.Amount, "2:2", language))
	answer := fcodeunits.RequestInput(title, []fcodeunits.InputField{
		{Name: "void_date", Label: ts.Message("CHECK_VOID_DATE", language), Type: "date", Required: true, Default: system.Format(rec.Check_date)},
		{Name: "void_type", Label: ts.Message("CHECK_VOID_TYPE", language), Type: "option", Default: defaultType,
			Options: []string{ts.Message("CHECK_VOID_REVERSE", language), ts.Message("CHECK_VOID_CHECK_ONLY", language)}},
	})
	if answer == nil {
		return fcodeunits.Result{Success: true}, nil
	}
	var voidDate types.Date
	ok := system.Evaluate(&voidDate, answer["void_date"])
	if !ok {
		return fcodeunits.Result{}, apperrors.CheckVoidDateBeforeCheckDate(format.Date(rec.Check_date, language))
	}
	voidType := VoidTypeReverseOriginalEntry
	if answer["void_type"] == "2" {
		voidType = VoidTypeVoidCheckOnly
	}
	if voidType == VoidTypeReverseOriginalEntry && !reverseAllowed {
		return fcodeunits.Result{}, apperrors.CheckReverseNotAllowed(rec.Check_no.String())
	}

	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, rec.GetCompany(), rec.GetDBType())
	checkLedgEntry := gtables.NewCheckLedgerEntry(src)
	err = checkLedgEntry.GetOrError(rec.Entry_no)
	if err == nil {
		err = New(src).FinancialVoidCheck(checkLedgEntry, voidDate, voidType)
	}
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(system.StrSubstNo(ts.Message("CHECK_FIN_VOIDED", language), rec.Check_no))
	result.Refresh = true
	return result, nil
}
