package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// tariffNumberLogic is the business logic of table 260 Tariff_Number.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/tariffnumber_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type tariffNumberLogic struct{}

func init() { gtables.RegisterTariffNumberLogic(tariffNumberLogic{}) }

// OnInsert runs before a new record is inserted
func (tariffNumberLogic) OnInsert(rec *gtables.TariffNumber) error {
	return nil
}

// OnModify runs before a record is modified
func (tariffNumberLogic) OnModify(rec *gtables.TariffNumber) error {
	return nil
}

// OnDelete runs before a record is deleted
func (tariffNumberLogic) OnDelete(rec *gtables.TariffNumber) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (tariffNumberLogic) OnRename(rec *gtables.TariffNumber) error {
	return nil
}
