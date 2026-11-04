package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bomJournalBatchLogic is the business logic of table 234 BOM Journal Batch: the batches of
// a BOM journal template, each with its own lines and numbering.
type bomJournalBatchLogic struct{}

func init() { gtables.RegisterBOMJournalBatchLogic(bomJournalBatchLogic{}) }

// OnInsert: the batch's template exists
func (bomJournalBatchLogic) OnInsert(rec *gtables.BOMJournalBatch) error {
	rec.LockTable()
	bomJnlTemplate := gtables.NewBOMJournalTemplate(rec)
	return bomJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete deletes the batch's journal lines
func (bomJournalBatchLogic) OnDelete(rec *gtables.BOMJournalBatch) error {
	bomJnlLine := gtables.NewBOMJournalLine(rec)
	bomJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	bomJnlLine.SetRange("journal_batch_name", rec.Name)
	bomJnlLine.DeleteAll()
	return nil
}

// OnRename: the batch's journal lines move with it
func (bomJournalBatchLogic) OnRename(rec *gtables.BOMJournalBatch) error {
	xRec := rec.XRec()
	bomJnlLine := gtables.NewBOMJournalLine(rec)
	bomJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	bomJnlLine.SetRange("journal_batch_name", xRec.Name)
	for bomJnlLine.FindFirst() {
		if !bomJnlLine.Rename(rec.Journal_template_name, rec.Name, bomJnlLine.Line_no) {
			err := bomJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.BOMJournalLineTableName, bomJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}

// SetupNewBOMJnlBatch gives a new batch the numbering and reason code of its template
// (SetupNewBatch)
func SetupNewBOMJnlBatch(rec *gtables.BOMJournalBatch) error {
	bomJnlTemplate := gtables.NewBOMJournalTemplate(rec)
	err := bomJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.No_series = bomJnlTemplate.No_series
	rec.Posting_no_series = bomJnlTemplate.Posting_no_series
	rec.Reason_code = bomJnlTemplate.Reason_code
	return nil
}

// OnValidate_Reason_code: the batch's journal lines take the new reason code
func (bomJournalBatchLogic) OnValidate_Reason_code(rec *gtables.BOMJournalBatch) error {
	xRec := rec.XRec()
	if rec.Reason_code != xRec.Reason_code {
		bomJnlLine := gtables.NewBOMJournalLine(rec)
		bomJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		bomJnlLine.SetRange("journal_batch_name", rec.Name)
		bomJnlLine.ModifyAll("reason_code", rec.Reason_code)
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (bomJournalBatchLogic) OnValidate_No_series(rec *gtables.BOMJournalBatch) error {
	if rec.No_series != "" {
		bomJnlTemplate := gtables.NewBOMJournalTemplate(rec)
		err := bomJnlTemplate.GetOrError(rec.Journal_template_name)
		if err != nil {
			return err
		}
		if bomJnlTemplate.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.BOMJournalBatchTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			return rec.Validate("posting_no_series", "")
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series; the batch's lines take it
func (bomJournalBatchLogic) OnValidate_Posting_no_series(rec *gtables.BOMJournalBatch) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.BOMJournalBatchTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	bomJnlLine := gtables.NewBOMJournalLine(rec)
	bomJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	bomJnlLine.SetRange("journal_batch_name", rec.Name)
	bomJnlLine.ModifyAll("posting_no_series", rec.Posting_no_series)
	return nil
}
