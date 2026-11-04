package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userThemeLogic is the business logic of table 903 User_Theme.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/usertheme_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type userThemeLogic struct{}

func init() { gtables.RegisterUserThemeLogic(userThemeLogic{}) }

// OnInsert runs before a new record is inserted
func (userThemeLogic) OnInsert(rec *gtables.UserTheme) error {
	return nil
}

// OnModify runs before a record is modified
func (userThemeLogic) OnModify(rec *gtables.UserTheme) error {
	return nil
}

// OnDelete runs before a record is deleted
func (userThemeLogic) OnDelete(rec *gtables.UserTheme) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (userThemeLogic) OnRename(rec *gtables.UserTheme) error {
	return nil
}
