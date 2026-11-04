package fajnl

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func init() {
	functions.Register("table", gtables.FALedgerEntryTableName, "CancelEntries", functions.For(CancelEntries), functions.WithDialog())
}

// CancelEntries is Cancel Entries on FA Ledger Entries (codeunit 5624 Cancel FA Entries): the
// selected entries, after a question, as journal lines that cancel them, in one transaction
func CancelEntries(rec *gtables.FALedgerEntry) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("FA_CANCEL_ENTRIES_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	faLedgEntry := rec
	faLedgEntry.SetSelectionFilter()
	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	faLedgEntry.SetDB(tx)
	err = TransferLine(faLedgEntry, false)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return system.Message(ts.Message("FA_CANCEL_ENTRIES_TRANSFERRED", lang)), nil
}

// cancelFAEntries: the journals codeunit 5624 fills, found with the first line of each
type cancelFAEntries struct {
	deprBook                          *gtables.DepreciationBook
	deprBookCode                      types.Code
	genJnlUsedOnce, faJnlUsedOnce     bool
	genJnlTemplate, genJnlBatch       types.Code
	faJnlTemplate, faJnlBatch         types.Code
	genJnlNextLineNo, faJnlNextLineNo int
	genJnlDocumentNo, faJnlDocumentNo types.Code
}

// TransferLine (TransferLine): the entries in the filters, last first, as lines with the opposite
// amount and FA Error Entry No. — in the general journal of the book's FA Journal Setup when the
// book posts the type to the G/L, else in its FA journal; with balAccount the balancing lines too
func TransferLine(faLedgEntry *gtables.FALedgerEntry, balAccount bool) error {
	var c cancelFAEntries
	for found := faLedgEntry.Find("+"); found; found = faLedgEntry.Next(-1) {
		if c.deprBookCode == "" {
			c.deprBookCode = faLedgEntry.Depreciation_book_code
		}
		if c.deprBookCode != faLedgEntry.Depreciation_book_code {
			return apperrors.FACancelSameBook(gtables.FALedgerEntryTableName)
		}
		if faLedgEntry.Fa_no == "" {
			return apperrors.FAAlreadyCanceled(strconv.Itoa(faLedgEntry.Entry_no))
		}
		fa := gtables.NewFixedAsset(faLedgEntry)
		err := fa.GetOrError(faLedgEntry.Fa_no)
		if err != nil {
			return err
		}
		c.deprBook = gtables.NewDepreciationBook(faLedgEntry)
		err = c.deprBook.GetOrError(faLedgEntry.Depreciation_book_code)
		if err != nil {
			return err
		}
		err = checkCancelType(faLedgEntry)
		if err != nil {
			return err
		}
		if glIntegration(c.deprBook, tables.FALedgEntryConvertPostingType(faLedgEntry)) && !fa.Budgeted_asset {
			err = c.insertGenJnlLine(faLedgEntry, balAccount)
		} else {
			err = c.insertFAJnlLine(faLedgEntry)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// checkCancelType (CheckType): disposal entries (and those after Salvage Value) are canceled
// with a disposal line
func checkCancelType(faLedgEntry *gtables.FALedgerEntry) error {
	if faLedgEntry.Fa_posting_type > gtables.FALedgerEntry_Fa_posting_type.SalvageValue ||
		faLedgEntry.Fa_posting_category != gtables.FALedgerEntry_Fa_posting_category.Blank {
		return apperrors.FACannotBeCanceled(gtables.FALedgerEntryTableName, strconv.Itoa(faLedgEntry.Entry_no),
			gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal)
	}
	return nil
}

func (c *cancelFAEntries) insertFAJnlLine(faLedgEntry *gtables.FALedgerEntry) error {
	var err error
	if !c.faJnlUsedOnce {
		gtables.NewFAJournalLine(faLedgEntry).LockTable()
		c.faJnlTemplate, c.faJnlBatch, c.faJnlNextLineNo, err = tables.FAJnlName(faLedgEntry, c.deprBook.Code)
		if err != nil {
			return err
		}
		c.faJnlUsedOnce = true
		c.faJnlDocumentNo, err = tables.GetFAJnlDocumentNo(faLedgEntry, c.faJnlTemplate, c.faJnlBatch, faLedgEntry.Fa_posting_date)
		if err != nil {
			return err
		}
	}
	faJnlLine := gtables.NewFAJournalLine(faLedgEntry)
	faJnlLine.Journal_template_name = c.faJnlTemplate
	faJnlLine.Journal_batch_name = c.faJnlBatch
	err = tables.FALedgEntryMoveToFAJnl(faLedgEntry, faJnlLine)
	if err != nil {
		return err
	}
	faJnlLine.Document_no = c.faJnlDocumentNo
	faJnlLine.Document_type = gtables.FAJournalLine_Document_type.Blank
	faJnlLine.External_document_no = ""
	faJnlLine.Dimension_1_code = faLedgEntry.Dimension_1_code
	faJnlLine.Dimension_2_code = faLedgEntry.Dimension_2_code
	faJnlLine.Fa_error_entry_no = faLedgEntry.Entry_no
	faJnlLine.Posting_no_series = tables.GetFANoSeries(faLedgEntry, c.faJnlTemplate, c.faJnlBatch)
	err = faJnlLine.Validate("amount", faJnlLine.Amount.Neg())
	if err != nil {
		return err
	}
	err = faJnlLine.Validate("correction", c.deprBook.Mark_errors_as_corrections)
	if err != nil {
		return err
	}
	c.faJnlNextLineNo += 10000
	faJnlLine.Line_no = c.faJnlNextLineNo
	return faJnlLine.InsertOrError(true)
}

func (c *cancelFAEntries) insertGenJnlLine(faLedgEntry *gtables.FALedgerEntry, balAccount bool) error {
	var err error
	if !c.genJnlUsedOnce {
		gtables.NewGenJournalLine(faLedgEntry).LockTable()
		c.genJnlTemplate, c.genJnlBatch, c.genJnlNextLineNo, err = tables.GenJnlName(faLedgEntry, c.deprBook.Code)
		if err != nil {
			return err
		}
		c.genJnlUsedOnce = true
		c.genJnlDocumentNo, err = tables.GetGenJnlDocumentNo(faLedgEntry, c.genJnlTemplate, c.genJnlBatch, faLedgEntry.Fa_posting_date)
		if err != nil {
			return err
		}
	}
	genJnlLine := gtables.NewGenJournalLine(faLedgEntry)
	genJnlLine.Journal_template_name = c.genJnlTemplate
	genJnlLine.Journal_batch_name = c.genJnlBatch
	err = tables.FALedgEntryMoveToGenJnl(faLedgEntry, genJnlLine)
	if err != nil {
		return err
	}
	genJnlLine.Document_no = c.genJnlDocumentNo
	genJnlLine.Document_type = gtables.GenJournalLine_Document_type.Blank
	genJnlLine.External_document_no = ""
	genJnlLine.Dimension_1_code = faLedgEntry.Dimension_1_code
	genJnlLine.Dimension_2_code = faLedgEntry.Dimension_2_code
	genJnlLine.Fa_error_entry_no = faLedgEntry.Entry_no
	err = genJnlLine.Validate("amount", genJnlLine.Amount.Neg())
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("correction", c.deprBook.Mark_errors_as_corrections)
	if err != nil {
		return err
	}
	genJnlLine.Posting_no_series = tables.GetGenNoSeries(faLedgEntry, c.genJnlTemplate, c.genJnlBatch)
	c.genJnlNextLineNo += 10000
	genJnlLine.Line_no = c.genJnlNextLineNo
	err = tables.SetGenJnlTrailCodes(genJnlLine)
	if err != nil {
		return err
	}
	err = genJnlLine.InsertOrError(true)
	if err != nil {
		return err
	}
	if balAccount {
		err = GetBalAcc(genJnlLine)
		if err != nil {
			return err
		}
		last := gtables.NewGenJournalLine(faLedgEntry)
		last.SetRange("journal_template_name", c.genJnlTemplate)
		last.SetRange("journal_batch_name", c.genJnlBatch)
		if last.FindLast() {
			c.genJnlNextLineNo = last.Line_no
		}
	}
	return nil
}
