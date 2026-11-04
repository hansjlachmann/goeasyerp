package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// standardTextLogic is the business logic of table 7 Standard_Text.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/standardtext_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type standardTextLogic struct{}

func init() { gtables.RegisterStandardTextLogic(standardTextLogic{}) }

// OnInsert runs before a new record is inserted
func (standardTextLogic) OnInsert(rec *gtables.StandardText) error {
	return nil
}

// OnModify runs before a record is modified
func (standardTextLogic) OnModify(rec *gtables.StandardText) error {
	return nil
}

// OnDelete runs before a record is deleted
func (standardTextLogic) OnDelete(rec *gtables.StandardText) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (standardTextLogic) OnRename(rec *gtables.StandardText) error {
	return nil
}
