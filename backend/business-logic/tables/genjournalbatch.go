package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// genJournalBatchLogic is the business logic of table 232 Gen. Journal Batch: the batches of a
// journal template, each with its own lines, balancing account and numbering.
type genJournalBatchLogic struct{}

func init() { gtables.RegisterGenJournalBatchLogic(genJournalBatchLogic{}) }

// OnInsert: the batch's template exists
func (genJournalBatchLogic) OnInsert(rec *gtables.GenJournalBatch) error {
	rec.LockTable()
	genJnlTemplate := gtables.NewGenJournalTemplate(rec)
	return genJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete deletes the batch's journal lines (with their triggers)
func (genJournalBatchLogic) OnDelete(rec *gtables.GenJournalBatch) error {
	genJnlLine := gtables.NewGenJournalLine(rec)
	genJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	genJnlLine.SetRange("journal_batch_name", rec.Name)
	return deleteGenJnlLines(genJnlLine)
}

// OnRename: the batch's journal lines move with it
func (genJournalBatchLogic) OnRename(rec *gtables.GenJournalBatch) error {
	xRec := rec.XRec()
	genJnlLine := gtables.NewGenJournalLine(rec)
	genJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	genJnlLine.SetRange("journal_batch_name", xRec.Name)
	for genJnlLine.FindFirst() {
		if !genJnlLine.Rename(rec.Journal_template_name, rec.Name, genJnlLine.Line_no) {
			err := genJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.GenJournalLineTableName, genJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}

// SetupNewBatch gives a new batch the balancing account, numbering and reason code of its
// template
func SetupNewBatch(rec *gtables.GenJournalBatch) error {
	genJnlTemplate := gtables.NewGenJournalTemplate(rec)
	err := genJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.Bal_account_type = gtables.GenJournalBatchBal_account_type(genJnlTemplate.Bal_account_type)
	rec.Bal_account_no = genJnlTemplate.Bal_account_no
	rec.No_series = genJnlTemplate.No_series
	rec.Posting_no_series = genJnlTemplate.Posting_no_series
	rec.Reason_code = genJnlTemplate.Reason_code
	return nil
}

// OnValidate_Reason_code: the batch's journal lines take the new reason code
func (genJournalBatchLogic) OnValidate_Reason_code(rec *gtables.GenJournalBatch) error {
	xRec := rec.XRec()
	if rec.Reason_code != xRec.Reason_code {
		return modifyGenJnlBatchLines(rec, "reason_code")
	}
	return nil
}

// OnValidate_Bal_account_type: Bal. Account No. starts again
func (genJournalBatchLogic) OnValidate_Bal_account_type(rec *gtables.GenJournalBatch) error {
	rec.Bal_account_no = ""
	return nil
}

// OnValidate_Bal_account_no: a G/L account must allow direct posting
func (genJournalBatchLogic) OnValidate_Bal_account_no(rec *gtables.GenJournalBatch) error {
	if rec.Bal_account_type == gtables.GenJournalBatch_Bal_account_type.GLAccount {
		return checkJnlBalGLAcc(rec, rec.Bal_account_no)
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (genJournalBatchLogic) OnValidate_No_series(rec *gtables.GenJournalBatch) error {
	if rec.No_series != "" {
		genJnlTemplate := gtables.NewGenJournalTemplate(rec)
		err := genJnlTemplate.GetOrError(rec.Journal_template_name)
		if err != nil {
			return err
		}
		if genJnlTemplate.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.GenJournalBatchTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			return rec.Validate("posting_no_series", "")
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series; the batch's lines take it
func (genJournalBatchLogic) OnValidate_Posting_no_series(rec *gtables.GenJournalBatch) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.GenJournalBatchTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return modifyGenJnlBatchLines(rec, "posting_no_series")
}

// modifyGenJnlBatchLines: the batch's journal lines take the batch's reason code or posting no.
// series — ModifyLines
func modifyGenJnlBatchLines(rec *gtables.GenJournalBatch, field string) error {
	genJnlLine := gtables.NewGenJournalLine(rec)
	genJnlLine.LockTable()
	genJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	genJnlLine.SetRange("journal_batch_name", rec.Name)
	for found := genJnlLine.FindSetBuffered(); found; found = genJnlLine.Next() { // modified in the loop: read all first
		var err error
		switch field {
		case "reason_code":
			err = genJnlLine.Validate("reason_code", rec.Reason_code)
		case "posting_no_series":
			err = genJnlLine.Validate("posting_no_series", rec.Posting_no_series)
		}
		if err != nil {
			return err
		}
		err = genJnlLine.ModifyOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}
