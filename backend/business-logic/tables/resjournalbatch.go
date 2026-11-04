package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resJournalBatchLogic is the business logic of table 236 Res. Journal Batch: the batches of
// a resource journal template, each with its own lines and numbering.
type resJournalBatchLogic struct{}

func init() { gtables.RegisterResJournalBatchLogic(resJournalBatchLogic{}) }

// OnInsert: the batch's template exists
func (resJournalBatchLogic) OnInsert(rec *gtables.ResJournalBatch) error {
	rec.LockTable()
	resJnlTemplate := gtables.NewResJournalTemplate(rec)
	return resJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete deletes the batch's journal lines
func (resJournalBatchLogic) OnDelete(rec *gtables.ResJournalBatch) error {
	resJnlLine := gtables.NewResJournalLine(rec)
	resJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	resJnlLine.SetRange("journal_batch_name", rec.Name)
	resJnlLine.DeleteAll()
	return nil
}

// OnRename: the batch's journal lines move with it
func (resJournalBatchLogic) OnRename(rec *gtables.ResJournalBatch) error {
	xRec := rec.XRec()
	resJnlLine := gtables.NewResJournalLine(rec)
	resJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	resJnlLine.SetRange("journal_batch_name", xRec.Name)
	for resJnlLine.FindFirst() {
		if !resJnlLine.Rename(rec.Journal_template_name, rec.Name, resJnlLine.Line_no) {
			err := resJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.ResJournalLineTableName, resJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}

// SetupNewResJnlBatch gives a new batch the numbering and reason code of its template
// (SetupNewBatch)
func SetupNewResJnlBatch(rec *gtables.ResJournalBatch) error {
	resJnlTemplate := gtables.NewResJournalTemplate(rec)
	err := resJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.No_series = resJnlTemplate.No_series
	rec.Posting_no_series = resJnlTemplate.Posting_no_series
	rec.Reason_code = resJnlTemplate.Reason_code
	return nil
}

// OnValidate_Reason_code: the batch's journal lines take the new reason code
func (resJournalBatchLogic) OnValidate_Reason_code(rec *gtables.ResJournalBatch) error {
	xRec := rec.XRec()
	if rec.Reason_code != xRec.Reason_code {
		resJnlLine := gtables.NewResJournalLine(rec)
		resJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		resJnlLine.SetRange("journal_batch_name", rec.Name)
		resJnlLine.ModifyAll("reason_code", rec.Reason_code)
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (resJournalBatchLogic) OnValidate_No_series(rec *gtables.ResJournalBatch) error {
	if rec.No_series != "" {
		resJnlTemplate := gtables.NewResJournalTemplate(rec)
		err := resJnlTemplate.GetOrError(rec.Journal_template_name)
		if err != nil {
			return err
		}
		if resJnlTemplate.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.ResJournalBatchTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			return rec.Validate("posting_no_series", "")
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series; the batch's lines take it
func (resJournalBatchLogic) OnValidate_Posting_no_series(rec *gtables.ResJournalBatch) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.ResJournalBatchTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	resJnlLine := gtables.NewResJournalLine(rec)
	resJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	resJnlLine.SetRange("journal_batch_name", rec.Name)
	resJnlLine.ModifyAll("posting_no_series", rec.Posting_no_series)
	return nil
}
