package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// issuedFinChargeMemoLineLogic is the business logic of table 305 Issued_Fin_Charge_Memo_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/issuedfinchargememoline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type issuedFinChargeMemoLineLogic struct{}

func init() { gtables.RegisterIssuedFinChargeMemoLineLogic(issuedFinChargeMemoLineLogic{}) }

// OnInsert runs before a new record is inserted
func (issuedFinChargeMemoLineLogic) OnInsert(rec *gtables.IssuedFinChargeMemoLine) error {
	return nil
}

// OnModify runs before a record is modified
func (issuedFinChargeMemoLineLogic) OnModify(rec *gtables.IssuedFinChargeMemoLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (issuedFinChargeMemoLineLogic) OnDelete(rec *gtables.IssuedFinChargeMemoLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (issuedFinChargeMemoLineLogic) OnRename(rec *gtables.IssuedFinChargeMemoLine) error {
	return nil
}
