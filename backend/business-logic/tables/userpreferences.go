package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userPreferencesLogic is the business logic of table 2000000075 User_Preferences.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/userpreferences_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type userPreferencesLogic struct{}

func init() { gtables.RegisterUserPreferencesLogic(userPreferencesLogic{}) }

// OnInsert runs before a new record is inserted
func (userPreferencesLogic) OnInsert(rec *gtables.UserPreferences) error {
	return nil
}

// OnModify runs before a record is modified
func (userPreferencesLogic) OnModify(rec *gtables.UserPreferences) error {
	return nil
}

// OnDelete runs before a record is deleted
func (userPreferencesLogic) OnDelete(rec *gtables.UserPreferences) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (userPreferencesLogic) OnRename(rec *gtables.UserPreferences) error {
	return nil
}
