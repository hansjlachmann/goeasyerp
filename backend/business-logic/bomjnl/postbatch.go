package bomjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// maxPostingNoSeries is how many posting number series one journal may use (classic array)
const maxPostingNoSeries = 10

// BOMJnlPostBatch is codeunit 33 BOM Jnl.-Post Batch: it posts the lines of a BOM journal
// batch — every line checked, document numbers checked against the batch's number series and
// renumbered from the posting number series, every line posted — and deletes the posted lines.
// Open it on the posting's transaction; an error leaves everything to the caller's rollback.
//
// A recurring journal (recurring.go) posts the lines that are due and keeps them, each moved
// on by its Recurring Frequency.
type BOMJnlPostBatch struct {
	src                ftables.Source
	bomJnlCheckLine    BOMJnlCheckLine
	bomJnlPostLine     *BOMJnlPostLine
	noSeriesMgt        *noseries.Manager
	postingNoSeriesMgt map[types.Code]*noseries.Manager
}

// PostBatchResult is what the posting tells its caller: the BOM Register it wrote (0: nothing
// posted) and the batch the journal is in afterwards (a numbered batch moves to the next name)
type PostBatchResult struct {
	RegisterNo       int
	JournalBatchName types.Code
}

// NewBOMJnlPostBatch opens the batch posting in the posting's database (its transaction)
func NewBOMJnlPostBatch(src ftables.Source) *BOMJnlPostBatch {
	return &BOMJnlPostBatch{src: src}
}

// Run posts the batch of a journal line (Code)
func (b *BOMJnlPostBatch) Run(rec *gtables.BOMJournalLine) (PostBatchResult, error) {
	bomJnlLine := gtables.NewBOMJournalLine(b.src)
	bomJnlLine.LockTable()
	bomJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	bomJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	result := PostBatchResult{JournalBatchName: rec.Journal_batch_name}

	bomJnlTemplate := gtables.NewBOMJournalTemplate(b.src)
	err := bomJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	bomJnlBatch := gtables.NewBOMJournalBatch(b.src)
	err = bomJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(bomJnlBatch.Name.String())) > bomJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.BOMJournalBatchTableName, "name", bomJnlBatch.MaxStrLen("name"))
	}

	if bomJnlTemplate.Recurring {
		tables.SetRecurringFilters(bomJnlLine, "posting_date")
	}
	if !bomJnlLine.FindFirst() {
		return result, nil
	}

	language := fcodeunits.CurrentLanguage()
	window := system.OpenWindow(i18n.GetInstance().Message("BOM_JNL_POST_WINDOW", language))
	defer window.Close()
	window.Update(1, rec.Journal_batch_name)

	// Check Lines
	lineCount := 0
	for found := bomJnlLine.FindSet(); found; found = bomJnlLine.Next() {
		lineCount = lineCount + 1
		window.Update(2, lineCount)
		bomJnlLine2 := gtables.NewBOMJournalLine(b.src)
		bomJnlLine2.Copy(bomJnlLine)
		if bomJnlLine2.Item_no != "" {
			err = tables.CheckRecurringLine(bomJnlLine2, bomJnlTemplate.Recurring, bomJnlLine2.Recurring_method == gtables.BOMJournalLine_Recurring_method.Variable, "quantity")
			if err != nil {
				return result, err
			}
		}
		err = b.bomJnlCheckLine.RunCheck(bomJnlLine2)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCount

	// Find next register no.
	bomLedgEntry := gtables.NewBOMLedgerEntry(b.src)
	bomLedgEntry.LockTable()
	bomReg := gtables.NewBOMRegister(b.src)
	bomReg.LockTable()
	bomRegNo := 1
	if bomReg.FindLast() {
		bomRegNo = bomReg.No + 1
	}

	// Post lines
	b.bomJnlPostLine = NewBOMJnlPostLine(b.src)
	b.postingNoSeriesMgt = map[types.Code]*noseries.Manager{}
	lineCount = 0
	var lastDocNo, lastDocNo2, lastPostedDocNo types.Code
	for found := bomJnlLine.FindSetBuffered(); found; found = bomJnlLine.Next() {
		bomJnlLine3 := gtables.NewBOMJournalLine(b.src)
		bomJnlLine3.Copy(bomJnlLine)
		lineCount = lineCount + 1
		window.Update(3, lineCount)
		window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		if !tables.BOMJnlLineEmptyLine(bomJnlLine3) &&
			bomJnlBatch.No_series != "" &&
			bomJnlLine3.Document_no != lastDocNo2 {
			if b.noSeriesMgt == nil {
				b.noSeriesMgt = noseries.New(b.src)
			}
			nextNo, err := b.noSeriesMgt.GetNextNo(bomJnlBatch.No_series, bomJnlLine3.Posting_date, false)
			if err != nil {
				return result, err
			}
			err = bomJnlLine3.TestField("document_no", nextNo)
			if err != nil {
				return result, err
			}
		}
		lastDocNo2 = bomJnlLine3.Document_no
		if bomJnlLine3.Item_no != "" && bomJnlLine3.Recurring_method != 0 {
			bomJnlLine3.Document_no, bomJnlLine3.Description = tables.RecurringText(b.src, bomJnlLine3.Posting_date, bomJnlLine3.Document_no, bomJnlLine3.Description,
				bomJnlLine3.MaxStrLen("document_no"), bomJnlLine3.MaxStrLen("description"))
		}
		if bomJnlLine3.Posting_no_series == "" {
			bomJnlLine3.Posting_no_series = bomJnlBatch.No_series
		} else {
			if !tables.BOMJnlLineEmptyLine(bomJnlLine3) {
				if bomJnlLine3.Document_no == lastDocNo {
					bomJnlLine3.Document_no = lastPostedDocNo
				} else {
					noSeriesMgt2, ok := b.postingNoSeriesMgt[bomJnlLine3.Posting_no_series]
					if !ok {
						if len(b.postingNoSeriesMgt) >= maxPostingNoSeries {
							return result, apperrors.JnlMaxPostingNoSeries(maxPostingNoSeries)
						}
						noSeriesMgt2 = noseries.New(b.src)
						b.postingNoSeriesMgt[bomJnlLine3.Posting_no_series] = noSeriesMgt2
					}
					lastDocNo = bomJnlLine3.Document_no
					bomJnlLine3.Document_no, err = noSeriesMgt2.GetNextNo(bomJnlLine3.Posting_no_series, bomJnlLine3.Posting_date, false)
					if err != nil {
						return result, err
					}
					lastPostedDocNo = bomJnlLine3.Document_no
				}
			}
		}
		err = b.bomJnlPostLine.Run(bomJnlLine3)
		if err != nil {
			return result, err
		}
	}

	// Copy register no. and current journal batch name to the BOM journal
	bomReg = gtables.NewBOMRegister(b.src)
	if !bomReg.FindLast() || bomReg.No != bomRegNo {
		bomRegNo = 0
	}
	result.RegisterNo = bomRegNo

	// Update/delete lines
	// A recurring journal keeps its lines, the next date ahead
	if bomRegNo != 0 && bomJnlTemplate.Recurring {
		err = b.updateRecurringLines(bomJnlLine)
		if err != nil {
			return result, err
		}
	}
	if bomRegNo != 0 && !bomJnlTemplate.Recurring {
		bomJnlLine2 := gtables.NewBOMJournalLine(b.src)
		bomJnlLine2.CopyFilters(bomJnlLine)
		bomJnlLine2.SetFilter("item_no", "<>''")
		_ = bomJnlLine2.FindLast() // Remember the last line
		bomJnlLine.DeleteAll()

		bomJnlLine3 := gtables.NewBOMJournalLine(b.src)
		bomJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		bomJnlLine3.SetRange("journal_batch_name", rec.Journal_batch_name)
		if !bomJnlLine3.FindLast() {
			if system.IncStr(rec.Journal_batch_name.String()) != "" {
				err = bomJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				bomJnlBatch.Name = types.Code(system.IncStr(rec.Journal_batch_name.String()))
				_ = bomJnlBatch.Insert(false) // IF INSERT THEN: an existing batch of that name is used
				result.JournalBatchName = bomJnlBatch.Name
			}
		}

		bomJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if bomJnlBatch.No_series == "" && !bomJnlLine3.FindLast() {
			bomJnlLine3.Init()
			bomJnlLine3.Journal_template_name = rec.Journal_template_name
			bomJnlLine3.Journal_batch_name = result.JournalBatchName
			bomJnlLine3.Line_no = 10000
			err = bomJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
			err = tables.SetUpNewBOMJnlLine(bomJnlLine3, bomJnlLine2)
			if err != nil {
				return result, err
			}
			err = bomJnlLine3.ModifyOrError(false)
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
