package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// exchRateAdjmtRegLogic is the business logic of table 86 Exch_Rate_Adjmt_Reg.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/exchrateadjmtreg_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type exchRateAdjmtRegLogic struct{}

func init() { gtables.RegisterExchRateAdjmtRegLogic(exchRateAdjmtRegLogic{}) }

// OnInsert runs before a new record is inserted
func (exchRateAdjmtRegLogic) OnInsert(rec *gtables.ExchRateAdjmtReg) error {
	return nil
}

// OnModify runs before a record is modified
func (exchRateAdjmtRegLogic) OnModify(rec *gtables.ExchRateAdjmtReg) error {
	return nil
}

// OnDelete runs before a record is deleted
func (exchRateAdjmtRegLogic) OnDelete(rec *gtables.ExchRateAdjmtReg) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (exchRateAdjmtRegLogic) OnRename(rec *gtables.ExchRateAdjmtReg) error {
	return nil
}
