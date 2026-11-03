package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// dimension1Logic is the business logic of table 11 Dimension 1 (the classic Dimension 1).
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/dimension1_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type dimension1Logic struct{}

func init() { gtables.RegisterDimension1Logic(dimension1Logic{}) }

// OnInsert runs before a new record is inserted
func (dimension1Logic) OnInsert(rec *gtables.Dimension1) error {
	return nil
}

// OnModify runs before a record is modified
func (dimension1Logic) OnModify(rec *gtables.Dimension1) error {
	return nil
}

// OnDelete runs before a record is deleted
func (dimension1Logic) OnDelete(rec *gtables.Dimension1) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (dimension1Logic) OnRename(rec *gtables.Dimension1) error {
	return nil
}
