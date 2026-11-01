package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// issuedFinChargeMemoHeaderLogic is the business logic of table 304 Issued_Fin_Charge_Memo_Header.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/issuedfinchargememoheader_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type issuedFinChargeMemoHeaderLogic struct{}

func init() { gtables.RegisterIssuedFinChargeMemoHeaderLogic(issuedFinChargeMemoHeaderLogic{}) }

// OnInsert runs before a new record is inserted
func (issuedFinChargeMemoHeaderLogic) OnInsert(rec *gtables.IssuedFinChargeMemoHeader) error {
	return nil
}

// OnModify runs before a record is modified
func (issuedFinChargeMemoHeaderLogic) OnModify(rec *gtables.IssuedFinChargeMemoHeader) error {
	return nil
}

// OnDelete runs before a record is deleted
func (issuedFinChargeMemoHeaderLogic) OnDelete(rec *gtables.IssuedFinChargeMemoHeader) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (issuedFinChargeMemoHeaderLogic) OnRename(rec *gtables.IssuedFinChargeMemoHeader) error {
	return nil
}
