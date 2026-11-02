package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// transportMethodLogic is the business logic of table 259 Transport_Method.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/transportmethod_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type transportMethodLogic struct{}

func init() { gtables.RegisterTransportMethodLogic(transportMethodLogic{}) }

// OnInsert runs before a new record is inserted
func (transportMethodLogic) OnInsert(rec *gtables.TransportMethod) error {
	return nil
}

// OnModify runs before a record is modified
func (transportMethodLogic) OnModify(rec *gtables.TransportMethod) error {
	return nil
}

// OnDelete runs before a record is deleted
func (transportMethodLogic) OnDelete(rec *gtables.TransportMethod) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (transportMethodLogic) OnRename(rec *gtables.TransportMethod) error {
	return nil
}
