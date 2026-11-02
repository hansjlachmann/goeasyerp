package itemjnl

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

// ItemJnlPostBatch is codeunit 23 Item Jnl.-Post Batch: it posts the lines of an item journal
// batch — every line checked, document numbers checked against the batch's number series and
// renumbered from the posting number series, every line posted — and deletes the posted lines.
// Open it on the posting's transaction; an error leaves everything to the caller's rollback.
//
// A recurring journal (recurring.go) posts the lines that are due and keeps them, each moved
// on by its Recurring Frequency.
type ItemJnlPostBatch struct {
	src                ftables.Source
	itemJnlCheckLine   ItemJnlCheckLine
	itemJnlPostLine    *ItemJnlPostLine
	noSeriesMgt        *noseries.Manager
	postingNoSeriesMgt map[types.Code]*noseries.Manager
}

// PostBatchResult is what the posting tells its caller: the Item Register it wrote (0: nothing
// posted) and the batch the journal is in afterwards (a numbered batch moves to the next name)
type PostBatchResult struct {
	RegisterNo       int
	JournalBatchName types.Code
}

// NewItemJnlPostBatch opens the batch posting in the posting's database (its transaction)
func NewItemJnlPostBatch(src ftables.Source) *ItemJnlPostBatch {
	return &ItemJnlPostBatch{src: src}
}

// Run posts the batch of a journal line (Code)
func (b *ItemJnlPostBatch) Run(rec *gtables.ItemJournalLine) (PostBatchResult, error) {
	itemJnlLine := gtables.NewItemJournalLine(b.src)
	itemJnlLine.LockTable()
	itemJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	itemJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	result := PostBatchResult{JournalBatchName: rec.Journal_batch_name}

	itemJnlTemplate := gtables.NewItemJournalTemplate(b.src)
	err := itemJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return result, err
	}
	itemJnlBatch := gtables.NewItemJournalBatch(b.src)
	err = itemJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return result, err
	}
	if system.StrLen(system.IncStr(itemJnlBatch.Name.String())) > itemJnlBatch.MaxStrLen("name") {
		return result, apperrors.JnlNameTooLong(gtables.ItemJournalBatchTableName, "name", itemJnlBatch.MaxStrLen("name"))
	}

	if itemJnlTemplate.Recurring {
		tables.SetRecurringFilters(itemJnlLine, "posting_date")
	}
	if !itemJnlLine.FindFirst() {
		return result, nil
	}

	language := fcodeunits.CurrentLanguage()
	window := system.OpenWindow(i18n.GetInstance().Message("ITEM_JNL_POST_WINDOW", language))
	defer window.Close()
	window.Update(1, rec.Journal_batch_name)

	// Check Lines
	lineCount := 0
	for found := itemJnlLine.FindSet(); found; found = itemJnlLine.Next() {
		lineCount = lineCount + 1
		window.Update(2, lineCount)
		itemJnlLine2 := gtables.NewItemJournalLine(b.src)
		itemJnlLine2.Copy(itemJnlLine)
		if itemJnlLine2.Item_no != "" {
			err = tables.CheckRecurringLine(itemJnlLine2, itemJnlTemplate.Recurring, itemJnlLine2.Recurring_method == gtables.ItemJournalLine_Recurring_method.Variable, "quantity")
			if err != nil {
				return result, err
			}
		}
		err = b.itemJnlCheckLine.RunCheck(itemJnlLine2)
		if err != nil {
			return result, err
		}
	}
	noOfRecords := lineCount

	// Find next register no.
	itemLedgEntry := gtables.NewItemLedgerEntry(b.src)
	itemLedgEntry.LockTable()
	itemReg := gtables.NewItemRegister(b.src)
	itemReg.LockTable()
	itemRegNo := 1
	if itemReg.FindLast() {
		itemRegNo = itemReg.No + 1
	}

	// Post lines
	b.itemJnlPostLine = NewItemJnlPostLine(b.src)
	b.postingNoSeriesMgt = map[types.Code]*noseries.Manager{}
	lineCount = 0
	var lastDocNo, lastDocNo2, lastPostedDocNo types.Code
	for found := itemJnlLine.FindSetBuffered(); found; found = itemJnlLine.Next() {
		itemJnlLine3 := gtables.NewItemJournalLine(b.src)
		itemJnlLine3.Copy(itemJnlLine)
		lineCount = lineCount + 1
		window.Update(3, lineCount)
		window.Update(4, system.Round(types.NewDecimalFromInt(int64(lineCount*10000/noOfRecords)), 1))
		if !tables.ItemJnlLineEmptyLine(itemJnlLine3) &&
			itemJnlBatch.No_series != "" &&
			itemJnlLine3.Document_no != lastDocNo2 {
			if b.noSeriesMgt == nil {
				b.noSeriesMgt = noseries.New(b.src)
			}
			nextNo, err := b.noSeriesMgt.GetNextNo(itemJnlBatch.No_series, itemJnlLine3.Posting_date, false)
			if err != nil {
				return result, err
			}
			err = itemJnlLine3.TestField("document_no", nextNo)
			if err != nil {
				return result, err
			}
		}
		lastDocNo2 = itemJnlLine3.Document_no
		if itemJnlLine3.Item_no != "" && itemJnlLine3.Recurring_method != 0 {
			itemJnlLine3.Document_no, itemJnlLine3.Description = tables.RecurringText(b.src, itemJnlLine3.Posting_date, itemJnlLine3.Document_no, itemJnlLine3.Description,
				itemJnlLine3.MaxStrLen("document_no"), itemJnlLine3.MaxStrLen("description"))
		}
		if itemJnlLine3.Posting_no_series == "" {
			itemJnlLine3.Posting_no_series = itemJnlBatch.No_series
		} else {
			if !tables.ItemJnlLineEmptyLine(itemJnlLine3) {
				if itemJnlLine3.Document_no == lastDocNo {
					itemJnlLine3.Document_no = lastPostedDocNo
				} else {
					noSeriesMgt2, ok := b.postingNoSeriesMgt[itemJnlLine3.Posting_no_series]
					if !ok {
						if len(b.postingNoSeriesMgt) >= maxPostingNoSeries {
							return result, apperrors.JnlMaxPostingNoSeries(maxPostingNoSeries)
						}
						noSeriesMgt2 = noseries.New(b.src)
						b.postingNoSeriesMgt[itemJnlLine3.Posting_no_series] = noSeriesMgt2
					}
					lastDocNo = itemJnlLine3.Document_no
					itemJnlLine3.Document_no, err = noSeriesMgt2.GetNextNo(itemJnlLine3.Posting_no_series, itemJnlLine3.Posting_date, false)
					if err != nil {
						return result, err
					}
					lastPostedDocNo = itemJnlLine3.Document_no
				}
			}
		}
		err = b.itemJnlPostLine.RunWithoutCheck(itemJnlLine3)
		if err != nil {
			return result, err
		}
	}

	// Copy register no. and current journal batch name to item journal
	itemReg = gtables.NewItemRegister(b.src)
	if !itemReg.FindLast() || itemReg.No != itemRegNo {
		itemRegNo = 0
	}
	result.RegisterNo = itemRegNo

	// Update/delete lines
	// A recurring journal keeps its lines, the next date ahead
	if itemRegNo != 0 && itemJnlTemplate.Recurring {
		err = b.updateRecurringLines(itemJnlLine)
		if err != nil {
			return result, err
		}
	}
	if itemRegNo != 0 && !itemJnlTemplate.Recurring {
		itemJnlLine2 := gtables.NewItemJournalLine(b.src)
		itemJnlLine2.CopyFilters(itemJnlLine)
		itemJnlLine2.SetFilter("item_no", "<>''")
		_ = itemJnlLine2.FindLast() // Remember the last line
		itemJnlLine.DeleteAll()

		itemJnlLine3 := gtables.NewItemJournalLine(b.src)
		itemJnlLine3.SetRange("journal_template_name", rec.Journal_template_name)
		itemJnlLine3.SetRange("journal_batch_name", rec.Journal_batch_name)
		if !itemJnlLine3.FindLast() {
			if system.IncStr(rec.Journal_batch_name.String()) != "" {
				err = itemJnlBatch.DeleteOrError(false)
				if err != nil {
					return result, err
				}
				itemJnlBatch.Name = types.Code(system.IncStr(rec.Journal_batch_name.String()))
				_ = itemJnlBatch.Insert(false) // IF INSERT THEN: an existing batch of that name is used
				result.JournalBatchName = itemJnlBatch.Name
			}
		}

		itemJnlLine3.SetRange("journal_batch_name", result.JournalBatchName)
		if itemJnlBatch.No_series == "" && !itemJnlLine3.FindLast() {
			itemJnlLine3.Init()
			itemJnlLine3.Journal_template_name = rec.Journal_template_name
			itemJnlLine3.Journal_batch_name = result.JournalBatchName
			itemJnlLine3.Line_no = 10000
			err = itemJnlLine3.InsertOrError(false)
			if err != nil {
				return result, err
			}
			err = tables.SetUpNewItemJnlLine(itemJnlLine3, itemJnlLine2)
			if err != nil {
				return result, err
			}
			err = itemJnlLine3.ModifyOrError(false)
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
