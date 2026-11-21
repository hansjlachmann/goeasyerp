package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// mainMenuTextLogic is the business logic of table 907 Main_Menu_Text.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/mainmenutext_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type mainMenuTextLogic struct{}

func init() { gtables.RegisterMainMenuTextLogic(mainMenuTextLogic{}) }

// OnInsert runs before a new record is inserted
func (mainMenuTextLogic) OnInsert(rec *gtables.MainMenuText) error {
	return nil
}

// OnModify runs before a record is modified
func (mainMenuTextLogic) OnModify(rec *gtables.MainMenuText) error {
	return nil
}

// OnDelete runs before a record is deleted
func (mainMenuTextLogic) OnDelete(rec *gtables.MainMenuText) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (mainMenuTextLogic) OnRename(rec *gtables.MainMenuText) error {
	return nil
}
