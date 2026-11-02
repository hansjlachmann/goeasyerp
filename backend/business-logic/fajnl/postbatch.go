package fajnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// maxPostingNoSeries is how many posting number series one journal may use (classic array)
const maxPostingNoSeries = 10

// FAJnlPostBatch is codeunit 5633 FA Jnl.-Post Batch: it posts the lines of an FA journal
// batch — every line checked, document numbers checked against the batch's number series and
// renumbered from the posting number series, every line posted — and deletes the posted lines.
// Open it on the posting's transaction; an error leaves everything to the caller's rollback.
//
// A recurring journal (recurring.go) posts the lines that are due and keeps them, each moved
// on by its Recurring Frequency.
type FAJnlPostBatch struct {
	src                ftables.Source
	faJnlCheckLine     FAJnlCheckLine
	faJnlPostLine      *FAJnlPostLine
	noSeriesMgt        *noseries.Manager
	postingNoSeriesMgt map[types.Code]*noseries.Manager
}

// PostBatchResult is what the posting tells its caller: the FA Register it wrote (0: nothing
// posted) and the batch the journal is in afterwards (a numbered batch moves to the next name)
type PostBatchResult struct {
	RegisterNo       int
	JournalBatchName types.Code
}

// NewFAJnlPostBatch opens the batch posting in the posting's database (its transaction)
func NewFAJnlPostBatch(src ftables.Source) *FAJnlPostBatch {
	return &FAJnlPostBatch{src: src}
}

// Run posts the batch of a journal line (Code)
func (b *FAJnlPostBatch) Run(rec *gtables.FAJournalLine) (PostBatchResult, error) {
	faJnlLine := gtables.NewFAJournalLine(b.src)
	faJnlLine.LockTable()
	faJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	faJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	result := PostBatchResult{JournalBatchName: rec.Journal_batch_name}

	faJnlTemplate := gtables.NewFAJournalTemplate(b.src)
	err := faJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	faJnlBatch := gtables.NewFAJournalBatch(b.src)
	err = faJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(faJnlBatch.Name.String())) > faJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.FAJournalBatchTableName, "name", faJnlBatch.MaxStrLen("name"))
	}

	if faJnlTemplate.Recurring {
		tables.SetRecurringFilters(faJnlLine, "fa_posting_date")
	}
	if !faJnlLine.FindFirst() {
		return result, nil
	}

	language := fcodeunits.CurrentLanguage()
	window := system.OpenWindow(i18n.GetInstance().Message("FA_JNL_POST_WINDOW", language))
	defer window.Close()
	window.Update(1, rec.Journal_batch_name)

	// Check Lines
	lineCount := 0
	for found := faJnlLine.FindSet(); found; found = faJnlLine.Next() {
		lineCount = lineCount + 1
		window.Update(2, lineCount)
		faJnlLine2 := gtables.NewFAJournalLine(b.src)
		faJnlLine2.Copy(faJnlLine)
		if faJnlLine2.Fa_no != "" {
			err = tables.CheckRecurringLine(faJnlLine2, faJnlTemplate.Recurring, faJnlLine2.Recurring_method == gtables.FAJournalLine_Recurring_method.VVariable, "amount")
			if err != nil {
				return result, err
			}
		}
		err = b.faJnlCheckLine.CheckFAJnlLine(faJnlLine2)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCount

	// Find next register no.
	faLedgEntry := gtables.NewFALedgerEntry(b.src)
	faLedgEntry.LockTable()
	faReg := gtables.NewFARegister(b.src)
	faReg.LockTable()
	faRegNo := 1
	if faReg.FindLast() {
		faRegNo = faReg.No + 1
	}

	// Post lines
	b.faJnlPostLine = NewFAJnlPostLine(b.src)
	b.postingNoSeriesMgt = map[types.Code]*noseries.Manager{}
	lineCount = 0
	var lastDocNo, lastDocNo2, lastPostedDocNo types.Code
	for found := faJnlLine.FindSetBuffered(); found; found = faJnlLine.Next() {
		faJnlLine3 := gtables.NewFAJournalLine(b.src)
		faJnlLine3.Copy(faJnlLine)
		lineCount = lineCount + 1
		window.Update(3, lineCount)
		window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		if !(faJnlLine3.Fa_no == "") &&
			faJnlBatch.No_series != "" &&
			faJnlLine3.Document_no != lastDocNo2 {
			if b.noSeriesMgt == nil {
				b.noSeriesMgt = noseries.New(b.src)
			}
			nextNo, err := b.noSeriesMgt.GetNextNo(faJnlBatch.No_series, faJnlLine3.Fa_posting_date, false)
			if err != nil {
				return result, err
			}
			err = faJnlLine3.TestField("document_no", nextNo)
			if err != nil {
				return result, err
			}
		}
		lastDocNo2 = faJnlLine3.Document_no
		if faJnlLine3.Fa_no != "" && faJnlLine3.Recurring_method != 0 {
			faJnlLine3.Document_no, faJnlLine3.Description = tables.RecurringText(b.src, faJnlLine3.Fa_posting_date, faJnlLine3.Document_no, faJnlLine3.Description,
				faJnlLine3.MaxStrLen("document_no"), faJnlLine3.MaxStrLen("description"))
		}
		if faJnlLine3.Posting_no_series == "" {
			faJnlLine3.Posting_no_series = faJnlBatch.No_series
		} else {
			if !(faJnlLine3.Fa_no == "") {
				if faJnlLine3.Document_no == lastDocNo {
					faJnlLine3.Document_no = lastPostedDocNo
				} else {
					noSeriesMgt2, ok := b.postingNoSeriesMgt[faJnlLine3.Posting_no_series]
					if !ok {
						if len(b.postingNoSeriesMgt) >= maxPostingNoSeries {
							return result, apperrors.JnlMaxPostingNoSeries(maxPostingNoSeries)
						}
						noSeriesMgt2 = noseries.New(b.src)
						b.postingNoSeriesMgt[faJnlLine3.Posting_no_series] = noSeriesMgt2
					}
					lastDocNo = faJnlLine3.Document_no
					faJnlLine3.Document_no, err = noSeriesMgt2.GetNextNo(faJnlLine3.Posting_no_series, faJnlLine3.Fa_posting_date, false)
					if err != nil {
						return result, err
					}
					lastPostedDocNo = faJnlLine3.Document_no
				}
			}
		}
		err = b.faJnlPostLine.FAJnlPostLine(faJnlLine3, false)
		if err != nil {
			return result, err
		}
	}

	// Copy register no. and current journal batch name to the FA journal
	faReg = gtables.NewFARegister(b.src)
	if !faReg.FindLast() || faReg.No != faRegNo {
		faRegNo = 0
	}
	result.RegisterNo = faRegNo

	// Update/delete lines
	// A recurring journal keeps its lines, the next date ahead
	if faRegNo != 0 && faJnlTemplate.Recurring {
		err = b.updateRecurringLines(faJnlLine)
		if err != nil {
			return result, err
		}
	}
	if faRegNo != 0 && !faJnlTemplate.Recurring {
		faJnlLine2 := gtables.NewFAJournalLine(b.src)
		faJnlLine2.CopyFilters(faJnlLine)
		faJnlLine2.SetFilter("fa_no", "<>''")
		_ = faJnlLine2.FindLast() // Remember the last line
		faJnlLine.DeleteAll()

		faJnlLine3 := gtables.NewFAJournalLine(b.src)
		faJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		faJnlLine3.SetRange("journal_batch_name", rec.Journal_batch_name)
		if !faJnlLine3.FindLast() {
			if system.IncStr(rec.Journal_batch_name.String()) != "" {
				err = faJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				faJnlBatch.Name = types.Code(system.IncStr(rec.Journal_batch_name.String()))
				_ = faJnlBatch.Insert(false) // IF INSERT THEN: an existing batch of that name is used
				result.JournalBatchName = faJnlBatch.Name
			}
		}

		faJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if faJnlBatch.No_series == "" && !faJnlLine3.FindLast() {
			faJnlLine3.Init()
			faJnlLine3.Journal_template_name = rec.Journal_template_name
			faJnlLine3.Journal_batch_name = result.JournalBatchName
			faJnlLine3.Line_no = 10000
			err = faJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
			err = tables.SetUpNewFAJnlLine(faJnlLine3, faJnlLine2)
			if err != nil {
				return result, err
			}
			err = faJnlLine3.ModifyOrError(false)
			if err != nil {
				return result, err
			}
		}
	}
	if b.noSeriesMgt != nil {
		err = b.noSeriesMgt.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	for _, noSeriesMgt2 := range b.postingNoSeriesMgt {
		err = noSeriesMgt2.SaveNoSeries()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
