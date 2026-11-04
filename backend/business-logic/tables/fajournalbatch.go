package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fAJournalBatchLogic is the business logic of table 5620 FA Journal Batch: the batches of
// an FA journal template, each with its own lines and numbering.
type fAJournalBatchLogic struct{}

func init() { gtables.RegisterFAJournalBatchLogic(fAJournalBatchLogic{}) }

// OnInsert: the batch's template exists
func (fAJournalBatchLogic) OnInsert(rec *gtables.FAJournalBatch) error {
	rec.LockTable()
	faJnlTemplate := gtables.NewFAJournalTemplate(rec)
	return faJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete deletes the batch's journal lines
func (fAJournalBatchLogic) OnDelete(rec *gtables.FAJournalBatch) error {
	faJnlLine := gtables.NewFAJournalLine(rec)
	faJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	faJnlLine.SetRange("journal_batch_name", rec.Name)
	faJnlLine.DeleteAll()
	return nil
}

// OnRename: the batch's journal lines move with it
func (fAJournalBatchLogic) OnRename(rec *gtables.FAJournalBatch) error {
	xRec := rec.XRec()
	faJnlLine := gtables.NewFAJournalLine(rec)
	faJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	faJnlLine.SetRange("journal_batch_name", xRec.Name)
	for faJnlLine.FindFirst() {
		if !faJnlLine.Rename(rec.Journal_template_name, rec.Name, faJnlLine.Line_no) {
			err := faJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.FAJournalLineTableName, faJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}

// SetupNewFAJnlBatch gives a new batch the numbering and reason code of its template
// (SetupNewBatch)
func SetupNewFAJnlBatch(rec *gtables.FAJournalBatch) error {
	faJnlTemplate := gtables.NewFAJournalTemplate(rec)
	err := faJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.No_series = faJnlTemplate.No_series
	rec.Posting_no_series = faJnlTemplate.Posting_no_series
	rec.Reason_code = faJnlTemplate.Reason_code
	return nil
}

// OnValidate_Reason_code: the batch's journal lines take the new reason code
func (fAJournalBatchLogic) OnValidate_Reason_code(rec *gtables.FAJournalBatch) error {
	xRec := rec.XRec()
	if rec.Reason_code != xRec.Reason_code {
		faJnlLine := gtables.NewFAJournalLine(rec)
		faJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		faJnlLine.SetRange("journal_batch_name", rec.Name)
		faJnlLine.ModifyAll("reason_code", rec.Reason_code)
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (fAJournalBatchLogic) OnValidate_No_series(rec *gtables.FAJournalBatch) error {
	if rec.No_series != "" {
		faJnlTemplate := gtables.NewFAJournalTemplate(rec)
		err := faJnlTemplate.GetOrError(rec.Journal_template_name)
		if err != nil {
			return err
		}
		if faJnlTemplate.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.FAJournalBatchTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			return rec.Validate("posting_no_series", "")
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series; the batch's lines take it
func (fAJournalBatchLogic) OnValidate_Posting_no_series(rec *gtables.FAJournalBatch) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.FAJournalBatchTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	faJnlLine := gtables.NewFAJournalLine(rec)
	faJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	faJnlLine.SetRange("journal_batch_name", rec.Name)
	faJnlLine.ModifyAll("posting_no_series", rec.Posting_no_series)
	return nil
}
