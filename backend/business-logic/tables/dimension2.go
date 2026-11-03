package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// dimension2Logic is the business logic of table 12 Dimension 2 (the classic Dimension 2).
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/dimension2_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type dimension2Logic struct{}

func init() { gtables.RegisterDimension2Logic(dimension2Logic{}) }

// OnInsert runs before a new record is inserted
func (dimension2Logic) OnInsert(rec *gtables.Dimension2) error {
	return nil
}

// OnModify runs before a record is modified
func (dimension2Logic) OnModify(rec *gtables.Dimension2) error {
	return nil
}

// OnDelete runs before a record is deleted
func (dimension2Logic) OnDelete(rec *gtables.Dimension2) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (dimension2Logic) OnRename(rec *gtables.Dimension2) error {
	return nil
}
