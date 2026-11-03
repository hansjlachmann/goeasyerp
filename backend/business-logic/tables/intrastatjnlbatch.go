package tables

import (
	"strconv"

	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// intrastatJnlBatchLogic is the business logic of table 262 Intrastat Jnl. Batch.
type intrastatJnlBatchLogic struct{}

func init() { gtables.RegisterIntrastatJnlBatchLogic(intrastatJnlBatchLogic{}) }

// OnInsert: the batch's template exists
func (intrastatJnlBatchLogic) OnInsert(rec *gtables.IntrastatJnlBatch) error {
	rec.LockTable()
	intraJnlTemplate := gtables.NewIntrastatJnlTemplate(rec)
	return intraJnlTemplate.GetOrError(rec.Journal_template_name)
}

// OnDelete: the batch's lines go with it
func (intrastatJnlBatchLogic) OnDelete(rec *gtables.IntrastatJnlBatch) error {
	intraJnlLine := gtables.NewIntrastatJnlLine(rec)
	intraJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	intraJnlLine.SetRange("journal_batch_name", rec.Name)
	intraJnlLine.DeleteAll()
	return nil
}

// OnValidate_Statistics_period: not once reported; YYMM with a month 01-12
func (intrastatJnlBatchLogic) OnValidate_Statistics_period(rec *gtables.IntrastatJnlBatch) error {
	err := rec.TestField("reported", false)
	if err != nil {
		return err
	}
	if len(rec.Statistics_period) != 4 {
		return apperrors.IntrastatStatisticsPeriod(gtables.IntrastatJnlBatchTableName)
	}
	month, err := strconv.Atoi(string(rec.Statistics_period[2:4]))
	if err != nil || month < 1 || month > 12 {
		return apperrors.IntrastatMonth()
	}
	return nil
}

// OnValidate_Amounts_in_add_currency: not once reported
func (intrastatJnlBatchLogic) OnValidate_Amounts_in_add_currency(rec *gtables.IntrastatJnlBatch) error {
	return rec.TestField("reported", false)
}

// OnValidate_Currency_identifier: not once reported
func (intrastatJnlBatchLogic) OnValidate_Currency_identifier(rec *gtables.IntrastatJnlBatch) error {
	return rec.TestField("reported", false)
}
