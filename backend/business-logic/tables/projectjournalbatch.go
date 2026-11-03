package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectJournalBatchLogic is the business logic of table 236 Res. Journal Batch: the batches of
// a project journal template, each with its own lines and numbering.
type projectJournalBatchLogic struct{}

func init() { gtables.RegisterProjectJournalBatchLogic(projectJournalBatchLogic{}) }

// OnInsert: the batch's template exists
func (projectJournalBatchLogic) OnInsert(rec *gtables.ProjectJournalBatch) error {
	rec.LockTable()
	projectJnlTemplate := gtables.NewProjectJournalTemplate(rec)
	return projectJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete deletes the batch's journal lines
func (projectJournalBatchLogic) OnDelete(rec *gtables.ProjectJournalBatch) error {
	projectJnlLine := gtables.NewProjectJournalLine(rec)
	projectJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	projectJnlLine.SetRange("journal_batch_name", rec.Name)
	projectJnlLine.DeleteAll()
	return nil
}

// OnRename: the batch's journal lines move with it
func (projectJournalBatchLogic) OnRename(rec *gtables.ProjectJournalBatch) error {
	xRec := rec.XRec()
	projectJnlLine := gtables.NewProjectJournalLine(rec)
	projectJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	projectJnlLine.SetRange("journal_batch_name", xRec.Name)
	for projectJnlLine.FindFirst() {
		if !projectJnlLine.Rename(rec.Journal_template_name, rec.Name, projectJnlLine.Line_no) {
			err := projectJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.ProjectJournalLineTableName, projectJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}

// SetupNewProjectJnlBatch gives a new batch the numbering and reason code of its template
// (SetupNewBatch)
func SetupNewProjectJnlBatch(rec *gtables.ProjectJournalBatch) error {
	projectJnlTemplate := gtables.NewProjectJournalTemplate(rec)
	err := projectJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.No_series = projectJnlTemplate.No_series
	rec.Posting_no_series = projectJnlTemplate.Posting_no_series
	rec.Reason_code = projectJnlTemplate.Reason_code
	return nil
}

// OnValidate_Reason_code: the batch's journal lines take the new reason code
func (projectJournalBatchLogic) OnValidate_Reason_code(rec *gtables.ProjectJournalBatch) error {
	xRec := rec.XRec()
	if rec.Reason_code != xRec.Reason_code {
		projectJnlLine := gtables.NewProjectJournalLine(rec)
		projectJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
		projectJnlLine.SetRange("journal_batch_name", rec.Name)
		projectJnlLine.ModifyAll("reason_code", rec.Reason_code)
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (projectJournalBatchLogic) OnValidate_No_series(rec *gtables.ProjectJournalBatch) error {
	if rec.No_series != "" {
		projectJnlTemplate := gtables.NewProjectJournalTemplate(rec)
		err := projectJnlTemplate.GetOrError(rec.Journal_template_name)
		if err != nil {
			return err
		}
		if projectJnlTemplate.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.ProjectJournalBatchTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			return rec.Validate("posting_no_series", "")
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series; the batch's lines take it
func (projectJournalBatchLogic) OnValidate_Posting_no_series(rec *gtables.ProjectJournalBatch) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.ProjectJournalBatchTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	projectJnlLine := gtables.NewProjectJournalLine(rec)
	projectJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	projectJnlLine.SetRange("journal_batch_name", rec.Name)
	projectJnlLine.ModifyAll("posting_no_series", rec.Posting_no_series)
	return nil
}
