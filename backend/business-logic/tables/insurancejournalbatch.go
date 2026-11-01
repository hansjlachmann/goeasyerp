package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// insuranceJournalBatchLogic is the business logic of table 5634 Insurance Journal Batch: the
// batches of an insurance journal template, each with its own lines and numbering.
type insuranceJournalBatchLogic struct{}

func init() { gtables.RegisterInsuranceJournalBatchLogic(insuranceJournalBatchLogic{}) }

// OnInsert: the batch's template exists
func (insuranceJournalBatchLogic) OnInsert(rec *gtables.InsuranceJournalBatch) error {
	rec.LockTable()
	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(rec)
	return insuranceJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete deletes the batch's journal lines
func (insuranceJournalBatchLogic) OnDelete(rec *gtables.InsuranceJournalBatch) error {
	insuranceJnlLine := gtables.NewInsuranceJournalLine(rec)
	insuranceJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	insuranceJnlLine.SetRange("journal_batch_name", rec.Name)
	insuranceJnlLine.DeleteAll()
	return nil
}

// OnRename: the batch's journal lines move with it
func (insuranceJournalBatchLogic) OnRename(rec *gtables.InsuranceJournalBatch) error {
	xRec := rec.XRec()
	insuranceJnlLine := gtables.NewInsuranceJournalLine(rec)
	insuranceJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	insuranceJnlLine.SetRange("journal_batch_name", xRec.Name)
	for insuranceJnlLine.FindFirst() {
		if !insuranceJnlLine.Rename(rec.Journal_template_name, rec.Name, insuranceJnlLine.Line_no) {
			err := insuranceJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.InsuranceJournalLineTableName, insuranceJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}

// OnValidate_Reason_code: the batch's journal lines take the new reason code
func (insuranceJournalBatchLogic) OnValidate_Reason_code(rec *gtables.InsuranceJournalBatch) error {
	xRec := rec.XRec()
	if rec.Reason_code != xRec.Reason_code {
		insuranceJnlLine := gtables.NewInsuranceJournalLine(rec)
		insuranceJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		insuranceJnlLine.SetRange("journal_batch_name", rec.Name)
		insuranceJnlLine.ModifyAll("reason_code", rec.Reason_code)
	}
	return nil
}

// OnValidate_No_series: the posting no. series must differ
func (insuranceJournalBatchLogic) OnValidate_No_series(rec *gtables.InsuranceJournalBatch) error {
	if rec.No_series != "" && rec.No_series == rec.Posting_no_series {
		return rec.Validate("posting_no_series", "")
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series; the batch's lines take it
func (insuranceJournalBatchLogic) OnValidate_Posting_no_series(rec *gtables.InsuranceJournalBatch) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.InsuranceJournalBatchTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	insuranceJnlLine := gtables.NewInsuranceJournalLine(rec)
	insuranceJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	insuranceJnlLine.SetRange("journal_batch_name", rec.Name)
	insuranceJnlLine.ModifyAll("posting_no_series", rec.Posting_no_series)
	return nil
}

// SetupNewInsuranceJnlBatch gives a new batch the numbering and reason code of its template
// (SetupNewBatch)
func SetupNewInsuranceJnlBatch(rec *gtables.InsuranceJournalBatch) error {
	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(rec)
	err := insuranceJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.No_series = insuranceJnlTemplate.No_series
	rec.Posting_no_series = insuranceJnlTemplate.Posting_no_series
	rec.Reason_code = insuranceJnlTemplate.Reason_code
	return nil
}
