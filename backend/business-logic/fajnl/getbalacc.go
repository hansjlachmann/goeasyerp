package fajnl

import (
	"strconv"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func init() {
	functions.Register("table", gtables.GenJournalLineTableName, "InsertFABalAccount", functions.For(InsertFABalAccount))
}

// InsertFABalAccount is the Fixed Asset G/L Journal's Insert FA Bal. Account (codeunit 5601
// GetBalAcc): a fixed asset line without balancing account gets G/L lines below it for its
// balancing amount — the FA posting group's balancing account of the posting type, split over
// its FA allocations — with the line's description and the journal's source code, reason code
// and posting no. series
func InsertFABalAccount(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	genJnlLine := gtables.NewGenJournalLine(rec)
	err := genJnlLine.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "journal_batch_name": rec.Journal_batch_name, "line_no": rec.Line_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = GetBalAcc(genJnlLine)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// GetBalAcc (GetBalAcc) inserts the balancing G/L lines of a saved fixed asset journal line
// below it
func GetBalAcc(genJnlLine *gtables.GenJournalLine) error {
	rec := genJnlLine
	err := genJnlLine.TestField("bal_account_no", types.Code(""))
	if err != nil {
		return err
	}
	err = genJnlLine.TestField("account_type", gtables.GenJournalLine_Account_type.FixedAsset)
	if err != nil {
		return err
	}
	for _, field := range []string{"account_no", "depreciation_book_code", "posting_group", "fa_posting_type"} {
		err = genJnlLine.TestField(field)
		if err != nil {
			return err
		}
	}
	var g faInsertGLAcc
	err = g.insertBufferBalAcc(rec, allocationType(genJnlLine.Fa_posting_type-1), genJnlLine.Amount.Neg(), genJnlLine.Depreciation_book_code,
		genJnlLine.Posting_group, genJnlLine.Dimension_1_code, genJnlLine.Dimension_2_code, false, false)
	if err != nil {
		return err
	}
	// the free line numbers below the line: one step per new line
	next := gtables.NewGenJournalLine(rec)
	next.SetRange("journal_template_name", genJnlLine.Journal_template_name)
	next.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
	next.SetFilter("line_no", ">"+strconv.Itoa(genJnlLine.Line_no))
	noOfEmptyLines := 10000
	if next.FindFirst() {
		noOfEmptyLines = (next.Line_no - genJnlLine.Line_no) / (len(g.buffer) + 1)
	}
	if noOfEmptyLines < 1 {
		return apperrors.FANoSpaceForBalAccounts()
	}
	lineNo := genJnlLine.Line_no
	for _, b := range g.buffer {
		lineNo += noOfEmptyLines
		balLine := gtables.NewGenJournalLine(rec)
		balLine.Copy(genJnlLine)
		balLine.Line_no = lineNo
		balLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
		balLine.Depreciation_book_code = ""
		balLine.Posting_group = ""
		err = balLine.Validate("fa_posting_type", gtables.GenJournalLine_Fa_posting_type.Blank)
		if err != nil {
			return err
		}
		err = balLine.Validate("account_no", b.AccountNo)
		if err != nil {
			return err
		}
		err = balLine.Validate("amount", b.Amount)
		if err != nil {
			return err
		}
		balLine.Dimension_1_code = b.Dimension1Code
		balLine.Dimension_2_code = b.Dimension2Code
		balLine.Description = genJnlLine.Description
		// the asset's additional-currency rate on its balancing lines too, so the transaction
		// balances in the additional currency
		balLine.Fa_add_currency_factor = genJnlLine.Fa_add_currency_factor
		err = tables.SetGenJnlTrailCodes(balLine)
		if err != nil {
			return err
		}
		err = balLine.InsertOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}
