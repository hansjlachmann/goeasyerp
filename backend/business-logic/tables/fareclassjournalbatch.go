package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// fAReclassJournalBatchLogic is the business logic of table 5623 FA Reclass. Journal Batch: the
// batches of an FA reclass. journal template.
type fAReclassJournalBatchLogic struct{}

func init() { gtables.RegisterFAReclassJournalBatchLogic(fAReclassJournalBatchLogic{}) }

// OnDelete deletes the batch's journal lines
func (fAReclassJournalBatchLogic) OnDelete(rec *gtables.FAReclassJournalBatch) error {
	faReclassJnlLine := gtables.NewFAReclassJournalLine(rec)
	faReclassJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	faReclassJnlLine.SetRange("journal_batch_name", rec.Name)
	faReclassJnlLine.DeleteAll()
	return nil
}

// OnRename: the batch's journal lines move with it
func (fAReclassJournalBatchLogic) OnRename(rec *gtables.FAReclassJournalBatch) error {
	xRec := rec.XRec()
	faReclassJnlLine := gtables.NewFAReclassJournalLine(rec)
	faReclassJnlLine.SetRange("journal_template_name", xRec.Journal_template_name)
	faReclassJnlLine.SetRange("journal_batch_name", xRec.Name)
	for faReclassJnlLine.FindFirst() {
		if !faReclassJnlLine.Rename(rec.Journal_template_name, rec.Name, faReclassJnlLine.Line_no) {
			err := faReclassJnlLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.FAReclassJournalLineTableName, faReclassJnlLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}
