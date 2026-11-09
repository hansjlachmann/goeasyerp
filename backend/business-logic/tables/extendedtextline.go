package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// extendedTextLineLogic is the business logic of table 280 Extended_Text_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/extendedtextline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type extendedTextLineLogic struct{}

func init() { gtables.RegisterExtendedTextLineLogic(extendedTextLineLogic{}) }

// OnInsert runs before a new record is inserted
func (extendedTextLineLogic) OnInsert(rec *gtables.ExtendedTextLine) error {
	return nil
}

// OnModify runs before a record is modified
func (extendedTextLineLogic) OnModify(rec *gtables.ExtendedTextLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (extendedTextLineLogic) OnDelete(rec *gtables.ExtendedTextLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (extendedTextLineLogic) OnRename(rec *gtables.ExtendedTextLine) error {
	return nil
}
