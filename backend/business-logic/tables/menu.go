package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// menuLogic is the business logic of table 900 Menu.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/menu_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type menuLogic struct{}

func init() { gtables.RegisterMenuLogic(menuLogic{}) }

// OnInsert runs before a new record is inserted
func (menuLogic) OnInsert(rec *gtables.Menu) error {
	return nil
}

// OnModify runs before a record is modified
func (menuLogic) OnModify(rec *gtables.Menu) error {
	return nil
}

// OnDelete runs before a record is deleted
func (menuLogic) OnDelete(rec *gtables.Menu) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (menuLogic) OnRename(rec *gtables.Menu) error {
	return nil
}
