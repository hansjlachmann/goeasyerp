package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemJournalBatchLogic is the business logic of table 233 Item Journal Batch: the batches of
// an item journal template, each with its own lines and numbering.
type itemJournalBatchLogic struct{}

func init() { gtables.RegisterItemJournalBatchLogic(itemJournalBatchLogic{}) }

// OnInsert: the batch's template exists
func (itemJournalBatchLogic) OnInsert(rec *gtables.ItemJournalBatch) error {
	rec.LockTable()
	itemJnlTemplate := gtables.NewItemJournalTemplate(rec)
	return itemJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete deletes the batch's journal lines
func (itemJournalBatchLogic) OnDelete(rec *gtables.ItemJournalBatch) error {
	itemJnlLine := gtables.NewItemJournalLine(rec)
	itemJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	itemJnlLine.SetRange("journal_batch_name", rec.Name)
	itemJnlLine.DeleteAll()
	return nil
}

// OnRename: the batch's journal lines move with it
func (itemJournalBatchLogic) OnRename(rec *gtables.ItemJournalBatch) error {
	xRec := rec.XRec()
	itemJnlLine := gtables.NewItemJournalLine(rec)
	itemJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	itemJnlLine.SetRange("journal_batch_name", xRec.Name)
	for itemJnlLine.FindFirst() {
		if !itemJnlLine.Rename(rec.Journal_template_name, rec.Name, itemJnlLine.Line_no) {
			err := itemJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.ItemJournalLineTableName, itemJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}

// SetupNewItemJnlBatch gives a new batch the numbering and reason code of its template
// (SetupNewBatch)
func SetupNewItemJnlBatch(rec *gtables.ItemJournalBatch) error {
	itemJnlTemplate := gtables.NewItemJournalTemplate(rec)
	err := itemJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.No_series = itemJnlTemplate.No_series
	rec.Posting_no_series = itemJnlTemplate.Posting_no_series
	rec.Reason_code = itemJnlTemplate.Reason_code
	return nil
}

// OnValidate_Reason_code: the batch's journal lines take the new reason code
func (itemJournalBatchLogic) OnValidate_Reason_code(rec *gtables.ItemJournalBatch) error {
	xRec := rec.XRec()
	if rec.Reason_code != xRec.Reason_code {
		itemJnlLine := gtables.NewItemJournalLine(rec)
		itemJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		itemJnlLine.SetRange("journal_batch_name", rec.Name)
		itemJnlLine.ModifyAll("reason_code", rec.Reason_code)
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (itemJournalBatchLogic) OnValidate_No_series(rec *gtables.ItemJournalBatch) error {
	if rec.No_series != "" {
		itemJnlTemplate := gtables.NewItemJournalTemplate(rec)
		err := itemJnlTemplate.GetOrError(rec.Journal_template_name)
		if err != nil {
			return err
		}
		if itemJnlTemplate.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.ItemJournalBatchTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			return rec.Validate("posting_no_series", "")
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series; the batch's lines take it
func (itemJournalBatchLogic) OnValidate_Posting_no_series(rec *gtables.ItemJournalBatch) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.ItemJournalBatchTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	itemJnlLine := gtables.NewItemJournalLine(rec)
	itemJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	itemJnlLine.SetRange("journal_batch_name", rec.Name)
	itemJnlLine.ModifyAll("posting_no_series", rec.Posting_no_series)
	return nil
}
