package checkmgt

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The payment journal's Void Check and Void All Checks (form 256, Payments menu): after asking,
// the line's printed check — or every printed computer check of the journal batch — is voided
// (VoidCheck), in one transaction
func init() {
	functions.Register("table", gtables.GenJournalLineTableName, "VoidCheck", functions.For(VoidCheckOfLine), functions.WithDialog())
	functions.Register("table", gtables.GenJournalLineTableName, "VoidAllChecks", functions.For(VoidAllChecks), functions.WithDialog())
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
