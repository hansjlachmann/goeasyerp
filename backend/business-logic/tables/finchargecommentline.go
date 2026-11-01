package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// finChargeCommentLineLogic is the business logic of table 306 Fin_Charge_Comment_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/finchargecommentline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type finChargeCommentLineLogic struct{}

func init() { gtables.RegisterFinChargeCommentLineLogic(finChargeCommentLineLogic{}) }

// OnInsert runs before a new record is inserted
func (finChargeCommentLineLogic) OnInsert(rec *gtables.FinChargeCommentLine) error {
	return nil
}

// OnModify runs before a record is modified
func (finChargeCommentLineLogic) OnModify(rec *gtables.FinChargeCommentLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (finChargeCommentLineLogic) OnDelete(rec *gtables.FinChargeCommentLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (finChargeCommentLineLogic) OnRename(rec *gtables.FinChargeCommentLine) error {
	return nil
}
