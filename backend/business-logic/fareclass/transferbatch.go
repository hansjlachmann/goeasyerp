package fareclass

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// FAReclassCheckLine is codeunit 5641 FA Reclass. Check Line: one instance per batch (all lines
// in one depreciation book)
type FAReclassCheckLine struct {
	deprBookCode types.Code
}

// Run checks a line: both assets, the FA posting date and the book — the same in every line
func (c *FAReclassCheckLine) Run(rec *gtables.FAReclassJournalLine) error {
	if rec.Fa_no == "" && rec.New_fa_no == "" {
		return nil
	}
	if rec.Fa_no == "" && rec.New_fa_no != "" {
		return rec.TestField("fa_no")
	}
	if rec.Fa_no != "" && rec.New_fa_no == "" {
		return rec.TestField("new_fa_no")
	}
	for _, field := range []string{"fa_posting_date", "fa_no", "new_fa_no", "depreciation_book_code"} {
		err := rec.TestField(field)
		if err != nil {
			return err
		}
	}
	if c.deprBookCode == "" {
		c.deprBookCode = rec.Depreciation_book_code
	}
	if rec.Depreciation_book_code != c.deprBookCode {
		return apperrors.FAReclassSameBook(gtables.FAReclassJournalLineTableName)
	}
	return nil
}

// TransferBatchResult tells the caller whether lines were made (Line No. 0: nothing to
// reclassify) and the batch the journal is in afterwards (a numbered batch moves to the next
// name)
type TransferBatchResult struct {
	Done             bool
	JournalBatchName types.Code
}

// TransferBatch is codeunit 5643 FA Reclass. Transfer Batch: every line of the batch checked,
// then turned into FA/general journal lines (in FA Posting Date order); the reclass. lines are
// deleted (a numbered batch moves to the next name) and an empty line is left. Run it on the
// caller's transaction.
func TransferBatch(src ftables.Source, rec *gtables.FAReclassJournalLine) (TransferBatchResult, error) {
	faReclassJnlLine := gtables.NewFAReclassJournalLine(src)
	faReclassJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	faReclassJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	faReclassJnlLine.LockTable()
	result := TransferBatchResult{JournalBatchName: rec.Journal_batch_name}
	faReclassJnlTemplate := gtables.NewFAReclassJournalTemplate(src)
	err := faReclassJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	faReclassJnlBatch := gtables.NewFAReclassJournalBatch(src)
	err = faReclassJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(faReclassJnlBatch.Name.String())) > faReclassJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.FAReclassJournalBatchTableName, "name", faReclassJnlBatch.MaxStrLen("name"))
	}
	if !faReclassJnlLine.FindFirst() {
		return result, nil
	}
	window := system.OpenWindow(i18n.GetInstance().Message("FA_RECLASS_WINDOW", fcodeunits.CurrentLanguage()))
	defer window.Close()
	window.Update(1, rec.Journal_batch_name)

	var faReclassCheckLine FAReclassCheckLine
	lineCounter := 0
	for found := faReclassJnlLine.FindSet(); found; found = faReclassJnlLine.Next() {
		lineCounter = lineCounter + 1
		window.Update(2, lineCounter)
		err = faReclassCheckLine.Run(faReclassJnlLine)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCounter
	lineCounter = 0
	oneFAReclassDone := false
	var faReclassTransferLine FAReclassTransferLine
	faReclassJnlLine.SetCurrentKey("journal_template_name", "journal_batch_name", "fa_posting_date")
	for found := faReclassJnlLine.FindSetBuffered(); found; found = faReclassJnlLine.Next() {
		lineCounter = lineCounter + 1
		window.Update(3, lineCounter)
		window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCounter*10000/noOfRecords)), 1))
		err = faReclassTransferLine.FAReclassLine(faReclassJnlLine, &oneFAReclassDone)
		if err != nil {
			return result, err
		}
	}
	result.Done = oneFAReclassDone
	if oneFAReclassDone {
		faReclassJnlLine2 := gtables.NewFAReclassJournalLine(src)
		faReclassJnlLine2.CopyFilters(faReclassJnlLine)
		faReclassJnlLine2.SetFilter("fa_no", "<>''")
		faReclassJnlLine2.FindLast() // remember the last line
		faReclassJnlLine.DeleteAll()
		faReclassJnlLine3 := gtables.NewFAReclassJournalLine(src)
		faReclassJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		faReclassJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if !faReclassJnlLine3.FindLast() {
			if system.IncStr(result.JournalBatchName.String()) != "" {
				err = faReclassJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				faReclassJnlBatch.Name = types.Code(system.IncStr(result.JournalBatchName.String()))
				faReclassJnlBatch.Insert(false)
				result.JournalBatchName = faReclassJnlBatch.Name
			}
		}
		faReclassJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if !faReclassJnlLine3.FindLast() {
			faReclassJnlLine3.Init()
			faReclassJnlLine3.Journal_template_name = rec.Journal_template_name
			faReclassJnlLine3.Journal_batch_name = result.JournalBatchName
			faReclassJnlLine3.Line_no = 10000
			faReclassJnlLine3.Posting_date = faReclassJnlLine2.Posting_date
			err = faReclassJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
