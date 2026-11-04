package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// financeChargeTextLogic is the business logic of table 301 Finance_Charge_Text.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/financechargetext_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type financeChargeTextLogic struct{}

func init() { gtables.RegisterFinanceChargeTextLogic(financeChargeTextLogic{}) }

// OnInsert runs before a new record is inserted
func (financeChargeTextLogic) OnInsert(rec *gtables.FinanceChargeText) error {
	return nil
}

// OnModify runs before a record is modified
func (financeChargeTextLogic) OnModify(rec *gtables.FinanceChargeText) error {
	return nil
}

// OnDelete runs before a record is deleted
func (financeChargeTextLogic) OnDelete(rec *gtables.FinanceChargeText) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (financeChargeTextLogic) OnRename(rec *gtables.FinanceChargeText) error {
	return nil
}
