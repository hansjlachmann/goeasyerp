package resjnl

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

// ResJnlPostBatch is codeunit 213 Res. Jnl.-Post Batch: it posts the lines of a resource
// journal batch — every line checked, document numbers checked against the batch's number series and
// renumbered from the posting number series, every line posted — and deletes the posted lines.
// Open it on the posting's transaction; an error leaves everything to the caller's rollback.
//
// A recurring journal (recurring.go) posts the lines that are due and keeps them, each moved
// on by its Recurring Frequency.
type ResJnlPostBatch struct {
	src                ftables.Source
	resJnlCheckLine    ResJnlCheckLine
	resJnlPostLine     *ResJnlPostLine
	noSeriesMgt        *noseries.Manager
	postingNoSeriesMgt map[types.Code]*noseries.Manager
}

// PostBatchResult is what the posting tells its caller: the Resource Register it wrote (0: nothing
// posted) and the batch the journal is in afterwards (a numbered batch moves to the next name)
type PostBatchResult struct {
	RegisterNo       int
	JournalBatchName types.Code
}

// NewResJnlPostBatch opens the batch posting in the posting's database (its transaction)
func NewResJnlPostBatch(src ftables.Source) *ResJnlPostBatch {
	return &ResJnlPostBatch{src: src}
}

// Run posts the batch of a journal line (Code)
func (b *ResJnlPostBatch) Run(rec *gtables.ResJournalLine) (PostBatchResult, error) {
	resJnlLine := gtables.NewResJournalLine(b.src)
	resJnlLine.LockTable()
	resJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	resJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	result := PostBatchResult{JournalBatchName: rec.Journal_batch_name}

	resJnlTemplate := gtables.NewResJournalTemplate(b.src)
	err := resJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	resJnlBatch := gtables.NewResJournalBatch(b.src)
	err = resJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(resJnlBatch.Name.String())) > resJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.ResJournalBatchTableName, "name", resJnlBatch.MaxStrLen("name"))
	}

	if resJnlTemplate.Recurring {
		tables.SetRecurringFilters(resJnlLine, "posting_date")
	}
	if !resJnlLine.FindFirst() {
		return result, nil
	}

	language := fcodeunits.CurrentLanguage()
	window := system.OpenWindow(i18n.GetInstance().Message("RES_JNL_POST_WINDOW", language))
	defer window.Close()
	window.Update(1, rec.Journal_batch_name)

	// Check Lines
	lineCount := 0
	for found := resJnlLine.FindSet(); found; found = resJnlLine.Next() {
		lineCount = lineCount + 1
		window.Update(2, lineCount)
		resJnlLine2 := gtables.NewResJournalLine(b.src)
		resJnlLine2.Copy(resJnlLine)
		if resJnlLine2.Resource_no != "" {
			err = tables.CheckRecurringLine(resJnlLine2, resJnlTemplate.Recurring, resJnlLine2.Recurring_method == gtables.ResJournalLine_Recurring_method.Variable, "quantity")
			if err != nil {
				return result, err
			}
		}
		err = b.resJnlCheckLine.RunCheck(resJnlLine2)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCount

	// Find next register no.
	resLedgEntry := gtables.NewResLedgerEntry(b.src)
	resLedgEntry.LockTable()
	resReg := gtables.NewResourceRegister(b.src)
	resReg.LockTable()
	var resRegNo int
	if resReg.FindLast() && resReg.To_entry_no == 0 {
		resRegNo = resReg.No
	} else {
		resRegNo = resReg.No + 1
	}

	// Post lines
	b.resJnlPostLine = NewResJnlPostLine(b.src)
	b.postingNoSeriesMgt = map[types.Code]*noseries.Manager{}
	lineCount = 0
	var lastDocNo, lastDocNo2, lastPostedDocNo types.Code
	for found := resJnlLine.FindSetBuffered(); found; found = resJnlLine.Next() {
		resJnlLine3 := gtables.NewResJournalLine(b.src)
		resJnlLine3.Copy(resJnlLine)
		lineCount = lineCount + 1
		window.Update(3, lineCount)
		window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		if !tables.ResJnlLineEmptyLine(resJnlLine3) &&
			resJnlBatch.No_series != "" &&
			resJnlLine3.Document_no != lastDocNo2 {
			if b.noSeriesMgt == nil {
				b.noSeriesMgt = noseries.New(b.src)
			}
			nextNo, err := b.noSeriesMgt.GetNextNo(resJnlBatch.No_series, resJnlLine3.Posting_date, false)
			if err != nil {
				return result, err
			}
			err = resJnlLine3.TestField("document_no", nextNo)
			if err != nil {
				return result, err
			}
		}
		lastDocNo2 = resJnlLine3.Document_no
		if resJnlLine3.Resource_no != "" && resJnlLine3.Recurring_method != 0 {
			resJnlLine3.Document_no, resJnlLine3.Description = tables.RecurringText(b.src, resJnlLine3.Posting_date, resJnlLine3.Document_no, resJnlLine3.Description,
				resJnlLine3.MaxStrLen("document_no"), resJnlLine3.MaxStrLen("description"))
		}
		if resJnlLine3.Posting_no_series == "" {
			resJnlLine3.Posting_no_series = resJnlBatch.No_series
		} else {
			if !tables.ResJnlLineEmptyLine(resJnlLine3) {
				if resJnlLine3.Document_no == lastDocNo {
					resJnlLine3.Document_no = lastPostedDocNo
				} else {
					noSeriesMgt2, ok := b.postingNoSeriesMgt[resJnlLine3.Posting_no_series]
					if !ok {
						if len(b.postingNoSeriesMgt) >= maxPostingNoSeries {
							return result, apperrors.JnlMaxPostingNoSeries(maxPostingNoSeries)
						}
						noSeriesMgt2 = noseries.New(b.src)
						b.postingNoSeriesMgt[resJnlLine3.Posting_no_series] = noSeriesMgt2
					}
					lastDocNo = resJnlLine3.Document_no
					resJnlLine3.Document_no, err = noSeriesMgt2.GetNextNo(resJnlLine3.Posting_no_series, resJnlLine3.Posting_date, false)
					if err != nil {
						return result, err
					}
					lastPostedDocNo = resJnlLine3.Document_no
				}
			}
		}
		err = b.resJnlPostLine.Run(resJnlLine3)
		if err != nil {
			return result, err
		}
	}

	// Copy register no. and current journal batch name to the resource journal
	resReg = gtables.NewResourceRegister(b.src)
	if !resReg.FindLast() || resReg.No != resRegNo {
		resRegNo = 0
	}
	result.RegisterNo = resRegNo

	// Update/delete lines
	// A recurring journal keeps its lines, the next date ahead
	if resRegNo != 0 && resJnlTemplate.Recurring {
		err = b.updateRecurringLines(resJnlLine)
		if err != nil {
			return result, err
		}
	}
	if resRegNo != 0 && !resJnlTemplate.Recurring {
		resJnlLine2 := gtables.NewResJournalLine(b.src)
		resJnlLine2.CopyFilters(resJnlLine)
		resJnlLine2.SetFilter("resource_no", "<>''")
		_ = resJnlLine2.FindLast() // Remember the last line
		resJnlLine.DeleteAll()

		resJnlLine3 := gtables.NewResJournalLine(b.src)
		resJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		resJnlLine3.SetRange("journal_batch_name", rec.Journal_batch_name)
		if !resJnlLine3.FindLast() {
			if system.IncStr(rec.Journal_batch_name.String()) != "" {
				err = resJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				resJnlBatch.Name = types.Code(system.IncStr(rec.Journal_batch_name.String()))
				_ = resJnlBatch.Insert(false) // IF INSERT THEN: an existing batch of that name is used
				result.JournalBatchName = resJnlBatch.Name
			}
		}

		resJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if resJnlBatch.No_series == "" && !resJnlLine3.FindLast() {
			resJnlLine3.Init()
			resJnlLine3.Journal_template_name = rec.Journal_template_name
			resJnlLine3.Journal_batch_name = result.JournalBatchName
			resJnlLine3.Line_no = 10000
			err = resJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
			err = tables.SetUpNewResJnlLine(resJnlLine3, resJnlLine2)
			if err != nil {
				return result, err
			}
			err = resJnlLine3.ModifyOrError(false)
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
